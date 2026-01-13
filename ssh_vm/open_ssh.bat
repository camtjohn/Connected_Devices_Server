@echo off
setlocal

set KEY_PATH=id_ed25519_windows_to_vm
set USER=ubuntu
set HOST=170.9.225.9

:: Resolve key path relative to this script
cd /d "%~dp0"
if not exist "%KEY_PATH%" (
    echo SSH key not found at '%KEY_PATH%'
    pause
    exit /b 1
)

:: Check if OpenSSH is available
where ssh >nul 2>&1
if errorlevel 1 (
    echo OpenSSH client not found. Install 'OpenSSH Client' in Windows Optional Features.
    pause
    exit /b 1
)

:: Fix SSH key permissions (remove inheritance and restrict to current user only)
echo Fixing SSH key permissions...
icacls "%KEY_PATH%" /inheritance:r >nul 2>&1
icacls "%KEY_PATH%" /grant:r "%USERNAME%:R" >nul 2>&1
icacls "%KEY_PATH%" /remove "BUILTIN\Users" >nul 2>&1

:: Launch new PowerShell window with SSH connection that stays open
start "SSH Connection" powershell.exe -NoExit -Command "ssh -i '%KEY_PATH%' %USER%@%HOST%"
