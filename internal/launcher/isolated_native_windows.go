//go:build windows && webview

package launcher

import (
	"os"
	"os/exec"
)

// nativeIsolatedSupported: нативные WebView2-окна доступны в GUI-сборке под
// Windows (план 78, п. 4.2) — на них работает сам лаунчер, runtime заведомо есть.
func nativeIsolatedSupported() bool { return true }

// nativeIsolatedCommand строит запуск второго нативного окна: сам же exe
// (`onebase-gui window --url ...`) с переменной ONEBASE_WEBVIEW_PROFILE,
// которую читает наш патч webview.h (third_party/webview_go) — у каждого окна
// свой каталог профиля WebView2, а значит свой cookie-jar.
func nativeIsolatedCommand(profileDir, url string) (*exec.Cmd, bool) {
	exe, err := os.Executable()
	if err != nil {
		return nil, false
	}
	cmd := exec.Command(exe, "window", "--url", url, "--title", "onebase — Предприятие")
	cmd.Env = append(os.Environ(), "ONEBASE_WEBVIEW_PROFILE="+profileDir)
	return cmd, true
}
