package prettyconsole

import (
	"errors"
	"fmt"
	"io"
	"log"
	"testing"
	"time"

	"go.uber.org/multierr"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func BenchmarkWithoutFields(b *testing.B) {
	b.Logf("Logging without any structured context.")
	b.Run("Zap", func(b *testing.B) {
		logger := newZapLogger(zap.DebugLevel)
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Info(getMessage(0))
			}
		})
	})
	b.Run("Zap.Sugar", func(b *testing.B) {
		logger := newZapLogger(zap.DebugLevel).Sugar()
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Info(getMessage(0))
			}
		})
	})
	b.Run("Zap.SugarFormatting", func(b *testing.B) {
		logger := newZapLogger(zap.DebugLevel).Sugar()
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Infof("%v %v %v %s %v %v %v %v %v %s\n", fakeFmtArgs()...)
			}
		})
	})
	b.Run("Zap.Pretty", func(b *testing.B) {
		logger := newZapLogger(zap.DebugLevel)
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Info(getMessage(0))
			}
		})
	})
	b.Run("Zap.Pretty.Sugar", func(b *testing.B) {
		logger := newZapLogger(zap.DebugLevel).Sugar()
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Info(getMessage(0))
			}
		})
	})
	b.Run("Zap.Pretty.SugarFormatting", func(b *testing.B) {
		logger := newZapLogger(zap.DebugLevel).Sugar()
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Infof("%v %v %v %s %v %v %v %v %v %s\n", fakeFmtArgs()...)
			}
		})
	})
	b.Run("stdlib.Println", func(b *testing.B) {
		logger := log.New(io.Discard, "", log.LstdFlags)
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Println(getMessage(0))
			}
		})
	})
	b.Run("stdlib.Printf", func(b *testing.B) {
		logger := log.New(io.Discard, "", log.LstdFlags)
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Printf("%v %v %v %s %v %v %v %v %v %s\n", fakeFmtArgs()...)
			}
		})
	})
}

func BenchmarkAccumulatedContext(b *testing.B) {
	b.Logf("Logging with some accumulated context.")
	b.Run("Zap", func(b *testing.B) {
		logger := newZapLogger(zap.DebugLevel).With(fakeFields()...)
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Info(getMessage(0))
			}
		})
	})
	b.Run("Zap.Sugar", func(b *testing.B) {
		logger := newZapLogger(zap.DebugLevel).With(fakeFields()...).Sugar()
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Info(getMessage(0))
			}
		})
	})
	b.Run("Zap.SugarFormatting", func(b *testing.B) {
		logger := newZapLogger(zap.DebugLevel).With(fakeFields()...).Sugar()
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Infof("%v %v %v %s %v %v %v %v %v %s\n", fakeFmtArgs()...)
			}
		})
	})
	b.Run("Zap.Pretty", func(b *testing.B) {
		logger := newPrettyZapLogger(zap.DebugLevel).With(fakeFields()...)
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Info(getMessage(0))
			}
		})
	})
	b.Run("Zap.Pretty.Sugar", func(b *testing.B) {
		logger := newPrettyZapLogger(zap.DebugLevel).With(fakeFields()...).Sugar()
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Info(getMessage(0))
			}
		})
	})
	b.Run("Zap.Pretty.SugarFormatting", func(b *testing.B) {
		logger := newPrettyZapLogger(zap.DebugLevel).With(fakeFields()...).Sugar()
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Infof("%v %v %v %s %v %v %v %v %v %s\n", fakeFmtArgs()...)
			}
		})
	})
}

func BenchmarkAddingFields(b *testing.B) {
	b.Logf("Logging with additional context at each log site.")
	b.Run("Zap", func(b *testing.B) {
		logger := newZapLogger(zap.DebugLevel)
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Info(getMessage(0), fakeFields()...)
			}
		})
	})
	b.Run("Zap.Sugar", func(b *testing.B) {
		logger := newZapLogger(zap.DebugLevel).Sugar()
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Infow(getMessage(0), fakeSugarFields()...)
			}
		})
	})
	b.Run("Zap.Pretty", func(b *testing.B) {
		logger := newPrettyZapLogger(zap.DebugLevel)
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Info(getMessage(0), fakeFields()...)
			}
		})
	})
	b.Run("Zap.Pretty.Sugar", func(b *testing.B) {
		logger := newPrettyZapLogger(zap.DebugLevel).Sugar()
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				logger.Infow(getMessage(0), fakeSugarFields()...)
			}
		})
	})
}

