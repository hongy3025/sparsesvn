@echo off

echo Building sparsesvn.exe (strip debug info)...
go build -ldflags="-s -w" -trimpath -o sparsesvn.exe ./cmd/sparsesvn
if %ERRORLEVEL% neq 0 exit /b %ERRORLEVEL%

where upx >nul 2>nul
if %ERRORLEVEL% equ 0 (
    echo Compressing sparsesvn.exe with UPX...
    upx --best -q sparsesvn.exe
    if %ERRORLEVEL% neq 0 (
        echo Warning: UPX compression failed, using uncompressed binary.
    )
) else (
    echo UPX not found in PATH, skipping compression.
)

echo Build complete.
