package prettyconsole

import (
	"bytes"
	"io"
	"unicode/utf8"
)

// String escaping for the pretty console encoder: user-supplied values are
// JSON-style escaped so that control bytes (including ANSI sequences) can
// never reach the terminal unfiltered.

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
		_, _ = e.buf.Write(s[i : i+size]) // Explicitly ignore errors
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
