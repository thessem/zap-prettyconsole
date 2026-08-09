package prettyconsole

import (
	"bytes"
	"io"
	"unicode/utf8"
)

// String escaping for the pretty console encoder: user-supplied values are
// JSON-style escaped so that control bytes (including ANSI sequences) can
// never reach the terminal unfiltered.
//
// Escaping is driven by a 256-entry class table so the hot loop takes one
// predictable branch per byte, and runs of plain bytes are copied in bulk
// rather than byte-by-byte.

const (
	classPlain  byte = iota // append verbatim
	classEscape             // needs an escape sequence
	classMulti              // start of a multi-byte UTF-8 sequence
)

var byteClass = func() (t [256]byte) {
	for i := range t {
		switch b := byte(i); {
		case b >= utf8.RuneSelf:
			t[i] = classMulti
		case b < 0x20, b == '\\', b == '"':
			t[i] = classEscape
		default:
			t[i] = classPlain
		}
	}
	return t
}()

// Single-character strings used by escapes, so the escape path never
// builds tiny strings at runtime.
var hexDigitStr = [16]string{
	"0", "1", "2", "3", "4", "5", "6", "7",
	"8", "9", "a", "b", "c", "d", "e", "f",
}

// escapeByte writes the coloured escape sequence for a byte that needs
// one (class classEscape).
func (e *prettyConsoleEncoder) escapeByte(b byte) {
	switch b {
	case '\\':
		e.colorizeAtLevel(`\\`)
	case '"':
		e.colorizeAtLevel(`\"`)
	case '\n':
		e.colorizeAtLevel(`\n`)
	case '\r':
		e.colorizeAtLevel(`\r`)
	case '\t':
		e.colorizeAtLevel(`\t`)
	default:
		// Encode bytes < 0x20, except for the escape sequences above.
		e.colorizeAtLevel(`\u00`)
		e.colorizeAtLevel(hexDigitStr[b>>4])
		e.colorizeAtLevel(hexDigitStr[b&0xF])
	}
}

// addSafeString JSON-escapes a string and appends it to the internal buffer.
func (e *prettyConsoleEncoder) addSafeString(s string) {
	for i := 0; i < len(s); {
		c := s[i]
		if byteClass[c] == classPlain {
			j := i + 1
			for j < len(s) && byteClass[s[j]] == classPlain {
				j++
			}
			e.buf.AppendString(s[i:j])
			i = j
			continue
		}
		if byteClass[c] == classEscape {
			e.escapeByte(c)
			i++
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			e.buf.AppendString(`\ufffd`)
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
		c := s[i]
		if byteClass[c] == classPlain {
			j := i + 1
			for j < len(s) && byteClass[s[j]] == classPlain {
				j++
			}
			_, _ = e.buf.Write(s[i:j]) // Explicitly ignore errors
			i = j
			continue
		}
		if byteClass[c] == classEscape {
			e.escapeByte(c)
			i++
			continue
		}
		r, size := utf8.DecodeRune(s[i:])
		if r == utf8.RuneError && size == 1 {
			e.buf.AppendString(`\ufffd`)
			i++
			continue
		}
		_, _ = e.buf.Write(s[i : i+size]) // Explicitly ignore errors
		i += size
	}
}

var manySpacesBytes = []byte(manySpaces)

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
		for ind := i.indent; ind > 0; {
			chunk := ind
			if chunk > len(manySpacesBytes) {
				chunk = len(manySpacesBytes)
			}
			n, _ := i.buf.Write(manySpacesBytes[:chunk])
			written += n
			ind -= chunk
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
