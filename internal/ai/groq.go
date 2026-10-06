// internal/ai/groq.go
package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

// groqAPIKey is set at build time with
// -ldflags "-X github.com/ksauraj/shellrecap/internal/ai.groqAPIKey=..."
// and falls back to the GROQ_API_KEY environment variable
var groqAPIKey string

// groqAPIURL is a variable so tests can point it at a fake server
var groqAPIURL = "https://api.groq.com/openai/v1/chat/completions"

// groqStrictModels support strict structured outputs, which guarantee the
// response matches the schema. Other models get plain JSON mode.
var groqStrictModels = map[string]bool{
	"openai/gpt-oss-120b": true,
	"openai/gpt-oss-20b":  true,
	"qwen/qwen3.8-27b":    true,
}

type groqProvider struct{}

func (groqProvider) key() string {
	if groqAPIKey != "" {
		return groqAPIKey
	}
	return os.Getenv("GROQ_API_KEY")
}

// defaultModels prefers Qwen, which writes the most personal slides of the
// free plan's models, and falls back to gpt-oss-120b, the most capable
// production model, since Qwen is only a preview model
func (groqProvider) defaultModels() []string {
	return []string{"qwen/qwen3.8-27b", "openai/gpt-oss-120b"}
}

func (g groqProvider) generate(model, system, user string) (string, error) {
	payload := map[string]interface{}{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		},
		// Keeps each request well inside the free plan's 8K tokens a minute
		"max_completion_tokens": 2048,
	}
	if groqStrictModels[model] {
		payload["response_format"] = map[string]interface{}{
			"type": "json_schema",
			"json_schema": map[string]interface{}{
				"name":   "recap_slides",
				"strict": true,
				"schema": slidesSchema,
			},
		}
	} else {
		payload["response_format"] = map[string]interface{}{"type": "json_object"}
	}
	if strings.HasPrefix(model, "openai/gpt-oss") {
		// Short slides don't need long deliberation, and reasoning tokens
		// count against the rate limits
		payload["reasoning_effort"] = "low"
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal payload: %v", err)
	}
	req, err := http.NewRequest("POST", groqAPIURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+g.key())

	resp, err := (&http.Client{Timeout: requestTimeout}).Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", apiError("groq", resp.StatusCode, raw)
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", fmt.Errorf("failed to decode response: %v", err)
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("groq returned no choices")
	}
	return result.Choices[0].Message.Content, nil
}
