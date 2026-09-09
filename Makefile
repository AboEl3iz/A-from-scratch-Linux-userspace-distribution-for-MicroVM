# ==============================================================================
# Karim MicroVM OS — Unified Production Makefile
# ==============================================================================

SHELL := /bin/bash
PROJECT_NAME := karim-microvm-os
BUILD_DIR := build
DIST_DIR := dist
KERNEL_VER := 6.6.45
KERNEL_DIR := $(BUILD_DIR)/linux-$(KERNEL_VER)
KERNEL_BZIMAGE := $(KERNEL_DIR)/arch/x86/boot/bzImage

# Toolchain definitions
CC ?= musl-gcc
CFLAGS ?= -O2 -Wall -Wextra -static -pedantic
GO ?= go
CLANG ?= clang
MKSQUASHFS ?= mksquashfs
MKERIFS ?= mkfs.erofs
CPIO ?= cpio
QEMU ?= qemu-system-x86_64

# Targets
INIT_BIN := $(BUILD_DIR)/init
SVCD_BIN := $(BUILD_DIR)/karim-svcd
SECD_BIN := $(BUILD_DIR)/karim-secd
VSOCKD_BIN := $(BUILD_DIR)/karim-vsockd
CLI_BIN  := $(DIST_DIR)/karim
INITRD_CPIO := $(DIST_DIR)/initramfs.cpio
ROOTFS_IMG  := $(DIST_DIR)/rootfs.sqsh

.PHONY: all check-tools kernel init svcd secd vsockd ebpf initramfs rootfs build-image run run-debug test clean distclean help

all: check-tools init svcd secd vsockd ebpf initramfs rootfs cli ## Build complete Karim MicroVM artifacts

# ------------------------------------------------------------------------------
# 1. Tooling Verification
# ------------------------------------------------------------------------------
check-tools: ## Check if essential build tools exist on the host
	@echo "==> Verifying host toolchain..."
	@which $(CC) > /dev/null || (echo "WARNING/NOTICE: $(CC) not found. Install musl-tools (sudo apt install musl-tools) or fallback to gcc -static." && exit 1)
	@which $(GO) > /dev/null || (echo "ERROR: $(GO) not found. Install Go." && exit 1)
	@which $(CLANG) > /dev/null || (echo "ERROR: $(CLANG) not found. Install clang." && exit 1)
	@which $(QEMU) > /dev/null || (echo "ERROR: $(QEMU) not found. Install qemu-system-x86." && exit 1)
	@which $(MKSQUASHFS) > /dev/null || (echo "ERROR: $(MKSQUASHFS) not found. Install squashfs-tools." && exit 1)
	@echo "==> Toolchain check passed."

KERNEL_WORK_DIR := /tmp/karim-kernel-$(KERNEL_VER)

"$(DIST_DIR)/bzImage":
	@mkdir -p "$(BUILD_DIR)" "$(DIST_DIR)"
	@if [ ! -f "$(BUILD_DIR)/linux-$(KERNEL_VER).tar.xz" ]; then \
		echo "==> Downloading Linux Kernel v$(KERNEL_VER)..."; \
		wget -c https://cdn.kernel.org/pub/linux/kernel/v6.x/linux-$(KERNEL_VER).tar.xz -O "$(BUILD_DIR)/linux-$(KERNEL_VER).tar.xz"; \
	fi
	@if [ ! -d "$(KERNEL_WORK_DIR)" ]; then \
		echo "==> Extracting kernel source to space-free directory $(KERNEL_WORK_DIR)..."; \
		mkdir -p "$(KERNEL_WORK_DIR)"; \
		tar -xf "$(BUILD_DIR)/linux-$(KERNEL_VER).tar.xz" -C "$(KERNEL_WORK_DIR)" --strip-components=1; \
	fi
	@echo "==> Generating KVM guest kernel config (defconfig + kvm_guest.config)..."
	@$(MAKE) -C "$(KERNEL_WORK_DIR)" defconfig
	@$(MAKE) -C "$(KERNEL_WORK_DIR)" kvm_guest.config
	@if [ -f kernel/microvm_defconfig ]; then \
		echo "==> Merging custom microVM kernel config..."; \
		cat kernel/microvm_defconfig >> "$(KERNEL_WORK_DIR)/.config"; \
		$(MAKE) -C "$(KERNEL_WORK_DIR)" olddefconfig; \
	fi
	@echo "==> Compiling Linux Kernel bzImage..."
	@$(MAKE) -C "$(KERNEL_WORK_DIR)" -j$$(nproc) bzImage
	@rm -f "$(DIST_DIR)/bzImage"
	@cp "$(KERNEL_WORK_DIR)/arch/x86/boot/bzImage" "$(DIST_DIR)/bzImage"
	@echo "==> Kernel bzImage ready at $(DIST_DIR)/bzImage"

