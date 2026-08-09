# Install Tapioca

Tapioca releases contain the `tapioca` command and the native runtime files
needed for the supported platform. Model weights are downloaded separately
when they are first used.

## macOS Apple Silicon

The simplest installation is:

```bash
curl -fsSL https://tapioca.rootfruit.cc/install.sh | sh
```

The installer verifies the archive, installs under your user account, and
adds `~/.local/bin` to the appropriate shell profile. Open a new Terminal
window after it completes.

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

The simplest PowerShell installation is:

```powershell
irm https://tapioca.rootfruit.cc/install.ps1 | iex
```

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
variable to make the command available in future PowerShell windows. The
one-command installer does this automatically and verifies the release archive
before extracting it.

Keep `runtime`, its DLLs, and `tapioca.exe` together. GGUF text models use
Vulkan. CUDA image/video generation requires an NVIDIA GPU. Image generation
on AMD or Intel GPUs uses DirectML and Python 3.11–3.14:

```powershell
tapioca image sd-turbo:onnx-directml --prompt "A red fox in snow"
```

MiniMax-H3 does not require a system Python installation, Git, or the CUDA
Toolkit on Windows x64. Tapioca downloads a pinned Python 3.12 environment and
its private media runtime on the first generation. Install only a current
NVIDIA driver. The runtime is cached under `%USERPROFILE%\.tapioca\runtime`.

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

## Linux x64

The installer detects x64 and ARM64 automatically:

```bash
curl -fsSL https://tapioca.rootfruit.cc/install.sh | sh
```

The x64 bundle includes Vulkan-enabled llama.cpp for GGUF text models. NVIDIA
CUDA is used by compatible Diffusers image/video models and PyTorch speech
models. Install Python 3.10 or newer and a current NVIDIA driver before the
first image, video, or speech run.

Manual archive name: `tapioca-linux-amd64.tar.gz`.

## Linux ARM64

Use the same installer command. The ARM64 bundle includes native Tapioca and
Vulkan llama.cpp. Text models and CPU speech generation are supported; CUDA
image/video generation is not included in the first ARM64 release.

Manual archive name: `tapioca-linux-arm64.tar.gz`.

## Verify the installation

```text
tapioca version
tapioca catalog
```

If either command fails, see [Troubleshooting](../reference/troubleshooting.md).
