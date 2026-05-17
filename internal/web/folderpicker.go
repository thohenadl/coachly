package web

import (
	"errors"
	"os/exec"
	"runtime"
	"strings"
)

// errPickerCanceled is returned when the user dismissed the OS folder dialog.
var errPickerCanceled = errors.New("user canceled folder picker")

// pickFolder shows the native OS folder selection dialog and returns the
// absolute path the user chose. If the user cancels, errPickerCanceled is
// returned. On unsupported platforms, an error describing the situation is
// returned and the caller should fall back to manual entry.
func pickFolder(prompt string) (string, error) {
	switch runtime.GOOS {
	case "darwin":
		return pickFolderDarwin(prompt)
	case "windows":
		return pickFolderWindows(prompt)
	case "linux":
		return pickFolderLinux(prompt)
	default:
		return "", errors.New("folder picker not supported on this platform")
	}
}

func pickFolderDarwin(prompt string) (string, error) {
	escaped := strings.ReplaceAll(prompt, `"`, `\"`)
	script := `tell application "System Events" to activate
return POSIX path of (choose folder with prompt "` + escaped + `")`
	out, err := exec.Command("osascript", "-e", script).Output()
	if err != nil {
		// osascript exits non-zero with stderr "User canceled. (-128)" when
		// the user dismisses the dialog. Treat any error as cancel — the
		// caller surfaces a benign 204 either way.
		return "", errPickerCanceled
	}
	return strings.TrimRight(string(out), "\r\n"), nil
}

func pickFolderWindows(prompt string) (string, error) {
	escaped := strings.ReplaceAll(prompt, `'`, `''`)
	script := `Add-Type -AssemblyName System.Windows.Forms | Out-Null
$d = New-Object System.Windows.Forms.FolderBrowserDialog
$d.Description = '` + escaped + `'
$d.ShowNewFolderButton = $true
if ($d.ShowDialog() -eq [System.Windows.Forms.DialogResult]::OK) { Write-Output $d.SelectedPath }`
	out, err := exec.Command("powershell", "-NoProfile", "-STA", "-Command", script).Output()
	if err != nil {
		return "", errPickerCanceled
	}
	path := strings.TrimRight(string(out), "\r\n")
	if path == "" {
		return "", errPickerCanceled
	}
	return path, nil
}

func pickFolderLinux(prompt string) (string, error) {
	if _, err := exec.LookPath("zenity"); err == nil {
		out, err := exec.Command("zenity", "--file-selection", "--directory", "--title="+prompt).Output()
		if err != nil {
			return "", errPickerCanceled
		}
		return strings.TrimRight(string(out), "\r\n"), nil
	}
	if _, err := exec.LookPath("kdialog"); err == nil {
		out, err := exec.Command("kdialog", "--getexistingdirectory", "--title", prompt).Output()
		if err != nil {
			return "", errPickerCanceled
		}
		return strings.TrimRight(string(out), "\r\n"), nil
	}
	return "", errors.New("no folder picker available (install zenity or kdialog)")
}
