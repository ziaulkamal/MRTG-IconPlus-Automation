@echo off
title MRTG Build Tool - PLN Icon Plus

echo.
echo  ======================================
echo    MRTG Chart Automation - Build Tool
echo    PLN Icon Plus
echo  ======================================
echo.

:: Setup MinGW PATH
set PATH=C:\msys64\mingw64\bin;%PATH%
set CGO_ENABLED=1

:: Generate exe icon jika belum ada
if not exist "app_windows.syso" (
    echo [1/2] Generating exe icon...
    go run gen_icon.go
    echo.
) else (
    echo [1/2] Exe icon sudah ada, skip.
)

:: Build tanpa console window (-H windowsgui)
echo [2/2] Building mrtg_automation.exe ...
go build -ldflags="-H windowsgui" -o mrtg_automation.exe .

if %ERRORLEVEL% EQU 0 (
    echo.
    echo  ✅ Build berhasil! File: mrtg_automation.exe
    echo  📌 Catatan: Terminal akan tersembunyi saat dijalankan
) else (
    echo.
    echo  ❌ Build gagal!
)

echo.
pause
