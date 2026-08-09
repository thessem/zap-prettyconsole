package prettyconsole

import (
	"bytes"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// The encoder's raw output is full of ANSI escape sequences, which make
// expected values impossible to read or diff. Tests therefore compare a
// tagged form where each escape sequence is replaced by a readable tag.
// The mapping is bijective, so a tagged comparison is exactly as strict as
// comparing the raw bytes.
var ansiTags = []struct{ raw, tag string }{
	{"\x1b[90m", "<gray>"},
	{"\x1b[36m", "<cyan>"},
	{"\x1b[32m", "<green>"},
	{"\x1b[33m", "<yellow>"},
	{"\x1b[31m", "<red>"},
	{"\x1b[1m", "<bold>"},
	{"\x1b[0m", "<r>"},
}

var ansiOther = regexp.MustCompile(`\x1b\[([0-9;]*)m`)

// tagANSI converts raw encoder output into the readable tagged form.
func tagANSI(s string) string {
	for _, t := range ansiTags {
		s = strings.ReplaceAll(s, t.raw, t.tag)
	}
	// Any sequence without a friendly name keeps its code, so nothing is
	// ever lost in the round trip.
	return ansiOther.ReplaceAllString(s, "<esc:$1>")
}

// stripANSI removes all ANSI escape sequences, for tests that only care
// about the text content.
func stripANSI(s string) string {
	return ansiOther.ReplaceAllString(s, "")
}

func TestTagANSI(t *testing.T) {
	assert.Equal(t, "<green>INF<r> plain <bold>x<r> <esc:95>y<r>",
		tagANSI("\x1b[32mINF\x1b[0m plain \x1b[1mx\x1b[0m \x1b[95my\x1b[0m"))
	assert.Equal(t, "INF plain x y",
		stripANSI("\x1b[32mINF\x1b[0m plain \x1b[1mx\x1b[0m \x1b[95my\x1b[0m"))
}

var update = flag.Bool("update", false, "rewrite golden files with actual test output")

// assertGolden compares got against testdata/<test name>.golden. Running
// the tests with -update regenerates the files instead, so an intentional
// output change becomes a reviewable git diff rather than a hand-edit.
func assertGolden(t *testing.T, got string) {
	t.Helper()
	path := filepath.Join("testdata", strings.ReplaceAll(t.Name(), "/", "_")+".golden")
	if *update {
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(got), 0o644))
		return
	}
	want, err := os.ReadFile(path)
	require.NoErrorf(t, err, "missing golden file %s (create it with: go test -run '%s' -update)", path, t.Name())
	assert.Equalf(t, string(want), got,
		"output differs from %s (regenerate with: go test -run '%s' -update)", path, t.Name())
}

// encodePlain encodes a message-only entry with the given fields using a
// config without time or level, and returns the output stripped of colour
// codes, so assertions can focus on content.
func encodePlain(t *testing.T, fields ...zapcore.Field) string {
	t.Helper()
	cfg := NewEncoderConfig()
	cfg.TimeKey = zapcore.OmitKey
	cfg.LevelKey = zapcore.OmitKey
	enc := NewEncoder(cfg)
	buf, err := enc.EncodeEntry(zapcore.Entry{Level: zapcore.InfoLevel, Message: "msg"}, fields)
	require.NoError(t, err)
	defer buf.Free()
	return stripANSI(buf.String())
}

// rPath removes stacktrace line-numbers from golden comparisons. Remember
// to manually test with -trimpath. "@" appears in stacktrace paths when the
// toolchain itself lives in the module cache (e.g.
// golang.org/toolchain@v0.0.1-go1.21.13.linux-amd64)
var rPath = regexp.MustCompile(`(github|\/|testing|runtime)[\w\.\\\/\-@]*:\d+`)

// runGoldenCases encodes each entry and compares the tagged output against
// this test's golden file.
func runGoldenCases(t *testing.T, tests []goldenCase) {
	t.Helper()
	enc := NewEncoder(NewEncoderConfig())
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			buf, err := enc.EncodeEntry(tt.ent, tt.fields)
			if assert.NoError(t, err, "Unexpected encoding error.") {
				assertGolden(t, tagANSI(rPath.ReplaceAllString(buf.String(), "/<some_file>:<line_number>")))
			}
		})
	}
}

