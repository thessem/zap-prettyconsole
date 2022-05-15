package prettyconsole

import (
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"time"

	"github.com/Code-Hex/dd"
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
		enc.AppendString(colorize(t.Format(format), colorDarkGray))
	}
}

func defaultDurationEncoder(dur time.Duration, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(dur.String())
}

var defaultColours = map[zapcore.Level]func(string) string{
	// DIY trace level
	zapcore.DebugLevel - 1: func(s string) string { return colorize(s, colorDarkGray) },
	zapcore.DebugLevel:     func(s string) string { return colorize(s, colorCyan) },
	zapcore.InfoLevel:      func(s string) string { return colorize(s, colorGreen) },
	zapcore.WarnLevel:      func(s string) string { return colorize(s, colorYellow) },
	zapcore.ErrorLevel:     func(s string) string { return colorize(s, colorRed) },
	zapcore.FatalLevel:     func(s string) string { return colorize(colorize(s, colorRed), colorBold) },
	zapcore.DPanicLevel:    func(s string) string { return colorize(colorize(s, colorRed), colorBold) },
	zapcore.PanicLevel:     func(s string) string { return colorize(colorize(s, colorRed), colorBold) },
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
	enc.AppendString(defaultColours[l](str))
}

func defaultCallerEncoder(caller zapcore.EntryCaller, enc zapcore.PrimitiveArrayEncoder) {
	var str string
	if cwd, err := os.Getwd(); err == nil {
		if rel, _ := filepath.Rel(cwd, caller.FullPath()); err == nil {
			str = rel
		}
	}
	enc.AppendString(colorize(str, colorBold))
}

func defaultNameEncoder(name string, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(colorize(name, colorBold))
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
func colorize(s string, col int) string {
	return "\x1b[" + strconv.Itoa(col) + "m" + s + "\x1b[0m"
}
