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
    New-Item -ItemType Directory -Force $installRoot | Out-Null
    Expand-Archive -Path $archive -DestinationPath $installRoot -Force

    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    if (($userPath -split ";") -notcontains $installRoot) {
        $updated = if ($userPath) { "$userPath;$installRoot" } else { $installRoot }
        [Environment]::SetEnvironmentVariable("Path", $updated, "User")
        $env:Path = "$env:Path;$installRoot"
    }
    Write-Host "Installed Tapioca at $installRoot"
    Write-Host "Open a new PowerShell window, then run: tapioca version"
}
finally {
    if (Test-Path $temporary) {
        Remove-Item -Recurse -Force $temporary
    }
}
