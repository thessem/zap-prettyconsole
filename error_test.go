package prettyconsole

import (
	"errors"
	"fmt"
	"testing"
	"time"

	pkgerrors "github.com/pkg/errors"
	"go.uber.org/multierr"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type testPanicError string

func (t *testPanicError) Error() string {
	panic("Panic!")
}

type nilCauseError struct{}

func (nilCauseError) Error() string {
	return "Error has nil cause"
}

func (nilCauseError) Cause() error {
	return nil
}

func TestEncodeEntryErrors(t *testing.T) {
	tests := []goldenCase{
		{
			desc: "Minimal Error",
			ent: zapcore.Entry{
				Level:   zapcore.ErrorLevel,
				Message: "test message",
				Time:    time.Date(2018, 6, 19, 16, 33, 42, 99, time.UTC),
			},
			fields: []zapcore.Field{
				zap.Error(errors.New("Something \nwent wrong")),
			},
		},
		{
			desc: "Errors",
			ent: zapcore.Entry{
				Level:   zapcore.ErrorLevel,
				Message: "test message",
				Time:    time.Date(2018, 6, 19, 16, 33, 42, 99, time.UTC),
				Stack:   zap.Stack("ignored").String,
			},

			fields: []zapcore.Field{
				zap.Error(errors.New("something \nwent wrong")),
				zap.NamedError("stack", pkgerrors.New("an error with a stacktrace has occurred")),
				zap.NamedError("nested", pkgerrors.Wrap(
					pkgerrors.Wrapf(multierr.Combine(
						pkgerrors.New("cause 1"),
						pkgerrors.Wrapf(
							multierr.Combine(
								errors.New("deeper cause 1"),
								errors.New("deeper cause 2")),
							"deeper error with two causes"),
					), "error with 2 causes"),
					"error with stacktrace",
				)),
				zap.NamedError("nil_panic", (*testPanicError)(nil)),
				zap.NamedError("normal_panic", &[]testPanicError{"panic!"}[0]),
				zap.Stack("named_stracktrace"),
			},
		},
		{
			desc: "Go v1.20 Errors",
			ent: zapcore.Entry{
				Level:   zapcore.ErrorLevel,
				Message: "test message",
				Time:    time.Date(2018, 6, 19, 16, 33, 42, 99, time.UTC),
				Stack:   zap.Stack("ignored").String,
			},

			fields: []zapcore.Field{
				zap.Error(fmt.Errorf("error with context: %w", errors.New("cause 1"))),
				zap.Error(fmt.Errorf("errors with context: %w, %w", errors.New("cause 1"), errors.New("cause 2"))),
				zap.Error(errors.Join(errors.New("joined cause 1"), errors.New("joined cause 2"))),
				zap.Error(fmt.Errorf("Joined and fmt: %w and %w", errors.Join(fmt.Errorf("joined 1"), fmt.Errorf("joined 2")), fmt.Errorf("fmt error"))),
				zap.NamedError("nil_cause_error", nilCauseError{}),
				zap.NamedError("nill_error", nil),
			},
		},
	}
	runGoldenCases(t, tests)
}
