//go:build go1.20

package main

import (
	"errors"
	"fmt"
	"testing"

	prettyconsole "github.com/thessem/zap-prettyconsole"
	"go.uber.org/zap"
)

func TestGoErrors(t *testing.T) {
	logger := prettyconsole.NewLogger(zap.DebugLevel)
	joinedErr := errors.Join(
		errors.New("1/2 things went wrong"),
		errors.New("2/2 things went wrong"),
	)
	logger.Error("only joined error", zap.Error(joinedErr))

	fmtErr := fmt.Errorf("two errors: %w and %w", errors.New("first error"), errors.New("second error"))
	logger.Error("fmt errors", zap.Error(fmtErr))

	fmtAndJoinedErr := fmt.Errorf("two errors: %w and %w", joinedErr, fmtErr)
	logger.Error("fmt and joined errors", zap.Error(fmtAndJoinedErr))
}
