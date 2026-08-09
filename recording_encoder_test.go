package prettyconsole

import (
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestWith(t *testing.T) {
	cfg := NewEncoderConfig()
	cfg.TimeKey = zapcore.OmitKey
	enc := NewEncoder(cfg)
	buf := testBufferWriterSync{}
	pretty := zap.New(zapcore.NewCore(enc, &buf, zap.NewAtomicLevel()))

	// Regular With
	// WRN > wtf bark1=barv1 fook1=foov1
	pretty1 := pretty.With(zap.String("fook1", "foov1"))
	pretty1.Warn("wtf", zap.String("bark1", "barv1"))
	expected := "<yellow>WRN<r><yellow> <r><bold><yellow>><r><r><yellow> <r>wtf<yellow> <r><yellow>bark1=<r>barv1<yellow> <r><yellow>fook1=<r>foov1\n"
	got := tagANSI(buf.buf.String())
	assert.Equalf(t, expected, got, "Incorrect encoded entry, received: \n%v", got)
	buf.buf.Reset()

	// Adding a namespace with With
	// WRN > wtf fook1=foov1
	//   ↳ fook11.bark11=barv11 .bark12=barv12
	pretty11 := pretty1.With(zap.Namespace("fook11"))
	pretty11 = pretty11.With(zap.String("bark12", "barv12"))
	pretty11.Warn("wtf", zap.String("bark11", "barv11"))
	expected = "<yellow>WRN<r><yellow> <r><bold><yellow>><r><r><yellow> <r>wtf<yellow> <r><yellow>fook1=<r>foov1\n<yellow>  ↳ fook11<r><yellow>.bark11=<r>barv11<yellow> <r><yellow>.bark12=<r>barv12\n"
	got = tagANSI(buf.buf.String())
	assert.Equalf(t, expected, got, "Incorrect encoded entry, received: \n%v", got)
	buf.buf.Reset()

	// Making sure pretty didn't get modified above
	// WRN > wtf bark2=barv2 fook2=foov2
	pretty2 := pretty.With(zap.String("fook2", "foov2"))
	pretty2.Warn("wtf", zap.String("bark2", "barv2"))
	expected = "<yellow>WRN<r><yellow> <r><bold><yellow>><r><r><yellow> <r>wtf<yellow> <r><yellow>bark2=<r>barv2<yellow> <r><yellow>fook2=<r>foov2\n"
	got = tagANSI(buf.buf.String())
	assert.Equalf(t, expected, got, "Incorrect encoded entry, received: \n%v", got)
	buf.buf.Reset()
}

// TestWithAllFieldTypes drives every recordingEncoder method: fields added
// via Logger.With go through the recording encoder rather than directly to
// the console encoder.
func TestWithAllFieldTypes(t *testing.T) {
	cfg := NewEncoderConfig()
	cfg.TimeKey = zapcore.OmitKey
	enc := NewEncoder(cfg)
	sink := &testBufferWriterSync{}
	logger := zap.New(zapcore.NewCore(enc, sink, zap.NewAtomicLevel()))

	logger.With(
		zap.Binary("binary", []byte{0xde, 0xad}),
		zap.ByteString("bytestring", []byte("bs")),
		zap.Bool("bool", true),
		zap.Complex128("c128", 1+2i),
		zap.Complex64("c64", 3+4i),
		zap.Duration("duration", time.Second),
		zap.Float64("f64", 1.5),
		zap.Float32("f32", 2.5),
		zap.Int("int", 1),
		zap.Int64("i64", 2),
		zap.Int32("i32", 3),
		zap.Int16("i16", 4),
		zap.Int8("i8", 5),
		zap.String("string", "sv"),
		zap.Time("time", time.Date(2022, 6, 19, 16, 33, 42, 0, time.UTC)),
		zap.Uint("uint", 6),
		zap.Uint64("u64", 7),
		zap.Uint32("u32", 8),
		zap.Uint16("u16", 9),
		zap.Uint8("u8", 10),
		zap.Uintptr("uintptr", 11),
		zap.Reflect("reflect", struct{ A int }{12}),
		zap.Array("array", testArray{1, 2}),
		zap.Object("object", testStableMap{"k": "v"}),
		zap.Namespace("ns"),
		zap.String("inner", "iv"),
	).Info("all types")

	out := stripANSI(sink.buf.String())
	for _, want := range []string{
		"binary=3q0=", "bytestring=bs", "bool=true", "c128=1+2i", "c64=3+4i",
		"duration=1s", "f64=1.5", "f32=2.5", "int=1", "i64=2", "i32=3",
		"i16=4", "i8=5", "string=sv", "time=2022-06-19T16:33:42Z", "uint=6",
		"u64=7", "u32=8", "u16=9", "u8=10", "uintptr=11", "reflect",
		"array=[1, 2]", "object.k=v", "ns.inner=iv",
	} {
		assert.Contains(t, out, want)
	}
}

func TestRecordingEncoderDirectMethods(t *testing.T) {
	r := &recordingEncoder{}
	r.AddInt("i", -1)
	r.AddUint("u", 1)
	assert.Equal(t, []zapcore.Field{zap.Int("i", -1), zap.Uint("u", 1)}, r.fields)
}

// The tests below target the accumulated-context cache: recorded fields
// are pre-sorted once and their rendering is cached per level when an
// entry adds no fields of its own.

// TestContextCacheMatchesFreshEncoder is the core identity property: a
// cached rendering must be byte-identical to a fresh encoder's output,
// across levels and repeated calls.
func TestContextCacheMatchesFreshEncoder(t *testing.T) {
	cfg := NewEncoderConfig()
	cfg.TimeKey = zapcore.OmitKey
	fields := []zap.Field{
		zap.String("b", "2"), zap.String("a", "1"), zap.Int("n", 7),
		zap.Namespace("ns"), zap.String("z", "26"), zap.String("y", "25"),
		zap.Object("obj", testStableMap{"k": "v"}),
	}

	cachedSink := &testBufferWriterSync{}
	cached := zap.New(zapcore.NewCore(NewEncoder(cfg), cachedSink, zap.NewAtomicLevel())).With(fields...)

	for _, log := range []func(string, ...zap.Field){cached.Info, cached.Warn, cached.Error, cached.Info} {
		// A fresh logger re-renders everything from scratch each time.
		freshSink := &testBufferWriterSync{}
		fresh := zap.New(zapcore.NewCore(NewEncoder(cfg), freshSink, zap.NewAtomicLevel())).With(fields...)

		cachedSink.buf.Reset()
		log("msg")
		switch { // mirror the level on the fresh logger
		case strings.Contains(tagANSI(cachedSink.buf.String()), "WRN"):
			fresh.Warn("msg")
		case strings.Contains(tagANSI(cachedSink.buf.String()), "ERR"):
			fresh.Error("msg")
		default:
			fresh.Info("msg")
		}
		assert.Equal(t, tagANSI(freshSink.buf.String()), tagANSI(cachedSink.buf.String()),
			"cached context rendering must be byte-identical to a fresh encoder")
	}
}

// TestContextCacheWithPerLineFields checks the merge path: per-line fields
// must still interleave alphabetically with cached, pre-sorted context.
func TestContextCacheWithPerLineFields(t *testing.T) {
	cfg := NewEncoderConfig()
	cfg.TimeKey = zapcore.OmitKey
	sink := &testBufferWriterSync{}
	logger := zap.New(zapcore.NewCore(NewEncoder(cfg), sink, zap.NewAtomicLevel())).
		With(zap.String("ccc", "context"), zap.String("aaa", "context"))

	// Warm the no-fields cache first, then log with fields: the cache must
	// not leak into the merge path.
	logger.Info("warm")
	sink.buf.Reset()
	logger.Info("msg", zap.String("bbb", "line"))
	out := stripANSI(sink.buf.String())
	a, b, c := strings.Index(out, "aaa="), strings.Index(out, "bbb="), strings.Index(out, "ccc=")
	assert.True(t, a < b && b < c, "expected aaa < bbb < ccc interleaved, got %q", out)
}

// TestContextCacheCloneInvalidation: deriving a new logger via With must
// not inherit the parent's cache.
func TestContextCacheCloneInvalidation(t *testing.T) {
	cfg := NewEncoderConfig()
	cfg.TimeKey = zapcore.OmitKey
	sink := &testBufferWriterSync{}
	parent := zap.New(zapcore.NewCore(NewEncoder(cfg), sink, zap.NewAtomicLevel())).
		With(zap.String("parent", "v"))
	parent.Info("warm parent cache")

	child := parent.With(zap.String("child", "w"))
	sink.buf.Reset()
	child.Info("msg")
	out := stripANSI(sink.buf.String())
	assert.Contains(t, out, "parent=v")
	assert.Contains(t, out, "child=w")
}

// TestContextCacheFreezesMarshalers documents a deliberate semantics
// change that matches zap's own encoders: marshalers in accumulated
// context are rendered once per level, not once per line, so later
// mutations of the marshaled object are not observed on the no-fields
// path.
func TestContextCacheFreezesMarshalers(t *testing.T) {
	cfg := NewEncoderConfig()
	cfg.TimeKey = zapcore.OmitKey
	sink := &testBufferWriterSync{}
	m := &mutableMarshaler{value: "before"}
	logger := zap.New(zapcore.NewCore(NewEncoder(cfg), sink, zap.NewAtomicLevel())).
		With(zap.Object("obj", m))

	logger.Info("first")
	m.value = "after"
	sink.buf.Reset()
	logger.Info("second")
	assert.Contains(t, stripANSI(sink.buf.String()), "value=before",
		"context marshalers freeze at first render, matching zap's pre-encoding")
}

type mutableMarshaler struct{ value string }

func (m *mutableMarshaler) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("value", m.value)
	return nil
}

