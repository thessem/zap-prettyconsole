package prettyconsole

import (
	"os"
	"sort"
	"strings"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/buffer"
	"go.uber.org/zap/zapcore"
)

var _ = zap.RegisterEncoder("pretty_console", func(ec zapcore.EncoderConfig) (zapcore.Encoder, error) {
	return NewEncoder(ec), nil
})

func NewConfig() zap.Config {
	return zap.Config{
		Level:            zap.NewAtomicLevelAt(zapcore.DebugLevel),
		Development:      true,
		Encoding:         "pretty_console",
		EncoderConfig:    NewEncoderConfig(),
		OutputPaths:      []string{"stderr"},
		ErrorOutputPaths: []string{"stderr"},
	}
}

func NewEncoder(cfg zapcore.EncoderConfig) zapcore.Encoder {
	// Like zapcore's encoders, treat an unset line ending as the default:
	// it is also used internally to lay out namespaces and indents.
	if cfg.LineEnding == "" {
		cfg.LineEnding = zapcore.DefaultLineEnding
	}
	return &recordingEncoder{e: prettyConsoleEncoder{
		buf:             nil,
		cfg:             &cfg,
		level:           0,
		namespaceIndent: 0,
		inList:          false,

		_listSepComma: "," + cfg.ConsoleSeparator,
		_listSepSpace: cfg.ConsoleSeparator,
		listSep:       cfg.ConsoleSeparator,
		listSepIndent: -1,
	}}
}

func NewEncoderConfig() zapcore.EncoderConfig {
	return zapcore.EncoderConfig{
		MessageKey:          "M",
		LevelKey:            "L",
		TimeKey:             "T",
		NameKey:             "N",
		CallerKey:           zapcore.OmitKey,
		FunctionKey:         zapcore.OmitKey,
		StacktraceKey:       "S",
		SkipLineEnding:      false,
		LineEnding:          zapcore.DefaultLineEnding,
		EncodeLevel:         defaultLevelEncoder,
		EncodeTime:          DefaultTimeEncoder(time.Kitchen),
		EncodeDuration:      defaultDurationEncoder,
		EncodeCaller:        defaultCallerEncoder,
		EncodeName:          defaultNameEncoder,
		NewReflectedEncoder: defaultReflectedEncoder,
		ConsoleSeparator:    " ",
	}
}

func NewLogger(lvl zapcore.Level) *zap.Logger {
	ec := NewEncoderConfig()
	enc := NewEncoder(ec)
	return zap.New(zapcore.NewCore(
		enc,
		os.Stdout,
		lvl,
	))
}

// Test interface conformance
var _ zapcore.Encoder = (*prettyConsoleEncoder)(nil)

type prettyConsoleEncoder struct {
	buf *buffer.Buffer

	cfg   *zapcore.EncoderConfig
	level zapcore.Level

	namespaceIndent int
	inList          bool
	listSep         string
	// listSepIndent >= 0 means the separator is a line break followed by
	// that many spaces (built without allocating); -1 means use listSep.
	listSepIndent int
	keyPrefix     string

	_listSepComma string
	_listSepSpace string
}

// Clone implements zapcore.Encoder
func (e prettyConsoleEncoder) Clone() zapcore.Encoder {
	clone := e.clone()
	_, _ = clone.buf.Write(e.buf.Bytes())
	return clone
}

func (e prettyConsoleEncoder) clone() *prettyConsoleEncoder {
	clone := getPrettyConsoleEncoder()
	clone.buf = getBuffer()

	clone.cfg = e.cfg
	clone.level = e.level

	clone.namespaceIndent = e.namespaceIndent
	clone.inList = e.inList
	clone.listSep = e.listSep
	clone.listSepIndent = e.listSepIndent
	clone.keyPrefix = e.keyPrefix

	clone._listSepComma = e._listSepComma
	clone._listSepSpace = e._listSepSpace

	return clone
}

