$ErrorActionPreference = "Stop"

$repo = "kyndlo/tapioca"
$arch = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString().ToLower()
switch ($arch) {
    "x64" { $asset = "tapioca-windows-amd64.zip" }
    "arm64" { $asset = "tapioca-windows-arm64.zip" }
    default { throw "Unsupported Windows architecture: $arch" }
}

$installRoot = if ($env:TAPIOCA_INSTALL_DIR) { $env:TAPIOCA_INSTALL_DIR } else { Join-Path $HOME "Apps\tapioca" }
$temporary = Join-Path ([System.IO.Path]::GetTempPath()) ("tapioca-install-" + [guid]::NewGuid())
$archive = Join-Path $temporary $asset
$url = "https://github.com/$repo/releases/latest/download/$asset"

try {
    New-Item -ItemType Directory -Force $temporary | Out-Null
    Write-Host "Downloading Tapioca for Windows $arch..."
    Invoke-WebRequest -Uri $url -OutFile $archive
    $checksumFile = "$archive.sha256"
    Invoke-WebRequest -Uri "${url}.sha256" -OutFile $checksumFile
    $expected = ((Get-Content $checksumFile -Raw).Trim() -split "\s+")[0].ToLower()
    $actual = (Get-FileHash $archive -Algorithm SHA256).Hash.ToLower()
    if (-not $expected -or $actual -ne $expected) {
        throw "Downloaded Tapioca archive checksum did not match"
    }
    New-Item -ItemType Directory -Force $installRoot | Out-Null
    Expand-Archive -Path $archive -DestinationPath $installRoot -Force

    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    if (($userPath -split ";") -notcontains $installRoot) {
        $updated = if ($userPath) { "$userPath;$installRoot" } else { $installRoot }
        [Environment]::SetEnvironmentVariable("Path", $updated, "User")
        $env:Path = "$env:Path;$installRoot"
    }
    Write-Host "Installed Tapioca at $installRoot"
    & (Join-Path $installRoot "tapioca.exe") version
    Write-Host "Open a new PowerShell window, then run: tapioca run qwen3:4b-q4_k_m"
}
finally {
    if (Test-Path $temporary) {
        Remove-Item -Recurse -Force $temporary
    }
}
