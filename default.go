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
		buf.AppendString(ansiDarkGray)
		buf.AppendTime(t, format)
		buf.AppendString(ansiReset)
		enc.AppendString(buf.String())
		buf.Free()
	}
}

const ansiDarkGray = "\x1b[90m"

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

// defaultLevelLabels holds the fully coloured label for each known level,
// so encoding a level is a single precomputed append.
var defaultLevelLabels = func() (labels [len(defaultColours)]string) {
	names := map[zapcore.Level]string{
		zapcore.DebugLevel - 1: "TRC", // DIY trace level
		zapcore.DebugLevel:     "DBG",
		zapcore.InfoLevel:      "INF",
		zapcore.WarnLevel:      "WRN",
		zapcore.ErrorLevel:     "ERR",
		zapcore.FatalLevel:     "FTL",
		zapcore.DPanicLevel:    "DPNC",
		zapcore.PanicLevel:     "PNC",
	}
	for l, name := range names {
		labels[l+defaultColourOffset] = levelColourPrefixes[l+defaultColourOffset] + name + ansiReset
	}
	return labels
}()

var unknownLevelLabel = levelColourPrefixes[zapcore.PanicLevel+defaultColourOffset] + "???" + ansiReset

func defaultLevelEncoder(l zapcore.Level, enc zapcore.PrimitiveArrayEncoder) {
	if idx := int(l) + defaultColourOffset; idx >= 0 && idx < len(defaultLevelLabels) && defaultLevelLabels[idx] != "" {
		enc.AppendString(defaultLevelLabels[idx])
		return
	}
	enc.AppendString(unknownLevelLabel)
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
