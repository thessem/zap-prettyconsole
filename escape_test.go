package prettyconsole

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/buffer"
	"go.uber.org/zap/zapcore"
)

func TestEscaping(t *testing.T) {
	tests := []struct {
		desc     string
		in       string
		contains string
	}{
		{"Newline", "a\nb", `a\nb`},
		{"CarriageReturn", "a\rb", `a\rb`},
		{"Tab", "a\tb", `a\tb`},
		{"QuoteAndBackslash", `a"b\c`, `a\"b\\c`},
		{"ControlChars", "bell\x07null\x00", `bell\u0007null\u0000`},
		{"AnsiInjection", "\x1b[31mred", `\u001b[31mred`},
		{"InvalidUTF8", "ok\xff\xfego", `ok\ufffd\ufffdgo`},
		{"MultiByteUTF8", "héllo 世界 👍", "héllo 世界 👍"},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			// zap.String exercises addSafeString, zap.ByteString exercises
			// appendSafeByte; both must agree.
			out := encodePlain(t, zap.String("s", tt.in), zap.ByteString("b", []byte(tt.in)))
			assert.Contains(t, out, "s="+tt.contains)
			assert.Contains(t, out, "b="+tt.contains)
		})
	}
}

// TestNoAnsiInjection asserts the security property directly: user-supplied
// values must never smuggle a real ANSI escape sequence into the raw output.
func TestNoAnsiInjection(t *testing.T) {
	cfg := NewEncoderConfig()
	enc := NewEncoder(cfg)
	buf, err := enc.EncodeEntry(
		zapcore.Entry{Level: zapcore.InfoLevel, Message: "evil \x1b[31m message"},
		[]zapcore.Field{zap.String("k", "\x1b]0;title\x07\x1b[9999D")},
	)
	require.NoError(t, err)
	defer buf.Free()
	for _, seq := range []string{"\x1b[31m ", "\x1b]", "\x1b[9999D"} {
		assert.NotContains(t, buf.String(), seq)
	}
}

func TestIndentingWriter(t *testing.T) {
	tests := []struct {
		desc     string
		expected string
		input    string
	}{
		{
			desc:     "Empty",
			input:    "",
			expected: "",
		},
		{
			desc:     "No newlines",
			input:    "hello",
			expected: "hello",
		},
		{
			desc:     "Newlines",
			input:    "hello\nHow are\n\nYou?\n",
			expected: "hello\t\t  How are\t\t  \t\t  You?\t\t  ",
		},
		{
			desc:     "Trailing newline",
			input:    "T\n",
			expected: "T\t\t  ",
		},
		{
			desc:     "Leading newline",
			input:    "\nT",
			expected: "\t\t  T",
		},
		{
			desc:     "Only newline",
			input:    "\n",
			expected: "\t\t  ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			buf := buffer.Buffer{}
			iw := indentingWriter{indent: 2, buf: &buf, lineEnding: []byte{'\t', '\t'}}
			n, err := iw.Write([]byte(tt.input))
			assert.NoError(t, err)
			assert.Equal(t, buf.Len(), n)
			assert.Equal(t, tt.expected, buf.String())
		})
	}
}

// ansiSequence matches the colour codes the encoder itself emits.
var ansiSequence = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// FuzzEscaping feeds arbitrary user data (message, values, and bytes —
// keys are developer-controlled constants in practice, so they stay fixed)
// through the encoder and checks structural invariants:
//   - encoding never fails or panics
//   - output ends with the configured line ending
//   - after removing the encoder's own colour codes, no control bytes
//     other than newlines remain: user data cannot smuggle in terminal
//     escapes, they must have been escaped to plain text
func FuzzEscaping(f *testing.F) {
	f.Add("hello", "world", []byte{0x00}, int64(1), 3.14)
	f.Add("", "", []byte(nil), int64(0), 0.0)
	f.Add("new\nline", "esc\x1b[31m", []byte{0xff, 0xfe}, int64(-1), -1.5)
	f.Add("大きな", "\x1b]0;t\a", []byte("plain"), int64(1<<62), 1e300)
	f.Add(strings.Repeat("x", 4096), "\r\t\"\\", []byte{0x1b, '[', '2', 'J'}, int64(-1<<62), 0.0/1e-323)
	// SWAR boundary seeds: escapes at every offset within and across
	// 8-byte words.
	f.Add(strings.Repeat("a", 7)+"\n"+strings.Repeat("b", 8), strings.Repeat("c", 15)+"\"", []byte(strings.Repeat("d", 9)+"\x00"), int64(8), 8.0)
	f.Add("\""+strings.Repeat("e", 23), strings.Repeat("f", 6)+"\\"+strings.Repeat("g", 6), []byte{0x7f, 0x80, 0xc2, 0xa9}, int64(16), 0.5)

	enc := NewEncoder(NewEncoderConfig())
	f.Fuzz(func(t *testing.T, msg, val string, raw []byte, n int64, fl float64) {
		buf, err := enc.EncodeEntry(
			zapcore.Entry{Level: zapcore.InfoLevel, Message: msg, Time: time.Unix(0, n%1e9).UTC()},
			[]zapcore.Field{
				zap.String("s", val),
				zap.ByteString("b", raw),
				zap.Binary("bin", raw),
				zap.Int64("n", n),
				zap.Float64("f", fl),
				zap.Strings("arr", []string{val, msg}),
			},
		)
		require.NoError(t, err)
		defer buf.Free()
		out := buf.String()

		require.True(t, strings.HasSuffix(out, zapcore.DefaultLineEnding), "missing line ending")

		plain := ansiSequence.ReplaceAllString(out, "")
		for i := 0; i < len(plain); i++ {
			c := plain[i]
			if c < 0x20 && c != '\n' {
				t.Fatalf("raw control byte %#x survived escaping at %d in %q", c, i, plain)
			}
		}
	})
}

