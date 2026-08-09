package prettyconsole

import (
	"context"
	"encoding/json"
	"math"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func dump(t *testing.T, v interface{}) string {
	t.Helper()
	var sb strings.Builder
	require.NoError(t, dumpValue(&sb, v))
	return sb.String()
}

type dumpBasics struct {
	Exported   int
	unexported string
}

type dumpEmbed struct {
	dumpBasics
	Extra bool
}

type dumpCyclic struct {
	Name string
	Self *dumpCyclic
}

type (
	dumpNamedInt  int
	dumpNamedTime time.Time
	dumpBlob      []byte
)

func TestDumpScalars(t *testing.T) {
	tests := []struct {
		desc     string
		in       interface{}
		expected string
	}{
		{"Nil", nil, "nil"},
		{"Bool", true, "true"},
		{"Int", -42, "-42"},
		{"NamedInt", dumpNamedInt(7), "7"},
		{"Uint", uint16(65535), "65535"},
		{"Uintptr", uintptr(0xbeef), "0xbeef"},
		{"Float", 2.5, "2.5"},
		{"FloatWhole", 2.0, "2"},
		{"FloatNaN", math.NaN(), "NaN"},
		{"FloatInf", math.Inf(-1), "-Inf"},
		{"Complex", 1 + 2i, "(1+2i)"},
		{"ComplexNegImag", 1 - 2i, "(1-2i)"},
		{"String", "with \"quotes\" and \n", `"with \"quotes\" and \n"`},
		{"Duration", 90 * time.Second, `"1m30s"`},
		{"NotADuration", int64(90e9), "90000000000"},
		{"Time", time.Date(2024, 1, 15, 14, 30, 45, 0, time.UTC), `"2024-01-15T14:30:45Z"`},
		{"NamedTime", dumpNamedTime(time.Date(2024, 1, 15, 14, 30, 45, 0, time.UTC)), `"2024-01-15T14:30:45Z"`},
		{"Func", func() {}, "<func()>"},
		{"Chan", make(chan int), "<chan int>"},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			assert.Equal(t, tt.expected, dump(t, tt.in))
		})
	}
}

func TestDumpNils(t *testing.T) {
	type holder struct {
		P *int
		S []int
		M map[string]int
		I interface{}
	}
	assert.Equal(t, "prettyconsole.holder{\n"+
		"  P: (*int)(nil),\n"+
		"  S: ([]int)(nil),\n"+
		"  M: (map[string]int)(nil),\n"+
		"  I: nil,\n"+
		"}", dump(t, holder{}))
}

func TestDumpStructs(t *testing.T) {
	t.Run("UnexportedFields", func(t *testing.T) {
		assert.Equal(t, "prettyconsole.dumpBasics{\n  Exported: 1,\n  unexported: \"secret\",\n}",
			dump(t, dumpBasics{Exported: 1, unexported: "secret"}))
	})
	t.Run("Embedded", func(t *testing.T) {
		out := dump(t, dumpEmbed{dumpBasics: dumpBasics{Exported: 2}, Extra: true})
		assert.Contains(t, out, "dumpBasics: prettyconsole.dumpBasics{")
		assert.Contains(t, out, "Extra: true,")
	})
	t.Run("Anonymous", func(t *testing.T) {
		assert.Equal(t, "struct { A int }{\n  A: 12,\n}", dump(t, struct{ A int }{12}))
	})
	t.Run("Empty", func(t *testing.T) {
		assert.Equal(t, "struct {}{}", dump(t, struct{}{}))
	})
	t.Run("UnexportedTime", func(t *testing.T) {
		// The reflect.NewAt bypass: unexported time.Time fields must still
		// render as timestamps, not as wall/ext/loc internals.
		type wrap struct{ at time.Time }
		out := dump(t, wrap{at: time.Date(2024, 1, 15, 14, 30, 45, 0, time.UTC)})
		assert.Contains(t, out, `at: "2024-01-15T14:30:45Z"`)
		assert.NotContains(t, out, "wall")
	})
	t.Run("PointerToStruct", func(t *testing.T) {
		assert.Equal(t, "&struct { A int }{\n  A: 1,\n}", dump(t, &struct{ A int }{1}))
	})
}

