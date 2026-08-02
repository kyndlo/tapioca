# Tapioca Desktop

Electron, React, TypeScript, and Vite foundation for the Tapioca desktop app.
This package currently provides the secure application shell and typed
sidecar boundary. Feature screens are intentionally placeholders.

## Requirements

- Node.js 22 or newer
- npm 10 or newer
- The Tapioca repository checked out locally

## Local development

```bash
cd desktop
npm install
npm run dev
```

The development command starts Vite, watches the Electron main and preload
bundles, and launches Electron after all three are ready. Its `predev` hook
first runs `go build` with fixed arguments and writes the host binary to
`../bin/tapioca-control` (`.exe` on Windows). `npm run build` performs the same
sidecar build automatically.

## Verification

```bash
npm test
npm run typecheck
npm run build
npm start
npm run package
```

`npm start` uses the production renderer bundle, so run `npm run build` first.

## Security model

- Renderer sandboxing and context isolation are enabled.
- Node integration is disabled in the renderer.
- Navigation and popup creation are denied by default.
- The renderer receives a narrow, typed `window.tapioca` API from the preload.
- Every request and response crossing the IPC boundary is validated with Zod.
- A restrictive Content Security Policy is applied in HTML and response
  headers.

## Sidecar protocol

`src/shared/sidecar.ts` implements the canonical numeric v1 NDJSON envelopes
shared with `../contracts/control/v1`. The Electron main process launches
`tapioca-control` directly with `shell: false`, completes a protocol handshake,
correlates responses by request ID, and forwards validated job events through
the narrow preload API. Lines are capped at 4 MiB and diagnostics are drained
separately from protocol output.

Development builds discover the sidecar at `../bin/tapioca-control` relative to
the desktop app root. The app window is created only after the handshake has
either succeeded or definitively failed, so its first runtime status cannot
race startup. Packaged builds use
`resources/sidecar/tapioca-control` (`.exe` on Windows). Shutdown closes stdin,
waits for a graceful exit, and uses a bounded platform-specific termination
strategy only when needed. Windows termination calls `taskkill` directly with
fixed `/PID <pid> /T` arguments and adds `/F` only for the forced phase; no
command shell or interpolated command string is used.

`npm run package` uses the checked-in electron-builder configuration. It embeds
the host sidecar at that exact resources path and creates host-native artifacts
under `release/`. Signing credentials are intentionally supplied by the release
environment rather than stored in this package.

## Structure

```text
electron/          Electron main process and preload bridge
src/app/           App shell and navigation
src/routes/        Placeholder feature screens
src/shared/        IPC and NDJSON contracts
src/styles/        Tapioca design tokens and application styles
public/            Copied, package-local brand assets
```
