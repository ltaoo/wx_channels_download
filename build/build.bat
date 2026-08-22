@echo off
setlocal enabledelayedexpansion

for %%I in ("%~dp0..") do set "PROJECT_DIR=%%~fI"
if not defined OUTPUT_DIR for %%I in ("%~dp0.") do set "OUTPUT_DIR=%%~fI"
for %%I in ("%OUTPUT_DIR%") do set "OUTPUT_DIR=%%~fI"

set "ZIP_ENABLED=false"
set "TARGET=%~1"

if "%TARGET%" equ "" goto usage
if not "%~3" equ "" goto usage

if "%~2" equ "" goto option_parsed
if /I "%~2" equ "--zip" (
    set "ZIP_ENABLED=true"
    goto option_parsed
)
if /I "%~2" equ "--no-zip" goto option_parsed
echo ERROR: Unknown option: %~2
goto usage

:option_parsed
if not exist "%OUTPUT_DIR%" mkdir "%OUTPUT_DIR%"
if errorlevel 1 (
    echo ERROR: Failed to create output directory: %OUTPUT_DIR%
    exit /b 1
)

for /F "tokens=4" %%V in ('findstr /B /C:"var AppVer = " "%PROJECT_DIR%\main.go"') do set "APP_VERSION=%%~V"
if defined VERSION (
    set "RELEASE_VERSION=%VERSION%"
) else (
    set "RELEASE_VERSION=%APP_VERSION%"
)
if not defined RELEASE_VERSION (
    echo ERROR: Unable to determine the release version from main.go
    exit /b 1
)
if /I not "!RELEASE_VERSION:~0,1!" equ "v" set "RELEASE_VERSION=v!RELEASE_VERSION!"
set "VERSION_TO_VALIDATE=!RELEASE_VERSION!"
powershell -NoProfile -Command "if ($env:VERSION_TO_VALIDATE -notmatch '^v[0-9A-Za-z._-]+$') { exit 1 }"
if errorlevel 1 (
    echo ERROR: Invalid release version: !RELEASE_VERSION!
    exit /b 1
)

if /I "%TARGET%" equ "windows" goto windows
if /I "%TARGET%" equ "win" goto windows
if /I "%TARGET%" equ "windows-sunnynet" goto windows-sunnynet
if /I "%TARGET%" equ "all" goto all
goto usage

:windows
echo Building Windows x86_64...
set CGO_ENABLED=0
set GOOS=windows
set GOARCH=amd64
set "BUILD_OUTPUT=%OUTPUT_DIR%\wx_video_download_windows_x86_64.exe"
set "FINAL_OUTPUT=%OUTPUT_DIR%\wx_channel.exe"
pushd "%PROJECT_DIR%"
call "%PROJECT_DIR%\build\build-go.bat" -trimpath -tags "with_gvisor,embed_inject,sqlite_only,embed_frontend_inject" -ldflags="-s -w -X main.Mode=release" -o "%BUILD_OUTPUT%" .
if errorlevel 1 (
    popd
    echo ERROR: Windows build failed.
    exit /b 1
)
call :package_zip "%BUILD_OUTPUT%" "windows_x86_64" "wx_video_download"
if errorlevel 1 (
    popd
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
echo Usage: build.bat [target] [--zip^|--no-zip]
echo   windows         - Windows x86_64
echo   windows-sunnynet - Windows SunnyNet ^(requires Docker^)
echo   all             - Build all targets
echo   --zip           - Generate a ZIP archive after building
echo   --no-zip        - Only generate the binary ^(default^)
exit /b 1

:all
echo Building Windows...
call :windows
if errorlevel 1 exit /b 1
echo.
echo All done!
exit /b 0

:package_zip
if /I not "%ZIP_ENABLED%" equ "true" exit /b 0

set "PACKAGE_BINARY=%~1"
set "PACKAGE_PLATFORM=%~2"
set "PACKAGE_PREFIX=%~3"
for %%I in ("%PACKAGE_BINARY%") do set "PACKAGE_BINARY_NAME=%%~nxI"
set "PACKAGE_ARCHIVE=%OUTPUT_DIR%\%PACKAGE_PREFIX%_%RELEASE_VERSION%_%PACKAGE_PLATFORM%.zip"
set "PACKAGE_DIR=%TEMP%\wx_video_download_package_%RANDOM%_%RANDOM%"

mkdir "%PACKAGE_DIR%"
if errorlevel 1 (
    echo ERROR: Failed to create temporary package directory: %PACKAGE_DIR%
    exit /b 1
)

copy /Y "%PACKAGE_BINARY%" "%PACKAGE_DIR%\%PACKAGE_BINARY_NAME%" >nul
if errorlevel 1 goto package_failed
copy /Y "%PROJECT_DIR%\internal\config\config.template.yaml" "%PACKAGE_DIR%\config.yaml" >nul
if errorlevel 1 goto package_failed
copy /Y "%PROJECT_DIR%\LICENSE" "%PACKAGE_DIR%\LICENSE" >nul
if errorlevel 1 goto package_failed

set "ZIP_SOURCE_DIR=%PACKAGE_DIR%"
set "ZIP_BINARY_NAME=%PACKAGE_BINARY_NAME%"
set "ZIP_ARCHIVE_PATH=%PACKAGE_ARCHIVE%"
powershell -NoProfile -Command "Compress-Archive -LiteralPath (Join-Path $env:ZIP_SOURCE_DIR $env:ZIP_BINARY_NAME), (Join-Path $env:ZIP_SOURCE_DIR 'config.yaml'), (Join-Path $env:ZIP_SOURCE_DIR 'LICENSE') -DestinationPath $env:ZIP_ARCHIVE_PATH -Force"
if errorlevel 1 goto package_failed

rmdir /S /Q "%PACKAGE_DIR%"
echo Done: %PACKAGE_ARCHIVE%
exit /b 0

:package_failed
rmdir /S /Q "%PACKAGE_DIR%"
echo ERROR: Failed to create ZIP archive: %PACKAGE_ARCHIVE%
exit /b 1
