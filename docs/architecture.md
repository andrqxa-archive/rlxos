# AvyOS Architecture

This document describes how AvyOS is organized in source code and how major runtime components fit together.

## Architecture Goals

AvyOS is built as a pure Go userspace stack with these constraints:

- no CGO dependencies in core userspace binaries
- no POSIX-compatibility target as a design requirement
- service-first architecture for system capabilities
- small, readable codebase that is easy to iterate on

## Repository Layout

```text
avyos/
|- api/        IPC contracts (`api.json`) and generated client bindings
|- apps/       User-facing applications
|- cmd/        Command-line tools and core user commands
|- services/   Long-running daemons (display, login, settings, etc.)
|- pkg/        Shared libraries used by apps/commands/services
|- docs/       Project and user documentation
|- tools/      Build and code generation tools
|- config/     Default runtime config and service definitions
|- data/       Static runtime assets (icons, wallpapers, themes)
|- kernel/     Kernel integration/config artifacts
`- _cache/     Build outputs and generated docs
```

## Runtime Layers

### 1. Boot and Init

- boot artifacts are built into an image
- `cmd/init` runs as PID 1
- init mounts and prepares runtime state, then starts configured services

### 2. Core Services

Services expose system functionality through IPC APIs:

- `services/display` - compositor/windowing backend
- `services/login` - authentication and session startup
- `services/settings` - key/value settings storage and watch events
- `services/service` - service control and lifecycle operations
- `services/uevent` - device event forwarding
- `services/distro` - distro/container-style runtime operations

### 3. APIs and IPC Contracts

`api/<name>/api.json` defines wire-level contracts:

- service metadata (name, id, package)
- data types (request/response/event payloads)
- request and event IDs
- human-readable descriptions for docs generation

`tools/apigen` turns these contracts into generated client/server glue.

### 4. Applications and Commands

- `apps/` contains desktop applications
- `cmd/` contains shell commands and system tools
- both layers use `pkg/` libraries plus IPC clients from `api/`

### 5. Shared Libraries (`pkg/`)

`pkg/` is the reusable core used everywhere:

- rendering/input/UI support (`pkg/graphics/...`)
- IPC transport (`pkg/sutra`)
- filesystem and identity helpers (`pkg/fs`, `pkg/identity`)
- logging/formatting/utilities (`pkg/logger`, `pkg/format`, etc.)

## API and Process Model

At a high level:

1. apps/commands connect to a service client in `api/<name>`.
2. client sends typed payloads over Sutra IPC.
3. service receives request and executes privileged/system logic.
4. response/event payloads return through typed API bindings.

This keeps application logic isolated from low-level service internals.

## Documentation Model

`tools/docgen` generates the docs site with grouped sections:

- `docs/` markdown guides
- `apps/` reference pages (doc comments)
- `cmd/` reference pages (structured from `flag.Usage` output)
- `services/` reference pages (doc comments)
- `api/` reference pages (from `api.json` metadata)
- `pkg/` API reference (godoc-style exports)

## Build Artifacts

A typical build produces:

- system binaries and assets in `_cache/`
- bootable disk image(s)
- generated docs in `_cache/docs/`

## Current Scope

AvyOS is still experimental. Interfaces, service boundaries, and UX details are actively changing as the platform evolves.
