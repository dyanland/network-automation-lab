@echo off
REM Build script for Windows

SET PROJECT_NAME=network-library
SET VERSION=1.0.0-poc
SET BUILD_DIR=build

echo ========================================
echo Building Network Migration Go Library
echo Version: %VERSION%
echo ========================================

REM Clean build directory
if exist %BUILD_DIR% rmdir /s /q %BUILD_DIR%
mkdir %BUILD_DIR%

cd go-library

echo.
echo Downloading dependencies...
go mod download
go mod tidy

echo.
echo Building binaries...

REM Build for Windows (64-bit)
echo   - Windows (amd64)...
set GOOS=windows
set GOARCH=amd64
go build -ldflags="-s -w" -o ..\%BUILD_DIR%\%PROJECT_NAME%-windows-amd64.exe main.go

REM Build for Linux (64-bit)
echo   - Linux (amd64)...
set GOOS=linux
set GOARCH=amd64
go build -ldflags="-s -w" -o ..\%BUILD_DIR%\%PROJECT_NAME%-linux-amd64 main.go

REM Build for macOS (Intel)
echo   - macOS (amd64)...
set GOOS=darwin
set GOARCH=amd64
go build -ldflags="-s -w" -o ..\%BUILD_DIR%\%PROJECT_NAME%-darwin-amd64 main.go

cd ..

echo.
echo Build completed successfully!
echo.
echo Built binaries:
dir %BUILD_DIR%

echo.
echo ========================================
echo Deployment Instructions:
echo ========================================
echo 1. Run the server:
echo    network-library-windows-amd64.exe
echo.
echo 2. Run Robot Framework tests:
echo    robot robot-tests\testcases\poc_test.robot
echo ========================================

pause
