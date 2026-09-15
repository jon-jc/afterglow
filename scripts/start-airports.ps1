$ErrorActionPreference = 'Stop'
Set-Location (Split-Path -Parent $PSScriptRoot)
Write-Host 'Airport catalog: http://127.0.0.1:8091. Initial source download runs in the background. Press Ctrl+C to stop.'
go run ./cmd/airports
