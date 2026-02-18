<p align="center">
  <img src="data/icons/logo/logo.png" width="180" alt="AvyOS Logo"/>
</p>

<h1 align="center">AvyOS</h1>

<p align="center">
  A Linux-based operating system written entirely in pure Go —<br/>
  no external packages, no CGO, no POSIX compatibility.
</p>

<p align="center">
  <img src="docs/assets/interface.png" width="60%" alt="Interface"/>
</p>

## Overview

AvyOS is an experimental operating system that reimagines the traditional Linux userspace from the ground up. Everything — from init to shell to the tiling window manager — is implemented in pure Go with zero C dependencies.

**Key characteristics:**

- **Pure Go** — Built with `CGO_ENABLED=0`, no C toolchain required
- **Non-POSIX** — Clean, modern interfaces without legacy baggage
- **TUI-only** — Terminal-based interface (like tmux meets tiling WM)
- **Immutable core** — System files in `/avyos/` are read-only
- **Containerized apps** — Applications run with capability-based isolation

## Documentation

| Document                                   | Description                                |
| ------------------------------------------ | ------------------------------------------ |
| [Filesystem Hierarchy](docs/filesystem.md) | AvyOS directory structure and mount points |
| [Architecture](docs/architecture.md)       | Source code organization and components    |
| [First Boot](docs/firstboot.md)            | Initial system setup process               |
| [Desktop Guide](docs/welcome-tour.md)      | TUI desktop and keyboard shortcuts         |

## Why Go?

- **Faster development** — Go's simplicity makes building an OS userspace enjoyable
- **Easier maintenance** — Single-developer project needs readable, maintainable code
- **No CGO complexity** — Eliminates entire categories of build issues
- **Strong standard library** — Networking, crypto, compression without dependencies

## Non-Goals

AvyOS is **not** POSIX-compatible. This is intentional.

POSIX compatibility would require implementing decades of legacy interfaces. Instead, AvyOS focuses on:

- **Simplicity** — Clean interfaces designed for the system, not compatibility
- **Developer experience** — Easy to understand, modify, and extend
- **Modern design** — No baggage from the 1970s

For running POSIX applications, AvyOS provides a Linux compatibility layer at `/linux/<distro>` using containerization.

## Building

```bash
# Build disk image
make GOARCH=arm64

# Test your build
make GOARCH=arm64 run

# Manually run the release build
# Set $AVYOS = path/to/avyos source for firmware
# For arm64
qemu-system-aarch64 -smp 2 -m 2G  \
  -M virt -cpu cortex-a57         \
  -display gtk                    \
  -serial mon:stdio               \
  -vga none                       \
  -device virtio-gpu-pci          \
  -device virtio-keyboard-pci     \
  -device virtio-mouse-pci        \
  -drive if=pflash,file=$AVYOS_SOURCE/external/arm64/firmware,readonly=on,format=raw \
  -drive if=pflash,file=$AVYOS_SOURCE/external/arm64/variables,format=raw \
  -drive file=avyos-main-arm64.img,format=raw

# For amd64
qemu-system-x86_64 -smp 2 -m 2G   \
  -display gtk                    \
  -serial mon:stdio               \
  -vga none                       \
  -device virtio-gpu-pci          \
  -device virtio-keyboard-pci     \
  -device virtio-mouse-pci        \
  -drive if=pflash,file=$AVYOS_SOURCE/external/amd64/firmware,readonly=on,format=raw \
  -drive if=pflash,file=$AVYOS_SOURCE/external/amd64/variables,format=raw \
  -drive file=avyos-main-amd64.img,format=raw
```

> Use __admin:admin__ as default credentials

## Status

AvyOS is experimental — a proof of concept that a usable Linux userspace can be built entirely in Go.

| Component          | Status    | Notes                                               |
| ------------------ | --------- | --------------------------------------------------- |
| Boot & Init        | ✅ Done    | Basic init system with service manager              |
| Shell              | ✅ Done    | Interactive shell with builtins                     |
| IPC                | ✅ Done    | Sutra message bus + code generator                  |
| UI Toolkit         | 🟡 Basic   | Basic widget toolkit (wayland, drmkms, fb, display) |
| Desktop/WM         | 🟡 Minimal | Basic window manager with taskbar support           |
| Networking         | 🟡 Basic   | Static IPv4 support only                            |
| Core Utils         | 🟡 Minimal | 23 essential commands                               |
| Audio              | ❌ TODO    | ALSA or direct hardware                             |
| USB/Input          | ❌ TODO    | Device hotplug, input handling                      |
| Package Manager    | ❌ TODO    | Package format, repos                               |
| Hardware Detection | ❌ TODO    | PCI, device enumeration                             |

## Acknowledgments

This project was developed with the help of LLM models.

## License

GNU General Public License v3.0