func TestDumpMaps(t *testing.T) {
	t.Run("SortedDeterministic", func(t *testing.T) {
		m := map[string]int{"c": 3, "a": 1, "b": 2}
		want := `map[string]int{"a": 1, "b": 2, "c": 3}`
		for i := 0; i < 20; i++ {
			assert.Equal(t, want, dump(t, m))
		}
	})
	t.Run("IntKeysSorted", func(t *testing.T) {
		// Keys sort by rendered form, which is deterministic if not
		// numerically ordered for ints.
		out := dump(t, map[int]string{10: "x", 2: "y"})
		assert.Equal(t, `map[int]string{10: "x", 2: "y"}`, out)
	})
	t.Run("Empty", func(t *testing.T) {
		assert.Equal(t, "map[string]int{}", dump(t, map[string]int{}))
	})
	t.Run("StructValuesMultiline", func(t *testing.T) {
		out := dump(t, map[string]struct{ A int }{"k": {1}})
		assert.Equal(t, "map[string]struct { A int }{\n  \"k\": struct { A int }{\n    A: 1,\n  },\n}", out)
	})
	t.Run("ManyEntriesMultiline", func(t *testing.T) {
		m := map[int]int{}
		for i := 0; i < 10; i++ {
			m[i] = i
		}
		out := dump(t, m)
		assert.Contains(t, out, "{\n")
		assert.Contains(t, out, "0: 0,")
	})
}

func TestDumpSequences(t *testing.T) {
	tests := []struct {
		desc     string
		in       interface{}
		expected string
	}{
		{"InlineInts", []int{1, 2, 3}, "[]int{1, 2, 3}"},
		{"EmptySlice", []int{}, "[]int{}"},
		{"NonByteArray", [3]int{1, 2, 3}, "[3]int{1, 2, 3}"},
		{
			"BreakAtNine",
			[]int{1, 2, 3, 4, 5, 6, 7, 8, 9},
			"[]int{\n  1, 2, 3, 4, 5, 6, 7, 8,\n  9,\n}",
		},
		{
			"StructsOnePerLine",
			[]struct{ A int }{{1}, {2}},
			"[]struct { A int }{\n  struct { A int }{\n    A: 1,\n  },\n  struct { A int }{\n    A: 2,\n  },\n}",
		},
		{"InterfaceElems", []interface{}{1, "two", nil}, `[]interface {}{1, "two", nil}`},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			assert.Equal(t, tt.expected, dump(t, tt.in))
		})
	}
}

func TestDumpBytes(t *testing.T) {
	tests := []struct {
		desc     string
		in       interface{}
		expected string
	}{
		{"EmptyByteSlice", []byte{}, "[]byte{}"},
		{"SingleByte", []byte{0xff}, "[]byte{ff}"},
		{"InlineBytes", []byte{0xde, 0xad, 0xbe, 0xef}, "[]byte{de ad be ef}"},
		{"NamedByteSlice", dumpBlob{0x01, 0x02}, "[]byte{01 02}"},
		{
			"HexdumpWithOffsets",
			[]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
			"[]byte{\n  00 01 02 03 04 05 06 07 // 00000000\n  08 09 0a 0b 0c 0d 0e 0f // 00000008\n  10 // 00000010\n}",
		},
		// dd needed one hand-written formatter per array size; reflection
		// handles every size, including ones dd never covered.
		{"Array1", [1]byte{0xab}, `"ab"`},
		{"Array3", [3]byte{1, 2, 3}, `"010203"`},
		{"Array7", [7]byte{0, 1, 2, 3, 4, 5, 6}, `"00010203040506"`},
		{"Array20", [20]byte{}, `"` + strings.Repeat("00", 20) + `"`},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			assert.Equal(t, tt.expected, dump(t, tt.in))
		})
	}
}

func TestDumpCycles(t *testing.T) {
	t.Run("SelfPointer", func(t *testing.T) {
		c := &dumpCyclic{Name: "a"}
		c.Self = c
		assert.Equal(t, "&prettyconsole.dumpCyclic{\n  Name: \"a\",\n  Self: <cycle>,\n}", dump(t, c))
	})
	t.Run("MutualPointers", func(t *testing.T) {
		a := &dumpCyclic{Name: "a"}
		b := &dumpCyclic{Name: "b", Self: a}
		a.Self = b
		out := dump(t, a)
		assert.Contains(t, out, `Name: "b"`)
		assert.Contains(t, out, "<cycle>")
	})
	t.Run("CyclicMap", func(t *testing.T) {
		m := map[string]interface{}{}
		m["self"] = m
		assert.Equal(t, `map[string]interface {}{"self": <cycle>}`, dump(t, m))
	})
	t.Run("CyclicSlice", func(t *testing.T) {
		s := make([]interface{}, 1)
		s[0] = s
		assert.Contains(t, dump(t, s), "<cycle>")
	})
	t.Run("DiamondIsNotACycle", func(t *testing.T) {
		shared := &dumpBasics{Exported: 9}
		type pair struct{ L, R *dumpBasics }
		out := dump(t, pair{L: shared, R: shared})
		assert.Equal(t, 2, strings.Count(out, "Exported: 9"), "shared pointer must print fully both times")
		assert.NotContains(t, out, "<cycle>")
	})
}

