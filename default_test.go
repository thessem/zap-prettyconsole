package prettyconsole

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestLevels(t *testing.T) {
	tests := []struct {
		level zapcore.Level
		label string
	}{
		{zapcore.DebugLevel - 1, "TRC"}, // DIY trace level
		{zapcore.DebugLevel, "DBG"},
		{zapcore.InfoLevel, "INF"},
		{zapcore.WarnLevel, "WRN"},
		{zapcore.ErrorLevel, "ERR"},
		{zapcore.DPanicLevel, "DPNC"},
		{zapcore.PanicLevel, "PNC"},
		{zapcore.FatalLevel, "FTL"},
		// Levels outside the known range must render (as panic-style),
		// never crash the encoder: zapcore.Level is an int8 and custom
		// levels are legal.
		{zapcore.Level(42), "???"},
		{zapcore.Level(-42), "???"},
	}
	cfg := NewEncoderConfig()
	cfg.TimeKey = zapcore.OmitKey
	enc := NewEncoder(cfg)
	for _, tt := range tests {
		t.Run(tt.label+"_"+tt.level.String(), func(t *testing.T) {
			buf, err := enc.EncodeEntry(zapcore.Entry{Level: tt.level, Message: "m"}, nil)
			require.NoError(t, err)
			defer buf.Free()
			assert.True(t, strings.HasPrefix(stripANSI(buf.String()), tt.label+" "),
				"expected prefix %q in %q", tt.label, stripANSI(buf.String()))
		})
	}
}

func TestCallerAndFunction(t *testing.T) {
	cfg := NewEncoderConfig()
	cfg.TimeKey = zapcore.OmitKey
	cfg.LevelKey = zapcore.OmitKey
	cfg.CallerKey = "C"
	cfg.FunctionKey = "F"
	enc := NewEncoder(cfg)

	cwd, err := os.Getwd()
	require.NoError(t, err)
	caller := zapcore.NewEntryCaller(0, filepath.Join(cwd, "some_file.go"), 42, true)
	caller.Function = "pkg.SomeFunction"

	buf, err := enc.EncodeEntry(zapcore.Entry{Level: zapcore.InfoLevel, Message: "m", Caller: caller}, nil)
	require.NoError(t, err)
	defer buf.Free()
	out := stripANSI(buf.String())
	assert.Contains(t, out, "some_file.go:42")
	assert.Contains(t, out, "pkg.SomeFunction")
}

type noopReflected struct{}

func (noopReflected) Encode(interface{}) error { return nil }

// TestEncoderFallbacks covers the branches that recover from user-supplied
// no-op encoder callbacks.
func TestEncoderFallbacks(t *testing.T) {
	cfg := NewEncoderConfig()
	cfg.TimeKey = zapcore.OmitKey
	cfg.LevelKey = zapcore.OmitKey
	cfg.EncodeDuration = func(time.Duration, zapcore.PrimitiveArrayEncoder) {}
	cfg.EncodeTime = func(time.Time, zapcore.PrimitiveArrayEncoder) {}
	cfg.NewReflectedEncoder = func(w io.Writer) zapcore.ReflectedEncoder { return noopReflected{} }
	enc := NewEncoder(cfg)

	when := time.Date(2022, 6, 19, 16, 33, 42, 0, time.UTC)
	buf, err := enc.EncodeEntry(zapcore.Entry{Level: zapcore.InfoLevel, Message: "m"}, []zapcore.Field{
		zap.Duration("dur", 90*time.Second),
		zap.Durations("durs", []time.Duration{time.Second}),
		zap.Times("times", []time.Time{when}),
		zap.Reflect("reflect", struct{ A int }{1}),
	})
	require.NoError(t, err)
	defer buf.Free()
	out := stripANSI(buf.String())
	assert.Contains(t, out, "dur=1m30s")                    // AddDuration falls back to Duration.String
	assert.Contains(t, out, "durs=[1000000000]")            // AppendDuration falls back to nanoseconds
	assert.Contains(t, out, "times=[2022-06-19T16:33:42Z]") // AppendTime falls back to RFC3339
	assert.Contains(t, out, "A: 1")                         // AddReflected falls back to the reflection dumper
}

func TestReflectedTimeFormatting(t *testing.T) {
	// Create a struct with time.Time field
	type TestStruct struct {
		Timestamp time.Time
		Message   string
	}

	testTime := time.Date(2024, 1, 15, 14, 30, 45, 0, time.UTC)
	testData := TestStruct{
		Timestamp: testTime,
		Message:   "test",
	}

	enc := NewEncoder(NewEncoderConfig())
	ent := zapcore.Entry{
		Level:   zapcore.InfoLevel,
		Message: "reflected time test",
		Time:    time.Date(2024, 1, 15, 14, 30, 0, 0, time.UTC),
	}

	buf, err := enc.EncodeEntry(ent, []zapcore.Field{
		zap.Reflect("data", testData),
	})

	assert.NoError(t, err)

	output := buf.String()

	// Verify RFC3339 format appears in output
	// Expected: 2024-01-15T14:30:45Z
	assert.Contains(t, output, "2024-01-15T14:30:45Z",
		"Timestamp should be formatted in RFC3339 format")
	// Ensure not using Go's internal time format with wall clock
	assert.NotContains(t, output, "wall=",
		"Should not use Go's internal time format")
}

