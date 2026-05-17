package launcher

import (
	"bytes"
	"os/exec"
	"runtime"
	"strings"
)

// OpenPath opens a directory or file in the OS file explorer / default app.
func OpenPath(path string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", path)
	case "darwin":
		cmd = exec.Command("open", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	return cmd.Start()
}

// BrowseDir opens a native folder picker dialog and returns the selected path.
// Returns ("", nil) if the user cancelled.
func BrowseDir(title string) (string, error) {
	switch runtime.GOOS {
	case "windows":
		script := `Add-Type -AssemblyName System.Windows.Forms
$d = New-Object System.Windows.Forms.FolderBrowserDialog
$d.Description = '` + title + `'
$d.ShowNewFolderButton = $true
if ($d.ShowDialog() -eq 'OK') { Write-Output $d.SelectedPath }`
		out, err := runPowerShell(script)
		return strings.TrimSpace(out), err
	case "darwin":
		out, err := exec.Command("osascript", "-e",
			`choose folder with prompt "`+title+`"`).Output()
		if err != nil {
			return "", nil
		}
		p := strings.TrimSpace(string(out))
		p = strings.TrimPrefix(p, "alias ")
		return p, nil
	default:
		out, err := exec.Command("zenity", "--file-selection", "--directory", "--title="+title).Output()
		if err != nil {
			return "", nil
		}
		return strings.TrimSpace(string(out)), nil
	}
}

// BrowseFile opens a native file picker dialog and returns the selected path.
// filter is a Windows-style filter string, e.g. "SQLite (*.db)|*.db".
// Returns ("", nil) if the user cancelled.
func BrowseFile(title, filter string) (string, error) {
	switch runtime.GOOS {
	case "windows":
		script := `Add-Type -AssemblyName System.Windows.Forms
$d = New-Object System.Windows.Forms.OpenFileDialog
$d.Title = '` + title + `'
$d.Filter = '` + filter + `'
$d.CheckFileExists = $false
if ($d.ShowDialog() -eq 'OK') { Write-Output $d.FileName }`
		out, err := runPowerShell(script)
		return strings.TrimSpace(out), err
	default:
		out, err := exec.Command("zenity", "--file-selection", "--title="+title).Output()
		if err != nil {
			return "", nil
		}
		return strings.TrimSpace(string(out)), nil
	}
}

func runPowerShell(script string) (string, error) {
	// -WindowStyle Hidden prevents the PowerShell console window from flashing.
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", script)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	if err := cmd.Run(); err != nil {
		return "", nil // user cancelled = exit code != 0
	}
	return buf.String(), nil
}
