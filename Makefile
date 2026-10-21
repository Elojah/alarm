GO                 = go
GOFMT              = gofmt

# The examples tinygo-run and tinygo-build cover, and the board tinygo-build targets.
TINYGO             = tinygo
TINYGO_TARGET     ?= pico
EXAMPLES           = $(patsubst %/,%,$(wildcard examples/*/))
# Prints the lines under // Output: in an example's main_test.go, without their //.
EXAMPLE_OUTPUT     = awk '/\/\/ Output:/ { f = 1; next } f && /^}/ { f = 0 } f { sub(/^\t\/\/ ?/, ""); print }'

# For CI
ifneq ($(wildcard ./bin/golangci-lint),)
	GOLINT = $(CURDIR)/bin/golangci-lint
else
	GOLINT = golangci-lint
endif

V                 = 0
Q                 = $(if $(filter 1,$V),,@)
M                 = $(shell printf "\033[0;35m▶\033[0m")

.PHONY: all
all: check

# Tidy
.PHONY: tidy
tidy: ## Run go mod tidy
	$(info $(M) running go mod tidy) @
	$Q $(GO) mod tidy

# Check
.PHONY: check
check: lint test ## Run lint and tests

# Lint
.PHONY: lint
lint: ## Run golangci-lint
	$(info $(M) running $(GOLINT))
	$Q $(GOLINT) run

# Test
.PHONY: test
# GOMAXPROCS 1 interleaves goroutines differently from several cores, and -shuffle, whose seed -v prints, catches tests
# that depend on each other.
test: ## Run tests with race detector, shuffled, with GOMAXPROCS 1 and 4
	$(info $(M) running go test) @
	$Q $(GO) test -cover -race -shuffle=on -cpu 1,4 -v ./...

# Bench
.PHONY: bench
# BenchmarkRun_Late waits on the real clock: its ns/op is the period, its metrics how late times fire.
bench: ## Run the benchmarks: the cost of a time, and how late times fire
	$(info $(M) running go test -bench) @
	$Q $(GO) test -run '^$$' -bench . -benchmem ./...

# TinyGo
# testing/synctest, which the timing tests use, does not link with TinyGo, and tinygo test runs no Example. So tinygo-run
# runs each example and checks its output, and tinygo-build only compiles for TINYGO_TARGET.
.PHONY: tinygo
tinygo: tinygo-run tinygo-build ## Run tinygo-run and tinygo-build

.PHONY: tinygo-run
# An example passes if it prints its Example's output block.
tinygo-run: ## Run each example with TinyGo, and diff what it prints with its Example's output
	$(info $(M) running each example with tinygo run) @
	$Q set -e; \
	out=$$(mktemp -d); trap 'rm -rf "$$out"' EXIT; \
	for e in $(EXAMPLES); do \
		echo "$(M) $$e"; \
		$(EXAMPLE_OUTPUT) $$e/main_test.go > "$$out/want"; \
		$(TINYGO) run ./$$e > "$$out/got"; \
		diff -u "$$out/want" "$$out/got"; \
	done

.PHONY: tinygo-build
# -size short prints how much flash and RAM each takes.
tinygo-build: ## Build each example with TinyGo for TINYGO_TARGET (pico), without running it
	$(info $(M) building each example for $(TINYGO_TARGET) with tinygo build) @
	$Q set -e; \
	out=$$(mktemp -d); trap 'rm -rf "$$out"' EXIT; \
	for e in $(EXAMPLES); do \
		echo "$(M) $$e"; \
		$(TINYGO) build -target=$(TINYGO_TARGET) -size short -o "$$out/main.elf" ./$$e; \
	done

## Helpers

.PHONY: go-version
go-version: ## Print go version used in this makefile
	$Q echo $(GO)

.PHONY: fmt
fmt: ## Format code
	$(info $(M) running $(GOFMT)) @
	$Q $(GOFMT) -l -w .

.PHONY: doc
doc: ## Print package documentation
	$(info $(M) running go doc) @
	$Q $(GO) doc -all .

.PHONY: help
help: ## Print this
	@grep -E '^[ a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'
