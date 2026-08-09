package prettyconsole

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Test interface conformance
var _ zapcore.ObjectEncoder = (*prettyConsoleEncoder)(nil)

func (e *prettyConsoleEncoder) AddFloat32(k string, v float32) { e.addFloat(k, float64(v), 32) }
func (e *prettyConsoleEncoder) AddFloat64(k string, v float64) { e.addFloat(k, v, 64) }
func (e *prettyConsoleEncoder) AddInt(k string, v int)         { e.AddInt64(k, int64(v)) }
func (e *prettyConsoleEncoder) AddInt32(k string, v int32)     { e.AddInt64(k, int64(v)) }
func (e *prettyConsoleEncoder) AddInt16(k string, v int16)     { e.AddInt64(k, int64(v)) }
func (e *prettyConsoleEncoder) AddInt8(k string, v int8)       { e.AddInt64(k, int64(v)) }
func (e *prettyConsoleEncoder) AddUint(k string, v uint)       { e.AddUint64(k, uint64(v)) }
func (e *prettyConsoleEncoder) AddUint32(k string, v uint32)   { e.AddUint64(k, uint64(v)) }
func (e *prettyConsoleEncoder) AddUint16(k string, v uint16)   { e.AddUint64(k, uint64(v)) }
func (e *prettyConsoleEncoder) AddUint8(k string, v uint8)     { e.AddUint64(k, uint64(v)) }
func (e *prettyConsoleEncoder) AddUintptr(k string, v uintptr) { e.AddUint64(k, uint64(v)) }
func (e *prettyConsoleEncoder) AddBinary(key string, value []byte) {
	e.addSeparator()
	e.addKey(key)
	// The base64 alphabet needs no escaping, so write it directly - via a
	// stack buffer for the common small case.
	if n := base64.StdEncoding.EncodedLen(len(value)); n <= 64 {
		var arr [64]byte
		base64.StdEncoding.Encode(arr[:n], value)
		_, _ = e.buf.Write(arr[:n])
	} else {
		_, _ = e.buf.Write(base64.StdEncoding.AppendEncode(nil, value))
	}
	e.inList = true
	e.setListSep(e._listSepSpace)
}

func (e *prettyConsoleEncoder) AddComplex64(k string, v complex64) {
	e.addComplex(k, complex128(v), 32)
}

func (e *prettyConsoleEncoder) AddComplex128(k string, v complex128) {
	e.addComplex(k, v, 64)
}

func (e *prettyConsoleEncoder) OpenNamespace(key string) {
	if e.namespaceIndent == 0 {
		e.buf.AppendString(e.cfg.LineEnding)
		e.colorizeAtLevel("  ↳ " + key)
		e.namespaceIndent = 4 + len(key)
	} else {
		if e.inList {
			e.buf.AppendString(e.cfg.LineEnding)
			appendSpaces(e.buf, e.namespaceIndent)
		}
		if len(key) > 0 {
			e.colorizeAtLevel(e.keyPrefix + key)
		}
		e.namespaceIndent += 1 + len(key)
	}
	e.inList = false
	e.setListSep(e._listSepSpace)
	e.keyPrefix = "."
}

func (e *prettyConsoleEncoder) AddObject(key string, marshaler zapcore.ObjectMarshaler) error {
	enc := e.clone()
	enc.OpenNamespace(key)

	if err := marshaler.MarshalLogObject(enc); err != nil {
		return err
	}

	_, _ = e.buf.Write(enc.buf.Bytes())
	putPrettyConsoleEncoder(enc)

	e.inList = true
	e.setIndentSep()
	return nil
}

func (e *prettyConsoleEncoder) AddArray(key string, marshaler zapcore.ArrayMarshaler) error {
	enc := e.clone()
	enc.OpenNamespace(key)

	enc.colorizeAtLevel("=[")
	enc.namespaceIndent += 2
	l := enc.buf.Len()

	if err := marshaler.MarshalLogArray(enc); err != nil {
		return err
	}
	if bytes.ContainsRune(enc.buf.Bytes()[l:], '\n') {
		enc.buf.AppendString(e.cfg.LineEnding)
		appendSpaces(enc.buf, enc.namespaceIndent-1)
	}
	enc.colorizeAtLevel("]")

	_, _ = e.buf.Write(enc.buf.Bytes())
	putPrettyConsoleEncoder(enc)

	e.inList = true
	e.setIndentSep()
	return nil
}

func (e *prettyConsoleEncoder) AddReflected(key string, value interface{}) error {
	enc := e.clone()
	enc.OpenNamespace(key)

	enc.colorizeAtLevel("=")
	enc.namespaceIndent += 1
	l := enc.buf.Len()
	iw := indentingWriter{
		buf:        enc.buf,
		indent:     enc.namespaceIndent,
		lineEnding: []byte(e.cfg.LineEnding),
	}

	switch v := value.(type) {
	case formattedString:
		if _, err := iw.Write([]byte(v)); err != nil {
			return err
		}
	default:
		if e.cfg.NewReflectedEncoder != nil {
			if err := e.cfg.NewReflectedEncoder(iw).Encode(value); err != nil {
				return err
			}
		}
		if l-enc.buf.Len() == 0 {
			// User-supplied reflectedEncoder is absent or a no-op. Fall
			// back to the reflection dumper
			if err := defaultReflectedEncoder(iw).Encode(value); err != nil {
				return err
			}
		}
	}

	_, _ = e.buf.Write(enc.buf.Bytes())
	putPrettyConsoleEncoder(enc)

	e.inList = true
	e.setIndentSep()
	return nil
}

