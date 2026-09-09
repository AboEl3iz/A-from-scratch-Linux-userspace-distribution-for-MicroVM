package svcd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
)

const CGroupBaseDir = "/sys/fs/cgroup/karim"

// CGroupManager encapsulates cgroup v2 operations for service sandboxing.
type CGroupManager struct {
	basePath string
}

// NewCGroupManager initializes base cgroup hierarchy at /sys/fs/cgroup/karim.
func NewCGroupManager() (*CGroupManager, error) {
	cm := &CGroupManager{basePath: CGroupBaseDir}

	// Create root karim cgroup leaf if it doesn't exist
	if err := os.MkdirAll(cm.basePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create base cgroup directory %s: %w", cm.basePath, err)
	}

	// Enable memory and cpu controllers in parent /sys/fs/cgroup and base /sys/fs/cgroup/karim
	_ = enableSubtreeControllers("/sys/fs/cgroup", "+memory +cpu")
	_ = enableSubtreeControllers(cm.basePath, "+memory +cpu")

	return cm, nil
}

// CreateServiceCGroup creates a leaf cgroup for a service and applies memory/CPU resource constraints.
func (cm *CGroupManager) CreateServiceCGroup(spec *ServiceSpec) (string, error) {
	serviceCGroupPath := filepath.Join(cm.basePath, spec.Name)
	if err := os.MkdirAll(serviceCGroupPath, 0755); err != nil {
		return "", fmt.Errorf("failed to create service cgroup %s: %w", serviceCGroupPath, err)
	}

	// Enforce memory limit if specified
	if spec.MemoryLimit > 0 {
		memMaxPath := filepath.Join(serviceCGroupPath, "memory.max")
		memStr := strconv.FormatInt(spec.MemoryLimit, 10)
		if err := os.WriteFile(memMaxPath, []byte(memStr), 0644); err != nil {
			fmt.Printf("[karim-svcd] Notice: failed to write memory.max for %s: %v\n", spec.Name, err)
		}
	}

	// Enforce CPU quota if specified (quota_us period_us)
	if spec.CPUQuota > 0 {
		cpuMaxPath := filepath.Join(serviceCGroupPath, "cpu.max")
		period := 100000 // 100ms default period
		quota := int(spec.CPUQuota * float64(period) / 100.0)
		cpuStr := fmt.Sprintf("%d %d", quota, period)
		if err := os.WriteFile(cpuMaxPath, []byte(cpuStr), 0644); err != nil {
			fmt.Printf("[karim-svcd] Notice: failed to write cpu.max for %s: %v\n", spec.Name, err)
		}
	}

	return serviceCGroupPath, nil
}

// AttachProcess adds a PID to the service's cgroup.procs.
func (cm *CGroupManager) AttachProcess(serviceName string, pid int) error {
	procsPath := filepath.Join(cm.basePath, serviceName, "cgroup.procs")
	pidStr := strconv.Itoa(pid)
	if err := os.WriteFile(procsPath, []byte(pidStr), 0644); err != nil {
		return fmt.Errorf("failed to attach PID %d to %s: %w", pid, procsPath, err)
	}
	return nil
}

// SetupChildProcAttr returns SysProcAttr for pre-exec process isolation (e.g. death signal).
func SetupChildProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		Pdeathsig: syscall.SIGKILL,
	}
}

func enableSubtreeControllers(parentDir, controllers string) error {
	subtreePath := filepath.Join(parentDir, "cgroup.subtree_control")
	return os.WriteFile(subtreePath, []byte(controllers), 0644)
}