// TestDumpDeepNesting is the regression test for the pathology that
// motivated replacing dd: a deep (non-cyclic) structure took super-
// quadratic time there (over a minute at depth 1600). Here it must
// complete promptly, truncating at maxDumpDepth.
func TestDumpDeepNesting(t *testing.T) {
	type node struct{ Child *node }
	root := &node{}
	cur := root
	for i := 0; i < 10000; i++ {
		cur.Child = &node{}
		cur = cur.Child
	}
	start := time.Now()
	out := dump(t, root)
	elapsed := time.Since(start)
	assert.Contains(t, out, "<max depth>")
	assert.Less(t, elapsed, 2*time.Second, "deep structures must not hang the logger")
}

// TestDumpConcurrent exercises the dump state pool under the race
// detector.
func TestDumpConcurrent(t *testing.T) {
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				var sb strings.Builder
				_ = dumpValue(&sb, map[string][]int{"a": {1, 2}, "b": nil})
			}
		}()
	}
	wg.Wait()
}

func BenchmarkDumpValue(b *testing.B) {
	v := struct {
		Name    string
		Age     int
		Tags    []string
		Meta    map[string]int
		TraceID [16]byte
		Raw     []byte
	}{
		Name: "bench", Age: 42,
		Tags: []string{"a", "b", "c"},
		Meta: map[string]int{"x": 1, "y": 2},
		Raw:  []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11},
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var sb strings.Builder
		_ = dumpValue(&sb, v)
	}
}

// The tests below are mined from the failure classes reported against
// go-spew and Code-Hex/dd, plus this project's own closed issues, to make
// sure replacing dd regressed none of them.

// panickyStringer implements every method a dumper might be tempted to
// call. go-spew invoked these and accumulated panic bugs (#45, #115,
// #141, #144); this dumper never calls user methods, so it is immune by
// construction.
type panickyStringer struct{ Field int }

func (panickyStringer) String() string   { panic("String called") }
func (panickyStringer) Error() string    { panic("Error called") }
func (panickyStringer) GoString() string { panic("GoString called") }

func TestDumpNeverInvokesUserMethods(t *testing.T) {
	out := dump(t, panickyStringer{Field: 1})
	assert.Contains(t, out, "Field: 1")

	// spew #141/#115: Stringers as map keys and values got invalid
	// receivers or panicked.
	out = dump(t, map[panickyStringer]panickyStringer{{Field: 2}: {Field: 3}})
	assert.Contains(t, out, "Field: 2")
	assert.Contains(t, out, "Field: 3")

	// spew #144: custom error wrapping another custom error.
	out = dump(t, struct{ Err error }{Err: panickyStringer{Field: 4}})
	assert.Contains(t, out, "Field: 4")
}

// TestDumpPointerChains mirrors go-spew's corpus, which exercises value,
// pointer, pointer-to-pointer and nil-pointer variants of every type.
func TestDumpPointerChains(t *testing.T) {
	v := 42
	p := &v
	pp := &p
	var np *int
	assert.Equal(t, "&42", dump(t, p))
	assert.Equal(t, "&&42", dump(t, pp))
	assert.Equal(t, "(*int)(nil)", dump(t, np))

	s := "x"
	ps := &s
	assert.Equal(t, `&"x"`, dump(t, ps))
}

// TestDumpStructKeyedMap covers go-spew #108: sorting map keys that are
// structs with private fields panicked there.
func TestDumpStructKeyedMap(t *testing.T) {
	type privKey struct{ id int }
	m := map[privKey]string{{id: 2}: "two", {id: 1}: "one"}
	out := dump(t, m)
	assert.Contains(t, out, `"one"`)
	assert.Contains(t, out, `"two"`)
	// Deterministic ordering by rendered key.
	assert.Equal(t, out, dump(t, m))
}

// TestDumpPointerKeyedMap must not panic and must render pointee content.
func TestDumpPointerKeyedMap(t *testing.T) {
	k := &dumpBasics{Exported: 5}
	out := dump(t, map[*dumpBasics]string{k: "v"})
	assert.Contains(t, out, "Exported: 5")
	assert.Contains(t, out, `"v"`)
}

// TestDumpNaNMapKeys: maps can legally hold several NaN keys; sorting
// rendered keys must not panic and output must stay stable in shape.
func TestDumpNaNMapKeys(t *testing.T) {
	m := map[float64]int{math.NaN(): 1, math.NaN(): 2, 1.5: 3}
	out := dump(t, m)
	assert.Contains(t, out, "NaN: ")
	assert.Contains(t, out, "1.5: 3")
}

