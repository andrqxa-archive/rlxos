# AvyOS Architecture

This document describes the source code organization and system architecture of AvyOS.

## Source Tree

```
avyos/
├── cmd/            # Command-line utilities
├── apps/           # User applications
├── services/       # System services
├── pkg/            # Core libraries
├── tools/          # Development tools
├── config/         # Default configurations
├── data/           # Static data files
├── scripts/        # Build scripts
├── external/       # Cross-platform binaries
├── docs/           # Documentation
└── _cache/         # Build artifacts
```

## Core Packages (`pkg/`)

The `pkg/` directory contains reusable libraries that form the foundation of AvyOS.

### UI Toolkit (`pkg/ui/`)

A complete TUI widget toolkit for building terminal applications.

**Core (`pkg/ui/`):**
- `app.go` — Application lifecycle management
- `canvas.go` — Drawing surface and rendering
- `widget.go` — Base widget interface
- `style.go` — Styling and theming
- `event.go` — Event types (keyboard, mouse)
- `layout.go` — Layout calculations

**Widgets (`pkg/ui/widgets/`):**
| Widget | Description |
|--------|-------------|
| `box.go` | Container with borders |
| `button.go` | Clickable button |
| `input.go` | Single-line text input |
| `textarea.go` | Multi-line text area |
| `list.go` | Selectable list |
| `table.go` | Data table |
| `tabs.go` | Tab container |
| `scroll.go` | Scrollable container |
| `modal.go` | Modal dialog |
| `checkbox.go` | Checkbox input |
| `radio.go` | Radio button group |
| `progress.go` | Progress bar |
| `select.go` | Dropdown select |
| `flex.go` | Flexbox layout |
| `stack.go` | Stack layout |
| `text.go` | Static text display |

**Backend (`pkg/ui/backend/`):**
- `term.go` — Terminal rendering backend

### IPC System (`pkg/sutra/`)

Message bus for inter-process communication.

- `bus.go` — Message bus implementation
- `client.go` — Client connection handling
- `service.go` — Service registration
- `protocol.go` — Wire protocol

### Identity System (`pkg/identity/`)

User authentication and capability management.

- `identity.go` — User identity management
- `auth.go` — Authentication (password hashing)
- `capability.go` — Capability definitions
- `types.go` — Common types

### Other Packages

| Package | File | Description |
|---------|------|-------------|
| `pkg/fs/` | `fs.go`, `resolve.go` | Filesystem utilities |
| `pkg/pty/` | `pty.go` | Pseudo-terminal support |
| `pkg/term/` | `term.go` | Terminal utilities |
| `pkg/logger/` | `logger.go` | Structured logging |
| `pkg/format/` | `format.go` | Text formatting/colors |
| `flag` (stdlib) | `flag` package | Command-line parsing |
| `pkg/ini/` | `ini.go` | INI file parsing |

## Commands (`cmd/`)

Commands are standalone executables that provide system functionality.

### System Management

| Command | Description |
|---------|-------------|
| `cmd/init/` | Init system (PID 1), service supervisor |
| `cmd/shell/` | Interactive command shell |
| `cmd/system/` | System management utilities |
| `cmd/power/` | Shutdown, reboot, suspend |

### File Operations

| Command | Description |
|---------|-------------|
| `cmd/list/` | List directory contents |
| `cmd/read/` | Read file contents |
| `cmd/write/` | Write to files |
| `cmd/copy/` | Copy files and directories |
| `cmd/move/` | Move/rename files |
| `cmd/delete/` | Delete files |
| `cmd/mkdir/` | Create directories |
| `cmd/link/` | Create symbolic links |
| `cmd/find/` | Search for files |
| `cmd/tree/` | Display directory tree |
| `cmd/info/` | File information |
| `cmd/mount/` | Mount filesystems |

### Networking & IPC

| Command | Description |
|---------|-------------|
| `cmd/net/` | Network configuration |
| `cmd/request/` | Send IPC requests |

### Process & Identity

| Command | Description |
|---------|-------------|
| `cmd/process/` | Process management |
| `cmd/identity/` | User management |

## Applications (`apps/`)