kernel: "$(DIST_DIR)/bzImage" ## Download and compile minimal Linux kernel bzImage

# ------------------------------------------------------------------------------
# 3. PID 1 Static C Init (`karim-init`)
# ------------------------------------------------------------------------------
SAMPLE_APP_BIN := $(BUILD_DIR)/sample_app

"$(INIT_BIN)": init/init.c
	@mkdir -p "$(BUILD_DIR)"
	@echo "==> Compiling static C init (PID 1)..."
	@if which musl-gcc > /dev/null 2>&1; then \
		musl-gcc $(CFLAGS) -o "$@" init/init.c; \
	else \
		gcc $(CFLAGS) -o "$@" init/init.c; \
	fi
	@strip "$@"
	@echo "==> Static init binary size: $$(du -h "$@" | cut -f1)"

"$(SAMPLE_APP_BIN)": init/sample_app.c
	@mkdir -p "$(BUILD_DIR)"
	@echo "==> Compiling static sample application binary..."
	@if which musl-gcc > /dev/null 2>&1; then \
		musl-gcc $(CFLAGS) -o "$@" init/sample_app.c; \
	else \
		gcc $(CFLAGS) -o "$@" init/sample_app.c; \
	fi
	@strip "$@"

init: "$(INIT_BIN)" "$(SAMPLE_APP_BIN)" ## Build static C init and sample app binaries

# ------------------------------------------------------------------------------
# 4. Go Supervisor & Daemons (`karim-svcd`, `karim-cli`)
# ------------------------------------------------------------------------------
svcd: ## Build Go PID 1 successor supervisor
	@mkdir -p "$(BUILD_DIR)"
	@echo "==> Compiling karim-svcd (static Go binary)..."
	CGO_ENABLED=0 $(GO) build -ldflags="-s -w" -trimpath -o "$(BUILD_DIR)/karim-svcd" ./cmd/karim-svcd

secd: ## Build Go pre-exec isolation launcher tool (karim-secd)
	@mkdir -p "$(BUILD_DIR)"
	@echo "==> Compiling karim-secd (static Go binary)..."
	CGO_ENABLED=0 $(GO) build -ldflags="-s -w" -trimpath -o "$(BUILD_DIR)/karim-secd" ./cmd/karim-secd

vsockd: ## Build Go guest control plane daemon (karim-vsockd)
	@mkdir -p "$(BUILD_DIR)"
	@echo "==> Compiling karim-vsockd (static Go binary)..."
	CGO_ENABLED=0 $(GO) build -ldflags="-s -w" -trimpath -o "$(BUILD_DIR)/karim-vsockd" ./cmd/karim-vsockd

cli: ## Build host-side karim CLI tool
	@mkdir -p "$(DIST_DIR)"
	@if [ -d ./cmd/karim ]; then \
		echo "==> Compiling karim host CLI..."; \
		$(GO) build -ldflags="-s -w" -trimpath -o "$(DIST_DIR)/karim" ./cmd/karim; \
	else \
		echo "==> Skipping karim host CLI (Phase 4 component)..."; \
	fi

# ------------------------------------------------------------------------------
# 5. eBPF Probes & bpf2go Generation
# ------------------------------------------------------------------------------
ebpf: ## Generate Go bindings from C eBPF source via bpf2go
	@if [ -d ./ebpf ]; then \
		echo "==> Compiling eBPF probes..."; \
		$(GO) generate ./ebpf/...; \
	else \
		echo "==> Skipping eBPF probes (Phase 5 component)..."; \
	fi

