package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	prettyconsole "github.com/thessem/zap-prettyconsole"
)

type User struct {
	Name    string
	Age     int
	Address UserAddress
	Friend  *User
}

type UserAddress struct {
	Street string
	City   string
}

func (u *User) MarshalLogObject(e zapcore.ObjectEncoder) error {
	e.AddString("name", u.Name)
	e.AddInt("age", u.Age)
	if u.Friend != nil {
		_ = e.AddObject("friend", u.Friend)
	}
	e.OpenNamespace("address")
	e.AddString("street", u.Address.Street)
	e.AddString("city", u.Address.City)
	return nil
}

func TestObject(t *testing.T) {
	logger, _ := prettyconsole.NewConfig().Build()
	sugarLogger := logger.Sugar()

	u := &User{
		Name: "Big Bird",
		Age:  18,
		Address: UserAddress{
			Street: "Sesame Street",
			City:   "New York",
		},
		Friend: &User{
			Name: "Oscar the Grouch",
			Age:  31,
			Address: UserAddress{
				Street: "Wallaby Way",
				City:   "Sydney",
			},
		},
	}

	sugarLogger.Infow("Asking a Question",
		"question", "how do you get to sesame street?",
		"answer", "unsatisfying",
		"user", u,
	)
}

func TestSingleLine(t *testing.T) {
	logger := prettyconsole.NewLogger(zap.DebugLevel)
	logger.Info("doesn't this look nice",
		zap.Complex64("nice_level", 12i-14),
		zap.Time("the_time", time.Now()),
	)
}

func TestSimple(t *testing.T) {
	logger := prettyconsole.NewLogger(zap.DebugLevel)
	logger.Info("doesn't this look nice",
		zap.Complex64("nice_level", 12i-14),
		zap.Time("the_time", time.Now()),
		zap.Bools("nice_enough", []bool{true, false}),
		zap.Namespace("an_object"),
		zap.String("field_1", "value_1"),
		zap.String("field_2", "value_2"),
	)
}

func TestZapConsole(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	logger.Info("doesn't this look nice",
		zap.Complex64("nice_level", 12i-14),
		zap.Time("the_time", time.Now()),
		zap.Bools("nice_enough", []bool{true, false}),
		zap.Namespace("an_object"),
		zap.String("field_1", "value_1"),
		zap.String("field_2", "value_2"),
	)
}

func TestConfiguration(t *testing.T) {
	cfg := prettyconsole.NewConfig()
	cfg.EncoderConfig.CallerKey = zapcore.OmitKey
	cfg.EncoderConfig.TimeKey = zapcore.OmitKey
	cfg.EncoderConfig.ConsoleSeparator = "   "
	cfg.EncoderConfig.LineEnding = "\n\n"
	logger1, _ := cfg.Build()
	cfg.EncoderConfig.CallerKey = "C"
	cfg.EncoderConfig.FunctionKey = "F"
	logger2, _ := cfg.Build()

	logger1.Debug("it's configurable!", zap.Strings("and", []string{"you", "can", "space", "it", "out"}))
	logger2.Warn("you can also add more information")
}

func TestReflection(t *testing.T) {
	logger := prettyconsole.NewLogger(zap.DebugLevel)
	sugar := logger.Sugar()
	sugar.Debugw("reflection uses",
		"library", "github.com/Code-Hex/dd",
		"status", "lovely",
		"reflected", struct {
			Foo int
			Bar bool
		}{42, true},
	)
}

func TestFormatting(t *testing.T) {
	logger := prettyconsole.NewLogger(zap.DebugLevel)
	// Non-sugared version
	logger = logger.With(prettyconsole.FormattedString("sql", "SELECT * FROM\n\tusers\nWHERE\n\tname = 'James'"))
	sugar := logger.Sugar()
	mdb := "db.users.find({\n\tname: \"\x1b[31mJames\x1b[0m\"\n});"
	sugar.Debugw("string formatting",
		zap.Namespace("mdb"),
		// Sugared version
		"formatted", prettyconsole.FormattedStringValue(mdb),
		"unformatted", mdb,
	)
}

func TestErrors(t *testing.T) {
	logger := prettyconsole.NewLogger(zap.DebugLevel).WithOptions()
	err := errors.Wrap(
		errors.Wrap(
			multierr.Combine(
				fmt.Errorf("1/2 things went wrong"),
				fmt.Errorf("2/2 things went wrong"),
			),
			"because something deeper happened",
		),
		"something happened here",
	)
	logger.WithOptions(zap.WithCaller(true)).
		Error("something went wrong",
			zap.Error(err),
		)
}

// TraceContext simulates an OpenTelemetry trace context with byte array fields
type TraceContext struct {
	TraceID [16]byte // 128-bit trace ID
	SpanID  [8]byte  // 64-bit span ID
	Flags   byte     // Trace flags
}

func TestOTelTracing(t *testing.T) {
	logger := prettyconsole.NewLogger(zap.DebugLevel)
	sugar := logger.Sugar()

	// Simulate an OpenTelemetry trace context
	traceCtx := TraceContext{
		TraceID: [16]byte{
			0x4b, 0xf9, 0x2f, 0x35, 0x77, 0xb3, 0x4d, 0xa6,
			0xa3, 0xce, 0x92, 0x9d, 0x0e, 0x0e, 0x47, 0x21,
		},
		SpanID: [8]byte{0x00, 0xf0, 0x67, 0xaa, 0x0b, 0xa9, 0x02, 0xb7},
		Flags:  1,
	}

	sugar.Infow("Processing request with trace context",
		"method", "GET",
		"path", "/api/users",
		"trace", traceCtx,
	)
}
