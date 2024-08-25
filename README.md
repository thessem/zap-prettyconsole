# :zap: :nail_care: zap-prettyconsole - Pretty Console Output For Zap

An encoder for Uber's [zap] logger that makes complex structured log output easily readable by humans.
It prioritises displaying information in a clean and easy-to-understand way.

You get good-looking, colourised output like this:
![singleLine](https://github.com/thessem/zap-prettyconsole/blob/main/internal/readme/images/SingleLine.png?raw=true)

When your log output contains structured data, it looks like this:
![simple](https://github.com/thessem/zap-prettyconsole/blob/main/internal/readme/images/Simple.png?raw=true)

I personally feel the above is much more readable than the default "development" logger that comes with zap, which looks like this:
![zap_console](https://github.com/thessem/zap-prettyconsole/blob/main/internal/readme/images/ZapConsole.png?raw=true)

The above example was generated using:
```go
package main

import (
	"go.uber.org/zap"
	"github.com/thessem/zap-prettyconsole"
)

func main() {
	// Instead of zap.NewDevelopment()
	logger := prettyconsole.NewLogger(zap.DebugLevel)

	// I normally use the un-sugared logger. I like the type-safety.
	// You can use logger.Sugar() if you prefer though!
	logger.Info("doesn't this look nice",
		zap.Complex64("nice_level", 12i-14),
		zap.Time("the_time", time.Now()),
		zap.Bools("nice_enough", []bool{true, false}),
		zap.Namespace("an_object"),
		zap.String("field_1", "value_1"),
		zap.String("field_2", "value_2"),
	)
}
```

This is intended as a tool for local development, and not for running in production.
In production I reccomend you use zap's built in JSON mode.
Your logs in production will be getting parsed by computers, not humans, after-all.
Take a look at the [zap advanced configuration] example to configure zap to output "human" output locally, and "machine" output in production.

This package takes particular care to represent structural information with indents and newlines (slightly YAML style), hopefully making it easy to figure out what each key-value belongs to:
![object](https://github.com/thessem/zap-prettyconsole/blob/main/internal/readme/images/Object.png?raw=true)
which is the output of
```go
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

func main() {
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
```

This encoder was inspired by trying to parse multiple `github.com/pkg/errors/` errors, each with their own stacktraces.
I am a big fan of error wrapping and error stacktraces, I am not a fan of needing to copy text out of my terminal to see what happened.
![errors](https://github.com/thessem/zap-prettyconsole/blob/main/internal/readme/images/Errors.png?raw=true)

When objects that do not satisfy `ObjectMarshaler` are logged, zap-prettyconsole will use reflection (via the delightful [dd][dd] library) to print it instead:
![reflection](https://github.com/thessem/zap-prettyconsole/blob/main/internal/readme/images/Reflection.png?raw=true)

Strings passed to the logger will have their formatting printed and colourised, but you can opt out of this and print the raw strings.

![formatting](https://github.com/thessem/zap-prettyconsole/blob/main/internal/readme/images/Formatting.png?raw=true)

Generated using:

```go
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
```

This encoder respects all the normal encoder configuration settings.
You can change your separator character, newline characters, add caller/function information and add stacktraces if you like.

![configuration](https://github.com/thessem/zap-prettyconsole/blob/main/internal/readme/images/Configuration.png?raw=true)

## Performance

Whilst this library is described as "development mode" it is still coded to be as performant as possible, saving your CPU cycles for running lots of IDE plugins.

The main performance overhead introduced with this encoder is because of the stable field ordering, we sort every structured log field alphabetically.
Although the relative overhead is high, the absolute overhead is still quite small, and probably wont matter for development logging anyway!

Log a message and 10 fields:

| Package | Time | Time % to zap | Objects Allocated |
| :------ | :--: | :-----------: | :---------------: |
| :zap: zap | 562 ns/op | +0% | 5 allocs/op
| :zap: zap (sugared) | 877 ns/op | +56% | 10 allocs/op
| :zap: :nail_care: zap-prettyconsole | 1948 ns/op | +247% | 11 allocs/op
| :zap: :nail_care: zap-prettyconsole (sugared) | 2441 ns/op | +334% | 16 allocs/op

Log a message with a logger that already has 10 fields of context:

| Package | Time | Time % to zap | Objects Allocated |
| :------ | :--: | :-----------: | :---------------: |
| :zap: zap | 48 ns/op | +0% | 0 allocs/op
| :zap: zap (sugared) | 66 ns/op | +38% | 1 allocs/op
| :zap: :nail_care: zap-prettyconsole | 1627 ns/op | +3290% | 6 allocs/op
| :zap: :nail_care: zap-prettyconsole (sugared) | 1646 ns/op | +3329% | 7 allocs/op

Log a static string, without any context or `printf`-style templating:

| Package | Time | Time % to zap | Objects Allocated |
| :------ | :--: | :-----------: | :---------------: |
| :zap: zap | 51 ns/op | +0% | 0 allocs/op
| :zap: :nail_care: zap-prettyconsole | 54 ns/op | +6% | 0 allocs/op
| :zap: zap (sugared) | 62 ns/op | +22% | 1 allocs/op
| :zap: :nail_care: zap-prettyconsole (sugared) | 62 ns/op | +22% | 1 allocs/op

Released under the [MIT License](LICENSE.txt)

[zap]: https://github.com/uber-go/zap
[zap advanced configuration]: https://pkg.go.dev/go.uber.org/zap#example-package-AdvancedConfiguration
[dd]: https://github.com/Code-Hex/dd
