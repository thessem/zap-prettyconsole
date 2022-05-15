package prettyconsole

import (
	"encoding/base64"
	"strings"
	"time"

	"go.uber.org/zap/zapcore"
)

// Test interface conformance
var _ zapcore.ObjectEncoder = (*prettyConsoleEncoder)(nil)

func (e *prettyConsoleEncoder) AddFloat32(k string, v float32) { e.addFloat(k, float64(v), 32) }
func (e *prettyConsoleEncoder) AddFloat64(k string, v float64) { e.addFloat(k, float64(v), 64) }
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
	e.AddString(key, base64.StdEncoding.EncodeToString(value))
}
func (e *prettyConsoleEncoder) AddComplex64(k string, v complex64) {
	e.addComplex(k, complex128(v), 32)
}
func (e *prettyConsoleEncoder) AddComplex128(k string, v complex128) {
	e.addComplex(k, complex128(v), 64)
}

func (e *prettyConsoleEncoder) OpenNamespace(key string) {
	if e.namespaceIndent == 0 {
		e.buf.AppendString("\n  " + e.colorizeAtLevel("↳") + " ")
		if len(key) > 0 {
			e.buf.AppendString(e.colorizeAtLevel(key))
		}
		e.namespaceIndent = 4 + len(key)
	} else {
		if e.inList {
			e.buf.AppendString("\n")
			e.buf.AppendString(strings.Repeat(" ", e.namespaceIndent))
		}
		if len(key) > 0 {
			e.buf.AppendString(e.colorizeAtLevel(e.keyPrefix + key))
		}
		e.namespaceIndent += 1 + len(key)
	}
	e.inList = false
	e.listSep = e.cfg.ConsoleSeparator
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
	e.listSep = "\n" + strings.Repeat(" ", e.namespaceIndent)
	return nil
}

func (e *prettyConsoleEncoder) AddArray(key string, marshaler zapcore.ArrayMarshaler) error {
	enc := e.clone()
	enc.OpenNamespace(key)

	enc.buf.AppendString(e.colorizeAtLevel("=["))
	enc.namespaceIndent += 2

	if err := marshaler.MarshalLogArray(enc); err != nil {
		return err
	}
	if strings.Contains(strings.TrimSpace(enc.buf.String()), "\n") {
		enc.buf.AppendString("\n" + strings.Repeat(" ", enc.namespaceIndent-1))
	}
	enc.buf.AppendString(e.colorizeAtLevel("]"))

	e.buf.AppendString(enc.buf.String())
	putPrettyConsoleEncoder(enc)

	e.inList = true
	e.listSep = "\n" + strings.Repeat(" ", e.namespaceIndent)
	return nil
}

func (e *prettyConsoleEncoder) AddReflected(key string, value interface{}) error {
	enc := e.clone()
	enc.OpenNamespace(key)

	enc.buf.AppendString(e.colorizeAtLevel("="))
	enc.namespaceIndent += 1

	buf := _bufferPoolGet()
	if reflectedEncoder := e.cfg.NewReflectedEncoder(buf); e != nil {
		if err := reflectedEncoder.Encode(value); err != nil {
			return err
		}
	}
	if buf.Len() == 0 {
		// User-supplied reflectedEncoder is a no-op. Fall back to dd
		if err := defaultReflectedEncoder(buf).Encode(value); err != nil {
			return err
		}
	}
	// Indent the output of the encoder
	spaces := strings.Repeat(" ", enc.namespaceIndent)
	enc.buf.AppendString(strings.ReplaceAll(buf.String(), "\n", "\n"+spaces))

	e.buf.AppendString(enc.buf.String())
	putPrettyConsoleEncoder(enc)
	buf.Free()

	e.inList = true
	e.listSep = "\n" + strings.Repeat(" ", e.namespaceIndent)
	return nil
}

func (e *prettyConsoleEncoder) AddByteString(key string, value []byte) {
	e.addSeparator()
	e.addKey(key)
	e.appendSafeByte(value)

	e.inList = true
	e.listSep = e.cfg.ConsoleSeparator
}

func (e *prettyConsoleEncoder) AddBool(key string, value bool) {
	e.addSeparator()
	e.addKey(key)
	e.buf.AppendBool(value)

	e.inList = true
	e.listSep = e.cfg.ConsoleSeparator
}

func (e *prettyConsoleEncoder) addComplex(key string, c complex128, precision int) {
	e.addSeparator()
	e.addKey(key)
	// Cast to a platform-independent, fixed-size type.
	r, i := float64(real(c)), float64(imag(c))
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
	e.listSep = e.cfg.ConsoleSeparator
}

func (e *prettyConsoleEncoder) AddDuration(key string, value time.Duration) {
	e.addSeparator()
	e.addKey(key)
	cur := e.buf.Len()
	// Both of these append, and we're at the first element of the sublist
	e.inList = false
	if durationEncoder := e.cfg.EncodeDuration; e != nil {
		durationEncoder(value, e)
	}
	if cur == e.buf.Len() {
		// User-supplied EncodeDuration is a no-op. Fall back to Go format
		e.buf.AppendString(value.String())
	}

	e.inList = true
	e.listSep = e.cfg.ConsoleSeparator
}

func (e *prettyConsoleEncoder) addFloat(key string, value float64, precision int) {
	e.addSeparator()
	e.addKey(key)
	e.buf.AppendFloat(value, precision)

	e.inList = true
	e.listSep = e.cfg.ConsoleSeparator
}

func (e *prettyConsoleEncoder) AddInt64(key string, value int64) {
	e.addSeparator()
	e.addKey(key)
	e.buf.AppendInt(value)

	e.inList = true
	e.listSep = e.cfg.ConsoleSeparator
}

func (e *prettyConsoleEncoder) AddString(key, value string) {
	e.addSeparator()
	e.addKey(key)
	e.addSafeString(value)

	e.inList = true
	e.listSep = e.cfg.ConsoleSeparator
}

// addIndentedString appends a string, replacing any newlines with the
// current indent.
func (e *prettyConsoleEncoder) addIndentedString(key string, s string) {
	e.addSeparator()
	e.addKey(key)
	spaces := strings.Repeat(" ", e.namespaceIndent)
	e.buf.AppendString(strings.ReplaceAll(s, "\n", "\n"+spaces))

	e.inList = true
	e.listSep = e.cfg.ConsoleSeparator
}

func (e *prettyConsoleEncoder) AddTime(key string, value time.Time) {
	e.addSeparator()
	e.addKey(key)
	// Don't use configured time encoder as it's been customized to display the
	// log's time, .e.g, this will be coloured dark grey in time.Kitchen
	//cur := e.buf.Len()
	//if timeEncoder := e.cfg.EncodeTime; e != nil {
	//	timeEncoder(value, e)
	//}
	//if cur == e.buf.Len() {
	// User-supplied EncodeTime is a no-op. Fall back to RFC3339
	e.buf.AppendString(value.Format(time.RFC3339))
	//}

	e.inList = true
	e.listSep = e.cfg.ConsoleSeparator
}

func (e *prettyConsoleEncoder) AddUint64(key string, value uint64) {
	e.addSeparator()
	e.addKey(key)
	e.buf.AppendUint(value)

	e.inList = true
	e.listSep = e.cfg.ConsoleSeparator
}
