package secret

import (
	"strings"
	"testing"
)

const testKey = "AIzaSyD-not-a-real-key-0123456789abcdef"

func TestSealRoundTrip(t *testing.T) {
	sealed, err := Seal(testKey)
	if err != nil {
		t.Fatal(err)
	}
	if got := Open(sealed); got != testKey {
		t.Errorf("Open(Seal(key)) = %q, want the key", got)
	}
	if empty, _ := Seal(""); empty != "" || Open(empty) != "" {
		t.Errorf("an empty key should seal to nothing and open to nothing")
	}
}

func TestSealedKeyIsNotReadable(t *testing.T) {
	a, _ := Seal(testKey)
	b, _ := Seal(testKey)
	if a == b {
		t.Error("sealing a key twice gave the same value")
	}
	for _, part := range []string{testKey, "AIza", "not-a-real-key", "0123456789"} {
		if strings.Contains(a, part) {
			t.Errorf("the sealed key %q contains %q", a, part)
		}
	}
}

func TestOpenRejectsAnythingElse(t *testing.T) {
	sealed, _ := Seal(testKey)
	tampered := sealed[:len(sealed)-3] + "AAA"
	if tampered == sealed {
		tampered = sealed[:len(sealed)-3] + "BBB"
	}
	for _, v := range []string{testKey, "s1.", "s1.!!!", "s1.AAAA", tampered} {
		if got := Open(v); got != "" {
			t.Errorf("Open(%q) = %q, want nothing", v, got)
		}
	}
}
