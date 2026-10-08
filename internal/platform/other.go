// internal/platform/other.go

//go:build !windows

package platform

// knownFolder only knows the folders of Windows itself
func knownFolder(string) string {
	return ""
}

// consoleBackground needs the Windows console
func consoleBackground() (r, g, b uint8, ok bool) {
	return 0, 0, 0, false
}
