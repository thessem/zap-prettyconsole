package prettyconsole

import (
	"fmt"
	"io"
	"sync"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// TestConcurrentLogging exercises the encoder pools from many goroutines.
// It exists for the race detector: encoder or buffer reuse bugs show up
// here as data races or corrupted output, not as assertion failures.
func TestConcurrentLogging(t *testing.T) {
	enc := NewEncoder(NewEncoderConfig())
	logger := zap.New(zapcore.NewCore(enc, zapcore.Lock(zapcore.AddSync(io.Discard)), zap.NewAtomicLevel()))

	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			l := logger.With(zap.Int("goroutine", g), zap.Namespace("ns"), zap.String("k", "v"))
			for i := 0; i < 200; i++ {
				l.Info("message",
					zap.Int("i", i),
					zap.Object("obj", testStableMap{"a": 1, "b": "two"}),
					zap.Array("arr", testArray{1, "x", testArray{2}}),
					zap.Error(fmt.Errorf("wrapped: %w", errFixture)),
				)
			}
		}(g)
	}
	wg.Wait()
}

var errFixture = fmt.Errorf("fixture")
