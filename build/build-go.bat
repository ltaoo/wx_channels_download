@echo off
setlocal

if /I not "%~1"=="--zip" goto run_build
call "%~dp0build.bat" windows --zip
exit /b %ERRORLEVEL%

:run_build
for %%I in ("%~dp0..") do set "PROJECT_ROOT=%%~fI"
pushd "%~dp0tools"
go run ./cmd/buildgo -root "%PROJECT_ROOT%" -- %*
set "BUILD_EXIT_CODE=%ERRORLEVEL%"
popd

exit /b %BUILD_EXIT_CODE%
