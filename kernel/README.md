# Custom MicroVM Kernel Configurations

This directory can hold custom Linux kernel build configurations (e.g. `microvm_defconfig`).

If `kernel/microvm_defconfig` is absent, `make kernel` automatically generates a lightweight Linux KVM guest kernel config using standard `defconfig` + `kvm_guest.config`.
