package main

import (
	"fmt"
	"time"

	"github.com/pkg/errors"
	"github.com/thessem/zap-prettyconsole"
	"go.uber.org/multierr"
	"go.uber.org/zap"
)

func main() {
	logger, _ := prettyconsole.NewConfig().Build()
	sugar := logger.Sugar()
	sugar.Infow("failed to fetch URL",
		"url", "http://github.com",
		"attempt", 3,
		"backoff", time.Second,
	)

	err := errors.Wrap(
		multierr.Combine(
			fmt.Errorf("1/2 things went wrong"),
			fmt.Errorf("2/2 things went wrong"),
		),
		"something happened here",
	)
	sugar.Errorw("something went wrong",
		"error", err,
	)

	sugar.Debugw("newline\nin message?",
		"reflected", struct {
			Foo int
			Bar bool
		}{42, true},
	)

	logger.Warn("namespaces", zap.String("string", "val"), zap.Duration("dur", 42*time.Minute),
		zap.Namespace("some_namespace"), zap.String("s2", "v2"), zap.Int("i2", 2),
		zap.Namespace("deeper_namespace"), zap.String("s3", "v3"), zap.Int("i3", 3),
		zap.Namespace("another_namespace"), zap.Namespace("another_namespace_again"), zap.Bool("the_end", true),
	)
}