// TestPlainRunEndDifferential checks the SWAR fast path against the
// byte-class table exhaustively: every byte value at every position
// within a window wider than one SWAR word.
func TestPlainRunEndDifferential(t *testing.T) {
	reference := func(s string, i int) int {
		for i < len(s) && byteClass[s[i]] == classPlain {
			i++
		}
		return i
	}
	for b := 0; b < 256; b++ {
		for pos := 0; pos < 16; pos++ {
			buf := []byte(strings.Repeat("a", 16))
			buf[pos] = byte(b)
			s := string(buf)
			for start := 0; start <= pos; start++ {
				assert.Equal(t, reference(s, start), plainRunEnd(s, start),
					"byte %#x at position %d, start %d", b, pos, start)
				assert.Equal(t, reference(s, start), plainRunEnd(buf, start),
					"byte slice: byte %#x at position %d, start %d", b, pos, start)
			}
		}
	}
}

// TestEscapingLongStrings pushes multi-word content with escapes at word
// boundaries through the full encoder path.
func TestEscapingLongStrings(t *testing.T) {
	long := strings.Repeat("abcdefg", 100)
	out := encodePlain(t, zap.String("s", long))
	assert.Contains(t, out, long)

	boundary := strings.Repeat("a", 7) + "\n" + strings.Repeat("b", 8) + "\"" + strings.Repeat("c", 9)
	out = encodePlain(t, zap.String("s", boundary))
	assert.Contains(t, out, strings.Repeat("a", 7)+`\n`+strings.Repeat("b", 8)+`\"`+strings.Repeat("c", 9))
}

// TestIndentingWriterChunkedEquivalence: streamed writes must produce the
// same bytes as one combined write, since stacktraces are now streamed
// through the writer in fmt-sized chunks.
func TestIndentingWriterChunkedEquivalence(t *testing.T) {
	input := "line one\nline two\n\nline four"
	for _, chunks := range [][]string{
		{input},
		{"line one\n", "line two\n", "\n", "line four"},
		{"line one", "\nline two\n\nline f", "our"},
		{"l", "i", "n", "e", " ", "o", "n", "e", "\n", "line two\n\nline four"},
	} {
		var buf buffer.Buffer
		iw := indentingWriter{indent: 3, buf: &buf, lineEnding: []byte("\n")}
		for _, c := range chunks {
			_, err := iw.Write([]byte(c))
			require.NoError(t, err)
		}
		var want buffer.Buffer
		iww := indentingWriter{indent: 3, buf: &want, lineEnding: []byte("\n")}
		_, err := iww.Write([]byte(input))
		require.NoError(t, err)
		assert.Equal(t, want.String(), buf.String(), "chunks %q", chunks)
	}
}

// TestNewlineTrimWriter drops exactly one leading newline and only from
// the very start of the stream.
func TestNewlineTrimWriter(t *testing.T) {
	var buf buffer.Buffer
	tw := newlineTrimWriter{w: indentingWriter{indent: 0, buf: &buf, lineEnding: []byte("\n")}}
	_, _ = tw.Write([]byte("\nfirst"))
	_, _ = tw.Write([]byte("\nsecond"))
	assert.Equal(t, "first\nsecond", buf.String())
}
