[CmdletBinding()]
param(
    [ValidateSet("Catalog", "Smoke", "Heavy")]
    [string]$Tier = "Catalog",
    [string]$Tapioca = "tapioca",
    [string]$OutputDirectory = (Join-Path $PWD "tapioca-model-validation"),
    [switch]$AcceptGatedLicenses
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

function Invoke-Tapioca {
    param([Parameter(Mandatory)][string[]]$Arguments)
    Write-Host "tapioca $($Arguments -join ' ')" -ForegroundColor Cyan
    & $Tapioca @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "Tapioca exited with code $LASTEXITCODE"
    }
}

function Invoke-GatedPull {
    param([Parameter(Mandatory)][string]$Model)
    if (-not $AcceptGatedLicenses) {
        Write-Warning "Skipping gated model $Model. Re-run with -AcceptGatedLicenses after reviewing its terms and setting HF_TOKEN."
        return $false
    }
    if ([string]::IsNullOrWhiteSpace($env:HF_TOKEN) -and
        [string]::IsNullOrWhiteSpace($env:HUGGING_FACE_HUB_TOKEN)) {
        throw "HF_TOKEN or HUGGING_FACE_HUB_TOKEN is required for gated validation"
    }
    Invoke-Tapioca -Arguments @("pull", $Model, "--accept-license")
    return $true
}

Write-Host "Tapioca Windows/NVIDIA model validation" -ForegroundColor Green
& nvidia-smi
if ($LASTEXITCODE -ne 0) {
    throw "nvidia-smi failed; install or update the NVIDIA driver first"
}
Invoke-Tapioca -Arguments @("version")
Invoke-Tapioca -Arguments @("catalog")

if ($Tier -eq "Catalog") {
    Write-Host "Catalog validation complete. Use -Tier Smoke or -Tier Heavy to download and run models." -ForegroundColor Green
    exit 0
}

New-Item -ItemType Directory -Force -Path $OutputDirectory | Out-Null

# P0: current llama.cpp plus compact current-generation text model.
Invoke-Tapioca -Arguments @("pull", "gemma4:e2b-q4_0")
"Reply with only: GEMMA4_OK" | & $Tapioca run "gemma4:e2b-q4_0"
if ($LASTEXITCODE -ne 0) { throw "Gemma 4 inference smoke test failed" }

# P1: current CUDA image and voice runtimes.
Invoke-Tapioca -Arguments @(
    "image", "flux2-klein:4b-fp8-cuda",
    "--prompt", "A single pearl on black velvet, studio product photograph",
    "--width", "512", "--height", "512", "--steps", "4",
    "--output", (Join-Path $OutputDirectory "flux2-klein-fp8.png")
)
Invoke-Tapioca -Arguments @(
    "tts", "audio8-tts:0.6b", "--text", "Audio eight validation succeeded.",
    "--output", (Join-Path $OutputDirectory "audio8.wav")
)
Invoke-Tapioca -Arguments @(
    "tts", "qwen3-tts:1.7b-custom-voice", "--speaker", "Ryan",
    "--instruct", "Speak clearly and calmly", "--text", "Qwen custom voice validation succeeded.",
    "--output", (Join-Path $OutputDirectory "qwen3-custom-voice.wav")
)

if ($Tier -eq "Smoke") {
    Write-Host "Smoke validation complete: $OutputDirectory" -ForegroundColor Green
    exit 0
}

# P0 heavyweight text profile.
Invoke-Tapioca -Arguments @("pull", "qwen3.6:27b-q4_k_m")
"Reply with only: QWEN36_OK" | & $Tapioca run "qwen3.6:27b-q4_k_m"
if ($LASTEXITCODE -ne 0) { throw "Qwen 3.6 inference validation failed" }

# P2 standalone audio. Stable Audio 3 is gated.
if (Invoke-GatedPull "stable-audio-3:small-music") {
    Invoke-Tapioca -Arguments @(
        "audio", "stable-audio-3:small-music",
        "--prompt", "Soft analog synthesizer chord, no vocals",
        "--seconds", "5", "--steps", "4",
        "--output", (Join-Path $OutputDirectory "stable-audio-3.wav")
    )
}

# P2 synchronized video/audio. LTX-2.5 is gated and intentionally last because
# its curated snapshot is approximately 68 GiB.
if (Invoke-GatedPull "ltx-2.5:22b-bf16-cuda") {
    Invoke-Tapioca -Arguments @(
        "video", "ltx-2.5:22b-bf16-cuda",
        "--prompt", "Rain falling on green leaves with quiet natural ambience",
        "--width", "768", "--height", "512", "--frames", "9", "--fps", "24", "--steps", "4",
        "--output", (Join-Path $OutputDirectory "ltx-2.5.mp4")
    )
}

Write-Host "Heavy validation complete: $OutputDirectory" -ForegroundColor Green
