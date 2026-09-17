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

// MountSquashFSDevice mounts a SquashFS block device (e.g. /dev/vda or /dev/sda) onto lowerDir.
func MountSquashFSDevice(lowerDir string) error {
	if err := os.MkdirAll(lowerDir, 0755); err != nil {
		return fmt.Errorf("failed creating lowerdir %s: %w", lowerDir, err)
	}

	if mounted, _ := IsMountPoint(lowerDir); mounted {
		return nil
	}

	devs := []string{"/dev/vda", "/dev/sda", "/dev/initrd"}
	var lastErr error
	for _, dev := range devs {
		if _, err := os.Stat(dev); err == nil {
			fmt.Printf("[karim-stored] Mounting SquashFS block device %s on %s...\n", dev, lowerDir)
			err := unix.Mount(dev, lowerDir, "squashfs", unix.MS_RDONLY, "")
			if err == nil {
				fmt.Printf("[karim-stored] Successfully mounted SquashFS %s on %s\n", dev, lowerDir)
				return nil
			}
			lastErr = err
			fmt.Printf("[karim-stored] Warning: Failed mounting SquashFS %s: %v\n", dev, err)
		}
	}

	if lastErr != nil {
		return fmt.Errorf("could not mount any SquashFS device on %s: %w", lowerDir, lastErr)
	}
	return fmt.Errorf("no suitable SquashFS block device found for %s", lowerDir)
}

// LinkLowerDirToRoot recursively links missing files and directories from lowerDir onto rootfs /.
func LinkLowerDirToRoot(lowerDir string) error {
	entries, err := os.ReadDir(lowerDir)
	if err != nil {
		return fmt.Errorf("failed reading lowerdir %s: %w", lowerDir, err)
	}

	for _, entry := range entries {
		srcPath := filepath.Join(lowerDir, entry.Name())
		targetPath := "/" + entry.Name()

		// Do not link internal mount or system pseudo-dirs over existing ones
		if entry.Name() == "proc" || entry.Name() == "sys" || entry.Name() == "dev" || entry.Name() == "mnt" || entry.Name() == "run" {
			continue
		}

		targetInfo, err := os.Lstat(targetPath)
		if os.IsNotExist(err) {
			_ = os.Symlink(srcPath, targetPath)
			fmt.Printf("[karim-stored] Linked root entry %s -> %s\n", targetPath, srcPath)
			continue
		}

		// If target exists and is a directory (e.g. /bin or /etc), link missing child items
		if err == nil && targetInfo.IsDir() && entry.IsDir() {
			subEntries, err := os.ReadDir(srcPath)
			if err != nil {
				continue
			}
			for _, sub := range subEntries {
				subSrc := filepath.Join(srcPath, sub.Name())
				subTarget := filepath.Join(targetPath, sub.Name())
				if _, err := os.Lstat(subTarget); os.IsNotExist(err) {
					_ = os.Symlink(subSrc, subTarget)
					fmt.Printf("[karim-stored] Linked sub-entry %s -> %s\n", subTarget, subSrc)
				}
			}
		}
	}
	return nil
}

// PrepareAndMountRootOverlay creates a tmpfs base directory, sets up upper/work dirs,
// mounts SquashFS device onto lowerDir, links lowerDir binaries to rootfs, and mounts OverlayFS over targetDir.
func PrepareAndMountRootOverlay(lowerDir, baseOverlayDir, targetDir string) (*OverlayConfig, error) {
	// 1. Mount SquashFS block device onto lowerDir if present
	if err := MountSquashFSDevice(lowerDir); err != nil {
		fmt.Printf("[karim-stored] Notice: %v\n", err)
	}

	// 2. Expose lowerDir files onto rootfs via linking fallback (guarantees binaries like /bin/sh work across all kernels)
	if err := LinkLowerDirToRoot(lowerDir); err != nil {
		fmt.Printf("[karim-stored] Rootfs link notice: %v\n", err)
	}

	// 3. Ensure base overlay directory exists
	if err := os.MkdirAll(baseOverlayDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create base overlay dir %s: %w", baseOverlayDir, err)
	}

	// 4. Mount tmpfs on baseOverlayDir if not already mounted
	mounted, err := IsMountPoint(baseOverlayDir)
	if err != nil || !mounted {
		fmt.Printf("[karim-stored] Mounting tmpfs on overlay base %s...\n", baseOverlayDir)
		err := unix.Mount("tmpfs", baseOverlayDir, "tmpfs", 0, "size=64M,mode=0755")
		if err != nil && err != unix.EBUSY {
			fmt.Printf("[karim-stored] Warning: failed to mount tmpfs on %s: %v\n", baseOverlayDir, err)
		}
	}

	// 5. Prepare upperdir and workdir subdirectories
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

	// 6. Attempt OverlayFS mount if target is custom mount point (non-root)
	if targetDir != "/" {
		if _, err := os.Stat(lowerDir); err == nil {
			if err := MountOverlay(cfg); err != nil {
				fmt.Printf("[karim-stored] Warning: MountOverlay target %s failed: %v\n", targetDir, err)
			}
		}
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