func (e prettyConsoleEncoder) EncodeEntry(entry zapcore.Entry, fields []zapcore.Field) (*buffer.Buffer, error) {
	e.buf = getBuffer()
	e.level = entry.Level

	raw := rawStringAppender{&e}

	// Add preamble
	if e.cfg.TimeKey != "" && e.cfg.EncodeTime != nil {
		e.cfg.EncodeTime(entry.Time, raw)
	}
	if e.cfg.LevelKey != "" && e.cfg.EncodeLevel != nil {
		e.cfg.EncodeLevel(entry.Level, raw)
	}
	if entry.LoggerName != "" && e.cfg.NameKey != "" && e.cfg.EncodeName != nil {
		e.cfg.EncodeName(entry.LoggerName, raw)
	}
	if entry.Caller.Defined {
		if e.cfg.CallerKey != "" && e.cfg.EncodeCaller != nil {
			e.cfg.EncodeCaller(entry.Caller, raw)
		}
		if e.cfg.FunctionKey != "" {
			raw.AppendString(entry.Caller.Function)
		}
	}
	e.addSeparator()
	e.buf.AppendString(ansiBold)
	e.colorizeAtLevel(">")
	e.buf.AppendString(ansiReset)
	e.inList = true

	// Add the message itself.
	if entry.Message != "" && e.cfg.MessageKey != "" {
		e.addSeparator()
		e.addSafeString(entry.Message)
		e.inList = true
	}

	// We are sorting all field keys alphabetically, except pushing multi-line
	// stuff (array, reflect, object, error in that order) to the back.
	//
	// Additionally we are only sorting within namespace boundaries, as we don't
	// want to re-order namespaces and destroy that structural information.
	prev := 0
	sortFunc := func(ii, jj int) bool {
		ii += prev
		jj += prev
		if fields[ii].Type == fields[jj].Type {
			return fields[ii].Key < fields[jj].Key
		}
		switch fields[ii].Type {
		case zapcore.ArrayMarshalerType:
			return fields[jj].Type == zapcore.ReflectType || fields[jj].Type == zapcore.ObjectMarshalerType || fields[jj].Type == zapcore.ErrorType
		case zapcore.ReflectType:
			return fields[jj].Type == zapcore.ObjectMarshalerType || fields[jj].Type == zapcore.ErrorType
		case zapcore.ObjectMarshalerType:
			return fields[jj].Type == zapcore.ErrorType
		case zapcore.ErrorType:
			return false
		}
		switch fields[jj].Type {
		case zapcore.ArrayMarshalerType, zapcore.ReflectType, zapcore.ObjectMarshalerType, zapcore.ErrorType:
			return true
		default:
			return fields[ii].Key < fields[jj].Key
		}
	}
	for idx, field := range fields {
		if field.Type == zapcore.NamespaceType {
			if idx-prev > 1 {
				sort.Slice(fields[prev:idx], sortFunc)
			}
			prev = idx + 1
		} else if idx == len(fields)-1 {
			if idx+1-prev > 1 {
				sort.Slice(fields[prev:idx+1], sortFunc)
			}
		}
	}

	// Write the fields
	for _, f := range fields {
		if f.Type == zapcore.ErrorType {
			if err := e.encodeError(f.Key, f.Interface.(error)); err != nil {
				_ = e.encodeError(f.Key+"_PANIC_DISPLAYING_ERROR", err)
			}
			e.inList = false
		} else {
			f.AddTo(&e)
		}
	}

	// Write the stacktrace
	if entry.Stack != "" && e.cfg.StacktraceKey != "" {
		e.namespaceIndent = 0
		e.OpenNamespace("")
		e.namespaceIndent += len("stacktrace=")
		e.keyPrefix = ""
		e.addIndentedString("stacktrace", strings.TrimPrefix(entry.Stack, "\n"))
	}

	// We're done :)
	if !e.cfg.SkipLineEnding {
		e.buf.AppendString(e.cfg.LineEnding)
	}

	return e.buf, nil
}

func (e *prettyConsoleEncoder) addSeparator() {
	if !e.inList {
		return
	}
	if e.listSepIndent >= 0 {
		// Line-break separator: coloured line ending plus indentation,
		// written without building an intermediate string.
		e.buf.AppendString(levelColourPrefix(e.level))
		e.buf.AppendString(e.cfg.LineEnding)
		appendSpaces(e.buf, e.listSepIndent)
		e.buf.AppendString(ansiReset)
		return
	}
	e.colorizeAtLevel(e.listSep)
}

// setListSep selects a plain-string separator for the next element.
func (e *prettyConsoleEncoder) setListSep(s string) {
	e.listSep = s
	e.listSepIndent = -1
}

// setIndentSep makes the next separator a line break plus the current
// namespace indentation.
func (e *prettyConsoleEncoder) setIndentSep() {
	e.listSepIndent = e.namespaceIndent
}

const manySpaces = "                                                                " // 64 spaces

// appendSpaces appends n spaces in large chunks.
func appendSpaces(buf *buffer.Buffer, n int) {
	for n > len(manySpaces) {
		buf.AppendString(manySpaces)
		n -= len(manySpaces)
	}
	if n > 0 {
		buf.AppendString(manySpaces[:n])
	}
}

func (e *prettyConsoleEncoder) addKey(key string) {
	e.colorizeAtLevel(e.keyPrefix + key + "=")
}

// colorize returns the string s wrapped in ANSI code c, coloured properly for
// the logging level we're at.
func (e *prettyConsoleEncoder) colorizeAtLevel(s string) {
	e.buf.AppendString(levelColourPrefix(e.level))
	e.buf.AppendString(s)
	e.buf.AppendString(ansiReset)
}

const (
	ansiReset = "\x1b[0m"
	ansiBold  = "\x1b[1m"
)

// levelColourPrefixes holds the full ANSI prefix for each level, so the
// hot path appends one precomputed string instead of assembling codes.
var levelColourPrefixes = func() (p [len(defaultColours)]string) {
	for i, cols := range defaultColours {
		for _, c := range cols {
			p[i] += "\x1b[" + c + "m"
		}
	}
	return p
}()

// levelColourPrefix returns the ANSI prefix for a level, treating levels
// outside the known range like defaultLevelEncoder treats them: as panics.
// zapcore.Level is an int8, so custom levels must not crash the encoder.
func levelColourPrefix(l zapcore.Level) string {
	idx := int(l) + defaultColourOffset
	if idx < 0 || idx >= len(defaultColours) || defaultColours[idx] == nil {
		idx = int(zapcore.PanicLevel) + defaultColourOffset
	}
	return levelColourPrefixes[idx]
}

// rawStringAppender will append strings without escaping them,
type rawStringAppender struct{ *prettyConsoleEncoder }

func (e rawStringAppender) AppendString(s string) {
	e.addSeparator()
	e.buf.AppendString(s)
	e.inList = true
}
