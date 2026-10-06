package ai

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const slidesJSON = `{"sections":[{"title":"Cloud Wrangler","description":"You live in az.","quotes":["az login, again"]}]}`

// fakeGemini serves Gemini responses with the given status and slides text
func fakeGemini(t *testing.T, status int, text string, requests *int) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*requests++
		if got := r.Header.Get("x-goog-api-key"); got != "gemini-key" {
			t.Errorf("gemini api key header = %q", got)
		}
		var payload map[string]interface{}
		json.NewDecoder(r.Body).Decode(&payload)
		config, _ := payload["generationConfig"].(map[string]interface{})
		if config["responseMimeType"] != "application/json" || config["responseSchema"] == nil {
			t.Errorf("generationConfig = %v", config)
		}
		w.WriteHeader(status)
		if status != http.StatusOK {
			w.Write([]byte(`{"error":{"code":503,"message":"This model is currently experiencing high demand."}}`))
			return
		}
		quoted, _ := json.Marshal(text)
		w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":` + string(quoted) + `}]},"finishReason":"STOP"}]}`))
	}))
	t.Cleanup(server.Close)
	old := geminiAPIBase
	geminiAPIBase = server.URL + "/"
	t.Cleanup(func() { geminiAPIBase = old })
}

// fakeGroq serves Groq responses and hands each request body to check
func fakeGroq(t *testing.T, status int, check func(payload map[string]interface{}), requests *int) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*requests++
		if got := r.Header.Get("Authorization"); got != "Bearer groq-key" {
			t.Errorf("groq authorization header = %q", got)
		}
		var payload map[string]interface{}
		json.NewDecoder(r.Body).Decode(&payload)
		if check != nil {
			check(payload)
		}
		w.WriteHeader(status)
		if status != http.StatusOK {
			w.Write([]byte(`{"error":{"message":"Rate limit reached for model","type":"tokens"}}`))
			return
		}
		quoted, _ := json.Marshal(slidesJSON)
		w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":` + string(quoted) + `},"finish_reason":"stop"}]}`))
	}))
	t.Cleanup(server.Close)
	old := groqAPIURL
	groqAPIURL = server.URL
	t.Cleanup(func() { groqAPIURL = old })
}

func withKeys(t *testing.T, gemini, groq string) {
	t.Helper()
	oldGemini, oldGroq := geminiAPIKey, groqAPIKey
	geminiAPIKey, groqAPIKey = "", ""
	t.Cleanup(func() { geminiAPIKey, groqAPIKey = oldGemini, oldGroq })
	t.Setenv("GEMINI_API_KEY", gemini)
	t.Setenv("GROQ_API_KEY", groq)
	t.Setenv("GEMINI_MODEL", "")
	t.Setenv("GROQ_MODEL", "")
}

func TestGeminiFirst(t *testing.T) {
	withKeys(t, "gemini-key", "groq-key")
	var geminiCalls, groqCalls int
	fakeGemini(t, http.StatusOK, slidesJSON, &geminiCalls)
	fakeGroq(t, http.StatusOK, nil, &groqCalls)

	result, err := Generate("Total commands: 10", Options{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Target.Provider != Gemini || result.Target.Model != "gemini-3.8-flash" {
		t.Errorf("target = %+v, want Gemini first", result.Target)
	}
	if groqCalls != 0 {
		t.Errorf("groq was called %d times although Gemini succeeded", groqCalls)
	}
	if result.Sections[0].Title != "Cloud Wrangler" || result.Sections[0].Quotes[0] != "az login, again" {
		t.Errorf("sections = %+v", result.Sections)
	}
}

func TestFallsBackToGroq(t *testing.T) {
	withKeys(t, "gemini-key", "groq-key")
	var geminiCalls, groqCalls int
	fakeGemini(t, http.StatusServiceUnavailable, "", &geminiCalls)
	fakeGroq(t, http.StatusOK, func(payload map[string]interface{}) {
		if payload["model"] != "qwen/qwen3.8-27b" {
			t.Errorf("model = %v, want Groq's preferred model", payload["model"])
		}
		if _, ok := payload["reasoning_effort"]; ok {
			t.Error("reasoning_effort sent to Qwen")
		}
		format, _ := payload["response_format"].(map[string]interface{})
		schema, _ := format["json_schema"].(map[string]interface{})
		if format["type"] != "json_schema" || schema["strict"] != true || schema["schema"] == nil {
			t.Errorf("response_format = %v, want a strict json_schema", format)
		}
	}, &groqCalls)

	result, err := Generate("Total commands: 10", Options{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Target.Provider != Groq {
		t.Errorf("target = %+v, want the Groq fallback", result.Target)
	}
	// Both Gemini models are tried before Groq
	if geminiCalls != 2 || len(result.Failures) != 2 || !strings.Contains(result.Failures[0].Error(), "high demand") {
		t.Errorf("gemini calls = %d, failures = %v; want both Gemini models to fail first", geminiCalls, result.Failures)
	}
}

func TestGroqBackupModel(t *testing.T) {
	withKeys(t, "", "groq-key")
	var models []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]interface{}
		json.NewDecoder(r.Body).Decode(&payload)
		model, _ := payload["model"].(string)
		models = append(models, model)
		if model == "qwen/qwen3.8-27b" {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error":{"message":"The model qwen/qwen3.8-27b has been decommissioned"}}`))
			return
		}
		if payload["reasoning_effort"] != "low" {
			t.Errorf("reasoning_effort = %v, want low for gpt-oss", payload["reasoning_effort"])
		}
		quoted, _ := json.Marshal(slidesJSON)
		w.Write([]byte(`{"choices":[{"message":{"content":` + string(quoted) + `}}]}`))
	}))
	defer server.Close()
	old := groqAPIURL
	groqAPIURL = server.URL
	defer func() { groqAPIURL = old }()

	result, err := Generate("x", Options{Provider: Groq})
	if err != nil {
		t.Fatal(err)
	}
	if result.Target.Model != "openai/gpt-oss-120b" || strings.Join(models, ",") != "qwen/qwen3.8-27b,openai/gpt-oss-120b" {
		t.Errorf("target = %+v after trying %v, want the gpt-oss backup", result.Target, models)
	}
}

