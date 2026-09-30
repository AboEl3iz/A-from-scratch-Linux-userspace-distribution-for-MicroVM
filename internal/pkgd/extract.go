package pkgd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

// SafePathClean lexically scrubs and verifies that relPath remains strictly bounded inside destDir.
func SafePathClean(destDir, relPath string) (string, error) {
	if relPath == "" {
		return "", fmt.Errorf("empty entry path")
	}

	absDest, err := filepath.Abs(destDir)
	if err != nil {
		return "", fmt.Errorf("invalid destination directory %q: %w", destDir, err)
	}
	cleanDest := filepath.Clean(absDest)

	// Lexical scrubbing: reject relative traversal sequences and absolute paths
	cleanRel := filepath.Clean(relPath)
	if strings.HasPrefix(cleanRel, "/") || strings.HasPrefix(cleanRel, "../") || cleanRel == ".." {
		return "", fmt.Errorf("tar-slip violation: path traversal detected in %q", relPath)
	}

	targetPath := filepath.Join(cleanDest, cleanRel)
	if !strings.HasPrefix(targetPath, cleanDest+string(os.PathSeparator)) && targetPath != cleanDest {
		return "", fmt.Errorf("tar-slip violation: path %q escapes root %q", relPath, cleanDest)
	}

	// Verify existing parent components to prevent symlink breakout attacks
	curr := cleanDest
	parts := strings.Split(cleanRel, string(os.PathSeparator))
	for i := 0; i < len(parts)-1; i++ {
		if parts[i] == "" || parts[i] == "." {
			continue
		}
		curr = filepath.Join(curr, parts[i])
		fi, err := os.Lstat(curr)
		if err != nil {
			if os.IsNotExist(err) {
				break
			}
			return "", err
		}
		if fi.Mode()&os.ModeSymlink != 0 {
			resolved, err := filepath.EvalSymlinks(curr)
			if err != nil {
				return "", fmt.Errorf("failed resolving symlink %s: %w", curr, err)
			}
			if !strings.HasPrefix(resolved, cleanDest+string(os.PathSeparator)) && resolved != cleanDest {
				return "", fmt.Errorf("tar-slip violation: parent symlink %s points outside root to %s", curr, resolved)
			}
		}
	}

	return targetPath, nil
}

// ValidateSymlinkTarget verifies that a symlink target value does not resolve outside destDir.
func ValidateSymlinkTarget(destDir, targetPath, linkVal string) error {
	absDest, err := filepath.Abs(destDir)
	if err != nil {
		return err
	}
	cleanDest := filepath.Clean(absDest)

	var resolvedTarget string
	if strings.HasPrefix(linkVal, "/") {
		resolvedTarget = filepath.Join(cleanDest, linkVal)
	} else {
		resolvedTarget = filepath.Join(filepath.Dir(targetPath), linkVal)
	}
	cleanTarget := filepath.Clean(resolvedTarget)

	if !strings.HasPrefix(cleanTarget, cleanDest+string(os.PathSeparator)) && cleanTarget != cleanDest {
		return fmt.Errorf("tar-slip violation: symlink target %q escapes root %q", linkVal, cleanDest)
	}
	return nil
}

// SafeOpenFile attempts openat2 with RESOLVE_BENEATH to enforce VFS kernel boundary checks,
// falling back to os.OpenFile if openat2 is unavailable.
func SafeOpenFile(destDir, targetPath string, flags int, mode os.FileMode) (*os.File, error) {
	absDest, err := filepath.Abs(destDir)
	if err != nil {
		return nil, err
	}
	cleanDest := filepath.Clean(absDest)

	dirFd, err := unix.Open(cleanDest, unix.O_PATH|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err == nil {
		defer unix.Close(dirFd)
		rel, err := filepath.Rel(cleanDest, targetPath)
		if err == nil && !strings.HasPrefix(rel, "..") {
			how := &unix.OpenHow{
				Flags:   uint64(flags | unix.O_CLOEXEC),
				Mode:    uint64(mode),
				Resolve: unix.RESOLVE_BENEATH,
			}
			fd, err := unix.Openat2(dirFd, rel, how)
			if err == nil {
				return os.NewFile(uintptr(fd), targetPath), nil
			}
		}
	}

	// Fallback to standard os.OpenFile after SafePathClean verification
	return os.OpenFile(targetPath, flags, mode)
}
