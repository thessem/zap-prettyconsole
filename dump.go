package prettyconsole

import (
	"io"
	"math"
	"reflect"
	"sort"
	"strconv"
	"sync"
	"time"
	"unsafe"
)

// This file implements the reflection-based value dumper used for
// zap.Reflect / zap.Any fields the encoder has no better answer for. It
// renders a readable, Go-syntax-flavoured representation:
//
//   - struct fields (exported and unexported) as "Name: value"
//   - maps with deterministically sorted keys
//   - time.Time as RFC3339 and time.Duration in Go's duration syntax
//   - []byte as a hexdump with offset comments, byte arrays of any size
//     as compact hex strings
//   - long scalar lists broken across lines
//
// Recursion is bounded: pointer/map/slice cycles render as <cycle> and
// nesting beyond maxDumpDepth renders as <max depth>, so pathological
// values can never hang or crash the logger.

const (
	// listBreakLen is the element count above which scalar lists break
	// onto multiple lines, and the number of elements printed per line.
	listBreakLen = 8
	// maxDumpDepth bounds recursion for non-cyclic but very deep values.
	maxDumpDepth = 64
)

var (
	timeType     = reflect.TypeOf(time.Time{})
	durationType = reflect.TypeOf(time.Duration(0))
)

type dumpState struct {
	buf     []byte
	depth   int
	visited map[uintptr]struct{}
}

var dumpPool = sync.Pool{New: func() interface{} {
	return &dumpState{buf: make([]byte, 0, 256), visited: make(map[uintptr]struct{})}
}}

// dumpValue writes a readable representation of v to w.
func dumpValue(w io.Writer, v interface{}) error {
	d := dumpPool.Get().(*dumpState)
	defer func() {
		d.buf = d.buf[:0]
		d.depth = 0
		for k := range d.visited {
			delete(d.visited, k)
		}
		dumpPool.Put(d)
	}()

	if v == nil {
		d.str("nil")
	} else {
		rv := reflect.ValueOf(v)
		// Make the value addressable so unexported struct fields further
		// down can be read (see bypass).
		if rv.Kind() != reflect.Pointer && rv.CanInterface() {
			pv := reflect.New(rv.Type())
			pv.Elem().Set(rv)
			rv = pv.Elem()
		}
		d.value(rv)
	}
	_, err := w.Write(d.buf)
	return err
}

func (d *dumpState) str(s string)    { d.buf = append(d.buf, s...) }
func (d *dumpState) byte_(b byte)    { d.buf = append(d.buf, b) }
func (d *dumpState) quoted(s string) { d.buf = strconv.AppendQuote(d.buf, s) }

// newline starts a fresh line at the current depth.
func (d *dumpState) newline() {
	d.byte_('\n')
	for i := 0; i < d.depth; i++ {
		d.str("  ")
	}
}

// bypass makes an unexported struct field usable with Interface(). It is
// the standard reflect.NewAt technique; it requires addressability, which
// dumpValue arranges at the top level.
func bypass(v reflect.Value) reflect.Value {
	if v.CanInterface() || !v.CanAddr() {
		return v
	}
	return reflect.NewAt(v.Type(), unsafe.Pointer(v.UnsafeAddr())).Elem()
}