Applications are user-facing programs with TUI interfaces.

### Welcome (`apps/welcome/`)

First-boot setup wizard. Features:
- Multi-page wizard UI
- Language selection (12 languages)
- Timezone configuration (15+ zones)
- User account creation
- Password setup with strength indicator

### Notepad (`apps/notepad/`)

Simple text editor with nano-style keybindings.

### Browser (`apps/browser/`)

Web browser application.

## Services (`services/`)

Services are long-running daemons with elevated privileges.

### Sutra (`services/sutra/`)

IPC message bus service.
- Listens on Unix socket (`/cache/runtime/sutra.sock`)
- Routes messages between clients and services
- Handles service registration and discovery

### Login (`services/login/`)

Authentication and login manager.
- Displays login UI with ASCII logo
- Authenticates users against identity database
- Launches first-boot wizard if needed
- Starts desktop service after successful login

### Desktop (`services/desktop/`)

Tiling window manager (compositor).
- BSP (Binary Space Partitioning) layout
- Grid, Main+Stack, and Spiral layouts
- Real PTY for each terminal pane
- Prefix-key commands (Ctrl+A)
- Vim-style command mode
- Mouse support

## Init System

The init system (`cmd/init/`) is PID 1 and manages:

1. **Early boot** — Mount filesystems, set up environment
2. **Service supervision** — Start and monitor services
3. **Process reaping** — Clean up orphaned processes

### Configuration (`config/init.conf`)

```ini
[init]
services = sutra login
```

### Service Files (`config/services/*.service`)

```ini
[service]
name = login
command = /avyos/services/login
tty = /cache/kernel/devices/tty1
restart = always
depends = firstboot
```

## Build System

### Makefile Targets

```bash
make GOARCH=arm64       # Build disk image
make GOARCH=arm64 run   # Build and run in QEMU
make clean              # Clean build artifacts
```

### Build Process

1. Compile Go binaries → `_cache/<arch>/system/`
2. Copy config and data files
3. Create squashfs image (`system.img`)
4. Create initramfs (`initramfs.img`)
5. Generate bootable disk with Limine bootloader

### Build Flags

- `CGO_ENABLED=0` — Pure Go, no C dependencies
- `-tags netgo` — Pure Go networking
- `-ldflags="-s -w"` — Strip debug info

## Security Model

### Capability-Based Access

Users have capabilities that grant permissions:

```
Identity:
  - ID: 1000
  - Name: alice
  - Home: /users/alice
  - Capabilities: ["unix:users", "unix:audio", "unix:video"]
  - Shell: /avyos/cmd/shell
```

### Authentication

Passwords are hashed using bcrypt:

```
Auth:
  - Type: "password" | "none" | "locked"
  - Hash: bcrypt hash
```

### Application Isolation

Applications in `/avyos/apps/` run with:
- Restricted capabilities
- Container isolation (planned)
- Limited filesystem access

## Data Flow

```
┌─────────────────────────────────────────────────────────┐
│                         Init                             │
│                    (PID 1, /cmd/init)                   │
└─────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────┐
│                    Sutra (IPC Bus)                      │
│                  (/services/sutra)                      │
└─────────────────────────────────────────────────────────┘
                            │
            ┌───────────────┼───────────────┐
            ▼               ▼               ▼
     ┌───────────┐   ┌───────────┐   ┌───────────┐
     │   Login   │   │  Desktop  │   │   Apps    │
     │ (service) │──▶│ (service) │──▶│ (welcome, │
     └───────────┘   └───────────┘   │  notepad) │
                                     └───────────┘
```

## Module Dependencies

```
avyos.dev
├── pkg/ui          (no deps)
├── pkg/term        (no deps)
├── pkg/pty         → pkg/term
├── pkg/logger      (no deps)
├── pkg/format      (no deps)
├── pkg/ini         (no deps)
├── pkg/fs          (no deps)
├── pkg/identity    → pkg/ini
├── pkg/sutra       (no deps)
├── cmd/*           → pkg/*
├── apps/*          → pkg/ui, pkg/identity
└── services/*      → pkg/ui, pkg/pty, pkg/sutra
```
