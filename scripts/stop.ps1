param([ValidateRange(1024,65535)][int]$Port = 8090)
$ErrorActionPreference = 'Stop'
try {
    $binaryDirectory = [IO.Path]::GetFullPath((Join-Path (Split-Path -Parent $PSScriptRoot) 'bin')) + [IO.Path]::DirectorySeparatorChar
    $listeners = @(Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction SilentlyContinue)
    if (-not $listeners) { Write-Host 'Afterglow is already stopped.'; exit 0 }
    foreach ($ownerId in ($listeners.OwningProcess | Select-Object -Unique)) {
        $appProcess = Get-Process -Id $ownerId -ErrorAction Stop
        $processPath = $appProcess.Path
        if (-not $processPath -or -not $processPath.StartsWith($binaryDirectory,[StringComparison]::OrdinalIgnoreCase) -or [IO.Path]::GetFileName($processPath) -notlike 'afterglow*.exe') {
            throw "Port $Port belongs to another application. It was not stopped."
        }
        Stop-Process -Id $ownerId
    }
    Write-Host 'Afterglow stopped. Your saved demo data remains in the data folder.'
} catch { Write-Host $_.Exception.Message -ForegroundColor Red; exit 1 }