func (d *dumpState) value(v reflect.Value) {
	if d.depth >= maxDumpDepth {
		d.str("<max depth>")
		return
	}

	// Special-cased named types, resolved before the kind switch. Types
	// sharing time.Time's underlying struct (named wrappers included) are
	// rendered as timestamps; durations only on the exact type, since any
	// int64-kind type is convertible to time.Duration.
	if v.Type() == durationType {
		d.quoted(time.Duration(v.Int()).String())
		return
	}
	if v.Kind() == reflect.Struct && v.Type().ConvertibleTo(timeType) {
		if bv := bypass(v); bv.CanInterface() {
			d.quoted(bv.Convert(timeType).Interface().(time.Time).Format(time.RFC3339))
			return
		}
	}

	switch v.Kind() {
	case reflect.Bool:
		d.buf = strconv.AppendBool(d.buf, v.Bool())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		d.buf = strconv.AppendInt(d.buf, v.Int(), 10)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		d.buf = strconv.AppendUint(d.buf, v.Uint(), 10)
	case reflect.Uintptr:
		d.str("0x")
		d.buf = strconv.AppendUint(d.buf, v.Uint(), 16)
	case reflect.Float32:
		d.float(v.Float(), 32)
	case reflect.Float64:
		d.float(v.Float(), 64)
	case reflect.Complex64, reflect.Complex128:
		c := v.Complex()
		d.byte_('(')
		d.float(real(c), 64)
		if imag(c) >= 0 || math.IsNaN(imag(c)) {
			d.byte_('+')
		}
		d.float(imag(c), 64)
		d.str("i)")
	case reflect.String:
		d.quoted(v.String())
	case reflect.Pointer:
		d.pointer(v)
	case reflect.Interface:
		if v.IsNil() {
			d.str("nil")
		} else {
			d.value(v.Elem())
		}
	case reflect.Slice, reflect.Array:
		d.sequence(v)
	case reflect.Map:
		d.mapValue(v)
	case reflect.Struct:
		d.structValue(v)
	case reflect.Chan, reflect.Func:
		d.byte_('<')
		d.str(v.Type().String())
		d.byte_('>')
	case reflect.UnsafePointer:
		d.str("0x")
		d.buf = strconv.AppendUint(d.buf, uint64(v.Pointer()), 16)
	default:
		d.byte_('<')
		d.str(v.Type().String())
		d.byte_('>')
	}
}

func (d *dumpState) float(f float64, bits int) {
	d.buf = strconv.AppendFloat(d.buf, f, 'g', -1, bits)
}

func (d *dumpState) pointer(v reflect.Value) {
	if v.IsNil() {
		d.byte_('(')
		d.str(v.Type().String())
		d.str(")(nil)")
		return
	}
	if d.enter(v.Pointer()) {
		d.str("<cycle>")
		return
	}
	defer d.leave(v.Pointer())
	d.byte_('&')
	d.value(v.Elem())
}

// enter records a container on the current dump path, reporting whether it
// is already there (a cycle). Path-based tracking means diamonds - the
// same value reachable twice without a loop - still print in full.
func (d *dumpState) enter(p uintptr) bool {
	if _, ok := d.visited[p]; ok {
		return true
	}
	d.visited[p] = struct{}{}
	return false
}

func (d *dumpState) leave(p uintptr) { delete(d.visited, p) }

// sequence renders slices and arrays. Byte sequences get hex treatment:
// arrays as one compact hex string (they are almost always IDs, hashes and
// addresses), slices as a hexdump.
func (d *dumpState) sequence(v reflect.Value) {
	if v.Kind() == reflect.Slice {
		if v.IsNil() {
			d.byte_('(')
			d.str(v.Type().String())
			d.str(")(nil)")
			return
		}
		if d.enter(v.Pointer()) {
			d.str("<cycle>")
			return
		}
		defer d.leave(v.Pointer())
	}
	if v.Type().Elem().Kind() == reflect.Uint8 {
		if v.Kind() == reflect.Array {
			d.byteArray(v)
		} else {
			d.byteSlice(v)
		}
		return
	}

	d.str(v.Type().String())
	n := v.Len()
	if n == 0 {
		d.str("{}")
		return
	}
	if n <= listBreakLen {
		mark := len(d.buf)
		d.byte_('{')
		for i := 0; i < n; i++ {
			if i > 0 {
				d.str(", ")
			}
			d.value(v.Index(i))
		}
		d.byte_('}')
		if d.inlineFits(mark) {
			return
		}
		d.buf = d.buf[:mark]
	}
	d.byte_('{')
	d.depth++
	if scalarKind(v.Type().Elem().Kind()) {
		for i := 0; i < n; i++ {
			if i%listBreakLen == 0 {
				d.newline()
			} else {
				d.byte_(' ')
			}
			d.value(v.Index(i))
			d.byte_(',')
		}
	} else {
		for i := 0; i < n; i++ {
			d.newline()
			d.value(v.Index(i))
			d.byte_(',')
		}
	}
	d.depth--
	d.newline()
	d.byte_('}')
}

