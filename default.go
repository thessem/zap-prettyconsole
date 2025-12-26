package prettyconsole

import (
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/Code-Hex/dd"
	"github.com/Code-Hex/dd/df"
	"go.uber.org/zap/buffer"
	"go.uber.org/zap/zapcore"
)

const (
	colorBlack = iota + 30
	colorRed
	colorGreen
	colorYellow
	colorBlue
	colorMagenta
	colorCyan
	colorWhite
	colorBold     = 1
	colorDarkGray = 90
)

func DefaultTimeEncoder(format string) func(time.Time, zapcore.PrimitiveArrayEncoder) {
	return func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
		buf := _bufferPoolGet()
		colorize(buf, t.Format(format), strconv.Itoa(colorDarkGray))
		enc.AppendString(buf.String())
		buf.Free()
	}
}

func defaultDurationEncoder(dur time.Duration, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(dur.String())
}

const defaultColourOffset = 2

var defaultColours = [10][]string{
	// DIY trace level
	zapcore.DebugLevel - 1 + defaultColourOffset: {strconv.Itoa(colorDarkGray)},
	zapcore.DebugLevel + defaultColourOffset:     {strconv.Itoa(colorCyan)},
	zapcore.InfoLevel + defaultColourOffset:      {strconv.Itoa(colorGreen)},
	zapcore.WarnLevel + defaultColourOffset:      {strconv.Itoa(colorYellow)},
	zapcore.ErrorLevel + defaultColourOffset:     {strconv.Itoa(colorRed)},
	zapcore.FatalLevel + defaultColourOffset:     {strconv.Itoa(colorRed), strconv.Itoa(colorBold)},
	zapcore.DPanicLevel + defaultColourOffset:    {strconv.Itoa(colorRed), strconv.Itoa(colorBold)},
	zapcore.PanicLevel + defaultColourOffset:     {strconv.Itoa(colorRed), strconv.Itoa(colorBold)},
}

func defaultLevelEncoder(l zapcore.Level, enc zapcore.PrimitiveArrayEncoder) {
	var str string

	switch l {
	// DIY trace level
	case zapcore.DebugLevel - 1:
		str = "TRC"
	case zapcore.DebugLevel:
		str = "DBG"
	case zapcore.InfoLevel:
		str = "INF"
	case zapcore.WarnLevel:
		str = "WRN"
	case zapcore.ErrorLevel:
		str = "ERR"
	case zapcore.FatalLevel:
		str = "FTL"
	case zapcore.DPanicLevel:
		str = "DPNC"
	case zapcore.PanicLevel:
		str = "PNC"
	default:
		l = zapcore.PanicLevel
		str = "???"
	}

	buf := _bufferPoolGet()
	colorize(buf, str, defaultColours[l+defaultColourOffset]...)
	enc.AppendString(buf.String())
	buf.Free()
}

func defaultCallerEncoder(caller zapcore.EntryCaller, enc zapcore.PrimitiveArrayEncoder) {
	callerFullPath := caller.FullPath()

	var str string
	if cwd, err := os.Getwd(); err == nil {
		if rel, err := filepath.Rel(cwd, callerFullPath); err == nil {
			str = rel
		}
	}
	if str == "" {
		// May have been built with -trimpath which will cause paths to be
		// package paths instead of file paths try trimming the main module
		// path else this will fall back to the full path
		str = callerFullPath
		if buildInfo, ok := debug.ReadBuildInfo(); ok {
			str = strings.TrimPrefix(str, buildInfo.Main.Path+"/")
		}
	}

	buf := _bufferPoolGet()
	colorize(buf, str, strconv.Itoa(colorBold))
	enc.AppendString(buf.String())
	buf.Free()
}

func defaultNameEncoder(name string, enc zapcore.PrimitiveArrayEncoder) {
	buf := _bufferPoolGet()
	colorize(buf, name, strconv.Itoa(colorBold))
	enc.AppendString(buf.String())
	buf.Free()
}

