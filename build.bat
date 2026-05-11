@echo off
chcp 65001 >nul 2>&1
setlocal
cd /d "%~dp0"

echo === onebase build ===
echo.

echo [1/2] onebase.exe  (CLI + server)...
set "CGO_ENABLED=0"
go build -ldflags="-s -w" -o onebase.exe ./cmd/onebase
if %errorlevel% neq 0 ( echo ERROR & pause & exit /b 1 )
echo     OK

echo.
echo [2/2] onebase-gui.exe  (native window)...
where gcc >nul 2>&1 || set "PATH=C:\msys64\ucrt64\bin;%PATH%"
set "CGO_ENABLED=1"
go build -tags webview -ldflags="-s -w -H windowsgui" -o onebase-gui.exe ./cmd/onebase
if %errorlevel% neq 0 (
    echo     GCC not found, building browser fallback...
    set "CGO_ENABLED=0"
    go build -ldflags="-s -w -H windowsgui" -o onebase-gui.exe ./cmd/onebase
    if %errorlevel% neq 0 (
        echo     SKIPPED. Use: onebase.exe start
    ) else (
        echo     OK  (browser mode -- no native window)
    )
) else (
    echo     OK  (native WebView2 window)
)

echo.
echo Done!
echo   Double-click onebase-gui.exe  -- opens a window.
echo   Or: onebase.exe start  -- opens in browser.
pause
