[CmdletBinding()]
param(
    [string]$ReportDirectory = (Join-Path ([System.IO.Path]::GetTempPath()) ("tapioca-validation-" + [guid]::NewGuid().ToString("N")))
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
$report = New-Item -ItemType Directory -Path $ReportDirectory -Force
$transcriptStarted = $false

function Invoke-Check {
    param([string]$Program, [string[]]$Arguments)
    Write-Host ("> " + $Program + " " + ($Arguments -join " "))
    & $Program @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "$Program failed with exit code $LASTEXITCODE"
    }
}

Push-Location $root
try {
    Start-Transcript -Path (Join-Path $report.FullName "validation.log") | Out-Null
    $transcriptStarted = $true
    if ($env:OS -ne "Windows_NT") { throw "Run this script on the Windows test PC." }

    $hardware = [ordered]@{
        date = (Get-Date).ToUniversalTime().ToString("o")
        os = (Get-CimInstance Win32_OperatingSystem).Caption
        memoryBytes = (Get-CimInstance Win32_ComputerSystem).TotalPhysicalMemory
        cpu = @((Get-CimInstance Win32_Processor).Name)
        gpu = @((Get-CimInstance Win32_VideoController).Name)
    }
    $hardware | ConvertTo-Json | Set-Content (Join-Path $report.FullName "hardware.json") -Encoding UTF8

    Invoke-Check "git" @("rev-parse", "HEAD")
    Invoke-Check "git" @("status", "--short")
    Invoke-Check "go" @("version")
    Invoke-Check "node" @("--version")
    Invoke-Check "python" @("--version")
    Invoke-Check "python" @("scripts/test-model-watch.py")
    Invoke-Check "python" @("-m", "unittest", "discover", "-s", "internal/speechruntime", "-p", "test_*.py")
    Invoke-Check "go" @("vet", "./...")
    Invoke-Check "go" @("test", "./...")
    Invoke-Check "go" @("run", "./cmd/catalog-manifest", "validate", "catalog/catalog.json")
    Invoke-Check "go" @("run", "./cmd/catalog-manifest", "validate", "catalog/candidates/granite-4.2.json")
    Invoke-Check "go" @("run", "./cmd/catalog-manifest", "validate", "catalog/candidates/audio8.json")

    $expectedHash = ((Get-Content "catalog/catalog.json.sha256" -Raw).Trim() -split '\s+')[0]
    $actualHash = (Get-FileHash "catalog/catalog.json" -Algorithm SHA256).Hash
    if ($actualHash -ine $expectedHash) { throw "Catalog SHA-256 mismatch" }

    Push-Location (Join-Path $root "desktop")
    try {
        Invoke-Check "npm.cmd" @("ci")
        Invoke-Check "npm.cmd" @("test", "--", "--reporter=dot")
        Invoke-Check "npm.cmd" @("run", "typecheck")
        Invoke-Check "npm.cmd" @("run", "build")
    } finally { Pop-Location }
    Write-Host "Repository checks passed. Real-model inference and interactive desktop QA still require separate qualification."
    Write-Host "Evidence saved to $($report.FullName)"
} finally {
    if ($transcriptStarted) { Stop-Transcript | Out-Null }
    Pop-Location
}
