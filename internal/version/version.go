package version

import (
	"runtime/debug"
	"sync"
	"time"
)

// Build is set via -ldflags="-X github.com/ivantit66/onebase/internal/version.Build=v1.2.3"
var Build = ""

// Правообладатель и лицензия платформы (план 69). Единый источник для экрана
// «О программе». Совпадают с файлом LICENSE в корне репозитория.
const (
	Author  = "Иван Титов"
	License = "MIT"
	Year    = "2026"
)

// vcsData — сведения о ревизии, которые `go build` автоматически зашивает в
// бинарь (см. debug.ReadBuildInfo). Читаются один раз, лениво.
type vcsData struct {
	revision string
	time     time.Time
	modified bool
}

var (
	vcsOnce  sync.Once
	vcsCache vcsData
)

// vcs возвращает VCS-сведения из build info. Пусто, если сборка была без
// стемпинга (например, `go build -buildvcs=false` или вне git-репозитория).
func vcs() vcsData {
	vcsOnce.Do(func() {
		if info, ok := debug.ReadBuildInfo(); ok {
			vcsCache = parseVCS(info.Settings)
		}
	})
	return vcsCache
}

// parseVCS вытаскивает revision/time/modified из настроек build info.
// Вынесено отдельно, чтобы покрыть юнит-тестом (в `go test` реальных
// VCS-настроек может не быть).
func parseVCS(settings []debug.BuildSetting) vcsData {
	var d vcsData
	for _, s := range settings {
		switch s.Key {
		case "vcs.revision":
			d.revision = s.Value
		case "vcs.time":
			if t, err := time.Parse(time.RFC3339, s.Value); err == nil {
				d.time = t
			}
		case "vcs.modified":
			d.modified = s.Value == "true"
		}
	}
	return d
}

// shortRev укорачивает git SHA до 7 символов (пусто, если ревизии нет).
func shortRev(rev string) string {
	if len(rev) >= 7 {
		return rev[:7]
	}
	return ""
}

// fmtDate форматирует дату коммита как дд.мм.гг (пусто для нулевого времени).
func fmtDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("02.01.06")
}

// String returns the platform version string (e.g. "build-370" or "dev-cb5276e").
func String() string {
	if Build != "" {
		return Build
	}
	// Fall back to VCS info embedded by go build.
	if c := Commit(); c != "" {
		return "dev-" + c
	}
	return "dev"
}

// Commit returns the short (7-char) git revision embedded at build time, or "".
func Commit() string {
	return shortRev(vcs().revision)
}

// CommitDate returns the commit date (дд.мм.гг) embedded at build time, or "".
func CommitDate() string {
	return fmtDate(vcs().time)
}

// Modified reports whether the working tree was dirty at build time.
func Modified() bool {
	return vcs().modified
}
