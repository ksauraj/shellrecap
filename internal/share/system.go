// internal/share/system.go
package share

import (
	"context"
	"errors"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Desktop reports whether there is likely a clipboard and a browser to
// hand the images to, which isn't the case over SSH or without a display
func Desktop() bool {
	if os.Getenv("SSH_CONNECTION") != "" || os.Getenv("SSH_TTY") != "" {
		return false
	}
	switch runtime.GOOS {
	case "darwin", "windows":
		return true
	default:
		return os.Getenv("DISPLAY") != "" || os.Getenv("WAYLAND_DISPLAY") != ""
	}
}

// PasteShortcut is the key combination that pastes on this system
func PasteShortcut() string {
	if runtime.GOOS == "darwin" {
		return "Cmd+V"
	}
	return "Ctrl+V"
}

// errNoClipboard is returned when no clipboard tool is available
var errNoClipboard = errors.New("no clipboard tool found")

// run runs a helper program, giving up after a few seconds. Clipboard
// tools like xclip and wl-copy stay in the background to serve the
// clipboard, so they are expected to return straight away.
func run(stdin string, name string, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	return cmd.Run()
}

func has(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// CopyImage puts a PNG on the clipboard so it can be pasted into a post
func CopyImage(path string) error {
	switch {
	case runtime.GOOS == "darwin":
		escaped := strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(path)
		return run("", "osascript", "-e", `set the clipboard to (read (POSIX file "`+escaped+`") as «class PNGf»)`)
	case runtime.GOOS == "windows":
		escaped := strings.ReplaceAll(path, "'", "''")
		return run("", "powershell", "-NoProfile", "-STA", "-Command",
			"Add-Type -AssemblyName System.Windows.Forms,System.Drawing; "+
				"[System.Windows.Forms.Clipboard]::SetImage([System.Drawing.Image]::FromFile('"+escaped+"'))")
	case os.Getenv("WAYLAND_DISPLAY") != "" && has("wl-copy"):
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return run(string(data), "wl-copy", "--type", "image/png")
	case os.Getenv("DISPLAY") != "" && has("xclip"):
		return run("", "xclip", "-selection", "clipboard", "-t", "image/png", "-i", path)
	}
	return errNoClipboard
}

// CopyText puts text on the clipboard
func CopyText(text string) error {
	switch {
	case runtime.GOOS == "darwin":
		return run(text, "pbcopy")
	case runtime.GOOS == "windows":
		return run(text, "powershell", "-NoProfile", "-Command", "Set-Clipboard -Value ([Console]::In.ReadToEnd())")
	case os.Getenv("WAYLAND_DISPLAY") != "" && has("wl-copy"):
		return run(text, "wl-copy")
	case os.Getenv("DISPLAY") != "" && has("xclip"):
		return run(text, "xclip", "-selection", "clipboard")
	case os.Getenv("DISPLAY") != "" && has("xsel"):
		return run(text, "xsel", "--clipboard", "--input")
	}
	return errNoClipboard
}

// OpenURL opens a link in the default browser
func OpenURL(link string) error {
	switch runtime.GOOS {
	case "darwin":
		return run("", "open", link)
	case "windows":
		return run("", "rundll32", "url.dll,FileProtocolHandler", link)
	default:
		return run("", "xdg-open", link)
	}
}

// Reveal opens the file manager with the file selected, or at least its
// folder, so it can be dragged into a post
func Reveal(path string) error {
	switch runtime.GOOS {
	case "darwin":
		return run("", "open", "-R", path)
	case "windows":
		// explorer exits with 1 even when it worked
		_ = run("", "explorer", "/select,"+path)
		return nil
	}
	// GNOME, KDE and most other desktops can select the file over D-Bus
	if has("gdbus") {
		fileURL := (&url.URL{Scheme: "file", Path: path}).String()
		if run("", "gdbus", "call", "--session", "--dest", "org.freedesktop.FileManager1",
			"--object-path", "/org/freedesktop/FileManager1",
			"--method", "org.freedesktop.FileManager1.ShowItems", "['"+fileURL+"']", "") == nil {
			return nil
		}
	}
	return run("", "xdg-open", filepath.Dir(path))
}
