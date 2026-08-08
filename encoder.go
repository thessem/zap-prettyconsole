package prettyconsole

import (
	"os"
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
	// Work on a pooled encoder: the preamble callbacks take the encoder
	// through an interface, which would force this stack copy to escape
	// to the heap on every entry.
	enc := getPrettyConsoleEncoder()
	*enc = e
	enc.buf = getBuffer()
	enc.level = entry.Level
	enc.encodePreamble(entry)
	sortFieldSegments(fields)
	enc.encodeFields(fields)
	enc.encodeFinish(entry)
	buf := enc.buf
	enc.buf = nil
	putPrettyConsoleEncoder(enc)
	return buf, nil
}

// encodePreamble writes the time/level/name/caller preamble and message,
// leaving the encoder ready for fields (inList set, default separator).
func (e *prettyConsoleEncoder) encodePreamble(entry zapcore.Entry) {
	raw := rawStringAppender{e}

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

	if entry.Message != "" && e.cfg.MessageKey != "" {
		e.addSeparator()
		e.addSafeString(entry.Message)
		e.inList = true
	}
}

// fieldLess orders fields alphabetically by key, except pushing multi-line
// types (array, reflect, object, error in that order) to the back.
func fieldLess(a, b *zapcore.Field) bool {
	if a.Type == b.Type {
		return a.Key < b.Key
	}
	switch a.Type {
	case zapcore.ArrayMarshalerType:
		return b.Type == zapcore.ReflectType || b.Type == zapcore.ObjectMarshalerType || b.Type == zapcore.ErrorType
	case zapcore.ReflectType:
		return b.Type == zapcore.ObjectMarshalerType || b.Type == zapcore.ErrorType
	case zapcore.ObjectMarshalerType:
		return b.Type == zapcore.ErrorType
	case zapcore.ErrorType:
		return false
	}
	switch b.Type {
	case zapcore.ArrayMarshalerType, zapcore.ReflectType, zapcore.ObjectMarshalerType, zapcore.ErrorType:
		return true
	default:
		return a.Key < b.Key
	}
}

// sortFieldSegments sorts fields with fieldLess within namespace
// boundaries: namespaces are never re-ordered, as that would destroy
// structural information. Insertion sort is used because field counts are
// small, it allocates nothing, and it is O(n) on the already-sorted
// prefixes the recording encoder prepares.
func sortFieldSegments(fields []zapcore.Field) {
	prev := 0
	for idx := range fields {
		if fields[idx].Type == zapcore.NamespaceType {
			insertionSortFields(fields[prev:idx])
			prev = idx + 1
		}
	}
	insertionSortFields(fields[prev:])
}

func insertionSortFields(fs []zapcore.Field) {
	for i := 1; i < len(fs); i++ {
		for j := i; j > 0 && fieldLess(&fs[j], &fs[j-1]); j-- {
			fs[j], fs[j-1] = fs[j-1], fs[j]
		}
	}
}

// encodeFields writes already-sorted fields.
func (e *prettyConsoleEncoder) encodeFields(fields []zapcore.Field) {
	for i := range fields {
		if fields[i].Type == zapcore.ErrorType {
			if err := e.encodeError(fields[i].Key, fields[i].Interface.(error)); err != nil {
				_ = e.encodeError(fields[i].Key+"_PANIC_DISPLAYING_ERROR", err)
			}
			e.inList = false
		} else {
			fields[i].AddTo(e)
		}
	}
}

// encodeFinish writes the stacktrace and line ending.
func (e *prettyConsoleEncoder) encodeFinish(entry zapcore.Entry) {
	if entry.Stack != "" && e.cfg.StacktraceKey != "" {
		e.namespaceIndent = 0
		e.OpenNamespace("")
		e.namespaceIndent += len("stacktrace=")
		e.keyPrefix = ""
		e.addIndentedString("stacktrace", strings.TrimPrefix(entry.Stack, "\n"))
	}
	if !e.cfg.SkipLineEnding {
		e.buf.AppendString(e.cfg.LineEnding)
	}
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

// colourIdx maps a level to its colour-table index, treating levels
// outside the known range like defaultLevelEncoder treats them: as panics.
// zapcore.Level is an int8, so custom levels must not crash the encoder.
func colourIdx(l zapcore.Level) int {
	idx := int(l) + defaultColourOffset
	if idx < 0 || idx >= len(defaultColours) || defaultColours[idx] == nil {
		idx = int(zapcore.PanicLevel) + defaultColourOffset
	}
	return idx
}

// levelColourPrefix returns the precomputed ANSI prefix for a level.
func levelColourPrefix(l zapcore.Level) string {
	return levelColourPrefixes[colourIdx(l)]
}

// rawStringAppender will append strings without escaping them,
type rawStringAppender struct{ *prettyConsoleEncoder }

func (e rawStringAppender) AppendString(s string) {
	e.addSeparator()
	e.buf.AppendString(s)
	e.inList = true
}
