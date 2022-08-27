# :zap: :nail_care: zap-prettyconsole - Pretty Console Output For zap

This is an encoder for Uber's [zap][zap] logger that makes complex log output
easily readable by humans. This is a logger for developers and prioritises
displaying information in a clean and easy to understand way.

This is intended as a development tool, for the same reason zap provide a
"development" logger and a "production" logger. It will hopefully make
figuring out what your program is doing while it's sitting on your local
computer easier, and the power of zap's JSON output will make it easy to
monitor your application while it's running remotely.

![simple](https://github.com/thessem/zap-prettyconsole/blob/main/internal/readme/simple.png?raw=true)
```go
package main

import (
	"go.uber.org/zap"
	"github.com/thessem/zap-prettyconsole"
)

func main() {
	logger := prettyconsole.NewLogger(zap.DebugLevel)
    logger.Info("doesn't this look nice",
        zap.Complex64("nice_level", 12i-14),
        zap.Bools("nice_enough", []bool{true, false}),
    )
}
```

<br><br>
The above can be compared to using the zap logger in its normal development
mode, which just shows you the data in a JSON string. This quickly gets
hard to read!
![normal](https://github.com/thessem/zap-prettyconsole/blob/main/internal/readme/normal.png?raw=true)

This package takes particular care to represent structural information with
indents and newlines (slightly YAML style), hopefully making it easy to figure
out what each key-value belongs to:
![object](https://github.com/thessem/zap-prettyconsole/blob/main/internal/readme/object.png?raw=true)
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
	e.OpenNamespace("address")
	e.AddString("street", u.Address.Street)
	e.AddString("city", u.Address.City)
	if u.Friend != nil {
		_ = e.AddObject("friend", u.Friend)
	}
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

This encoder was inspired by trying to parse multiple `github.com/pkg/errors/`
errors, each with their own stack-traces. I am a big fan of error wrapping and
error stack-traces, I am not a fan of needing to copy text out of my terminal to
see what happened.
![error](https://github.com/thessem/zap-prettyconsole/blob/main/internal/readme/error.png?raw=true)

When objects that do not satisfy `ObjectMarshaler` are logged, zap-prettyconsole
will use reflection (via the delightful [dd][dd] library) to print it instead:
![error](https://github.com/thessem/zap-prettyconsole/blob/main/internal/readme/reflection.png?raw=true)

## Performance

Whilst this library is described as "development mode" it is still coded to be
as performant as possible, saving your CPU cycles for running lots of IDE
plugins.

The extra allocations are mostly due to sorting the fields in alphabetical
order, which I think is worth it as a trade-off.

Log a message and 10 fields:

| Package | Time | Time % to zap | Objects Allocated |
| :------ | :--: | :-----------: | :---------------: |
| :zap: zap | 1048 ns/op | +0% | 5 allocs/op
| :zap: zap (sugared) | 1269 ns/op | +21% | 10 allocs/op
| :zap: :nail_care: zap-prettyconsole | 2792 ns/op | +166% | 11 allocs/op
| :zap: :nail_care: zap-prettyconsole (sugared) | 3153 ns/op | +201% | 16 allocs/op

Log a message with a logger that already has 10 fields of context:

| Package | Time | Time % to zap | Objects Allocated |
| :------ | :--: | :-----------: | :---------------: |
| :zap: zap | 110 ns/op | +0% | 0 allocs/op
| :zap: zap (sugared) | 137 ns/op | +25% | 1 allocs/op
| :zap: :nail_care: zap-prettyconsole | 244 ns/op | +122% | 3 allocs/op
| :zap: :nail_care: zap-prettyconsole (sugared) | 248 ns/op | +125% | 4 allocs/op

Log a static string, without any context or `printf`-style templating:

| Package | Time | Time % to zap | Objects Allocated |
| :------ | :--: | :-----------: | :---------------: |
| :zap: zap | 98 ns/op | +0% | 0 allocs/op
| :zap: zap (sugared) | 141 ns/op | +44% | 1 allocs/op
| :zap: :nail_care: zap-prettyconsole | 110 ns/op | +12% | 0 allocs/op
| :zap: :nail_care: zap-prettyconsole (sugared) | 139 ns/op | +42% | 1 allocs/op

## Current Status
I think this project is mostly working. I have not used it for any serious
development yet. I make no promises about a lack of bugs or memory leaks (but
everything seems okay to me). Happily accepting issues and suggestions.

Released under the [MIT License](LICENSE.txt)

[zap]: https://github.com/uber-go/zap
[dd]: github.com/Code-Hex/dd