type goldenCase struct {
	desc   string
	ent    zapcore.Entry
	fields []zapcore.Field
}

func TestEncodeEntry(t *testing.T) {
	tests := []goldenCase{
		{
			desc: "Minimal",
			ent: zapcore.Entry{
				Level: zap.InfoLevel,
				Time:  time.Date(2018, 6, 19, 16, 33, 42, 99, time.UTC),
			},
			fields: []zapcore.Field{},
		},
		{
			desc: "Basic",
			ent: zapcore.Entry{
				Level:      zap.InfoLevel,
				Time:       time.Date(2018, 6, 19, 16, 33, 42, 99, time.UTC),
				LoggerName: "TestLogger",
				Message:    "log\nmessage",
				Caller:     zapcore.NewEntryCaller(100, "/path/to/foo.go", 42, true),
			},
			fields: []zapcore.Field{
				zap.String("string", "test_\n_value"),
				zap.Strings("strings", []string{"\u001B1", "2\t"}),
				zap.Complex128p("complex", &[]complex128{12i - 8}[0]),
				zap.Int("int", -0),
				zap.Time("time", time.Date(2022, 6, 19, 16, 33, 42, 99, time.UTC)),
				zap.Duration("duration", 3*time.Hour),
				zap.Float64("float", -0.3e14),
			},
		},
		{
			desc: "Namespaces",
			ent: zapcore.Entry{
				Level:   zapcore.InfoLevel,
				Message: "test message",
				Time:    time.Date(2018, 6, 19, 16, 33, 42, 99, time.UTC),
			},
			fields: []zapcore.Field{
				zap.String("test_string", "test_message"),
				zap.Namespace("namespace"),
				zap.String("string3", "val3"),
				zap.String("string2", "val2"),
				zap.Namespace("namespace2"),
				zap.String("string5", "val5"),
				zap.String("string4", "val4"),
				zap.Namespace("namespace3"),
				zap.Namespace("namespace4"),
				zap.String("string7", "val7"),
				zap.String("string6", "val6"),
				zap.Namespace("namespace5"),
			},
		},
		{
			desc: "Pre-formatted strings",
			ent: zapcore.Entry{
				Level:   zapcore.InfoLevel,
				Message: "test message",
				Time:    time.Date(2018, 6, 19, 16, 33, 42, 99, time.UTC),
			},
			fields: []zapcore.Field{
				zap.String("test_string", "test_message"),
				FormattedString("colours", "<r><red>RED STRING!<r><red>"),
				zap.Namespace("namespace"),
				FormattedString("sql", "SELECT * FROM\n\tusers\nWHERE\n\tname = 'James'"),
				zap.Any("mdb", FormattedStringValue("db.users.find({\n\tname: \"James\"\n});")),
			},
		},
		{
			desc: "Objects",
			ent: zapcore.Entry{
				Level:   zapcore.InfoLevel,
				Message: "test message",
				Time:    time.Date(2018, 6, 19, 16, 33, 42, 99, time.UTC),
			},
			fields: []zapcore.Field{
				zap.Object("object", testStableMap{
					"1": testStableMap{
						"1": testStableMap{
							"1_leading_value": "leading_value",
							"2": testStableMap{
								"1": "string",
								"2": testArray{1, 2, 3, 4},
								"3": interface{}(2.0),
								"4": &testStableMap{"r1": []string{"r1", "r2", "r3", "r4", "r5", "r6", "r7", "r8", "r9", "r10"}},
							},
						},
						"2": "trailing_value",
					},
				}),
			},
		},
		{
			desc: "Arrays",
			ent: zapcore.Entry{
				Level:   zapcore.InfoLevel,
				Message: "test message",
				Time:    time.Date(2018, 6, 19, 16, 33, 42, 99, time.UTC),
			},
			fields: []zapcore.Field{
				zap.Array("array", testArray{
					testArray{1, 2, 3, 4},
					testArray{},
					testArray{1, 2, 3, testArray{1}},
					testArray{1, 2, 3, testArray{&testStableMap{"3": 3, "4": 4}}},
					testArray{testStableMap{"1": 1, "2": 2}, 3, 4, 5},
					testArray{1, 2, testStableMap{"3": 3, "4": 4}},
				}),
			},
		},
	}

	runGoldenCases(t, tests)
}

