# Build Tapioca from source

Building requires Go 1.25 or newer:

```bash
make build
```

Source builds can use a separately installed llama.cpp. On macOS:

```bash
brew install llama.cpp
```

Building the native macOS image runtime requires Xcode 26, Swift 6.2 or newer,
and the Xcode Metal Toolchain:

```bash
xcodebuild -downloadComponent MetalToolchain
```

Official GitHub release archives bundle the platform runtime dependencies and
are the simpler choice for most users.
