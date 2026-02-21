GO ?= go
GOFLAGS ?= -tags netgo
GOARCH ?= $(shell go env GOARCH)
DEBUG ?= 0

KERNEL ?= linux

CACHE_PATH ?= ${CURDIR}/_cache
DOCGEN_OUT ?= ${CACHE_PATH}/docs

-include config.inc

DEVICE_CACHE_PATH ?= ${CACHE_PATH}/${GOARCH}

SYSTEM_PATH	?= ${DEVICE_CACHE_PATH}/system
SYSTEM_IMAGE ?= ${SYSTEM_PATH}.img

INITRAMFS_PATH ?= ${DEVICE_CACHE_PATH}/initramfs
INITRAMFS_IMAGE ?= ${INITRAMFS_PATH}.img

ifeq (${KERNEL},linux)
KERNEL_IMAGE ?= ${CURDIR}/external/${GOARCH}/kernel.img
else
KERNEL_IMAGE ?= ${DEVICE_CACHE_PATH}/kernel.img
endif

DISK_IMAGE ?= ${DEVICE_CACHE_PATH}/disk.img
QEMU_DEBUG_LOG ?= ${DEVICE_CACHE_PATH}/qemu-debug.log

ifeq (${GOARCH},amd64)
QEMU ?= qemu-system-x86_64
else ifeq (${GOARCH},arm64)
QEMU ?= qemu-system-aarch64
QEMU_ARCH_ARGS ?= -M virt -cpu cortex-a57
else
QEMU ?= qemu-system-${GOARCH}
endif

ifeq (${GOARCH},arm64)
KARGS ?= console=ttyAMA0,115200
else
KARGS ?= console=ttyS0 console=tty0
endif


ifeq ($(shell go env GOOS),linux)
QEMU_ACCEL ?= -accel kvm
else ifeq ($(shell go env GOOS),darwin)
QEMU_ACCEL ?= -accel hvf
endif

ifeq (${QEMU_VNC},1)
QEMU_VNC_OPTIONS = -vnc :0
endif

QEMU_COMMON_ARGS ?= -smp 2 -m 2G \
	-serial mon:stdio \
	-vga none ${QEMU_ACCEL} ${QEMU_VNC_OPTIONS} \
	-device virtio-gpu-pci \
	-device virtio-keyboard-pci \
	-device virtio-mouse-pci

QEMU_FW_ARGS ?= -drive if=pflash,file=${CURDIR}/external/${GOARCH}/firmware,readonly=on,format=raw \
	-drive if=pflash,file=${DEVICE_CACHE_PATH}/variables,format=raw

QEMU_DEBUG_ARGS ?= -d int,guest_errors,cpu_reset -D ${QEMU_DEBUG_LOG} \
	-no-reboot -no-shutdown -s -S

GENIMAGE_DEPS := ./tools/genimage/main.go ./tools/genimage/assets/btrfs-512m.img.gz
DOCGEN_DEPS := ./tools/docgen/main.go
RUN_EXTRA_ARGS = $(if $(filter 1,$(DEBUG)),$(QEMU_DEBUG_ARGS),)
KERNEL_BUILD_FLAGS = $(if $(filter 1,$(DEBUG)),$(KERNEL_DEBUG_GCFLAGS) $(KERNEL_DEBUG_LDFLAGS),$(KERNEL_RELEASE_LDFLAGS))

KERNEL_COMMON_FLAGS = GOOS=linux GOARCH=${GOARCH} CGO_ENABLED=0 ${GO} build
KERNEL_RELEASE_LDFLAGS = -ldflags="-E main._entry -T -2147479552"
KERNEL_DEBUG_GCFLAGS = -gcflags="all=-N -l"
KERNEL_DEBUG_LDFLAGS = -ldflags="-E main._entry -T -2147479552 -compressdwarf=false"

COMMANDS = distro copy delete driver filter find identity info init link list mkdir mount move net open power process read request service session shell showoff system tree uevent write
APPS = appmenu background demo dock oobe terminal filemanager imageviewer taskmanager notepad power settingsmanager waylayer
SERVICES = distro display login uevent dbgd
GO_TARGETS = $(addprefix cmd/,${COMMANDS}) $(addsuffix /exec,$(addprefix apps/,${APPS})) $(addprefix services/,${SERVICES})
CONFIG_TARGETS = $(shell find config/ -type f)
DATA_TARGETS = $(shell find data/ -type f)
SYSTEM_TARGETS = $(GO_TARGETS) ${CONFIG_TARGETS} ${DATA_TARGETS} $(addsuffix /manifest.json,$(addprefix apps/,${APPS})) $(addsuffix /icon.png,$(addprefix apps/,${APPS}))

INITRAMFS_TARGETS = init

ifeq (${RELEASE},1)
GOFLAGS += -buildvcs=false -trimpath -ldflags="-s -w -buildid="
endif

