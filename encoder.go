package prettyconsole

import (
	"bytes"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

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
	return &recordingEncoder{e: prettyConsoleEncoder{
		buf:             nil,
		cfg:             &cfg,
		level:           0,
		namespaceIndent: 0,
		inList:          false,

		_listSepComma: "," + cfg.ConsoleSeparator,
		_listSepSpace: cfg.ConsoleSeparator,
		listSep:       cfg.ConsoleSeparator,
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
	keyPrefix       string

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
	// Kinda going about making it bold the long way here I imagine
	e.buf.AppendString("\x1b[")
	e.buf.AppendString(strconv.Itoa(colorBold))
	e.buf.AppendString("m")
	e.colorizeAtLevel(">")
	e.buf.AppendString("\x1b[0m")
	e.inList = true

	// Add the message itself.
	if entry.Message != "" && e.cfg.MessageKey != "" {
		e.addSeparator()
		e.appendSafeByte([]byte(entry.Message))
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
			sort.Slice(fields[prev:idx], sortFunc)
			prev = idx + 1
		} else if idx == len(fields)-1 {
			sort.Slice(fields[prev:idx+1], sortFunc)
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
	e.buf.AppendString(e.cfg.LineEnding)

	return e.buf, nil
}

func (e *prettyConsoleEncoder) addSeparator() {
	if e.inList {
		e.colorizeAtLevel(e.listSep)
		return
	}
}

func (e *prettyConsoleEncoder) addKey(key string) {
	e.colorizeAtLevel(e.keyPrefix + key + "=")
}

// addSafeString JSON-escapes a string and appends it to the internal buffer.
func (e *prettyConsoleEncoder) addSafeString(s string) {
	for i := 0; i < len(s); {
		if e.tryAddRune(s[i]) {
			i++
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		if e.tryAddRuneError(r, size) {
			i++
			continue
		}
		e.buf.AppendString(s[i : i+size])
		i += size
	}
}

// appendSafeByte is no-alloc equivalent of addSafeString(string(s)) for s
// []byte.
func (e *prettyConsoleEncoder) appendSafeByte(s []byte) {
	for i := 0; i < len(s); {
		if e.tryAddRune(s[i]) {
			i++
			continue
		}
		r, size := utf8.DecodeRune(s[i:])
		if e.tryAddRuneError(r, size) {
			i++
			continue
		}
		e.buf.Write(s[i : i+size])
		i += size
	}
}

// tryAddRune appends b if it is valid UTF-8 character represented in a
// single byte.
func (e *prettyConsoleEncoder) tryAddRune(b byte) bool {
	const _hex = "0123456789abcdef"

	if b >= utf8.RuneSelf {
		return false
	}
	if 0x20 <= b && b != '\\' && b != '"' {
		e.buf.AppendByte(b)
		return true
	}
	switch b {
	case '\\', '"':
		e.colorizeAtLevel("\\" + string(b))
	case '\n':
		e.colorizeAtLevel("\\n")
	case '\r':
		e.colorizeAtLevel("\\r")
	case '\t':
		e.colorizeAtLevel("\\t")
	default:
		// Encode bytes < 0x20, except for the escape sequences above.
		e.colorizeAtLevel(`\u00`)
		e.colorizeAtLevel(string(_hex[b>>4]))
		e.colorizeAtLevel(string(_hex[b&0xF]))
	}
	return true
}

func (e *prettyConsoleEncoder) tryAddRuneError(r rune, size int) bool {
	if r == utf8.RuneError && size == 1 {
		e.buf.AppendString(`\ufffd`)
		return true
	}
	return false
}

// colorize returns the string s wrapped in ANSI code c, coloured properly for
// the logging level we're at.
func (e *prettyConsoleEncoder) colorizeAtLevel(s string) {
	colorize(e.buf, s, defaultColours[e.level+defaultColourOffset]...)
}

// rawStringAppender will append strings without escaping them,
type rawStringAppender struct{ *prettyConsoleEncoder }

func (e rawStringAppender) AppendString(s string) {
	e.addSeparator()
	e.buf.AppendString(s)
	e.inList = true
}

type indentingWriter struct {
	buf        io.Writer
	indent     int
	lineEnding []byte
}

func (i indentingWriter) Write(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}
	idx := bytes.IndexByte(p, '\n')
	if idx == -1 {
		return i.buf.Write(p)
	}
	written, _ := i.buf.Write(p[0:idx])
	read := written
	n, _ = i.buf.Write(i.lineEnding)
	written += n
	read += 1
	for read <= len(p) {
		for ii := 0; ii < i.indent; ii++ {
			n, _ := i.buf.Write([]byte(" "))
			written += n
		}
		if read == len(p) {
			return written, nil
		}
		idx = bytes.IndexByte(p[read:], '\n')
		if idx == -1 {
			n, _ := i.buf.Write(p[read:])
			return written + n, nil
		}
		n, _ = i.buf.Write(p[read : read+idx])
		written += n
		read += n
		n, _ = i.buf.Write(i.lineEnding)
		written += n
		read += 1
	}
	return written, nil
}
