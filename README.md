# Pretty Console Output for Zap

This is an encoder for Uber's [zap][zap] logger that makes complex log output 
easily readable by humans. This is intended for development work where you 
don't have very many log messages and quickly understanding them is important.

This package takes particular care to represent structural information with 
indents and colours. Any `\n` that get escaped and printed are coloured, and 
any objects that are printed have their fields indented.

The indenting method tries to save space where it can, for example if there 
are multiple fields in a struct, this logger will still attempt to print 
them on a single line.

![example](https://github.com/thessem/zap-prettyconsole/blob/main/doc/example.png?raw=true)

```go
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
```

This package is benchmarked and (blazingly fast) performance is roughly the same
as the default zap production encoder.

This is not suitable for production, unless you never intend to automatically
parse your own logs. This encoder breaks assumptions that many log parsing
systems will make. It is not easily `grep`able for example. Luckily zap makes it
easy to switch to a different encoder in production 😁

## Current Status
This project is mostly done. The main branch should be in a working state, but
it may have history edits. There is currently no documentation. There may be
nasty bugs lurking inside, I haven't had much of a chance to use this personally
yet!

Released under the [MIT License](LICENSE.txt)

[zap]: https://github.com/uber-go/zap
