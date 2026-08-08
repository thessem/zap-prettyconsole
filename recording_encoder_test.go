package prettyconsole

import (
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
