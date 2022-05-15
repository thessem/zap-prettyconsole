package prettyconsole

import (
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"go.uber.org/zap"
	"go.uber.org/zap/buffer"
	"go.uber.org/zap/zapcore"
)

var _ = zap.RegisterEncoder("pretty_console", NewEncoder)

func NewEncoder(cfg zapcore.EncoderConfig) (zapcore.Encoder, error) {
	return &prettyConsoleEncoder{
		buf:             _bufferPoolGet(),
		cfg:             &cfg,
		level:           0,
		namespaceIndent: 0,
		inList:          false,
	}, nil
}

func NewEncoderConfig() zapcore.EncoderConfig {
	return zapcore.EncoderConfig{
		MessageKey:          "M",
		LevelKey:            "L",
		TimeKey:             "T",
		NameKey:             "N",
		CallerKey:           "C",
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
}

func (e *prettyConsoleEncoder) Clone() zapcore.Encoder {
	clone := e.clone()
	_, _ = clone.buf.Write(e.buf.Bytes())
	return clone
}

func (e *prettyConsoleEncoder) clone() *prettyConsoleEncoder {
	clone := getPrettyConsoleEncoder()
	clone.buf = getBuffer()

	clone.cfg = e.cfg
	clone.level = e.level

	clone.namespaceIndent = e.namespaceIndent
	clone.inList = e.inList
	clone.listSep = e.listSep
	clone.keyPrefix = e.keyPrefix

	return clone
}

func (e *prettyConsoleEncoder) EncodeEntry(entry zapcore.Entry, fields []zapcore.Field) (*buffer.Buffer, error) {
	enc := e.clone()
	enc.listSep = enc.cfg.ConsoleSeparator
	enc.level = entry.Level
	raw := rawStringAppender{enc}

	// Add preamble
	if enc.cfg.TimeKey != "" && enc.cfg.EncodeTime != nil {
		enc.cfg.EncodeTime(entry.Time, raw)
	}
	if enc.cfg.LevelKey != "" && enc.cfg.EncodeLevel != nil {
		enc.cfg.EncodeLevel(entry.Level, raw)
	}
	if entry.LoggerName != "" && enc.cfg.NameKey != "" && enc.cfg.EncodeName != nil {
		enc.cfg.EncodeName(entry.LoggerName, raw)
	}
	if entry.Caller.Defined {
		if enc.cfg.CallerKey != "" && enc.cfg.EncodeCaller != nil {
			enc.cfg.EncodeCaller(entry.Caller, raw)
		}
		if enc.cfg.FunctionKey != "" {
			raw.AppendString(entry.Caller.Function)
		}
	}
	raw.AppendString(colorize(enc.colorizeAtLevel(">"), colorBold))

	// Add the message itself.
	if entry.Message != "" && enc.cfg.MessageKey != "" {
		enc.addSeparator()
		enc.appendSafeByte([]byte(entry.Message))
		enc.inList = true
	}

	// Slice apart slices by namespaces so we're sorting within namespaces, so the Namespace fields retain their relative position
	var fieldss [][]zapcore.Field
	i := 0
	for idx, val := range fields {
		if val.Type == zapcore.NamespaceType {
			fieldss = append(fieldss, fields[i:idx])
			i = idx
		}
	}
	if i < len(fields) {
		fieldss = append(fieldss, fields[i:len(fields)])
	}

	// Sort the fields lexically, but move arrays, reflects, objects and errors
	// to the back (in that order)
	for _, fields := range fieldss {
		sort.Slice(fields, func(ii, jj int) bool {
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
		})
	}

	// Write the fields
	for _, fs := range fieldss {
		for _, f := range fs {
			if f.Type == zapcore.ErrorType {
				if err := enc.encodeError(f.Key, f.Interface.(error)); err != nil {
					_ = enc.encodeError(f.Key+"_PANIC_DISPLAYING_ERROR", err)
				}
				enc.inList = false
			} else {
				f.AddTo(enc)
			}
		}
	}

	// Write the stacktrace
	if entry.Stack != "" && enc.cfg.StacktraceKey != "" {
		enc.OpenNamespace("")
		enc.namespaceIndent += len("stacktrace=")
		enc.keyPrefix = ""
		enc.addIndentedString("stacktrace", strings.TrimPrefix(entry.Stack, "\n"))
	}

	// Make a (shallow) copy of buffer so encoder can be freed
	buf := enc.buf
	putPrettyConsoleEncoder(enc)

	return buf, nil
}

func (e *prettyConsoleEncoder) addSeparator() {
	if e.inList {
		e.buf.AppendString(e.colorizeAtLevel(e.listSep))
		return
	}
}

func (e *prettyConsoleEncoder) addKey(key string) {
	e.buf.AppendString(e.colorizeAtLevel(e.keyPrefix))
	e.buf.AppendString(e.colorizeAtLevel(key + "="))
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
		e.buf.AppendString(e.colorizeAtLevel("\\" + string(b)))
	case '\n':
		e.buf.AppendString(e.colorizeAtLevel("\\n"))
	case '\r':
		e.buf.AppendString(e.colorizeAtLevel("\\r"))
	case '\t':
		e.buf.AppendString(e.colorizeAtLevel("\\t"))
	default:
		// Encode bytes < 0x20, except for the escape sequences above.
		e.buf.AppendString(e.colorizeAtLevel(`\u00`))
		e.buf.AppendString(e.colorizeAtLevel(string(_hex[b>>4])))
		e.buf.AppendString(e.colorizeAtLevel(string(_hex[b&0xF])))
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
func (e *prettyConsoleEncoder) colorizeAtLevel(s string) string {
	return defaultColours[e.level](s)
}

// rawStringAppender will append strings without escaping them,
type rawStringAppender struct{ *prettyConsoleEncoder }

func (e rawStringAppender) AppendString(s string) {
	e.addSeparator()
	e.buf.AppendString(s)
	e.inList = true
}
