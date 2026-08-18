@echo off
setlocal

for %%I in ("%~dp0..") do set "PROJECT_ROOT=%%~fI"
pushd "%~dp0tools"
go run ./cmd/buildgo -root "%PROJECT_ROOT%" -- %*
set "BUILD_EXIT_CODE=%ERRORLEVEL%"
popd

exit /b %BUILD_EXIT_CODE%
