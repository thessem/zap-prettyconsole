package prettyconsole

import (
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

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

func defaultReflectedEncoder(w io.Writer) zapcore.ReflectedEncoder {
	return dumpEncoder{w: w}
}

type dumpEncoder struct {
	w io.Writer
}

func (d dumpEncoder) Encode(i interface{}) error {
	return dumpValue(d.w, i)
}

// colorize returns the string s wrapped in ANSI code c
func colorize(buf *buffer.Buffer, s string, cols ...string) {
	for _, col := range cols {
		buf.AppendString("\x1b[" + col + "m")
	}
	buf.AppendString(s)
	buf.AppendString("\x1b[0m")
}
