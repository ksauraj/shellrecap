package platform

import (
	"reflect"
	"testing"
	"unicode/utf16"
)

func utf16le(s string) []byte {
	var b []byte
	for _, u := range utf16.Encode([]rune(s)) {
		b = append(b, byte(u), byte(u>>8))
	}
	return b
}

func TestDistroNames(t *testing.T) {
	want := []string{"Ubuntu", "Debian", "kali-linux"}
	list := "Ubuntu\r\nDebian\r\ndocker-desktop\r\ndocker-desktop-data\r\nkali-linux\r\nrancher-desktop\r\n"

	// wsl.exe prints UTF-16, with or without a byte order mark
	if got := DistroNames(utf16le(list)); !reflect.DeepEqual(got, want) {
		t.Errorf("UTF-16: got %q, want %q", got, want)
	}
	if got := DistroNames(append([]byte{0xff, 0xfe}, utf16le(list)...)); !reflect.DeepEqual(got, want) {
		t.Errorf("UTF-16 with BOM: got %q, want %q", got, want)
	}
	// and UTF-8 when WSL_UTF8 is set
	if got := DistroNames([]byte(list)); !reflect.DeepEqual(got, want) {
		t.Errorf("UTF-8: got %q, want %q", got, want)
	}
	if got := DistroNames(nil); len(got) != 0 {
		t.Errorf("no distros: got %q", got)
	}
}

func TestLooksLikeWindowsPath(t *testing.T) {
	for path, want := range map[string]bool{
		`C:\Users\Name`: true, `d:/code`: true, `/home/name`: false, `C:`: false, `\\wsl$\Ubuntu`: false,
		`1:\x`: false, "": false,
	} {
		if got := looksLikeWindowsPath(path); got != want {
			t.Errorf("looksLikeWindowsPath(%q) = %v, want %v", path, got, want)
		}
	}
}