func TestNewConfigBuilds(t *testing.T) {
	cfg := NewConfig()
	logger, err := cfg.Build()
	require.NoError(t, err, "NewConfig should build via the registered pretty_console encoding")
	logger.Debug("config smoke test")

	NewLogger(zapcore.InfoLevel).Info("logger smoke test")
}

// TestEncodeEntryDeterministic re-encodes the same entry many times through
// the same encoder: pooled state leaking between calls would change output.
func TestEncodeEntryDeterministic(t *testing.T) {
	enc := NewEncoder(NewEncoderConfig())
	ent := zapcore.Entry{Level: zapcore.WarnLevel, Message: "m", Time: time.Unix(0, 0).UTC()}
	fields := []zapcore.Field{
		zap.String("s", "v"),
		zap.Object("o", testStableMap{"k": "v"}),
		zap.Array("a", testArray{1, 2}),
	}
	first, err := enc.EncodeEntry(ent, fields)
	require.NoError(t, err)
	want := first.String()
	first.Free()
	for i := 0; i < 100; i++ {
		buf, err := enc.EncodeEntry(ent, fields)
		require.NoError(t, err)
		assert.Equal(t, want, buf.String(), "iteration %d differs", i)
		buf.Free()
	}
}

// TestCloneIndependence checks that writing through a clone does not leak
// into the original encoder.
func TestCloneIndependence(t *testing.T) {
	enc := NewEncoder(NewEncoderConfig())
	clone := enc.Clone()
	clone.AddString("leak", "value")

	ent := zapcore.Entry{Level: zapcore.InfoLevel, Message: "m", Time: time.Unix(0, 0).UTC()}
	origBuf, err := enc.EncodeEntry(ent, nil)
	require.NoError(t, err)
	defer origBuf.Free()
	assert.NotContains(t, origBuf.String(), "leak")

	cloneBuf, err := clone.EncodeEntry(ent, nil)
	require.NoError(t, err)
	defer cloneBuf.Free()
	assert.Contains(t, cloneBuf.String(), "leak")
}

// TestInnerEncoderClone covers the console encoder's own Clone, which
// copies buffered bytes and isolates further writes from the original.
func TestInnerEncoderClone(t *testing.T) {
	cfg := NewEncoderConfig()
	inner := prettyConsoleEncoder{cfg: &cfg, buf: getBuffer(), listSep: " ", _listSepSpace: " ", _listSepComma: ", "}
	inner.buf.AppendString("seed")
	clone := inner.Clone().(*prettyConsoleEncoder)
	assert.Equal(t, "seed", clone.buf.String())
	clone.AddString("k", "v")
	assert.Equal(t, "seed", inner.buf.String())
}

type testStableMap map[string]interface{}

func (t testStableMap) MarshalLogObject(encoder zapcore.ObjectEncoder) error {
	// Put these in alphabetical order so order doesn't change test-to-test
	keys := make([]string, 0, len(t))
	for k := range t {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		switch v := t[k].(type) {
		case zapcore.ObjectMarshaler:
			_ = encoder.AddObject(k, v)
		case zapcore.ArrayMarshaler:
			_ = encoder.AddArray(k, v)
		case string:
			encoder.AddString(k, v)
		case int:
			encoder.AddInt(k, v)
		default:
			_ = encoder.AddReflected(k, v)
		}
	}
	return nil
}

type testArray []interface{}

func (t testArray) MarshalLogArray(encoder zapcore.ArrayEncoder) error {
	for _, val := range t {
		switch v := val.(type) {
		case zapcore.ObjectMarshaler:
			_ = encoder.AppendObject(v)
		case zapcore.ArrayMarshaler:
			_ = encoder.AppendArray(v)
		case string:
			encoder.AppendString(v)
		case int:
			encoder.AppendInt(v)
		default:
			_ = encoder.AppendReflected(v)
		}
	}
	return nil
}

