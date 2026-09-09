package stored

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

// OverlayConfig defines the directory locations required for Linux OverlayFS.
type OverlayConfig struct {
	LowerDir  string
	UpperDir  string
	WorkDir   string
	TargetDir string
}

// BuildOverlayOptions formats the option string for mount("overlay", ..., "overlay", 0, opts).
func BuildOverlayOptions(lowerDir, upperDir, workDir string) string {
	return fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s", lowerDir, upperDir, workDir)
}

// PrepareOverlayDirectories creates upperdir and workdir on a writable filesystem (or tmpfs).
func PrepareOverlayDirectories(baseOverlayDir string) (upperDir, workDir string, err error) {
	upperDir = filepath.Join(baseOverlayDir, "upper")
	workDir = filepath.Join(baseOverlayDir, "work")

	if err := os.MkdirAll(upperDir, 0755); err != nil {
		return "", "", fmt.Errorf("failed to create overlay upperdir %s: %w", upperDir, err)
	}

	if err := os.MkdirAll(workDir, 0755); err != nil {
		return "", "", fmt.Errorf("failed to create overlay workdir %s: %w", workDir, err)
	}

	return upperDir, workDir, nil
}

// MountOverlay performs the OverlayFS mount syscall combining lower, upper, and work dirs.
func MountOverlay(cfg OverlayConfig) error {
	if cfg.LowerDir == "" || cfg.UpperDir == "" || cfg.WorkDir == "" || cfg.TargetDir == "" {
		return fmt.Errorf("invalid OverlayConfig: all directory paths must be specified (%+v)", cfg)
	}

	// Ensure target directory exists
	if err := os.MkdirAll(cfg.TargetDir, 0755); err != nil {
		return fmt.Errorf("failed to create target mount dir %s: %w", cfg.TargetDir, err)
	}

	opts := BuildOverlayOptions(cfg.LowerDir, cfg.UpperDir, cfg.WorkDir)
	fmt.Printf("[karim-stored] Mounting OverlayFS: lower=%s, upper=%s, work=%s -> target=%s\n",
		cfg.LowerDir, cfg.UpperDir, cfg.WorkDir, cfg.TargetDir)

	err := unix.Mount("overlay", cfg.TargetDir, "overlay", 0, opts)
	if err != nil {
		return fmt.Errorf("mount overlay failed on %s: %w", cfg.TargetDir, err)
	}

	return nil
}

// PrepareAndMountRootOverlay creates a tmpfs base directory, sets up upper/work dirs,
// and mounts OverlayFS over targetDir.
func PrepareAndMountRootOverlay(lowerDir, baseOverlayDir, targetDir string) (*OverlayConfig, error) {
	// 1. Ensure base overlay directory exists
	if err := os.MkdirAll(baseOverlayDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create base overlay dir %s: %w", baseOverlayDir, err)
	}

	// 2. Mount tmpfs on baseOverlayDir if not already mounted
	mounted, err := IsMountPoint(baseOverlayDir)
	if err != nil || !mounted {
		fmt.Printf("[karim-stored] Mounting tmpfs on overlay base %s...\n", baseOverlayDir)
		err := unix.Mount("tmpfs", baseOverlayDir, "tmpfs", 0, "size=64M,mode=0755")
		if err != nil && err != unix.EBUSY {
			fmt.Printf("[karim-stored] Warning: failed to mount tmpfs on %s: %v\n", baseOverlayDir, err)
		}
	}

	// 3. Prepare upperdir and workdir subdirectories
	upperDir, workDir, err := PrepareOverlayDirectories(baseOverlayDir)
	if err != nil {
		return nil, err
	}

	cfg := OverlayConfig{
		LowerDir:  lowerDir,
		UpperDir:  upperDir,
		WorkDir:   workDir,
		TargetDir: targetDir,
	}

	// 4. Mount OverlayFS
	if err := MountOverlay(cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// UnmountOverlay unmounts an OverlayFS filesystem target using lazy unmount (MNT_DETACH).
func UnmountOverlay(targetDir string) error {
	fmt.Printf("[karim-stored] Unmounting target %s...\n", targetDir)
	err := unix.Unmount(targetDir, unix.MNT_DETACH)
	if err != nil && err != unix.EINVAL {
		return fmt.Errorf("failed to unmount %s: %w", targetDir, err)
	}
	return nil
}

// IsMountPoint checks if a directory path is an active mount point by inspecting /proc/mounts.
func IsMountPoint(path string) (bool, error) {
	file, err := os.Open("/proc/mounts")
	if err != nil {
		// Fallback: compare stat st_dev with parent directory
		return isMountPointStat(path)
	}
	defer file.Close()

	cleanPath := filepath.Clean(path)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 2 {
			if filepath.Clean(fields[1]) == cleanPath {
				return true, nil
			}
		}
	}

	return false, scanner.Err()
}

func isMountPointStat(path string) (bool, error) {
	var st, parentSt unix.Stat_t
	if err := unix.Stat(path, &st); err != nil {
		return false, err
	}
	parent := filepath.Dir(path)
	if err := unix.Stat(parent, &parentSt); err != nil {
		return false, err
	}
	return st.Dev != parentSt.Dev, nil
}
