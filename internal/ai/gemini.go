// internal/ai/gemini.go
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

// geminiAPIKey is set at build time with
// -ldflags "-X github.com/ksauraj/shellrecap/internal/ai.geminiAPIKey=..."
// and falls back to the GEMINI_API_KEY environment variable
var geminiAPIKey string

// geminiAPIBase is a variable so tests can point it at a fake server
var geminiAPIBase = "https://generativelanguage.googleapis.com/v1beta/models/"

// geminiSchema is slidesSchema in the OpenAPI subset Gemini expects
var geminiSchema = map[string]interface{}{
	"type": "OBJECT",
	"properties": map[string]interface{}{
		"sections": map[string]interface{}{
			"type": "ARRAY",
			"items": map[string]interface{}{
				"type": "OBJECT",
				"properties": map[string]interface{}{
					"title":       map[string]interface{}{"type": "STRING"},
					"description": map[string]interface{}{"type": "STRING"},
					"quotes": map[string]interface{}{
						"type":  "ARRAY",
						"items": map[string]interface{}{"type": "STRING"},
					},
				},
				"required": []string{"title", "description", "quotes"},
			},
		},
	},
	"required": []string{"sections"},
}

type geminiProvider struct{}

func (geminiProvider) key() string {
	if geminiAPIKey != "" {
		return geminiAPIKey
	}
	return os.Getenv("GEMINI_API_KEY")
}

func (geminiProvider) defaultModel() string {
	return "gemini-3.8-flash"
}

func (g geminiProvider) generate(model, system, user string) (string, error) {
	payload := map[string]interface{}{
		"systemInstruction": map[string]interface{}{
			"parts": []map[string]interface{}{{"text": system}},
		},
		"contents": []map[string]interface{}{
			{"role": "user", "parts": []map[string]interface{}{{"text": user}}},
		},
		"generationConfig": map[string]interface{}{
			"responseMimeType": "application/json",
			"responseSchema":   geminiSchema,
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal payload: %v", err)
	}

	req, err := http.NewRequest("POST", geminiAPIBase+model+":generateContent", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", g.key())

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
		return "", apiError("gemini", resp.StatusCode, raw)
	}

	var result struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text    string `json:"text"`
					Thought bool   `json:"thought"`
				} `json:"parts"`
			} `json:"content"`
			FinishReason string `json:"finishReason"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", fmt.Errorf("failed to decode response: %v", err)
	}
	if len(result.Candidates) == 0 {
		return "", fmt.Errorf("gemini returned no candidates")
	}
	var text strings.Builder
	for _, part := range result.Candidates[0].Content.Parts {
		if !part.Thought {
			text.WriteString(part.Text)
		}
	}
	return text.String(), nil
}
