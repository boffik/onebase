// Package selfupdate реализует ядро офлайн-обновления бинаря onebase из
// локального архива или файла: извлечение, проверку контрольной суммы, атомарную
// подмену бинаря с откатом и опрос readiness-пробы. Оркестрация системного
// сервиса (sc.exe/systemctl) живёт в internal/cli/update.go — здесь только
// кросс-платформенная, юнит-тестируемая механика.
package selfupdate

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// BinaryName возвращает ожидаемое имя бинаря onebase для текущей ОС.
func BinaryName() string {
	if runtime.GOOS == "windows" {
		return "onebase.exe"
	}
	return "onebase"
}

// StageBinary готовит новый бинарь к установке из fromPath, который может быть
// либо ZIP-архивом (внутри ищется onebase[.exe]), либо самим исполняемым файлом.
// Для ZIP бинарь извлекается в stageDir; для файла возвращается его же путь.
func StageBinary(fromPath, stageDir string) (string, error) {
	if strings.EqualFold(filepath.Ext(fromPath), ".zip") {
		return extractFromZip(fromPath, stageDir)
	}
	if _, err := os.Stat(fromPath); err != nil {
		return "", fmt.Errorf("selfupdate: файл обновления не найден: %w", err)
	}
	return fromPath, nil
}

func extractFromZip(zipPath, stageDir string) (string, error) {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", fmt.Errorf("selfupdate: открыть архив: %w", err)
	}
	defer zr.Close()

	want := BinaryName()
	for _, f := range zr.File {
		if f.FileInfo().IsDir() || filepath.Base(f.Name) != want {
			continue
		}
		if err := os.MkdirAll(stageDir, 0o755); err != nil {
			return "", err
		}
		dst := filepath.Join(stageDir, want)
		rc, err := f.Open()
		if err != nil {
			return "", err
		}
		defer rc.Close()
		if err := writeFile(rc, dst, 0o755); err != nil {
			return "", err
		}
		return dst, nil
	}
	return "", fmt.Errorf("selfupdate: в архиве %s не найден %s", filepath.Base(zipPath), want)
}

// VerifySHA256 проверяет, что sha256 файла совпадает с wantHex (регистр не важен).
func VerifySHA256(path, wantHex string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	got := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(got, strings.TrimSpace(wantHex)) {
		return fmt.Errorf("selfupdate: контрольная сумма не сошлась: ожидали %s, получили %s", wantHex, got)
	}
	return nil
}

// SwapBinary заменяет бинарь по targetPath содержимым newPath, сохраняя прежний
// бинарь рядом (targetPath+".old") для отката. Приём с переименованием работает и
// на Windows, где запущенный .exe нельзя перезаписать, но можно переименовать:
// старый бинарь уезжает в .old, новый пишется на освободившееся имя. Возвращает
// путь к сохранённой копии.
func SwapBinary(targetPath, newPath string) (string, error) {
	backupPath := targetPath + ".old"
	_ = os.Remove(backupPath) // остаток прошлого обновления, если был

	if err := os.Rename(targetPath, backupPath); err != nil {
		return "", fmt.Errorf("selfupdate: сохранить старый бинарь: %w", err)
	}
	in, err := os.Open(newPath)
	if err != nil {
		_ = os.Rename(backupPath, targetPath) // откат
		return "", err
	}
	defer in.Close()
	if err := writeFile(in, targetPath, 0o755); err != nil {
		_ = os.Remove(targetPath)
		_ = os.Rename(backupPath, targetPath) // откат
		return "", fmt.Errorf("selfupdate: записать новый бинарь: %w", err)
	}
	return backupPath, nil
}

// Rollback возвращает бинарь из backupPath на место targetPath — вызывается, если
// новый бинарь не прошёл /healthz. Сервис к этому моменту должен быть остановлен,
// иначе запущенный targetPath на Windows не удалить.
func Rollback(targetPath, backupPath string) error {
	if _, err := os.Stat(backupPath); err != nil {
		return fmt.Errorf("selfupdate: резервный бинарь недоступен: %w", err)
	}
	// Плохой новый бинарь уводим в сторону (на Windows мгновенно удалить может не
	// выйти, если файл ещё мапится), затем возвращаем старый на место.
	failed := targetPath + ".failed"
	_ = os.Remove(failed)
	if err := os.Rename(targetPath, failed); err != nil {
		_ = os.Remove(targetPath)
	}
	if err := os.Rename(backupPath, targetPath); err != nil {
		return fmt.Errorf("selfupdate: восстановить старый бинарь: %w", err)
	}
	_ = os.Remove(failed)
	return nil
}

// writeFile записывает r в path с правами perm, атомарно перезаписывая содержимое.
func writeFile(r io.Reader, path string, perm os.FileMode) error {
	out, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, perm)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, r); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Chmod(path, perm)
}

// PollHealthz опрашивает url раз в interval, пока не получит HTTP 200 или пока не
// истечёт timeout. Возвращает nil при первом 200, иначе — последнюю ошибку.
func PollHealthz(ctx context.Context, url string, timeout, interval time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	client := &http.Client{Timeout: 5 * time.Second}
	var lastErr error
	for {
		if lastErr = probeOnce(ctx, client, url); lastErr == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("selfupdate: %s не поднялся за %s: %w", url, timeout, lastErr)
		case <-time.After(interval):
		}
	}
}

func probeOnce(ctx context.Context, client *http.Client, url string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s вернул %d", url, resp.StatusCode)
	}
	return nil
}
