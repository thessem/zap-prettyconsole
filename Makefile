GOLINT = golint
STATICCHECK = staticcheck
BENCH_FLAGS ?= -cpuprofile=cpu.pprof -memprofile=mem.pprof -benchmem

MODULE_DIRS = .

# Many Go tools take file globs or directories as arguments instead of packages.
GO_FILES := $(shell \
	find . '(' -path '*/.*' -o -path './vendor' ')' -prune \
	-o -name '*.go' -print | cut -b3-)

.PHONY: all
all: test

.PHONY: test
test:
	@$(foreach dir,$(MODULE_DIRS),(cd $(dir) && go test -race ./...) &&) true

.PHONY: cover
cover:
	go test -race -coverprofile=cover.out -coverpkg=./... ./...
	go tool cover -html=cover.out -o cover.html

.PHONY: bench
BENCH ?= .
bench:
	@$(foreach dir,$(MODULE_DIRS), ( \
		cd $(dir) && \
		go list ./... | xargs -n1 go test -bench=$(BENCH) -run="^$$" $(BENCH_FLAGS) \
	) &&) true

.PHONY: updatereadme
updatereadme:
	rm -f README.md
	cat .readme.tmpl | go run internal/readme/readme.go > README.md

.PHONY: tidy
tidy:
	@$(foreach dir,$(MODULE_DIRS),(cd $(dir) && go mod tidy) &&) true