// TestContextCacheConcurrentFill hammers the lazy cache initialisation
// from many goroutines across levels; the race detector does the judging.
func TestContextCacheConcurrentFill(t *testing.T) {
	cfg := NewEncoderConfig()
	cfg.TimeKey = zapcore.OmitKey
	logger := zap.New(zapcore.NewCore(NewEncoder(cfg), zapcore.Lock(zapcore.AddSync(io.Discard)), zap.NewAtomicLevel())).
		With(zap.String("ctx", "v"), zap.Object("obj", testStableMap{"k": "v"}))

	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				switch g % 3 {
				case 0:
					logger.Info("no fields")
				case 1:
					logger.Warn("no fields other level")
				default:
					logger.Error("with fields", zap.Int("i", i))
				}
			}
		}(g)
	}
	wg.Wait()
}

// TestContextCacheWithStacktrace: the fast path must still append
// stacktraces after the cached context.
func TestContextCacheWithStacktrace(t *testing.T) {
	cfg := NewEncoderConfig()
	cfg.TimeKey = zapcore.OmitKey
	sink := &testBufferWriterSync{}
	logger := zap.New(zapcore.NewCore(NewEncoder(cfg), sink, zap.NewAtomicLevel()),
		zap.AddStacktrace(zapcore.ErrorLevel)).With(zap.String("ctx", "v"))

	logger.Error("boom")
	out := stripANSI(sink.buf.String())
	assert.Contains(t, out, "ctx=v")
	assert.Contains(t, out, "stacktrace=")
	assert.Less(t, strings.Index(out, "ctx=v"), strings.Index(out, "stacktrace="),
		"stacktrace must come after cached context")
}
