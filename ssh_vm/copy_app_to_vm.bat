@echo off
setlocal

set KEY_PATH=id_ed25519_windows_to_vm
set USER=ubuntu
set HOST=170.9.225.9
set REMOTE_PATH=~/

:: Check if a file was provided
if "%~1"=="" (
    echo Usage: Drag and drop a file onto this script, or run: copy_to_vm.bat "path\to\file"
    pause
    exit /b 1
)

set FILE_PATH=%~1

:: Resolve key path relative to this script
cd /d "%~dp0"
if not exist "%KEY_PATH%" (
    echo SSH key not found at '%KEY_PATH%'
    pause
    exit /b 1
)

:: Check if the file to copy exists
if not exist "%FILE_PATH%" (
    echo File not found: %FILE_PATH%
    pause
    exit /b 1
)

:: Check if OpenSSH is available
where scp >nul 2>&1
if errorlevel 1 (
    echo OpenSSH client not found. Install 'OpenSSH Client' in Windows Optional Features.
    pause
    exit /b 1
)

:: Fix SSH key permissions
echo Fixing SSH key permissions...
icacls "%KEY_PATH%" /inheritance:r >nul 2>&1
icacls "%KEY_PATH%" /grant:r "%USERNAME%:R" >nul 2>&1
icacls "%KEY_PATH%" /remove "BUILTIN\Users" >nul 2>&1

:: Copy file to VM
echo Copying %FILE_PATH% to %USER%@%HOST%:%REMOTE_PATH%
scp -i "%KEY_PATH%" "%FILE_PATH%" %USER%@%HOST%:%REMOTE_PATH%

if errorlevel 1 (
    echo.
    echo Copy failed!
    pause
    exit /b 1
) else (
    echo.
    echo Copy successful!
    pause
)