# ------------------------------------------------------------------------------
# 6. Initramfs Assembly (CPIO)
# ------------------------------------------------------------------------------
initramfs: "$(INIT_BIN)" "$(SAMPLE_APP_BIN)" svcd secd vsockd ## Pack static init, sample app, supervisor, secd, and vsockd into initramfs.cpio
	@mkdir -p "$(BUILD_DIR)/initramfs_root/dev" "$(BUILD_DIR)/initramfs_root/proc" "$(BUILD_DIR)/initramfs_root/sys" "$(BUILD_DIR)/initramfs_root/etc" "$(BUILD_DIR)/initramfs_root/bin" "$(BUILD_DIR)/initramfs_root/sbin" "$(BUILD_DIR)/initramfs_root/etc/karim/services"
	@cp "$(INIT_BIN)" "$(BUILD_DIR)/initramfs_root/init"
	@chmod +x "$(BUILD_DIR)/initramfs_root/init"
	@cp "$(SAMPLE_APP_BIN)" "$(BUILD_DIR)/initramfs_root/bin/sample_app"
	@chmod +x "$(BUILD_DIR)/initramfs_root/bin/sample_app"
	@cp "$(BUILD_DIR)/karim-svcd" "$(BUILD_DIR)/initramfs_root/sbin/karim-svcd" 2>/dev/null || true
	@cp "$(BUILD_DIR)/karim-secd" "$(BUILD_DIR)/initramfs_root/sbin/karim-secd" 2>/dev/null || true
	@cp "$(BUILD_DIR)/karim-vsockd" "$(BUILD_DIR)/initramfs_root/sbin/karim-vsockd" 2>/dev/null || true
	@cp -r config/services/*.toml "$(BUILD_DIR)/initramfs_root/etc/karim/services/" 2>/dev/null || true
	@echo "==> Generating CPIO archive..."
	@mkdir -p "$(DIST_DIR)"
	@cd "$(BUILD_DIR)/initramfs_root" && find . -print0 | $(CPIO) --null --create --format=newc > "$(CURDIR)/$(INITRD_CPIO)"
	@echo "==> Initramfs generated at $(INITRD_CPIO) ($$(du -h "$(INITRD_CPIO)" | cut -f1))"

# ------------------------------------------------------------------------------
# 7. Readonly Rootfs Assembly (SquashFS)
# ------------------------------------------------------------------------------
rootfs: svcd secd vsockd "$(SAMPLE_APP_BIN)" ## Assemble rootfs filesystem tree and compress to SquashFS
	@mkdir -p "$(BUILD_DIR)/rootfs_tree/sbin" "$(BUILD_DIR)/rootfs_tree/bin"
	@mkdir -p "$(BUILD_DIR)/rootfs_tree/etc/karim/services"
	@mkdir -p "$(BUILD_DIR)/rootfs_tree/run/karim"
	@mkdir -p "$(BUILD_DIR)/rootfs_tree/var" "$(BUILD_DIR)/rootfs_tree/tmp" "$(BUILD_DIR)/rootfs_tree/proc" "$(BUILD_DIR)/rootfs_tree/sys" "$(BUILD_DIR)/rootfs_tree/dev"
	@cp "$(BUILD_DIR)/karim-svcd" "$(BUILD_DIR)/rootfs_tree/sbin/karim-svcd" 2>/dev/null || true
	@cp "$(BUILD_DIR)/karim-secd" "$(BUILD_DIR)/rootfs_tree/sbin/karim-secd" 2>/dev/null || true
	@cp "$(BUILD_DIR)/karim-vsockd" "$(BUILD_DIR)/rootfs_tree/sbin/karim-vsockd" 2>/dev/null || true
	@cp "$(SAMPLE_APP_BIN)" "$(BUILD_DIR)/rootfs_tree/bin/sample_app" 2>/dev/null || true
	@cp -r config/services/*.toml "$(BUILD_DIR)/rootfs_tree/etc/karim/services/" 2>/dev/null || true
	@echo "==> Packing SquashFS root filesystem..."
	@mkdir -p "$(DIST_DIR)"
	@rm -f "$(ROOTFS_IMG)"
	@$(MKSQUASHFS) "$(BUILD_DIR)/rootfs_tree" "$(ROOTFS_IMG)" -noappend -comp xz
	@echo "==> Rootfs ready at $(ROOTFS_IMG) ($$(du -h "$(ROOTFS_IMG)" | cut -f1))"

# ------------------------------------------------------------------------------
# 8. QEMU MicroVM Execution Targets
# ------------------------------------------------------------------------------
VSOCK_ARG := $(shell test -r /dev/vhost-vsock -a -w /dev/vhost-vsock 2>/dev/null && echo "-device vhost-vsock-pci,guest-cid=3")

QEMU_ARGS := -m 512M \
	-smp 2 \
	-enable-kvm \
	-cpu host \
	-kernel $(DIST_DIR)/bzImage \
	-initrd $(INITRD_CPIO) \
	-drive file=$(ROOTFS_IMG),if=virtio,format=raw,readonly=on \
	-netdev user,id=net0 \
	-device virtio-net-pci,netdev=net0 \
	$(VSOCK_ARG) \
	-nographic

check-vsock: ## Verify /dev/vhost-vsock permissions for host-guest control plane
	@if [ ! -r /dev/vhost-vsock ] || [ ! -w /dev/vhost-vsock ]; then \
		echo "======================================================================"; \
		echo "NOTICE: /dev/vhost-vsock is not accessible by current user ($(USER))."; \
		echo "        VSOCK control plane device will be disabled in QEMU."; \
		echo "        To enable host-guest vsock control, run:"; \
		echo "        sudo chmod 666 /dev/vhost-vsock"; \
		echo "        (or add user to kvm group: sudo usermod -aG kvm $(USER))"; \
		echo "======================================================================"; \
	fi

run: all check-vsock ## Boot Karim MicroVM under QEMU with KVM acceleration
	@if [ ! -f "$(DIST_DIR)/bzImage" ]; then \
		echo "ERROR: Kernel image $(DIST_DIR)/bzImage missing. Run 'make kernel' or copy host kernel 'sudo cp /boot/vmlinuz dist/bzImage && sudo chmod 644 dist/bzImage'."; \
		exit 1; \
	fi
	@echo "==> Booting Karim MicroVM under QEMU..."
	$(QEMU) $(QEMU_ARGS) -append "console=ttyS0,115200 earlyprintk=ttyS0,115200 quiet panic=0 init=/init"

run-debug: all check-vsock ## Boot Karim MicroVM in debug mode (kernel logs visible)
	@if [ ! -f "$(DIST_DIR)/bzImage" ]; then \
		echo "ERROR: Kernel image $(DIST_DIR)/bzImage missing. Run 'make kernel' or copy host kernel 'sudo cp /boot/vmlinuz dist/bzImage && sudo chmod 644 dist/bzImage'."; \
		exit 1; \
	fi
	@echo "==> Booting Karim MicroVM (Debug Mode)..."
	$(QEMU) $(QEMU_ARGS) -append "console=ttyS0,115200 earlyprintk=ttyS0,115200 debug panic=0 init=/init karim.debug=1"

# ------------------------------------------------------------------------------
# 9. Integration Testing
# ------------------------------------------------------------------------------
test: ## Run guest-less network/cgroup integration harness outside QEMU
	@echo "==> Running unit tests and Linux netns integration tests..."
	$(GO) test -v -race ./internal/...

test-phase0: ## Run Phase 0 automated test harness
	@bash testing/manual_test_phase0.sh

test-phase1: ## Run Phase 1 automated test harness
	@bash testing/manual_test_phase1.sh

test-phase2: ## Run Phase 2 automated test harness
	@bash testing/manual_test_phase2.sh

test-phase3: ## Run Phase 3 automated test harness
	@bash testing/manual_test_phase3.sh

test-phase4: ## Run Phase 4 automated test harness
	@bash testing/manual_test_phase4.sh



# ------------------------------------------------------------------------------
# 10. Clean Targets
# ------------------------------------------------------------------------------
clean: ## Clean build intermediate files
	@echo "==> Cleaning build artifacts..."
	@rm -rf $(BUILD_DIR)/init $(BUILD_DIR)/karim-svcd $(BUILD_DIR)/karim-secd $(BUILD_DIR)/karim-vsockd $(BUILD_DIR)/initramfs_root $(BUILD_DIR)/rootfs_tree

distclean: clean ## Full clean including kernel download and dist binaries
	@echo "==> Wiping all build and dist outputs..."
	@rm -rf $(BUILD_DIR) $(DIST_DIR)

help: ## Show Makefile targets
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-18s\033[0m %s\n", $$1, $$2}'
