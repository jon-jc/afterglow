param([switch]$NoBrowser, [ValidateRange(1024,65535)][int]$Port = 8090, [string]$DataDirectory = '')
$ErrorActionPreference = 'Stop'
$projectDirectory = Split-Path -Parent $PSScriptRoot
Set-Location -LiteralPath $projectDirectory
$url = "http://127.0.0.1:$Port"
function Test-Afterglow {
    try {
        $runtime = Invoke-RestMethod "$url/api/v1/runtime" -TimeoutSec 2
        $ready = Invoke-RestMethod "$url/readyz" -TimeoutSec 2
        $page = Invoke-WebRequest "$url/" -UseBasicParsing -TimeoutSec 2
        return ($runtime.demo -eq $true -and $ready.status -eq 'ready' -and $page.Content.Contains('<title>Afterglow'))
    } catch { return $false }
}
function Open-Afterglow {
    Write-Host "Afterglow is ready: $url" -ForegroundColor Green
    Write-Host 'You can close this window. Afterglow keeps running in the background.'
    if (-not $NoBrowser) { Start-Process "$url/?launcher=1#overview" }
}
$mutex = New-Object System.Threading.Mutex($false, "Local\Afterglow-Launcher-$Port")
$ownsMutex = $false
try {
    try { $ownsMutex = $mutex.WaitOne(60000) } catch [System.Threading.AbandonedMutexException] { $ownsMutex = $true }
    if (-not $ownsMutex) { throw 'Another launch is still in progress. Try again in a moment.' }
    if (Test-Afterglow) { Open-Afterglow; exit 0 }
    $client = New-Object System.Net.Sockets.TcpClient
    $occupied = $false
    try { $client.Connect('127.0.0.1', $Port); $occupied = $true } catch {} finally { $client.Dispose() }
    if ($occupied) { throw "Port $Port is used by another application or an unhealthy instance. No process was stopped." }
    if (-not (Get-Command go -ErrorAction SilentlyContinue)) { throw 'Go is not on PATH. Install Go 1.27 or later, then double-click Start Afterglow again.' }
    $binaryDirectory = Join-Path $projectDirectory 'bin'
    $logDirectory = Join-Path $projectDirectory 'artifacts'
    if (-not $DataDirectory) { $DataDirectory = Join-Path $projectDirectory 'data' }
    foreach ($directory in @($binaryDirectory,$logDirectory,$DataDirectory)) { New-Item -ItemType Directory -Force -Path $directory | Out-Null }
    $binary = Join-Path $binaryDirectory "afterglow-launcher-$Port.exe"
    Write-Host 'Preparing Afterglow (the first launch may download Go dependencies)...'
    & go build -o $binary ./cmd/afterglow
    if ($LASTEXITCODE -ne 0) { throw 'The Go build failed. Review the message above; your existing data has not been changed.' }
    $settings = @{
        DEMO_MODE='true'; ADDR="127.0.0.1:$Port"; ROLE='all'; TRANSPORT='local'
        DATABASE_URL=(Join-Path ([System.IO.Path]::GetFullPath($DataDirectory)) 'afterglow.db')
        API_KEY=''; API_KEY_PREVIOUS=''; TENANT_ID='demo'
    }
    $prior = @{}
    try {
        foreach ($key in $settings.Keys) { $prior[$key]=[Environment]::GetEnvironmentVariable($key,'Process'); [Environment]::SetEnvironmentVariable($key,$settings[$key],'Process') }
        $process = Start-Process -FilePath $binary -WorkingDirectory $projectDirectory -WindowStyle Hidden -PassThru -RedirectStandardOutput (Join-Path $logDirectory "launcher-$Port.log") -RedirectStandardError (Join-Path $logDirectory "launcher-$Port-error.log")
    } finally { foreach ($key in $prior.Keys) { [Environment]::SetEnvironmentVariable($key,$prior[$key],'Process') } }
    $deadline = (Get-Date).AddSeconds(25)
    do {
        if (Test-Afterglow) { Open-Afterglow; exit 0 }
        $process.Refresh()
        if ($process.HasExited) { throw "Afterglow exited during startup. See artifacts/launcher-$Port-error.log." }
        Start-Sleep -Milliseconds 300
    } while ((Get-Date) -lt $deadline)
    throw "Startup is taking longer than expected. See artifacts/launcher-$Port-error.log, or try $url again shortly."
} catch {
    Write-Host "Could not start Afterglow: $($_.Exception.Message)" -ForegroundColor Red
    exit 1
} finally {
    if ($ownsMutex) { $mutex.ReleaseMutex() }
    $mutex.Dispose()
}
