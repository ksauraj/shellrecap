//go:build windows

package platform

import (
	"testing"
	"unsafe"
)

// Windows rejects the call unless the struct is laid out exactly like
// CONSOLE_SCREEN_BUFFER_INFOEX
func TestConsoleInfoLayout(t *testing.T) {
	var info consoleScreenBufferInfoEx
	if size := unsafe.Sizeof(info); size != 96 {
		t.Errorf("consoleScreenBufferInfoEx is %d bytes, want 96", size)
	}
	if offset := unsafe.Offsetof(info.colorTable); offset != 32 {
		t.Errorf("colorTable is at %d, want 32", offset)
	}
}

func TestKnownFolders(t *testing.T) {
	if knownFolder("Documents") == "" || knownFolder("Pictures") == "" {
		t.Error("Windows didn't say where Documents and Pictures are")
	}
}
