$ErrorActionPreference = 'Stop'
Set-Location (Split-Path -Parent $PSScriptRoot)
$env:DEMO_MODE = 'true'
$env:ADDR = '127.0.0.1:8090'
Write-Host 'Afterglow will open at http://127.0.0.1:8090. Press Ctrl+C to stop.'
go run ./cmd/afterglow
