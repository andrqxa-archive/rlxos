# Installation and QEMU Testing

AvyOS does not currently provide an official installer workflow.

Installation to physical hardware is not supported yet.

This page documents the supported path: testing a disk image in QEMU.

## Status

- Physical installation: not supported
- QEMU testing: supported
- VirtualBox: not tested yet
- VMware: not tested yet

If you validate VirtualBox or VMware workflows, contributions are welcome.

## Option 1: Use a prebuilt image

Get a release disk image from the project releases page and place it in a local working directory.

Expected filename pattern:

- `avyos-main-amd64.img`
- `avyos-main-arm64.img`

## Option 2: Build the image locally

From the repository root:

```bash
# Build amd64 image
make GOARCH=amd64

# Build arm64 image
make GOARCH=arm64
```

## Quick test command

If your environment is set up, the quickest path is:

```bash
# amd64 run
make GOARCH=amd64 run

# arm64 run
make GOARCH=arm64 run

# Optional: override host debug forward port (guest dbgd remains 5037)
make GOARCH=amd64 run DBG_PORT=5038
```

`make run` forwards `127.0.0.1:5037` on the host to `5037` in the guest for `dbgd`.

## Manual QEMU launch

Set `AVYOS_SOURCE` to your repository path so firmware files can be referenced.

### amd64

```bash
qemu-system-x86_64 -smp 2 -m 2G \
  -display gtk \
  -serial mon:stdio \
  -nic user,model=virtio-net-pci,hostfwd=tcp:127.0.0.1:5037-:5037 \
  -vga none \
  -device virtio-gpu-pci \
  -device virtio-keyboard-pci \
  -device virtio-mouse-pci \
  -drive if=pflash,file=$AVYOS_SOURCE/external/amd64/firmware,readonly=on,format=raw \
  -drive if=pflash,file=$AVYOS_SOURCE/external/amd64/variables,format=raw \
  -drive file=avyos-main-amd64.img,format=raw
```

### arm64

```bash
qemu-system-aarch64 -smp 2 -m 2G \
  -M virt -cpu cortex-a57 \
  -display gtk \
  -serial mon:stdio \
  -nic user,model=virtio-net-pci,hostfwd=tcp:127.0.0.1:5037-:5037 \
  -vga none \
  -device virtio-gpu-pci \
  -device virtio-keyboard-pci \
  -device virtio-mouse-pci \
  -drive if=pflash,file=$AVYOS_SOURCE/external/arm64/firmware,readonly=on,format=raw \
  -drive if=pflash,file=$AVYOS_SOURCE/external/arm64/variables,format=raw \
  -drive file=avyos-main-arm64.img,format=raw
```

## Login and first run

Default credentials for development images:

- username: `admin`
- password: `admin`

On first boot, OOBE may run depending on image state.

## Common issues

### Boots but no UI response

Check QEMU input devices:

- `virtio-keyboard-pci`
- `virtio-mouse-pci`

### Missing firmware file

Verify `AVYOS_SOURCE/external/<arch>/firmware` and `variables` exist.

### Image not found

Run QEMU from the directory where the disk image is located or pass an absolute path.
