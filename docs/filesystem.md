# AvyOS Filesystem Hierarchy

AvyOS uses a redesigned filesystem hierarchy that separates immutable system files from mutable user data.

## Overview

```
/
├── avyos/          # Immutable system root (squashfs)
│   ├── cmd/        # System commands
│   ├── config/     # Default configurations
│   ├── data/       # Static data files
│   ├── apps/       # User applications
│   └── services/   # System services
├── config/         # Mutable config overrides
├── users/          # User home directories
├── cache/          # Runtime and kernel filesystems
│   ├── kernel/     # Kernel-managed directories
│   └── runtime/    # Runtime state (like /run)
├── linux/          # Linux compatibility layer
└── volumes/        # User data volumes
```

## Immutable System (`/avyos/`)

The `/avyos/` directory contains the immutable system image, mounted as read-only squashfs.

### `/avyos/cmd/` — Commands

System commands and utilities. Mounted with `nosuid, nodev`.

| Command    | Description                        |
| ---------- | ---------------------------------- |
| `init`     | Init system (PID 1)                |
| `shell`    | Interactive command shell          |
| `system`   | System management                  |
| `power`    | Power management (shutdown/reboot) |
| `list`     | List directory contents            |
| `read`     | Read file contents                 |
| `write`    | Write to files                     |
| `copy`     | Copy files                         |
| `move`     | Move/rename files                  |
| `delete`   | Delete files                       |
| `mkdir`    | Create directories                 |
| `link`     | Create links                       |
| `find`     | Search for files                   |
| `tree`     | Display directory tree             |
| `info`     | File information                   |
| `mount`    | Mount filesystems                  |
| `net`      | Network configuration              |
| `process`  | Process management                 |
| `identity` | User/identity operations           |
| `request`  | IPC request tool                   |

### `/avyos/config/` — Default Configuration

Default system configuration files. Mounted with `noexec, nodev, nosuid`. Only text files allowed.

```
/avyos/config/
├── init.conf           # Init system configuration
├── services/           # Service definitions
│   ├── login.service
│   └── sutra.service
└── security/           # Security configurations
    ├── auth.conf
    ├── identity.conf
    └── capabilities.conf
```

### `/avyos/data/` — Static Data

Static data files like icons, fonts, and themes. Mounted with `noexec, nodev, nosuid`.

```
/avyos/data/
├── icons/
├── fonts/
└── themes/
```

### `/avyos/apps/` — Applications

User-facing applications. Run inside containers with capability restrictions.

| Application | Description              |
| ----------- | ------------------------ |
| `welcome`   | First-boot setup wizard  |
| `notepad`   | Text editor (nano-style) |
| `browser`   | Web browser              |

### `/avyos/services/` — System Services

Privileged system services that run with elevated capabilities.

| Service   | Description                  |
| --------- | ---------------------------- |
| `sutra`   | IPC message bus              |
| `login`   | Authentication/login manager |
| `desktop` | Tiling window manager        |

## Mutable Directories

### `/config/` — Configuration Overrides

Mutable configuration that overrides `/avyos/config/`. When a config file exists in both locations, `/config/` takes precedence.

Mounted with `noexec, nodev, nosuid`.

```
/config/
├── .firstboot-done     # Marker file after setup
├── init.conf           # Custom init config
└── security/
    └── identity.conf   # User identity database
```

### `/users/` — Home Directories

User home directories. Each user gets a directory at `/users/<username>/`.

```
/users/
└── alice/
    ├── .profile
    └── documents/
```

### `/cache/` — Runtime State

Runtime and temporary filesystems.

#### `/cache/kernel/` — Kernel Filesystems

Kernel-managed virtual filesystems:

| Path                      | Type     | Description               |
| ------------------------- | -------- | ------------------------- |
| `/cache/kernel/processes` | procfs   | Process information       |
| `/cache/kernel/sysfs`     | sysfs    | System/device information |
| `/cache/kernel/devices`   | devtmpfs | Device nodes              |
| `/cache/kernel/shared`    | tmpfs    | Shared memory             |

#### `/cache/runtime/` — Runtime State

Runtime state directory (equivalent to `/run` on traditional Linux).

```
/cache/runtime/
├── sutra.sock      # IPC socket
└── services/       # Service state
```

## Linux Compatibility Layer (`/linux/`)

For running traditional Linux applications, AvyOS provides containerized Linux environments:

```
/linux/
├── alpine/         # Alpine Linux container
├── debian/         # Debian container
└── fedora/         # Fedora container
```

Each container runs in isolation using a bwrap-like sandbox with:
- Namespace isolation (mount, pid, network)
- Capability restrictions
- Filesystem overlays

## Mount Options Summary

| Directory         | Mount Options           | Purpose                      |
| ----------------- | ----------------------- | ---------------------------- |
| `/avyos/`         | `ro`                    | Immutable system image       |
| `/avyos/cmd/`     | `nosuid, nodev`         | Prevent privilege escalation |
| `/avyos/config/`  | `noexec, nodev, nosuid` | Config files only            |
| `/avyos/data/`    | `noexec, nodev, nosuid` | Data files only              |
| `/config/`        | `noexec, nodev, nosuid` | Mutable configs              |
| `/users/`         | `nosuid, nodev`         | User data                    |
| `/cache/kernel/`  | varies                  | Kernel filesystems           |
| `/cache/runtime/` | `nosuid, nodev`         | Runtime state                |

## Design Principles

1. **Immutability** — System files cannot be modified at runtime
2. **Separation** — Clear boundaries between system, config, and user data
3. **Security** — Mount options prevent execution where not needed
4. **Simplicity** — Flat, predictable hierarchy
5. **Atomicity** — System updates replace the entire `/avyos/` image
