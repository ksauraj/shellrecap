// internal/gemini/gemini.go
package gemini

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type WrappedResponse struct {
	Sections []Section `json:"sections"`
}

type Section struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Quotes      []string `json:"quotes,omitempty"`
}

// apiKey is set at build time with
// -ldflags "-X github.com/ksauraj/shellrecap/internal/gemini.apiKey=..."
// and falls back to the GEMINI_API_KEY environment variable
var apiKey string

// defaultModel can be overridden with the GEMINI_MODEL environment variable
const defaultModel = "gemini-3.8-flash"

// geminiAPIBase is a variable so tests can point it at a fake server
var geminiAPIBase = "https://generativelanguage.googleapis.com/v1beta/models/"

// ErrNoAPIKey is returned when no API key was compiled in or set in the environment
var ErrNoAPIKey = errors.New("no Gemini API key (set GEMINI_API_KEY)")

func key() string {
	if apiKey != "" {
		return apiKey
	}
	return os.Getenv("GEMINI_API_KEY")
}

// Model returns the Gemini model in use
func Model() string {
	if m := os.Getenv("GEMINI_MODEL"); m != "" {
		return m
	}
	return defaultModel
}

// Available reports whether an API key is configured
func Available() bool {
	return key() != ""
}

const systemPrompt = `You write the AI slides of shellrecap, a Spotify-Wrapped-style recap of a developer's year in the terminal. The slides are shown in a terminal UI, so keep them short and punchy.

Write exactly 4 slides, in this order:
1. Persona: invent a creative terminal persona title for them (2-4 words) and justify it with their stats.
2. Roast: a good-natured roast of their habits (typos, clear, sudo, late nights, favourite commands).
3. Superpower: what they are clearly great at, and the evidence for it.
4. Forecast: playful predictions for their next year in the terminal.

Rules:
- Only use numbers that appear in the stats. Never invent numbers, dates or tools.
- Speak directly to the user as "you".
- title: at most 40 characters.
- description: 1-3 sentences, at most 280 characters.
- quotes: 1-2 witty one-liners per slide, each at most 90 characters, not attributed to anyone.
- Plain ASCII text only: no emoji, no markdown, no asterisks, no hashtags.`

// responseSchema makes Gemini return JSON matching WrappedResponse
var responseSchema = map[string]interface{}{
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

type generateResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text    string `json:"text"`
				Thought bool   `json:"thought"`
			} `json:"parts"`
		} `json:"content"`
		FinishReason string `json:"finishReason"`
	} `json:"candidates"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// GenerateWrapped asks Gemini for AI-written Wrapped slides based on a
// summary of the user's shell statistics
func GenerateWrapped(summary string) (WrappedResponse, error) {
	if !Available() {
		return WrappedResponse{}, ErrNoAPIKey
	}

	payload := map[string]interface{}{
		"systemInstruction": map[string]interface{}{
			"parts": []map[string]interface{}{{"text": systemPrompt}},
		},
		"contents": []map[string]interface{}{
			{
				"role":  "user",
				"parts": []map[string]interface{}{{"text": "Here are my shell stats:\n\n" + summary}},
			},
		},
		"generationConfig": map[string]interface{}{
			"responseMimeType": "application/json",
			"responseSchema":   responseSchema,
		},
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return WrappedResponse{}, fmt.Errorf("failed to marshal payload: %v", err)
	}

	req, err := http.NewRequest("POST", geminiAPIBase+Model()+":generateContent", bytes.NewBuffer(jsonPayload))
	if err != nil {
		return WrappedResponse{}, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", key())

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return WrappedResponse{}, fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	rawResponse, err := io.ReadAll(resp.Body)
	if err != nil {
		return WrappedResponse{}, fmt.Errorf("failed to read response body: %v", err)
	}
	logResponse(rawResponse)

	var result generateResponse
	if err := json.Unmarshal(rawResponse, &result); err != nil {
		return WrappedResponse{}, fmt.Errorf("failed to decode response (HTTP %d): %v", resp.StatusCode, err)
	}
	if result.Error != nil {
		return WrappedResponse{}, fmt.Errorf("gemini API error (HTTP %d): %s", resp.StatusCode, result.Error.Message)
	}
	if len(result.Candidates) == 0 {
		return WrappedResponse{}, fmt.Errorf("gemini returned no candidates (HTTP %d)", resp.StatusCode)
	}

	var text strings.Builder
	for _, part := range result.Candidates[0].Content.Parts {
		if !part.Thought {
			text.WriteString(part.Text)
		}
	}

	var wrappedResp WrappedResponse
	if err := json.Unmarshal([]byte(stripCodeFence(text.String())), &wrappedResp); err != nil {
		return WrappedResponse{}, fmt.Errorf("failed to parse text as JSON (finish reason %s): %v",
			result.Candidates[0].FinishReason, err)
	}
	if len(wrappedResp.Sections) == 0 {
		return WrappedResponse{}, errors.New("gemini returned no slides")
	}

	return wrappedResp, nil
}

// stripCodeFence removes a ```json fence in case the model adds one anyway
func stripCodeFence(text string) string {
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "```") {
		text = strings.TrimPrefix(text, "```json")
		text = strings.TrimPrefix(text, "```")
		text = strings.TrimSuffix(text, "```")
	}
	return strings.TrimSpace(text)
}

// logResponse appends the raw API response to gemini_response.log. Logging
// is best effort and never fails the request.
func logResponse(response []byte) {
	file, err := os.OpenFile("gemini_response.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer file.Close()
	fmt.Fprintf(file, "%s\n%s\n", time.Now().Format(time.RFC3339), response)
}
