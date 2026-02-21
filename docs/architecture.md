# AvyOS Architecture

Technical architecture specification for AvyOS\
A Go-native operating system layer built above the Linux kernel.

--

## 1. Overview

AvyOS is a Go-native system architecture built on top of the Linux
kernel.

It replaces the traditional GNU/POSIX userspace with a fully static,
capability-oriented, Go-based runtime. The Linux kernel is treated
strictly as:

-   Hardware abstraction
-   Syscall interface
-   Isolation primitive provider

AvyOS does not depend on:

-   GNU userland
-   glibc
-   systemd
-   dbus
-   POSIX compatibility requirements

All core components are built with:

    CGO_ENABLED=0

--

## 2. Architectural Layers

    Applications (Native + distro containers)
            ↓
    Runtime Services (Graphics, Input, Network)
            ↓
    Platform Core (init, IPC, Identity)
            ↓
    Linux Kernel
            ↓
    Hardware

--

## 3. Kernel Base

The Linux kernel provides:

-   Scheduling
-   Virtual memory
-   Filesystems
-   Networking
-   Namespaces
-   cgroups
-   DRM/KMS
-   evdev

AvyOS does not rely on any traditional Linux userspace stack.

--

## 4. Platform Core Layer

### 4.1 init (PID 1)

The `init` process:

-   Runs as PID 1
-   Performs deterministic system bootstrap
-   Mounts and prepares filesystem layout
-   Starts core services
-   Supervises runtime services
-   Maintains lifecycle state

There is no systemd or SysV layer.

--

## 5. IPC Architecture

AvyOS IPC is implemented using:

-   Unix domain sockets
-   Shared memory segments
-   Sutra protocol

### Transport

-   Control plane: Unix domain sockets
-   Data plane: Shared memory
-   Message format: Sutra protocol

Properties:

-   Peer-to-peer
-   No central message broker
-   Capability-gated endpoints
-   Structured message schema
-   Namespaced services

--

## 6. Filesystem Model

### Immutable Base

    /avyos/
    ├── apps
    ├── cmd
    ├── config
    ├── services
    ├── data

The entire `/avyos` tree is immutable.

--

### Mutable Areas

    /config
    /cache
        ├── kernel/
        │   ├── process
        │   ├── sysfs
        │   ├── devices
        │   └── shared
        └── runtime
    /users
    /apps

-   `/config` -- System configuration (modifiable)
-   `/cache/kernel` -- Kernel dynamic structures
-   `/cache/runtime` -- Ephemeral runtime state
-   `/users` -- User data
-   `/apps` -- User-installed applications

--

## 7. Graphics Stack

-   DRM + KMS primary path
-   fbdev fallback
-   Custom display protocol
-   Wayland compatibility via `waylayer`

Stack:

    Application
       ↓
    AvyOS Display Protocol
       ↓
    Compositor
       ↓
    DRM/KMS (or fbdev)
       ↓
    Linux Kernel

--

## 8. Compatibility Layer

Container-based Linux compatibility runtime named `distro`.

Characteristics:

-   Custom container implementation
-   Namespace-based isolation
-   Restricted filesystem access
-   No direct access to `/avyos`
-   Optional component

--

## 9. Identity & Capability Model

Capability-first design:

-   Required capabilities declared explicitly
-   Scoped privilege per service
-   No ambient authority

Examples:

-   display
-   network
-   ipc:compositor
-   lifecycle

--

## 10. Boot Process

1.  Firmware (UEFI)
2.  Bootloader
3.  Linux kernel
4.  init (PID 1)
5.  Core service activation
6.  Runtime services
7.  Session start

--

## 11. Package & Upgrade Model (Current State)

Currently:

-   No package manager
-   No generation switching
-   No atomic upgrade mechanism

System images are built as full artifacts.

--

## 12. Security Model

Security properties:

-   Immutable system base
-   Capability-scoped IPC
-   Namespace isolation
-   Minimal default service surface
-   No legacy userland

--

## 13. Design Constraints

-   Pure Go userspace
-   CGO disabled
-   No glibc
-   No GNU stack
-   No POSIX requirement

--

## 14. Non-Goals

-   Full POSIX compliance
-   GNU compatibility
-   systemd integration
-   Traditional FHS layout

--

## 15. Current Scope

Validated targets:

-   QEMU amd64
-   QEMU arm64

--

## 16. Future Directions

-   Formal capability enforcement
-   Deterministic upgrade mechanism
-   Transactional switching
-   Extended container runtime
-   Hardware enablement expansion

--

AvyOS is a Go-native operating system architecture built on Linux kernel
primitives with an immutable core and explicit capability boundaries.
