package gemini

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func fakeServer(t *testing.T, status int, body string) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("x-goog-api-key"); got != "test-key" {
			t.Errorf("api key header = %q", got)
		}
		if !strings.HasSuffix(r.URL.Path, "/test-model:generateContent") {
			t.Errorf("path = %q", r.URL.Path)
		}
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("bad payload: %v", err)
		}
		config, _ := payload["generationConfig"].(map[string]interface{})
		if config["responseMimeType"] != "application/json" || config["responseSchema"] == nil {
			t.Errorf("generationConfig = %v", config)
		}
		w.WriteHeader(status)
		w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)

	oldBase, oldKey := geminiAPIBase, apiKey
	geminiAPIBase, apiKey = server.URL+"/", "test-key"
	t.Cleanup(func() { geminiAPIBase, apiKey = oldBase, oldKey })
	t.Setenv("GEMINI_MODEL", "test-model")

	// Keep gemini_response.log out of the repo
	dir, _ := os.Getwd()
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(dir) })
}

func TestGenerateWrapped(t *testing.T) {
	slides := `{"sections":[{"title":"Cloud Wrangler","description":"You live in az.","quotes":["az login, again"]}]}`
	text, _ := json.Marshal(slides)
	fakeServer(t, http.StatusOK, `{"candidates":[{"content":{"parts":[{"text":`+string(text)+`}]},"finishReason":"STOP"}]}`)

	resp, err := GenerateWrapped("Total commands: 10")
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Sections) != 1 || resp.Sections[0].Title != "Cloud Wrangler" || resp.Sections[0].Quotes[0] != "az login, again" {
		t.Errorf("sections = %+v", resp.Sections)
	}
}

func TestGenerateWrappedAPIError(t *testing.T) {
	fakeServer(t, http.StatusNotFound, `{"error":{"code":404,"message":"model not found"}}`)

	_, err := GenerateWrapped("Total commands: 10")
	if err == nil || !strings.Contains(err.Error(), "model not found") {
		t.Errorf("err = %v, want the API error message", err)
	}
}

func TestGenerateWrappedNoKey(t *testing.T) {
	oldKey := apiKey
	apiKey = ""
	t.Cleanup(func() { apiKey = oldKey })
	t.Setenv("GEMINI_API_KEY", "")

	if _, err := GenerateWrapped("x"); err != ErrNoAPIKey {
		t.Errorf("err = %v, want ErrNoAPIKey", err)
	}
}

func TestStripCodeFence(t *testing.T) {
	if got := stripCodeFence("```json\n{\"a\":1}\n```"); got != `{"a":1}` {
		t.Errorf("stripCodeFence = %q", got)
	}
}
