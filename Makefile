BINARY := shellrecap
PKG    := github.com/ksauraj/shellrecap

# The API keys are optional: without them the binary falls back to the
# GEMINI_API_KEY and GROQ_API_KEY environment variables at runtime. Built in
# keys are sealed first, so they aren't stored in the binary as plain text,
# and the recipe isn't echoed, so they don't end up in build logs.
export GEMINI_API_KEY GROQ_API_KEY

.PHONY: build run test clean

build:
	@gemini=$$(go run ./internal/secret/sealkey GEMINI_API_KEY) && \
	groq=$$(go run ./internal/secret/sealkey GROQ_API_KEY) && \
	echo "go build -o $(BINARY) ./cmd/shellrecap" && \
	go build -ldflags "-s -w -X $(PKG)/internal/ai.geminiAPIKey=$$gemini -X $(PKG)/internal/ai.groqAPIKey=$$groq" \
		-o $(BINARY) ./cmd/shellrecap

run: build
	./$(BINARY)

test:
	go test ./...

clean:
	rm -f $(BINARY)
