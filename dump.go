package prettyconsole

import (
	"bytes"
	"io"
	"math"
	"reflect"
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
	buf   []byte
	depth int
	// visited is a stack of container addresses on the current dump path.
	// Depth is bounded by maxDumpDepth, so a linear scan beats a map.
	visited []uintptr
	// kbuf and entries are scratch space for sorting map keys; both grow
	// once and are reused stack-style across nested maps.
	kbuf    []byte
	entries []mapEntry
}

type mapEntry struct {
	off, end int // rendered key bytes within kbuf
	val      reflect.Value
}

var dumpPool = sync.Pool{New: func() interface{} {
	return &dumpState{
		buf:     make([]byte, 0, 256),
		visited: make([]uintptr, 0, 16),
		kbuf:    make([]byte, 0, 64),
		entries: make([]mapEntry, 0, 8),
	}
}}

// dumpValue writes a readable representation of v to w.
func dumpValue(w io.Writer, v interface{}) error {
	d := dumpPool.Get().(*dumpState)
	defer func() {
		d.buf = d.buf[:0]
		d.depth = 0
		d.visited = d.visited[:0]
		d.kbuf = d.kbuf[:0]
		d.entries = d.entries[:0]
		dumpPool.Put(d)
	}()

	if v == nil {
		d.str("nil")
	} else {
		rv := reflect.ValueOf(v)
		// Make the value addressable so unexported struct fields further
		// down can be read (see bypass) - but only when the type can
		// actually contain an unexported timestamp, so the common case
		// skips the copy.
		if rv.Kind() != reflect.Ptr && rv.CanInterface() && typeNeedsAddr(rv.Type()) {
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
	case reflect.Ptr:
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
	for _, q := range d.visited {
		if q == p {
			return true
		}
	}
	d.visited = append(d.visited, p)
	return false
}

// leave pops the most recent container; enter/leave calls strictly nest.
func (d *dumpState) leave(uintptr) { d.visited = d.visited[:len(d.visited)-1] }

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
	case reflect.Ptr, reflect.Interface, reflect.Slice, reflect.Array,
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
	// deterministically, whatever the key type. Keys are rendered through
	// the main buffer, then moved into the pooled kbuf scratch; base
	// offsets make this safe for nested maps.
	kbase, ebase := len(d.kbuf), len(d.entries)
	defer func() {
		d.kbuf = d.kbuf[:kbase]
		d.entries = d.entries[:ebase]
	}()
	iter := v.MapRange()
	for iter.Next() {
		mark := len(d.buf)
		d.value(iter.Key())
		off := len(d.kbuf)
		d.kbuf = append(d.kbuf, d.buf[mark:]...)
		d.buf = d.buf[:mark]
		d.entries = append(d.entries, mapEntry{off: off, end: len(d.kbuf), val: iter.Value()})
	}
	entries := d.entries[ebase:]
	// Insertion sort by rendered key: log maps are small, and this avoids
	// the allocations sort.Slice makes for its swapper and closure.
	for i := 1; i < len(entries); i++ {
		for j := i; j > 0 && bytes.Compare(
			d.kbuf[entries[j].off:entries[j].end],
			d.kbuf[entries[j-1].off:entries[j-1].end]) < 0; j-- {
			entries[j], entries[j-1] = entries[j-1], entries[j]
		}
	}

	if len(entries) <= listBreakLen {
		mark := len(d.buf)
		d.byte_('{')
		for i := range entries {
			if i > 0 {
				d.str(", ")
			}
			d.buf = append(d.buf, d.kbuf[entries[i].off:entries[i].end]...)
			d.str(": ")
			d.value(entries[i].val)
		}
		d.byte_('}')
		if d.inlineFits(mark) {
			return
		}
		d.buf = d.buf[:mark]
	}
	d.byte_('{')
	d.depth++
	for i := range entries {
		d.newline()
		d.buf = append(d.buf, d.kbuf[entries[i].off:entries[i].end]...)
		d.str(": ")
		d.value(entries[i].val)
		d.byte_(',')
	}
	d.depth--
	d.newline()
	d.byte_('}')
}

// fieldNameCache caches struct field names per type: reflect.Type.Field
// allocates a StructField copy on every call.
var fieldNameCache sync.Map // reflect.Type -> []string

func fieldNames(t reflect.Type) []string {
	if names, ok := fieldNameCache.Load(t); ok {
		return names.([]string)
	}
	names := make([]string, t.NumField())
	for i := range names {
		names[i] = t.Field(i).Name
	}
	fieldNameCache.Store(t, names)
	return names
}

func (d *dumpState) structValue(v reflect.Value) {
	t := v.Type()
	d.str(t.String())
	if t.NumField() == 0 {
		d.str("{}")
		return
	}
	names := fieldNames(t)
	d.byte_('{')
	d.depth++
	for i, name := range names {
		d.newline()
		d.str(name)
		d.str(": ")
		d.value(v.Field(i))
		d.byte_(',')
	}
	d.depth--
	d.newline()
	d.byte_('}')
}

// typeNeedsAddrCache caches whether a type can reach a time.Time-shaped
// struct through fields the bypass could serve (see typeNeedsAddr).
var typeNeedsAddrCache sync.Map // reflect.Type -> bool

// typeNeedsAddr reports whether dumping t could hit the unexported-
// timestamp bypass, which needs an addressable value. Interfaces and map
// contents are excluded: values reached through them are never
// addressable, so the top-level copy would not help anyway.
func typeNeedsAddr(t reflect.Type) bool {
	if v, ok := typeNeedsAddrCache.Load(t); ok {
		return v.(bool)
	}
	res := typeNeedsAddrWalk(t, make(map[reflect.Type]bool))
	typeNeedsAddrCache.Store(t, res)
	return res
}

func typeNeedsAddrWalk(t reflect.Type, seen map[reflect.Type]bool) bool {
	if seen[t] {
		return false
	}
	seen[t] = true
	switch t.Kind() {
	case reflect.Struct:
		if t.ConvertibleTo(timeType) {
			return true
		}
		for i := 0; i < t.NumField(); i++ {
			if typeNeedsAddrWalk(t.Field(i).Type, seen) {
				return true
			}
		}
	case reflect.Ptr, reflect.Slice, reflect.Array:
		return typeNeedsAddrWalk(t.Elem(), seen)
	}
	return false
}
