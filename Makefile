BENCH_FLAGS ?= -cpuprofile=cpu.pprof -memprofile=mem.pprof -benchmem

.PHONY: all
all: test

.PHONY: test
test:
	go test -race ./...

.PHONY: bench
BENCH ?= .
bench:
	go test -bench=$(BENCH) -run="^$$" $(BENCH_FLAGS)

.PHONY: tidy
tidy:
	 go mod tidy

./internal/readme/images/%.png: ./internal/readme/example_test.go
	@mkdir -p ./internal/readme/images/
	@if [ "$*" = "ZapConsole" ]; then\
			termshot -f $@ -- 'go test $^ -run=$* -v | sed -e "/---/,+10d" -e "/===/,1d" | fold -w 80';\
	else\
			termshot -f $@ -- 'go test $^ -run=$* -v | sed -e "/---/,+10d" -e "/===/,1d"';\
	fi

readme_images := $(shell go test ./internal/readme/example_test.go -v --list=. | sed -n '/^Test/p' | sed 's/Test//')
README.md: ./internal/readme/readme.tmpl $(addprefix ./internal/readme/images/,$(addsuffix .png,$(readme_images)))
	cat internal/readme/readme.tmpl | go run internal/readme/readme.go > README.md