func (e *prettyConsoleEncoder) AddByteString(key string, value []byte) {
	e.addSeparator()
	e.addKey(key)
	e.appendSafeByte(value)

	e.inList = true
	e.setListSep(e._listSepSpace)
}

func (e *prettyConsoleEncoder) AddBool(key string, value bool) {
	e.addSeparator()
	e.addKey(key)
	e.buf.AppendBool(value)

	e.inList = true
	e.setListSep(e._listSepSpace)
}

func (e *prettyConsoleEncoder) addComplex(key string, c complex128, precision int) {
	e.addSeparator()
	e.addKey(key)
	// Cast to a platform-independent, fixed-size type.
	r, i := real(c), imag(c)
	// Because we're always in a quoted string, we can use strconv without
	// special-casing NaN and +/-Inf.
	e.buf.AppendFloat(r, precision)
	// If imaginary part is less than 0, minus (-) sign is added by default
	// by AppendFloat.
	if i >= 0 {
		e.buf.AppendByte('+')
	}
	e.buf.AppendFloat(i, precision)
	e.buf.AppendByte('i')

	e.inList = true
	e.setListSep(e._listSepSpace)
}

func (e *prettyConsoleEncoder) AddDuration(key string, value time.Duration) {
	e.addSeparator()
	e.addKey(key)
	cur := e.buf.Len()
	// Both of these append, and we're at the first element of the sublist
	e.inList = false
	if e.cfg.EncodeDuration != nil {
		e.cfg.EncodeDuration(value, e)
	}
	if cur == e.buf.Len() {
		// User-supplied EncodeDuration is absent or a no-op. Fall back to
		// Go format
		e.buf.AppendString(value.String())
	}

	e.inList = true
	e.setListSep(e._listSepSpace)
}

func (e *prettyConsoleEncoder) addFloat(key string, value float64, precision int) {
	e.addSeparator()
	e.addKey(key)
	e.buf.AppendFloat(value, precision)

	e.inList = true
	e.setListSep(e._listSepSpace)
}

func (e *prettyConsoleEncoder) AddInt64(key string, value int64) {
	e.addSeparator()
	e.addKey(key)
	e.buf.AppendInt(value)

	e.inList = true
	e.setListSep(e._listSepSpace)
}

func (e *prettyConsoleEncoder) AddString(key, value string) {
	e.addSeparator()
	e.addKey(key)
	e.addSafeString(value)

	e.inList = true
	e.setListSep(e._listSepSpace)
}

// FormattedString is similar to zap.String() but it does not escape the
// printed value. This is useful for users who have formatted strings they want
// to preserve when they are logged.
//
// This is for use with a non-sugared logger. For a wrapper designed for use
// with a sugar logger, see FormattedStringValue().
func FormattedString(key string, value string) zap.Field {
	return zap.Any(key, formattedString(value))
}

// FormattedStringValue is similar to zap.String() but it does not escape the
// printed value. This is useful for users who have formatted strings they want
// to preserve when they are logged.
//
// This is for use with a sugared logger. For a wrapper designed for use with a
// non-sugar logger, see FormattedStringValue().
func FormattedStringValue(value string) formattedString {
	return formattedString(value)
}

type formattedString string

// addIndentedFormat streams v's %+v representation through the indenting
// writer, dropping the single leading newline pkg/errors-style formatters
// emit. Streaming avoids materialising stacktraces as one large string.
func (e *prettyConsoleEncoder) addIndentedFormat(key string, v interface{}) {
	e.addSeparator()
	e.addKey(key)
	tw := newlineTrimWriter{w: indentingWriter{
		buf:        e.buf,
		indent:     e.namespaceIndent,
		lineEnding: []byte(e.cfg.LineEnding),
	}}
	_, _ = fmt.Fprintf(&tw, "%+v", v)

	e.inList = true
	e.setListSep(e._listSepSpace)
}

// newlineTrimWriter drops a single leading newline from the stream.
type newlineTrimWriter struct {
	w       indentingWriter
	started bool
}

func (t *newlineTrimWriter) Write(p []byte) (int, error) {
	if !t.started {
		t.started = true
		if len(p) > 0 && p[0] == '\n' {
			n, err := t.w.Write(p[1:])
			return n + 1, err
		}
	}
	return t.w.Write(p)
}

// addIndentedString appends a string, replacing any newlines with the
// current indent.
func (e *prettyConsoleEncoder) addIndentedString(key string, s string) {
	e.addSeparator()
	e.addKey(key)
	iw := indentingWriter{
		buf:        e.buf,
		indent:     e.namespaceIndent,
		lineEnding: []byte(e.cfg.LineEnding),
	}
	_, _ = iw.Write([]byte(s))

	e.inList = true
	e.setListSep(e._listSepSpace)
}

func (e *prettyConsoleEncoder) AddTime(key string, value time.Time) {
	e.addSeparator()
	e.addKey(key)
	// Don't use configured time encoder as it's been customized to display the
	// log's time, .e.g, this will be coloured dark grey in time.Kitchen
	e.buf.AppendTime(value, time.RFC3339)

	e.inList = true
	e.setListSep(e._listSepSpace)
}

func (e *prettyConsoleEncoder) AddUint64(key string, value uint64) {
	e.addSeparator()
	e.addKey(key)
	e.buf.AppendUint(value)

	e.inList = true
	e.setListSep(e._listSepSpace)
}