func TestExplicitProviderAndModel(t *testing.T) {
	withKeys(t, "gemini-key", "groq-key")
	var geminiCalls, groqCalls int
	fakeGemini(t, http.StatusOK, slidesJSON, &geminiCalls)
	fakeGroq(t, http.StatusOK, func(payload map[string]interface{}) {
		if payload["model"] != "llama-3.1-8b-instant" {
			t.Errorf("model = %v, want the --model override", payload["model"])
		}
		format, _ := payload["response_format"].(map[string]interface{})
		if format["type"] != "json_object" {
			t.Errorf("response_format = %v, want JSON mode for a model without strict outputs", format)
		}
		if _, ok := payload["reasoning_effort"]; ok {
			t.Error("reasoning_effort sent to a model that doesn't reason")
		}
	}, &groqCalls)

	result, err := Generate("Total commands: 10", Options{Provider: Groq, Model: "llama-3.1-8b-instant"})
	if err != nil {
		t.Fatal(err)
	}
	if geminiCalls != 0 {
		t.Errorf("gemini was called %d times with --provider groq", geminiCalls)
	}
	if result.Target != (Target{Provider: Groq, Model: "llama-3.1-8b-instant"}) {
		t.Errorf("target = %+v", result.Target)
	}
}

func TestAllProvidersFail(t *testing.T) {
	withKeys(t, "gemini-key", "groq-key")
	var geminiCalls, groqCalls int
	fakeGemini(t, http.StatusServiceUnavailable, "", &geminiCalls)
	fakeGroq(t, http.StatusTooManyRequests, nil, &groqCalls)

	_, err := Generate("x", Options{})
	if err == nil || !strings.Contains(err.Error(), "high demand") || !strings.Contains(err.Error(), "Rate limit") {
		t.Errorf("err = %v, want both providers' errors", err)
	}
}

func TestSkipsProvidersWithoutKeys(t *testing.T) {
	withKeys(t, "", "groq-key")
	var geminiCalls, groqCalls int
	fakeGemini(t, http.StatusOK, slidesJSON, &geminiCalls)
	fakeGroq(t, http.StatusOK, nil, &groqCalls)

	result, err := Generate("x", Options{})
	if err != nil || result.Target.Provider != Groq || geminiCalls != 0 {
		t.Errorf("result = %+v, err = %v, gemini calls = %d; want Groq only", result.Target, err, geminiCalls)
	}

	withKeys(t, "", "")
	if _, err := Generate("x", Options{}); !errors.Is(err, ErrNoAPIKey) {
		t.Errorf("err = %v, want ErrNoAPIKey", err)
	}
}

func TestModelFromEnvironment(t *testing.T) {
	withKeys(t, "", "")
	t.Setenv("GROQ_MODEL", "openai/gpt-oss-20b")
	targets := Targets(Options{})
	want := []Target{{Gemini, "gemini-3.8-flash"}, {Gemini, "gemini-3.5-flash-lite"}, {Groq, "openai/gpt-oss-20b"}}
	if len(targets) != len(want) {
		t.Fatalf("targets = %+v, want %+v", targets, want)
	}
	for i := range want {
		if targets[i] != want[i] {
			t.Errorf("targets = %+v, want %+v", targets, want)
		}
	}
}

func TestValidateOptions(t *testing.T) {
	for _, opts := range []Options{{}, {Provider: Auto}, {Provider: Groq, Model: "m"}, {Provider: Gemini}} {
		if err := ValidateOptions(opts); err != nil {
			t.Errorf("ValidateOptions(%+v) = %v", opts, err)
		}
	}
	for _, opts := range []Options{{Provider: "openai"}, {Model: "m"}, {Provider: Auto, Model: "m"}} {
		if err := ValidateOptions(opts); err == nil {
			t.Errorf("ValidateOptions(%+v) accepted invalid options", opts)
		}
	}
}

func TestStripCodeFence(t *testing.T) {
	if got := stripCodeFence("```json\n{\"a\":1}\n```"); got != `{"a":1}` {
		t.Errorf("stripCodeFence = %q", got)
	}
}