var (
	errExample = errors.New("fail")

	_messages   = fakeMessages(1000)
	_tenInts    = []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 0}
	_tenStrings = []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"}
	_tenTimes   = []time.Time{
		time.Unix(0, 0),
		time.Unix(1, 0),
		time.Unix(2, 0),
		time.Unix(3, 0),
		time.Unix(4, 0),
		time.Unix(5, 0),
		time.Unix(6, 0),
		time.Unix(7, 0),
		time.Unix(8, 0),
		time.Unix(9, 0),
	}
	_oneUser = &user{
		Name:      "Jane Doe",
		Email:     "jane@test.com",
		CreatedAt: time.Date(1980, 1, 1, 12, 0, 0, 0, time.UTC),
	}
	_tenUsers = users{
		_oneUser,
		_oneUser,
		_oneUser,
		_oneUser,
		_oneUser,
		_oneUser,
		_oneUser,
		_oneUser,
		_oneUser,
		_oneUser,
	}
)

func fakeMessages(n int) []string {
	messages := make([]string, n)
	for i := range messages {
		messages[i] = fmt.Sprintf("Test logging, but use a somewhat realistic message length. (#%v)", i)
	}
	return messages
}

func getMessage(iter int) string {
	return _messages[iter%1000]
}

type users []*user

func (uu users) MarshalLogArray(arr zapcore.ArrayEncoder) error {
	var err error
	for i := range uu {
		err = multierr.Append(err, arr.AppendObject(uu[i]))
	}
	return err
}

type user struct {
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

func (u *user) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("name", u.Name)
	enc.AddString("email", u.Email)
	enc.AddInt64("createdAt", u.CreatedAt.UnixNano())
	return nil
}

func newZapLogger(lvl zapcore.Level) *zap.Logger {
	ec := zap.NewProductionEncoderConfig()
	ec.EncodeDuration = zapcore.NanosDurationEncoder
	ec.EncodeTime = zapcore.EpochNanosTimeEncoder
	enc := zapcore.NewJSONEncoder(ec)
	return zap.New(zapcore.NewCore(
		enc,
		&Discarder{},
		lvl,
	))
}

func newPrettyZapLogger(lvl zapcore.Level) *zap.Logger {
	ec := NewEncoderConfig()
	ec.EncodeDuration = zapcore.NanosDurationEncoder
	ec.EncodeTime = zapcore.EpochNanosTimeEncoder
	enc := NewEncoder(ec)
	return zap.New(zapcore.NewCore(
		enc,
		&Discarder{},
		lvl,
	))
}

func newSampledLogger(lvl zapcore.Level) *zap.Logger {
	return zap.New(zapcore.NewSamplerWithOptions(
		newZapLogger(zap.DebugLevel).Core(),
		100*time.Millisecond,
		10, // first
		10, // thereafter
	))
}

func fakeFields() []zap.Field {
	return []zap.Field{
		zap.Int("int", _tenInts[0]),
		zap.Ints("ints", _tenInts),
		zap.String("string", _tenStrings[0]),
		zap.Strings("strings", _tenStrings),
		zap.Time("time", _tenTimes[0]),
		zap.Times("times", _tenTimes),
		zap.Object("user1", _oneUser),
		zap.Object("user2", _oneUser),
		zap.Array("users", _tenUsers),
		zap.Error(errExample),
	}
}

func fakeSugarFields() []interface{} {
	return []interface{}{
		"int", _tenInts[0],
		"ints", _tenInts,
		"string", _tenStrings[0],
		"strings", _tenStrings,
		"time", _tenTimes[0],
		"times", _tenTimes,
		"user1", _oneUser,
		"user2", _oneUser,
		"users", _tenUsers,
		"error", errExample,
	}
}

func fakeFmtArgs() []interface{} {
	// Need to keep this a function instead of a package-global var so that we
	// pay the cast-to-interface{} penalty on each call.
	return []interface{}{
		_tenInts[0],
		_tenInts,
		_tenStrings[0],
		_tenStrings,
		_tenTimes[0],
		_tenTimes,
		_oneUser,
		_oneUser,
		_tenUsers,
		errExample,
	}
}

// A Syncer is a spy for the Sync portion of zapcore.WriteSyncer.
type Syncer struct {
	err    error
	called bool
}

// SetError sets the error that the Sync method will return.
func (s *Syncer) SetError(err error) {
	s.err = err
}

// Sync records that it was called, then returns the user-supplied error (if
// any).
func (s *Syncer) Sync() error {
	s.called = true
	return s.err
}

// Called reports whether the Sync method was called.
func (s *Syncer) Called() bool {
	return s.called
}

// A Discarder sends all writes to io.Discard.
type Discarder struct{ Syncer }

// Write implements io.Writer.
func (d *Discarder) Write(b []byte) (int, error) {
	return io.Discard.Write(b)
}