var reflectedListBreakSize = map[interface{}]int{
	new(byte): 16, reflect.TypeOf(*new(byte)): 16,
	new(bool): 8, *new(bool): 8,
	new(complex64): 8, *new(complex64): 8,
	new(time.Duration): 8, *new(time.Duration): 8,
	new(float32): 8, *new(float32): 8,
	new(float64): 8, *new(float64): 8,
	new(int): 8, *new(int): 8,
	new(int16): 8, *new(int16): 8,
	new(int32): 8, *new(int32): 8,
	new(int64): 8, *new(int64): 8,
	new(int16): 16, *new(int16): 16,
	new(string): 8, *new(string): 8,
	new(time.Time): 8, *new(time.Time): 8,
	new(uint): 8, *new(uint): 8,
	new(uint16): 8, *new(uint16): 8,
	new(uint32): 8, *new(uint32): 8,
	new(uint64): 8, *new(uint64): 8,
	new(uint16): 16, *new(uint16): 16,
}

// formatByteArrayAsHex converts a byte slice to a lowercase hex string with quotes.
// Used for formatting fixed-size byte arrays as compact hex strings.
func formatByteArrayAsHex(bytes []byte) string {
	var hexStr strings.Builder
	hexStr.WriteString("\"")
	for _, b := range bytes {
		// Format each byte as 2-character hex with leading zero if needed
		if b < 16 {
			hexStr.WriteByte('0')
		}
		hexStr.WriteString(strconv.FormatUint(uint64(b), 16))
	}
	hexStr.WriteString("\"")
	return hexStr.String()
}

func defaultReflectedEncoder(w io.Writer) zapcore.ReflectedEncoder {
	opts := make([]dd.OptionFunc, 0, len(reflectedListBreakSize)+7)
	for key, val := range reflectedListBreakSize {
		opts = append(opts, dd.WithListBreakLineSize(key, val))
	}
	opts = append(opts, df.WithTime(time.RFC3339))
	opts = append(opts, df.WithRichBytes())

	// Add custom formatters for fixed-size byte arrays
	// Note: Due to Go's type system, each [N]byte is a distinct type, so we need
	// separate WithDumpFunc calls for each size. We cannot create a single generic
	// formatter for "any byte array of any size" because:
	// - Go generics don't support array size as a type parameter
	// - WithDumpFunc uses reflect.TypeOf(v) to register formatters by exact type
	// The sizes below cover the most common use cases:

	// [4]byte - CRC32 checksums, IPv4 addresses
	opts = append(opts, dd.WithDumpFunc(func(v [4]byte, w dd.Writer) {
		w.Write(formatByteArrayAsHex(v[:]))
	}))

	// [8]byte - OpenTelemetry SpanID, uint64 as bytes
	opts = append(opts, dd.WithDumpFunc(func(v [8]byte, w dd.Writer) {
		w.Write(formatByteArrayAsHex(v[:]))
	}))

	// [16]byte - OpenTelemetry TraceID, UUID, MD5 hashes
	opts = append(opts, dd.WithDumpFunc(func(v [16]byte, w dd.Writer) {
		w.Write(formatByteArrayAsHex(v[:]))
	}))

	// [32]byte - SHA256 hashes, Ed25519 public keys
	opts = append(opts, dd.WithDumpFunc(func(v [32]byte, w dd.Writer) {
		w.Write(formatByteArrayAsHex(v[:]))
	}))

	// [64]byte - SHA512 hashes, Ed25519 signatures
	opts = append(opts, dd.WithDumpFunc(func(v [64]byte, w dd.Writer) {
		w.Write(formatByteArrayAsHex(v[:]))
	}))

	return ddEncoder{w: w, opts: opts}
}

type ddEncoder struct {
	w    io.Writer
	opts []dd.OptionFunc
}

func (d ddEncoder) Encode(i interface{}) error {
	_, err := d.w.Write([]byte(dd.Dump(i, d.opts...)))
	return err
}

// colorize returns the string s wrapped in ANSI code c
func colorize(buf *buffer.Buffer, s string, cols ...string) {
	for _, col := range cols {
		buf.AppendString("\x1b[" + col + "m")
	}
	buf.AppendString(s)
	buf.AppendString("\x1b[0m")
}
