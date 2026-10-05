BINARY := shellrecap
PKG    := github.com/ksauraj/shellrecap

# The Gemini key is optional: without it the binary falls back to the
# GEMINI_API_KEY environment variable at runtime
LDFLAGS :=
ifneq ($(GEMINI_API_KEY),)
LDFLAGS += -X $(PKG)/internal/gemini.apiKey=$(GEMINI_API_KEY)
endif

.PHONY: build run test clean

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/shellrecap

run: build
	./$(BINARY)

test:
	go test ./...

clean:
	rm -f $(BINARY)
