# install.ps1 — install the golinks CLI from source (Windows)
param(
    [string]$InstallDir = "$env:LOCALAPPDATA\Programs\golinks"
)

$ErrorActionPreference = "Stop"
$Binary = "golinks.exe"

if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    Write-Error "Go is required to build golinks. Install from https://go.dev/dl/"
    exit 1
}

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RepoRoot  = Split-Path -Parent $ScriptDir

Write-Host "Building golinks CLI..."
$env:CGO_ENABLED = "0"
go build -ldflags "-s -w" -o "$env:TEMP\$Binary" "$RepoRoot\cmd\cli"

if (-not (Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir | Out-Null
}

Move-Item -Force "$env:TEMP\$Binary" "$InstallDir\$Binary"
Write-Host "Installed to $InstallDir\$Binary"

# Add to PATH if not already present
$userPath = [Environment]::GetEnvironmentVariable("PATH", "User")
if ($userPath -notlike "*$InstallDir*") {
    [Environment]::SetEnvironmentVariable("PATH", "$userPath;$InstallDir", "User")
    Write-Host "Added $InstallDir to your PATH. Restart your terminal to apply."
}

Write-Host ""
Write-Host "Done! Verify with: golinks --help"
Write-Host ""
Write-Host "Quick start:"
Write-Host "  golinks config set server http://localhost:8080"
Write-Host "  golinks ls"
