@echo off
setlocal enabledelayedexpansion

for %%I in ("%~dp0.") do set "OUTPUT_DIR=%%~fI"
for %%I in ("%~dp0..") do set "PROJECT_DIR=%%~fI"

if "%1" equ "" goto usage

goto %1

:windows
echo Building Windows x86_64...
where gcc >nul 2>&1
if errorlevel 1 (
    echo ERROR: gcc was not found in PATH. A C compiler is required by go-sqlite3.
    exit /b 1
)
set CGO_ENABLED=1
set GOOS=windows
set GOARCH=amd64
set "BUILD_OUTPUT=%OUTPUT_DIR%\wx_video_download_windows_x86_64.exe"
set "FINAL_OUTPUT=%OUTPUT_DIR%\wx_channel.exe"
pushd "%PROJECT_DIR%"
call "%PROJECT_DIR%\build\build-go.bat" -trimpath -tags "with_gvisor,embed_inject,sqlite_only" -ldflags="-s -w" -o "%BUILD_OUTPUT%" .
if errorlevel 1 (
    popd
    echo ERROR: Windows build failed.
    exit /b 1
)
move /Y "%BUILD_OUTPUT%" "%FINAL_OUTPUT%" >nul
if errorlevel 1 (
    popd
    echo ERROR: Failed to move the Windows executable to %FINAL_OUTPUT%.
    exit /b 1
)
popd
echo Done: %FINAL_OUTPUT%
exit /b 0

:windows-sunnynet
echo Building Windows SunnyNet version...
echo This requires Docker on Windows:
echo   docker run --rm -v "%%cd%%:/workspace" -w /workspace golang:1.20 bash -c "...
echo Please run the Docker command manually from README.md
exit /b 1

:usage
echo Usage: build.bat [target]
echo   windows         - Windows x86_64
echo   windows-sunnynet - Windows SunnyNet ^(requires Docker^)
echo   all             - Build all targets
exit /b 1

:all
echo Building Windows...
call :windows
if errorlevel 1 exit /b 1
echo.
echo All done!
exit /b 0