// TestDumpStdlibInternals is informed by dd #21, whose tests asserted the
// exact internals of reflect.Value and context.Background and broke on a
// Go upgrade. We assert only that such values dump without panicking -
// users do log these by accident.
func TestDumpStdlibInternals(t *testing.T) {
	var sb strings.Builder
	require.NoError(t, dumpValue(&sb, reflect.ValueOf(42)))
	assert.NotEmpty(t, sb.String())

	sb.Reset()
	require.NoError(t, dumpValue(&sb, context.Background()))
	assert.NotEmpty(t, sb.String())
}

// TestDumpDecodedJSON mirrors dd's testdata corpus (large decoded JSON
// documents): deeply mixed maps and slices must render deterministically.
func TestDumpDecodedJSON(t *testing.T) {
	var v interface{}
	blob := `{"users":[{"name":"a","tags":["x","y"],"meta":{"n":1.5,"ok":true}},
	          {"name":"b","tags":[],"meta":{"n":null,"ok":false}}],"total":2}`
	require.NoError(t, json.Unmarshal([]byte(blob), &v))
	out := dump(t, v)
	assert.Contains(t, out, `"name": "a"`)
	assert.Contains(t, out, `"n": nil`)
	assert.Equal(t, out, dump(t, v), "decoded JSON must dump deterministically")
}

// TestDumpOTelSpanContextShape is the regression test for this project's
// issue #23. The real otel trace.SpanContext keeps its fields unexported;
// the earlier test used exported look-alikes, which hid the harder case.
func TestDumpOTelSpanContextShape(t *testing.T) {
	type traceID [16]byte
	type spanID [8]byte
	type spanContext struct {
		traceID    traceID
		spanID     spanID
		traceFlags byte //nolint:unused // read via reflection by the dumper
		remote     bool
	}
	sc := spanContext{
		traceID: traceID{
			0x53, 0xae, 0x01, 0xc0, 0x1f, 0x35, 0xb7, 0x1e,
			0xa7, 0x1b, 0x7b, 0xed, 0xef, 0x1c, 0x67, 0xdd,
		},
		spanID:     spanID{0x9a, 0x2b, 0x3c, 0x4d, 0x5e, 0x6f, 0x7a, 0x8b},
		traceFlags: 1,
		remote:     true,
	}
	out := dump(t, sc)
	assert.Contains(t, out, `"53ae01c01f35b71ea71b7bedef1c67dd"`)
	assert.Contains(t, out, `"9a2b3c4d5e6f7a8b"`)
	assert.Contains(t, out, "traceFlags: 1")
	assert.NotContains(t, out, "uint8")

	// And nested inside a map value, where reflection values are not
	// addressable - the harder unexported path.
	out = dump(t, map[string]spanContext{"ctx": sc})
	assert.Contains(t, out, `"53ae01c01f35b71ea71b7bedef1c67dd"`)
}

// TestDumpNonAddressableUnexportedTime documents the one degradation on
// the non-addressable path: an unexported time.Time reached through a map
// value cannot use the addressability bypass, so it falls back to a field
// dump. It must never panic.
func TestDumpNonAddressableUnexportedTime(t *testing.T) {
	type wrap struct{ at time.Time }
	out := dump(t, map[string]wrap{"w": {at: time.Unix(0, 0)}})
	assert.Contains(t, out, "at: ")
}

func TestDumpInvalidUTF8String(t *testing.T) {
	assert.Equal(t, `"ok\xff\xfego"`, dump(t, "ok\xff\xfego"))
}

func TestDumpUintptrAndUnsafe(t *testing.T) {
	assert.Equal(t, "0x0", dump(t, uintptr(0)))
	assert.Equal(t, "0x0", dump(t, unsafe.Pointer(nil)))
}

// TestDumpMapInPointerKey exercises the pooled map iterator's fallback: a
// nested map rendered while a map key is still being iterated.
func TestDumpMapInPointerKey(t *testing.T) {
	inner := map[string]int{"i": 1}
	m := map[*map[string]int]string{&inner: "v"}
	out := dump(t, m)
	assert.Contains(t, out, `"i": 1`)
	assert.Contains(t, out, `"v"`)
}

// TestDumpSequentialReuse: repeated dumps through the pooled state (and
// its reusable map iterator) must stay independent and deterministic.
func TestDumpSequentialReuse(t *testing.T) {
	m := map[string][]int{"a": {1}, "b": {2, 3}}
	first := dump(t, m)
	for i := 0; i < 10; i++ {
		assert.Equal(t, first, dump(t, m))
		assert.Equal(t, "nil", dump(t, nil))
	}
}