// inlineFits reports whether a speculatively inlined rendering starting at
// mark stayed single-line and reasonably narrow. If not, the caller rolls
// the buffer back and renders multi-line instead.
func (d *dumpState) inlineFits(mark int) bool {
	const maxInlineWidth = 80
	if len(d.buf)-mark > maxInlineWidth {
		return false
	}
	for _, b := range d.buf[mark:] {
		if b == '\n' {
			return false
		}
	}
	return true
}

func scalarKind(k reflect.Kind) bool {
	switch k {
	case reflect.Pointer, reflect.Interface, reflect.Slice, reflect.Array,
		reflect.Map, reflect.Struct:
		return false
	}
	return true
}

// byteArray renders [N]byte as a compact quoted hex string.
func (d *dumpState) byteArray(v reflect.Value) {
	d.byte_('"')
	for i := 0; i < v.Len(); i++ {
		d.hexByte(byte(v.Index(i).Uint()))
	}
	d.byte_('"')
}

// byteSlice renders []byte as hex: short slices inline, longer ones as a
// hexdump with offset comments.
func (d *dumpState) byteSlice(v reflect.Value) {
	n := v.Len()
	d.str("[]byte{")
	if n == 0 {
		d.byte_('}')
		return
	}
	if n <= listBreakLen {
		for i := 0; i < n; i++ {
			if i > 0 {
				d.byte_(' ')
			}
			d.hexByte(byte(v.Index(i).Uint()))
		}
		d.byte_('}')
		return
	}
	d.depth++
	for i := 0; i < n; i++ {
		if i%listBreakLen == 0 {
			if i > 0 {
				d.offsetComment(i - listBreakLen)
			}
			d.newline()
		} else {
			d.byte_(' ')
		}
		d.hexByte(byte(v.Index(i).Uint()))
	}
	d.offsetComment((n - 1) / listBreakLen * listBreakLen)
	d.depth--
	d.newline()
	d.byte_('}')
}

func (d *dumpState) hexByte(b byte) {
	const hexDigits = "0123456789abcdef"
	d.byte_(hexDigits[b>>4])
	d.byte_(hexDigits[b&0xf])
}

func (d *dumpState) offsetComment(offset int) {
	d.str(" // ")
	s := strconv.FormatUint(uint64(offset), 16)
	for i := len(s); i < 8; i++ {
		d.byte_('0')
	}
	d.str(s)
}

func (d *dumpState) mapValue(v reflect.Value) {
	if v.IsNil() {
		d.byte_('(')
		d.str(v.Type().String())
		d.str(")(nil)")
		return
	}
	if d.enter(v.Pointer()) {
		d.str("<cycle>")
		return
	}
	defer d.leave(v.Pointer())

	d.str(v.Type().String())
	if v.Len() == 0 {
		d.str("{}")
		return
	}

	// Render each key up front so entries can be ordered
	// deterministically, whatever the key type.
	type entry struct {
		key string
		val reflect.Value
	}
	entries := make([]entry, 0, v.Len())
	iter := v.MapRange()
	for iter.Next() {
		sub := dumpState{buf: make([]byte, 0, 16), visited: d.visited, depth: d.depth}
		sub.value(iter.Key())
		entries = append(entries, entry{key: string(sub.buf), val: iter.Value()})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].key < entries[j].key })

	if len(entries) <= listBreakLen {
		mark := len(d.buf)
		d.byte_('{')
		for i, e := range entries {
			if i > 0 {
				d.str(", ")
			}
			d.str(e.key)
			d.str(": ")
			d.value(e.val)
		}
		d.byte_('}')
		if d.inlineFits(mark) {
			return
		}
		d.buf = d.buf[:mark]
	}
	d.byte_('{')
	d.depth++
	for _, e := range entries {
		d.newline()
		d.str(e.key)
		d.str(": ")
		d.value(e.val)
		d.byte_(',')
	}
	d.depth--
	d.newline()
	d.byte_('}')
}

func (d *dumpState) structValue(v reflect.Value) {
	t := v.Type()
	d.str(t.String())
	if t.NumField() == 0 {
		d.str("{}")
		return
	}
	d.byte_('{')
	d.depth++
	for i := 0; i < t.NumField(); i++ {
		d.newline()
		d.str(t.Field(i).Name)
		d.str(": ")
		d.value(v.Field(i))
		d.byte_(',')
	}
	d.depth--
	d.newline()
	d.byte_('}')
}
