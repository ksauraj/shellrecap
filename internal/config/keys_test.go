package config

import (
	"os"
	"runtime"
	"strings"
	"testing"
)

func TestKeysAreSavedPrivately(t *testing.T) {
	t.Setenv("SHELLRECAP_CONFIG_DIR", t.TempDir())
	if k := LoadKeys(); k != (Keys{}) {
		t.Fatalf("keys before any were saved: %+v", k)
	}
	if err := SaveKeys(Keys{Gemini: "AIza-test", Groq: "gsk_test"}); err != nil {
		t.Fatal(err)
	}
	if k := LoadKeys(); k.Gemini != "AIza-test" || k.Groq != "gsk_test" {
		t.Errorf("loaded %+v, want what was saved", k)
	}

	p, _ := KeysPath()
	info, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Errorf("keys file mode = %v, want 0600", info.Mode().Perm())
	}
	// The keys stay out of the config file, which isn't private
	Save(Defaults())
	cp, _ := path()
	data, _ := os.ReadFile(cp)
	if strings.Contains(string(data), "AIza") {
		t.Errorf("the config file holds a key:\n%s", data)
	}

	if err := SaveKeys(Keys{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Errorf("removing every key left the file behind")
	}
}
