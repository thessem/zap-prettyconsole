package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/thessem/zap-prettyconsole"
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
	e.OpenNamespace("address")
	e.AddString("street", u.Address.Street)
	e.AddString("city", u.Address.City)
	if u.Friend != nil {
		_ = e.AddObject("friend", u.Friend)
	}
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