func TestReflectedByteSliceFormatting(t *testing.T) {
	// Test that byte slices get hex dump formatting
	type DataWithBytes struct {
		TraceID []byte
		SpanID  []byte
	}

	// Create test data with known byte patterns
	data := DataWithBytes{
		TraceID: []byte{
			0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
			0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f,
		},
		SpanID: []byte{0xa1, 0xb2, 0xc3, 0xd4, 0xe5, 0xf6, 0x07, 0x18},
	}

	enc := NewEncoder(NewEncoderConfig())
	ent := zapcore.Entry{
		Level:   zapcore.InfoLevel,
		Message: "byte slice test",
		Time:    time.Date(2024, 1, 15, 14, 30, 0, 0, time.UTC),
	}

	buf, err := enc.EncodeEntry(ent, []zapcore.Field{
		zap.Reflect("data", data),
	})

	assert.NoError(t, err)

	output := buf.String()

	// Byte slices get hexdump-style output
	// Should see hex bytes like "00 01 02 03 04 05 06 07"
	assert.Contains(t, output, "00 01 02 03 04 05 06 07",
		"TraceID should show hex dump format")
	assert.Contains(t, output, "a1 b2 c3 d4 e5 f6 07 18",
		"SpanID should show hex dump format")

	// Should include hexdump-style comment format
	assert.Contains(t, output, "// 00000000",
		"Should include hexdump address prefix")
}

func TestByteSliceEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		contains string
	}{
		{
			name:     "Empty byte slice",
			input:    struct{ Data []byte }{Data: []byte{}},
			contains: "Data:", // Should handle empty slices gracefully
		},
		{
			name:     "Single byte",
			input:    struct{ Value []byte }{Value: []byte{0xff}},
			contains: "ff",
		},
		{
			name: "Nested struct with byte slices",
			input: struct {
				Outer struct {
					Inner []byte
				}
			}{
				Outer: struct{ Inner []byte }{
					Inner: []byte{0xde, 0xad, 0xbe, 0xef},
				},
			},
			contains: "de ad be ef",
		},
	}

	enc := NewEncoder(NewEncoderConfig())
	ent := zapcore.Entry{
		Level:   zapcore.InfoLevel,
		Message: "edge case test",
		Time:    time.Date(2024, 1, 15, 14, 30, 0, 0, time.UTC),
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf, err := enc.EncodeEntry(ent, []zapcore.Field{
				zap.Reflect("data", tt.input),
			})
			assert.NoError(t, err)
			assert.Contains(t, buf.String(), tt.contains)
		})
	}
}

func TestOpenTelemetrySpanContext(t *testing.T) {
	// Test with real OpenTelemetry trace.SpanContext
	// This uses actual OTel types as a test-only dependency
	traceID := [16]byte{
		0x53, 0xae, 0x01, 0xc0, 0x1f, 0x35, 0xb7, 0x1e,
		0xa7, 0x1b, 0x7b, 0xed, 0xef, 0x1c, 0x67, 0xdd,
	}
	spanID := [8]byte{0x9a, 0x2b, 0x3c, 0x4d, 0x5e, 0x6f, 0x7a, 0x8b}

	// Create a span context using OTel's actual types
	// Note: We can't directly construct trace.SpanContext with our bytes,
	// so we'll create a wrapper struct that's representative
	type OTelLikeContext struct {
		TraceID [16]byte
		SpanID  [8]byte
		Flags   byte
	}

	spanCtx := OTelLikeContext{
		TraceID: traceID,
		SpanID:  spanID,
		Flags:   1,
	}

	enc := NewEncoder(NewEncoderConfig())
	ent := zapcore.Entry{
		Level:   zapcore.InfoLevel,
		Message: "OpenTelemetry trace context",
		Time:    time.Date(2024, 1, 15, 14, 30, 0, 0, time.UTC),
	}

	buf, err := enc.EncodeEntry(ent, []zapcore.Field{
		zap.Reflect("span_context", spanCtx),
	})

	assert.NoError(t, err)
	output := buf.String()

	// Print the actual output for documentation purposes
	t.Logf("OpenTelemetry SpanContext output:\n%s", output)

	// Byte arrays show as hex strings
	assert.Contains(t, output, "TraceID:", "Should contain TraceID field")
	assert.Contains(t, output, "SpanID:", "Should contain SpanID field")

	// Verify hex string format (lowercase hex without 0x prefix)
	assert.Contains(t, output, "\"53ae01c01f35b71ea71b7bedef1c67dd\"",
		"TraceID should be formatted as hex string")
	assert.Contains(t, output, "\"9a2b3c4d5e6f7a8b\"",
		"SpanID should be formatted as hex string")

	// Verify verbose individual byte format does NOT appear
	assert.NotContains(t, output, "uint8",
		"Should not show verbose uint8 array format")
	assert.NotContains(t, output, "83,",
		"Should not show individual decimal values")

	// Note: This test verifies that fixed-size byte arrays ([16]byte, [8]byte)
	// are formatted as compact hex strings by the reflection dumper.
}

