// internal/platform/platform.go

// Package platform answers the questions whose answers differ between
// Linux, macOS, Windows and WSL: where the Windows user's folders are,
// which WSL distros are installed, and how paths translate between the two.
package platform

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
	"unicode/utf16"
)

var (
	wslOnce sync.Once
	wsl     bool
)

// IsWSL reports whether this is Linux running under the Windows Subsystem
// for Linux
func IsWSL() bool {
	wslOnce.Do(func() {
		if runtime.GOOS != "linux" {
			return
		}
		if os.Getenv("WSL_DISTRO_NAME") != "" || os.Getenv("WSL_INTEROP") != "" {
			wsl = true
			return
		}
		release, err := os.ReadFile("/proc/sys/kernel/osrelease")
		wsl = err == nil && strings.Contains(strings.ToLower(string(release)), "microsoft")
	})
	return wsl
}

// run runs a program and returns what it printed, giving up after timeout
func run(timeout time.Duration, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	if runtime.GOOS == "linux" {
		// cmd.exe complains when started in a Linux folder
		if info, err := os.Stat("/mnt/c"); err == nil && info.IsDir() {
			cmd.Dir = "/mnt/c"
		}
	}
	out, err := cmd.Output()
	return string(out), err
}

var (
	interopOnce sync.Once
	interop     bool
)

// WSLInterop reports whether this is WSL and it can start Windows programs
func WSLInterop() bool {
	interopOnce.Do(func() {
		if !IsWSL() {
			return
		}
		_, err := os.Stat("/proc/sys/fs/binfmt_misc/WSLInterop")
		interop = err == nil || os.Getenv("WSL_INTEROP") != ""
	})
	return interop
}

// WindowsProgram finds a Windows program from inside WSL, even when the
// Windows folders aren't on PATH
func WindowsProgram(name string) string {
	if path, err := exec.LookPath(name); err == nil {
		return path
	}
	return filepath.Join("/mnt/c/Windows/System32", name)
}

var (
	windowsHomeOnce sync.Once
	windowsHome     string
)

// WindowsHome is the Windows user's profile folder, like C:\Users\Name, as
// a path this process can open: /mnt/c/Users/Name from inside WSL. It's ""
// on other systems, or when it can't be found.
func WindowsHome() string {
	switch {
	case runtime.GOOS == "windows":
		home, _ := os.UserHomeDir()
		return home
	case IsWSL():
		windowsHomeOnce.Do(func() { windowsHome = findWindowsHome() })
		return windowsHome
	}
	return ""
}

// systemProfiles are the folders under C:\Users that don't belong to a person
var systemProfiles = map[string]bool{"public": true, "default": true, "default user": true, "all users": true}

