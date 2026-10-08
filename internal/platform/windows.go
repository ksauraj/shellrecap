// internal/platform/windows.go

//go:build windows

package platform

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// knownFolder asks Windows where one of the user's folders is, which
// follows it when OneDrive or the user has moved it
func knownFolder(name string) string {
	ids := map[string]*windows.KNOWNFOLDERID{
		"Documents": windows.FOLDERID_Documents,
		"Pictures":  windows.FOLDERID_Pictures,
	}
	id, ok := ids[name]
	if !ok {
		return ""
	}
	path, err := windows.KnownFolderPath(id, windows.KF_FLAG_DEFAULT)
	if err != nil {
		return ""
	}
	return path
}

// consoleScreenBufferInfoEx is CONSOLE_SCREEN_BUFFER_INFOEX
type consoleScreenBufferInfoEx struct {
	size                uint32
	bufferSize          windows.Coord
	cursorPosition      windows.Coord
	attributes          uint16
	window              windows.SmallRect
	maximumWindowSize   windows.Coord
	popupAttributes     uint16
	fullscreenSupported int32
	colorTable          [16]uint32 // 0x00BBGGRR
}

var getConsoleScreenBufferInfoEx = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetConsoleScreenBufferInfoEx")

func consoleBackground() (r, g, b uint8, ok bool) {
	if getConsoleScreenBufferInfoEx.Find() != nil {
		return 0, 0, 0, false
	}
	var info consoleScreenBufferInfoEx
	info.size = uint32(unsafe.Sizeof(info))
	ret, _, _ := getConsoleScreenBufferInfoEx.Call(uintptr(windows.Stdout), uintptr(unsafe.Pointer(&info)))
	if ret == 0 {
		return 0, 0, 0, false
	}
	// The high four bits of the attributes pick the background's color
	color := info.colorTable[(info.attributes>>4)&0x0f]
	return uint8(color), uint8(color >> 8), uint8(color >> 16), true
}
