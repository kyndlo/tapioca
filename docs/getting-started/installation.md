# Install Tapioca

Tapioca releases contain the `tapioca` command and the native runtime files
needed for the supported platform. Model weights are downloaded separately
when they are first used.

## macOS Apple Silicon

The macOS build requires an Apple Silicon Mac. Download
`tapioca-darwin-arm64.tar.gz` from
[GitHub Releases](https://github.com/kyndlo/tapioca/releases), then run:

```bash
mkdir -p "$HOME/.local/tapioca"
tar -xzf tapioca-darwin-arm64.tar.gz -C "$HOME/.local/tapioca"
export PATH="$HOME/.local/tapioca:$PATH"
tapioca version
```

Add this line to `~/.zshrc` so new Terminal windows can find Tapioca:

```bash
export PATH="$HOME/.local/tapioca:$PATH"
```

Do not move the bundled `runtime` directory away from the `tapioca`
executable. It contains llama.cpp, Metal support, and the native image runtime.

## Windows x64

Download `tapioca-windows-amd64.zip` from
[GitHub Releases](https://github.com/kyndlo/tapioca/releases). Open PowerShell
in the download directory and run:

```powershell
New-Item -ItemType Directory -Force "$HOME\Apps\tapioca"
Expand-Archive .\tapioca-windows-amd64.zip "$HOME\Apps\tapioca" -Force
$env:Path = "$HOME\Apps\tapioca;$env:Path"
tapioca version
```

Add `%USERPROFILE%\Apps\tapioca` to the Windows user `Path` environment
variable to make the command available in future PowerShell windows.

Keep `runtime`, its DLLs, and `tapioca.exe` together. GGUF text models use
Vulkan. CUDA image/video generation requires an NVIDIA GPU. Image generation
on AMD or Intel GPUs uses DirectML and Python 3.11–3.14:

```powershell
tapioca image sd-turbo:onnx-directml --prompt "A red fox in snow"
```

## Windows ARM64

Download `tapioca-windows-arm64.zip` from
[GitHub Releases](https://github.com/kyndlo/tapioca/releases), then use the
same PowerShell installation steps above with that archive name. The bundle
contains native ARM64 Tapioca and CPU llama.cpp binaries.

Image generation uses native ARM64 ONNX Runtime on the CPU:

```powershell
tapioca image sd-turbo --prompt "A red fox in snow"
```

Install native ARM64 Python 3.11–3.14 first. CPU diffusion is functional but
slower than x64 DirectML or CUDA. Windows ARM64 video diffusion is not yet
supported.

## Verify the installation

```text
tapioca version
tapioca catalog
```

If either command fails, see [Troubleshooting](../reference/troubleshooting.md).
