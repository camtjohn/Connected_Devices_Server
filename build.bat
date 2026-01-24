@echo off
REM Build script for Connected Devices Server (Windows)
REM Builds for Oracle VM (Linux/arm64)
REM Usage: build.bat [debug|prod|both]

setlocal enabledelayedexpansion

set PROJECT_NAME=server_app
set TARGET_OS=linux
set TARGET_ARCH=arm64

if "%1"=="" (
    set BUILD_TYPE=both
) else (
    set BUILD_TYPE=%1
)

echo === Connected Devices Server Build Script ===
echo Target: %TARGET_OS%/%TARGET_ARCH%
echo.

REM Clean previous builds
echo Cleaning previous builds...
if exist "%PROJECT_NAME%_debug" del "%PROJECT_NAME%_debug"
if exist "%PROJECT_NAME%" del "%PROJECT_NAME%"
echo [OK] Clean complete
echo.

REM Build Debug
if "%BUILD_TYPE%"=="debug" (goto build_debug) else if "%BUILD_TYPE%"=="both" (goto build_debug) else (goto build_prod)

:build_debug
echo Building DEBUG version...
set GOOS=%TARGET_OS%
set GOARCH=%TARGET_ARCH%
go build -tags debug -o "%PROJECT_NAME%_debug" -v
if errorlevel 1 (
    echo [FAILED] Debug build failed
    exit /b 1
)
echo [OK] Debug build complete: %PROJECT_NAME%_debug
echo.

:build_prod
if "%BUILD_TYPE%"=="debug" (goto end) else if "%BUILD_TYPE%"=="both" (
    echo Building PRODUCTION version...
) else (
    echo Building PRODUCTION version...
)

set GOOS=%TARGET_OS%
set GOARCH=%TARGET_ARCH%
go build -o "%PROJECT_NAME%" -v
if errorlevel 1 (
    echo [FAILED] Production build failed
    exit /b 1
)
echo [OK] Production build complete: %PROJECT_NAME%
echo.

:end
echo === Build Summary ===
echo Location: %cd%
dir /b "%PROJECT_NAME%*" 2>nul
echo.
echo To deploy to Oracle VM:
echo   scp -i ^<keyfile^> %PROJECT_NAME%_debug ubuntu@^<vm-ip^>:~/server_app/
echo   scp -i ^<keyfile^> %PROJECT_NAME% ubuntu@^<vm-ip^>:~/server_app/
echo.
echo To run on VM:
echo   ssh ubuntu@^<vm-ip^> '~/server_app/%PROJECT_NAME%_debug'