func TestByteArraySizes(t *testing.T) {
	// Test various byte array sizes to verify they all format as hex strings
	type TestStruct struct {
		IPv4Address [4]byte  // 4 bytes - IPv4 addresses, CRC32
		SpanID      [8]byte  // 8 bytes - OpenTelemetry SpanID
		TraceID     [16]byte // 16 bytes - OpenTelemetry TraceID, UUID, MD5
		SHA256      [32]byte // 32 bytes - SHA256 hash
		SHA512      [64]byte // 64 bytes - SHA512 hash
	}

	testData := TestStruct{
		IPv4Address: [4]byte{192, 168, 1, 1},
		SpanID:      [8]byte{0xa1, 0xb2, 0xc3, 0xd4, 0xe5, 0xf6, 0x07, 0x18},
		TraceID:     [16]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f},
		SHA256:      [32]byte{0xab, 0xcd, 0xef, 0x01, 0x23, 0x45, 0x67, 0x89, 0x9a, 0xbc, 0xde, 0xf0, 0x12, 0x34, 0x56, 0x78, 0x87, 0x65, 0x43, 0x21, 0x0f, 0xed, 0xcb, 0xa9, 0x98, 0x76, 0x54, 0x32, 0x10, 0xfe, 0xdc, 0xba},
		SHA512: [64]byte{
			0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef, 0xfe, 0xdc, 0xba, 0x98, 0x76, 0x54, 0x32, 0x10,
			0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99, 0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff, 0x00,
			0xff, 0xee, 0xdd, 0xcc, 0xbb, 0xaa, 0x99, 0x88, 0x77, 0x66, 0x55, 0x44, 0x33, 0x22, 0x11, 0x00,
			0x0f, 0x1e, 0x2d, 0x3c, 0x4b, 0x5a, 0x69, 0x78, 0x87, 0x96, 0xa5, 0xb4, 0xc3, 0xd2, 0xe1, 0xf0,
		},
	}

	enc := NewEncoder(NewEncoderConfig())
	ent := zapcore.Entry{
		Level:   zapcore.InfoLevel,
		Message: "byte array sizes test",
		Time:    time.Date(2024, 1, 15, 14, 30, 0, 0, time.UTC),
	}

	buf, err := enc.EncodeEntry(ent, []zapcore.Field{
		zap.Reflect("data", testData),
	})

	assert.NoError(t, err)
	output := buf.String()

	t.Logf("Byte array sizes output:\n%s", output)

	// Verify [4]byte formats as hex (IPv4Address)
	assert.Contains(t, output, "\"c0a80101\"",
		"[4]byte should be formatted as hex string (c0a80101 = 192.168.1.1)")

	// Verify [8]byte formats as hex (SpanID)
	assert.Contains(t, output, "\"a1b2c3d4e5f60718\"",
		"[8]byte should be formatted as hex string")

	// Verify [16]byte formats as hex (TraceID)
	assert.Contains(t, output, "\"000102030405060708090a0b0c0d0e0f\"",
		"[16]byte should be formatted as hex string")

	// Verify [32]byte formats as hex (SHA256)
	assert.Contains(t, output, "\"abcdef01234567899abcdef01234567887654321"+
		"0fedcba998765432"+"10fedcba\"",
		"[32]byte should be formatted as hex string")

	// Verify [64]byte formats as hex (SHA512)
	assert.Contains(t, output, "\"0123456789abcdeffedcba9876543210112233445566778899aabbccddee"+
		"ff00ffeeddccbbaa99887766554433221100"+"0f1e2d3c4b5a69788796a5b4c3d2e1f0\"",
		"[64]byte should be formatted as hex string")

	// Verify verbose format does NOT appear for any size
	assert.NotContains(t, output, "uint8",
		"Should not show verbose uint8 array format for any byte array size")
	assert.NotContains(t, output, "192,",
		"Should not show individual decimal values (192 from IPv4Address)")
	assert.NotContains(t, output, "171,",
		"Should not show individual decimal values (171 = 0xab from SHA256)")
}
