package main

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/thessem/zap-prettyconsole"
	"go.uber.org/multierr"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
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

func Example_object() {
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

func Example_simple() {
	logger := prettyconsole.NewLogger(zap.DebugLevel)
	logger.Info("doesn't this look nice",
		zap.Complex64("nice_level", 12i-14),
		zap.Bools("nice_enough", []bool{true, false}),
	)
}

func Example_normal() {
	logger, _ := zap.NewDevelopment()
	logger.Info("old logger output for reference",
		zap.Complex64("nice_level", 12i-14),
		zap.Bools("nice_enough", []bool{true, false}),
	)
	fmt.Println("\n\n\n\n\n")
}

func Example_configuration() {
	cfg := prettyconsole.NewConfig()
	cfg.EncoderConfig.CallerKey = zapcore.OmitKey
	cfg.EncoderConfig.TimeKey = zapcore.OmitKey
	cfg.EncoderConfig.ConsoleSeparator = "   "
	logger, _ := cfg.Build()
	logger.Debug("it's configurable!", zap.Strings("and", []string{"you", "can", "space", "it", "out"}))
}

func Example_reflection() {
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

func Example_errors() {
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