type testBufferWriterSync struct {
	buf bytes.Buffer
}

func (w *testBufferWriterSync) Sync() error {
	return nil
}

func (w *testBufferWriterSync) Write(p []byte) (int, error) {
	return w.buf.Write(p)
}

// TestZeroValueEncoderConfig locks in nil-safety for hand-rolled configs:
// zapcore's own encoders accept a zero-value EncoderConfig with only keys
// set, so this encoder must too, falling back to sensible formats.
func TestZeroValueEncoderConfig(t *testing.T) {
	enc := NewEncoder(zapcore.EncoderConfig{MessageKey: "M"})
	buf, err := enc.EncodeEntry(zapcore.Entry{Message: "m", Level: zapcore.InfoLevel, Time: time.Unix(0, 0).UTC()}, []zapcore.Field{
		zap.Duration("dur", 90*time.Second),
		zap.Durations("durs", []time.Duration{time.Second}),
		zap.Time("time", time.Unix(0, 0).UTC()),
		zap.Times("times", []time.Time{time.Unix(0, 0).UTC()}),
		zap.Reflect("reflect", struct{ A int }{1}),
		zap.Object("obj", testStableMap{"k": "v"}),
		zap.Array("arr", testArray{1}),
		zap.Error(errors.New("boom")),
	})
	require.NoError(t, err)
	defer buf.Free()
	out := stripANSI(buf.String())
	assert.Contains(t, out, "dur=1m30s")
	assert.Contains(t, out, "durs=[1000000000]")
	assert.Contains(t, out, "times=[1970-01-01T00:00:00Z]")
	assert.Contains(t, out, "A: 1")
	assert.Contains(t, out, "error=boom")
	assert.True(t, strings.HasSuffix(out, "\n"), "empty LineEnding must default like zapcore")
}

// TestEncoderConfigKnobs covers the EncoderConfig options users commonly
// override when they embed this encoder in their own zap.Config.
func TestEncoderConfigKnobs(t *testing.T) {
	entry := zapcore.Entry{Message: "m", Level: zapcore.InfoLevel, Time: time.Unix(0, 0).UTC()}
	fields := []zapcore.Field{zap.String("a", "1"), zap.String("b", "2")}
	encode := func(t *testing.T, mutate func(*zapcore.EncoderConfig)) string {
		t.Helper()
		cfg := NewEncoderConfig()
		cfg.TimeKey = zapcore.OmitKey
		mutate(&cfg)
		buf, err := NewEncoder(cfg).EncodeEntry(entry, fields)
		require.NoError(t, err)
		defer buf.Free()
		return buf.String()
	}

	t.Run("ConsoleSeparator", func(t *testing.T) {
		out := stripANSI(encode(t, func(c *zapcore.EncoderConfig) { c.ConsoleSeparator = " | " }))
		assert.Contains(t, out, "a=1 | b=2")
	})
	t.Run("LineEnding", func(t *testing.T) {
		out := encode(t, func(c *zapcore.EncoderConfig) { c.LineEnding = "\r\n" })
		assert.True(t, strings.HasSuffix(out, "\r\n"))
	})
	t.Run("SkipLineEnding", func(t *testing.T) {
		out := encode(t, func(c *zapcore.EncoderConfig) { c.SkipLineEnding = true })
		assert.False(t, strings.HasSuffix(out, "\n"))
	})
	t.Run("OmitMessageKey", func(t *testing.T) {
		out := stripANSI(encode(t, func(c *zapcore.EncoderConfig) { c.MessageKey = zapcore.OmitKey }))
		assert.NotContains(t, out, "m ")
		assert.Contains(t, out, "a=1")
	})
	t.Run("PlainLevelEncoder", func(t *testing.T) {
		out := stripANSI(encode(t, func(c *zapcore.EncoderConfig) { c.EncodeLevel = zapcore.CapitalLevelEncoder }))
		assert.Contains(t, out, "INFO")
	})
	t.Run("TimeFormat", func(t *testing.T) {
		cfg := NewEncoderConfig()
		cfg.EncodeTime = DefaultTimeEncoder(time.RFC3339)
		buf, err := NewEncoder(cfg).EncodeEntry(entry, nil)
		require.NoError(t, err)
		defer buf.Free()
		assert.Contains(t, stripANSI(buf.String()), "1970-01-01T00:00:00Z")
	})
}