func findWindowsHome() string {
	// chcp 65001 makes cmd print UTF-8, for names with accents
	out, err := run(3*time.Second, WindowsProgram("cmd.exe"), "/c", "chcp", "65001", ">nul", "&", "echo", "%USERPROFILE%")
	if err == nil {
		if home := LinuxPath(strings.TrimSpace(out)); home != "" && isDir(home) {
			return home
		}
	}
	// Without interop, look for the only person's profile folder
	matches, _ := filepath.Glob("/mnt/c/Users/*/AppData")
	var homes []string
	for _, m := range matches {
		home := filepath.Dir(m)
		if !systemProfiles[strings.ToLower(filepath.Base(home))] {
			homes = append(homes, home)
		}
	}
	if len(homes) == 1 {
		return homes[0]
	}
	return ""
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// LinuxPath turns a Windows path like C:\Users\Name into the path WSL
// mounts it at, like /mnt/c/Users/Name. Other paths are returned as they
// are, and "" when it isn't a path.
func LinuxPath(path string) string {
	if !looksLikeWindowsPath(path) {
		return path
	}
	if out, err := run(2*time.Second, "wslpath", "-u", path); err == nil {
		if p := strings.TrimSpace(out); p != "" {
			return p
		}
	}
	return "/mnt/" + strings.ToLower(path[:1]) + strings.ReplaceAll(path[2:], `\`, "/")
}

func looksLikeWindowsPath(path string) bool {
	return len(path) >= 3 && path[1] == ':' && (path[2] == '\\' || path[2] == '/') &&
		(path[0]|0x20 >= 'a' && path[0]|0x20 <= 'z')
}

// WindowsPath turns a path this process can open into one Windows programs
// understand: /mnt/c/Users/Name becomes C:\Users\Name inside WSL
func WindowsPath(path string) string {
	if !IsWSL() {
		return path
	}
	if out, err := run(2*time.Second, "wslpath", "-w", path); err == nil {
		if p := strings.TrimSpace(out); p != "" {
			return p
		}
	}
	if strings.HasPrefix(path, "/mnt/") && len(path) >= 6 && (len(path) == 6 || path[6] == '/') {
		return strings.ToUpper(path[5:6]) + `:` + strings.ReplaceAll(path[6:], "/", `\`)
	}
	return path
}

// Documents lists the folders Windows may keep the Documents folder in,
// where PowerShell keeps its profile. OneDrive moves it when it backs it up.
func Documents() []string {
	var dirs []string
	if dir := knownFolder("Documents"); dir != "" {
		dirs = append(dirs, dir)
	}
	if home := WindowsHome(); home != "" {
		dirs = append(dirs, filepath.Join(home, "Documents"))
		onedrive, _ := filepath.Glob(filepath.Join(home, "OneDrive*", "Documents"))
		dirs = append(dirs, onedrive...)
	}
	return unique(dirs)
}

// Pictures is the Windows user's Pictures folder, or "" when this isn't
// Windows or WSL
func Pictures() string {
	if dir := knownFolder("Pictures"); dir != "" {
		return dir
	}
	if !IsWSL() || WindowsHome() == "" {
		return ""
	}
	// Ask Windows, since OneDrive may have moved it
	out, err := run(5*time.Second, WindowsProgram("powershell.exe"), "-NoProfile", "-NonInteractive", "-Command",
		"[Console]::OutputEncoding = [Text.Encoding]::UTF8; [Environment]::GetFolderPath('MyPictures')")
	if err == nil {
		if dir := LinuxPath(strings.TrimSpace(out)); dir != "" && isDir(dir) {
			return dir
		}
	}
	if dir := filepath.Join(WindowsHome(), "Pictures"); isDir(dir) {
		return dir
	}
	return ""
}

func unique(paths []string) []string {
	seen := make(map[string]bool)
	var kept []string
	for _, p := range paths {
		key := strings.ToLower(filepath.Clean(p))
		if !seen[key] {
			seen[key] = true
			kept = append(kept, p)
		}
	}
	return kept
}

// Distro is a WSL distro and its default user's home folder, as a path
// Windows can open
type Distro struct {
	Name string
	Home string // like \\wsl$\Ubuntu\home\name
}

// internalDistros are the distros apps like Docker Desktop install for
// themselves, which nobody types commands into
var internalDistros = []string{"docker-desktop", "rancher-desktop", "podman-machine"}

// WSLDistros lists the WSL distros installed, when running on Windows.
// Finding a distro's home starts it if it isn't running, which can take a
// few seconds, so the distros are asked at the same time.
func WSLDistros() []Distro {
	if runtime.GOOS != "windows" {
		return nil
	}
	out, err := run(5*time.Second, "wsl.exe", "--list", "--quiet")
	if err != nil {
		return nil
	}
	names := DistroNames([]byte(out))

	distros := make([]Distro, len(names))
	var wg sync.WaitGroup
	for i, name := range names {
		wg.Add(1)
		go func(i int, name string) {
			defer wg.Done()
			home, err := run(20*time.Second, "wsl.exe", "--distribution", name, "--exec", "printenv", "HOME")
			home = strings.TrimSpace(home)
			if err != nil || !strings.HasPrefix(home, "/") {
				return
			}
			distros[i] = Distro{Name: name, Home: `\\wsl$\` + name + strings.ReplaceAll(home, "/", `\`)}
		}(i, name)
	}
	wg.Wait()

	var found []Distro
	for _, d := range distros {
		if d.Name != "" {
			found = append(found, d)
		}
	}
	return found
}

// DistroNames reads the output of wsl.exe --list --quiet, which is UTF-16
// unless WSL_UTF8 is set, leaving out the distros apps use internally
func DistroNames(out []byte) []string {
	text := decodeUTF16(out)
	var names []string
	for _, line := range strings.Split(text, "\n") {
		name := strings.TrimSpace(strings.TrimPrefix(line, "\ufeff"))
		if name == "" || isInternal(name) {
			continue
		}
		names = append(names, name)
	}
	return names
}

func isInternal(name string) bool {
	lower := strings.ToLower(name)
	for _, prefix := range internalDistros {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
}

// decodeUTF16 decodes little-endian UTF-16, which is what Windows tools
// print; text that isn't UTF-16 is returned as it is
func decodeUTF16(b []byte) string {
	b = bytes.TrimPrefix(b, []byte{0xff, 0xfe})
	if len(b) < 2 || len(b)%2 != 0 || bytes.IndexByte(b, 0) < 0 {
		return string(b)
	}
	units := make([]uint16, len(b)/2)
	for i := range units {
		units[i] = uint16(b[2*i]) | uint16(b[2*i+1])<<8
	}
	return string(utf16.Decode(units))
}

// ConsoleBackground is the background color of the Windows console
// shellrecap runs in, read from the console's color table. It's only
// answered by the classic console, where Windows PowerShell is blue;
// Windows Terminal doesn't say.
func ConsoleBackground() (r, g, b uint8, ok bool) {
	if os.Getenv("WT_SESSION") != "" || os.Getenv("TERM_PROGRAM") != "" {
		return 0, 0, 0, false
	}
	return consoleBackground()
}
