BINARY := shellrecap
PKG    := github.com/ksauraj/shellrecap

# The API keys are optional: without them the binary falls back to the
# GEMINI_API_KEY and GROQ_API_KEY environment variables at runtime
LDFLAGS :=
ifneq ($(GEMINI_API_KEY),)
LDFLAGS += -X $(PKG)/internal/ai.geminiAPIKey=$(GEMINI_API_KEY)
endif
ifneq ($(GROQ_API_KEY),)
LDFLAGS += -X $(PKG)/internal/ai.groqAPIKey=$(GROQ_API_KEY)
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
