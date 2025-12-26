BENCH_FLAGS ?= -cpuprofile=cpu.pprof -memprofile=mem.pprof -benchmem

.PHONY: help
help:
	@echo "Available targets:"
	@echo "  all        - Run tests (default)"
	@echo "  test       - Run tests with race detector"
	@echo "  coverage   - Run tests with coverage report"
	@echo "  lint       - Run golangci-lint"
	@echo "  fmt        - Format code with gofumpt"
	@echo "  bench      - Run benchmarks (BENCH=pattern)"
	@echo "  tidy       - Tidy go modules"
	@echo "  clean      - Remove generated artifacts"
	@echo "  README.md  - Generate README"

.PHONY: all
all: test

.PHONY: test
test:
	go test -race ./...

.PHONY: coverage
coverage:
	go test -race -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -func=coverage.out
	@echo ""
	@echo "To view HTML coverage report, run:"
	@echo "  go tool cover -html=coverage.out"

.PHONY: lint
lint:
	golangci-lint run

.PHONY: fmt
fmt:
	gofumpt -w .

.PHONY: bench
BENCH ?= .
bench:
	go test -bench=$(BENCH) -run="^$$" $(BENCH_FLAGS)

.PHONY: tidy
tidy:
	go mod tidy

.PHONY: clean
clean:
	rm -f cpu.pprof mem.pprof coverage.out
	rm -rf ./internal/readme/images/

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