all: ${DISK_IMAGE}

.PHONY: clean update-certificates run compile_db docs

clean:
	rm -rf ${SYSTEM_PATH} ${INITRAMFS_PATH}
	rm -f ${SYSTEM_IMAGE} ${INITRAMFS_IMAGE} ${DISK_IMAGE}
ifneq (${KERNEL},linux)
	rm -f ${KERNEL_IMAGE}
endif

update-certificates:
	wget https://curl.se/ca/cacert.pem -O config/certificates/ca-certificates.crt

run: ${DISK_IMAGE} ${DEVICE_CACHE_PATH}/variables
	@if [ "${DEBUG}" = "1" ]; then mkdir -p $(dir ${QEMU_DEBUG_LOG}); fi
	${QEMU} ${QEMU_ARCH_ARGS} ${QEMU_COMMON_ARGS} ${RUN_EXTRA_ARGS} ${QEMU_FW_ARGS} \
		-drive file=$<,format=raw

compile_db:
	@:> depends.${GOARCH}.inc
	@for i in ${GO_TARGETS} ; do \
		echo "${SYSTEM_PATH}/$$i: $$(find $$(go list ${GOFLAGS} -deps avyos.dev/$${i%/exec} | grep '^avyos.dev' | sed 's#^avyos.dev/#${CURDIR}/#g') -type f -name '*.go' | sort | tr '\n' ' ')" >> depends.${GOARCH}.inc; \
	done

docs: ${DOCGEN_DEPS}
	${GO} run ./tools/docgen -out ${DOCGEN_OUT}

${DEVICE_CACHE_PATH}/variables: ${CURDIR}/external/${GOARCH}/variables
	@mkdir -p $(dir $@)
	cp -a $< $@

${DISK_IMAGE}: ${GENIMAGE_DEPS} ${KERNEL_IMAGE} ${INITRAMFS_IMAGE} ${SYSTEM_IMAGE}
	GOOS=$(shell go env GOOS) 			\
	GOARCH=$(shell go env GOARCH) 		\
	${GO} run ./tools/genimage 			\
		-target "${GOARCH}" 			\
		-kernel ${KERNEL_IMAGE} 		\
		-initrd ${INITRAMFS_IMAGE} 		\
		-rootfs ${SYSTEM_IMAGE}			\
		-protocol ${KERNEL}				\
		-kargs "${KARGS}"				\
		-limine-path ${CURDIR}/external/${GOARCH} \
		-out $@

${SYSTEM_PATH}/apps/%/exec:
	GOOS=linux GOARCH=${GOARCH} CGO_ENABLED=0 ${GO} build ${GOFLAGS} -o $@ $(@:${SYSTEM_PATH}/%/exec=avyos.dev/%)

${SYSTEM_PATH}/apps/%/manifest.json:
	@mkdir -p $(dir $@)
	cp $(@:${SYSTEM_PATH}/%=${CURDIR}/%) $@

${SYSTEM_PATH}/apps/%/icon.png:
	@mkdir -p $(dir $@)
	cp $(@:${SYSTEM_PATH}/%=${CURDIR}/%) $@

${SYSTEM_PATH}/config/%: ${CURDIR}/config/%
	@mkdir -p $(dir $@)
	cp -a $< $@

${SYSTEM_PATH}/data/%: ${CURDIR}/data/%
	@mkdir -p $(dir $@)
	cp -a $< $@

${SYSTEM_PATH}/%:
	GOOS=linux GOARCH=${GOARCH} CGO_ENABLED=0 ${GO} build ${GOFLAGS} -o $@ $(@:${SYSTEM_PATH}/%=avyos.dev/%)

${SYSTEM_IMAGE}: $(addprefix ${SYSTEM_PATH}/,${SYSTEM_TARGETS})
	mksquashfs ${SYSTEM_PATH} ${SYSTEM_IMAGE} -noappend -all-root -quiet

${INITRAMFS_PATH}/%: ${SYSTEM_PATH}/%
	@mkdir -p $(dir $@)
	cp -a $< $@

${INITRAMFS_PATH}/init: ${SYSTEM_PATH}/cmd/init
	@mkdir -p $(dir $@)
	cp -a $< $@

${INITRAMFS_IMAGE}: $(addprefix ${INITRAMFS_PATH}/,${INITRAMFS_TARGETS})
	(cd ${INITRAMFS_PATH}; find . -print0 | cpio --null --create --verbose --format=newc) > $@

${KERNEL_IMAGE}:
ifneq (${KERNEL},linux)
	@mkdir -p $(dir $@)
	${KERNEL_COMMON_FLAGS} ${KERNEL_BUILD_FLAGS} -o $@ avyos.dev/kernel
	python3 scripts/patch_phdr.py $@
endif

-include depends.${GOARCH}.inc
