package prettyconsole

import (
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"time"

	"github.com/Code-Hex/dd"
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
	var str string
	if cwd, err := os.Getwd(); err == nil {
		if rel, _ := filepath.Rel(cwd, caller.FullPath()); err == nil {
			str = rel
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

func defaultReflectedEncoder(w io.Writer) zapcore.ReflectedEncoder {
	opts := make([]dd.OptionFunc, 0, len(reflectedListBreakSize))
	for key, val := range reflectedListBreakSize {
		opts = append(opts, dd.WithListBreakLineSize(key, val))
	}
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