// TestSugaredLogger exercises the encoder the way sugared-logger users hold
// it, including the documented FormattedStringValue helper.
func TestSugaredLogger(t *testing.T) {
	cfg := NewEncoderConfig()
	cfg.TimeKey = zapcore.OmitKey
	sink := &testBufferWriterSync{}
	sugar := zap.New(zapcore.NewCore(NewEncoder(cfg), sink, zap.NewAtomicLevel())).Sugar()

	sugar.Infow("sugar message", "k", "v", "n", 1, "pretty", FormattedStringValue("a\nb"))
	out := stripANSI(sink.buf.String())
	assert.Contains(t, out, "sugar message")
	assert.Contains(t, out, "k=v")
	assert.Contains(t, out, "n=1")
	assert.Contains(t, out, "pretty=a\n")

	sink.buf.Reset()
	sugar.Errorw("failed", "err", errors.New("boom"))
	assert.Contains(t, stripANSI(sink.buf.String()), "err=boom")
}

// TestLoggerIntegration holds the encoder the way a production zap setup
// does: AddCaller, AddStacktrace, named loggers and accumulated fields.
func TestLoggerIntegration(t *testing.T) {
	cfg := NewEncoderConfig()
	cfg.TimeKey = zapcore.OmitKey
	cfg.CallerKey = "C"
	cfg.FunctionKey = "F"
	sink := &testBufferWriterSync{}
	logger := zap.New(zapcore.NewCore(NewEncoder(cfg), sink, zap.NewAtomicLevel()),
		zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))

	logger.Named("svc").Named("sub").With(zap.String("req", "42")).Error("kaboom")
	out := stripANSI(sink.buf.String())
	assert.Contains(t, out, "svc.sub")
	assert.Contains(t, out, "_test.go:", "AddCaller should render this file")
	assert.Contains(t, out, "TestLoggerIntegration", "FunctionKey should render the calling function")
	assert.Contains(t, out, "req=42")
	assert.Contains(t, out, "stacktrace=")
}

// TestFieldSortingStability locks in the ordering contract of
// sortFieldSegments: alphabetical within namespace segments, multi-line
// types pushed to the back, namespaces never reordered - and, now that the
// sort is an insertion sort, stability: equal keys keep their caller order.
func TestFieldSortingStability(t *testing.T) {
	t.Run("DuplicateKeysKeepOrder", func(t *testing.T) {
		out := encodePlain(t,
			zap.String("dup", "first"),
			zap.String("dup", "second"),
			zap.String("aaa", "third"),
		)
		first := strings.Index(out, "dup=first")
		second := strings.Index(out, "dup=second")
		require.NotEqual(t, -1, first)
		require.NotEqual(t, -1, second)
		assert.Less(t, first, second, "equal keys must keep caller order (stable sort)")
	})
	t.Run("TrailingNamespace", func(t *testing.T) {
		out := encodePlain(t, zap.String("b", "2"), zap.String("a", "1"), zap.Namespace("tail"))
		assert.Contains(t, out, "a=1 b=2")
		assert.Contains(t, out, "tail")
	})
	t.Run("OnlyNamespace", func(t *testing.T) {
		out := encodePlain(t, zap.Namespace("ns"))
		assert.Contains(t, out, "ns")
	})
	t.Run("MultilineTypesPushedBack", func(t *testing.T) {
		out := encodePlain(t,
			zap.Error(errors.New("boom")),
			zap.Object("obj", testStableMap{"k": "v"}),
			zap.Array("arr", testArray{1}),
			zap.String("zzz", "scalar"),
		)
		// scalar first despite key order, then array, object, error
		zzz, arr := strings.Index(out, "zzz="), strings.Index(out, "arr=")
		obj, errIdx := strings.Index(out, "obj"), strings.Index(out, "error=")
		assert.True(t, zzz < arr && arr < obj && obj < errIdx,
			"expected scalar < array < object < error, got %q", out)
	})
}
