package builder

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	cpioMagic    = "070701"
	cpioTrailer  = "TRAILER!!!"
	cpioHeaderSz = 110
)

// CPIOHeader represents a SVR4 newc CPIO entry header.
type CPIOHeader struct {
	Ino      uint32
	Mode     uint32
	UID      uint32
	GID      uint32
	NLink    uint32
	MTime    uint32
	FileSize uint32
	DevMaj   uint32
	DevMin   uint32
	RDevMaj  uint32
	RDevMin  uint32
	Name     string
}

// WriteCPIOHeader writes a 110-byte hex-encoded newc header to w.
func WriteCPIOHeader(w io.Writer, h *CPIOHeader) error {
	nameBytes := append([]byte(h.Name), 0) // NUL terminated
	nameSize := uint32(len(nameBytes))

	hdrStr := fmt.Sprintf("%s%08X%08X%08X%08X%08X%08X%08X%08X%08X%08X%08X%08X00000000",
		cpioMagic,
		h.Ino,
		h.Mode,
		h.UID,
		h.GID,
		h.NLink,
		h.MTime,
		h.FileSize,
		h.DevMaj,
		h.DevMin,
		h.RDevMaj,
		h.RDevMin,
		nameSize,
	)

	if len(hdrStr) != cpioHeaderSz {
		return fmt.Errorf("invalid header length %d (expected %d)", len(hdrStr), cpioHeaderSz)
	}

	if _, err := w.Write([]byte(hdrStr)); err != nil {
		return err
	}

	if _, err := w.Write(nameBytes); err != nil {
		return err
	}

	// Pad header + name to 4-byte boundary
	totalHdrLen := cpioHeaderSz + len(nameBytes)
	if pad := (4 - (totalHdrLen % 4)) % 4; pad > 0 {
		if _, err := w.Write(bytes.Repeat([]byte{0}, pad)); err != nil {
			return err
		}
	}

	return nil
}

// WriteCPIOEntry writes header and file body, adding necessary padding.
func WriteCPIOEntry(w io.Writer, h *CPIOHeader, r io.Reader) error {
	if err := WriteCPIOHeader(w, h); err != nil {
		return err
	}

	if h.FileSize > 0 && r != nil {
		n, err := io.Copy(w, r)
		if err != nil {
			return err
		}
		if uint32(n) != h.FileSize {
			return fmt.Errorf("file size mismatch for %s: wrote %d, expected %d", h.Name, n, h.FileSize)
		}

		// Pad data to 4-byte boundary
		if pad := (4 - (h.FileSize % 4)) % 4; pad > 0 {
			if _, err := w.Write(bytes.Repeat([]byte{0}, int(pad))); err != nil {
				return err
			}
		}
	}

	return nil
}

// WriteCPIOTrailer writes the special TRAILER!!! entry ending a CPIO newc archive.
func WriteCPIOTrailer(w io.Writer) error {
	h := &CPIOHeader{
		Name:     cpioTrailer,
		NLink:    1,
		Mode:     0,
		MTime:    0,
		FileSize: 0,
	}
	return WriteCPIOEntry(w, h, nil)
}

// PackCPIOInitramfs walks standard directory tree rootDir and packs a deterministic, byte-exact CPIO archive.
func PackCPIOInitramfs(rootDir string, destWriter io.Writer, fixedMTime time.Time) error {
	type entry struct {
		relPath  string
		fullPath string
		info     os.FileInfo
	}

	var entries []entry

	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(rootDir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil // root entry omitted or handled explicitly
		}

		// Convert backslashes to forward slashes for Linux compatibility
		rel = filepath.ToSlash(rel)
		entries = append(entries, entry{
			relPath:  rel,
			fullPath: path,
			info:     info,
		})
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed walking %s: %w", rootDir, err)
	}

	// 1. Sort entries lexicographically for 100% deterministic order
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].relPath < entries[j].relPath
	})

	mtimeSec := uint32(fixedMTime.Unix())
	if fixedMTime.IsZero() {
		mtimeSec = 0
	}

	var inoCounter uint32 = 1

	// 2. Write each file entry deterministically
	for _, e := range entries {
		var mode uint32
		sysMode := uint32(e.info.Mode())

		if e.info.IsDir() {
			mode = 0040755 // S_IFDIR | 0755
		} else if e.info.Mode()&os.ModeSymlink != 0 {
			mode = 0120777 // S_IFLNK | 0777
		} else {
			// Regular file: normalize permissions to 0755 if executable, 0644 otherwise
			if sysMode&0111 != 0 {
				mode = 0100755 // S_IFREG | 0755
			} else {
				mode = 0100644 // S_IFREG | 0644
			}
		}

		hdr := &CPIOHeader{
			Ino:      inoCounter,
			Mode:     mode,
			UID:      0, // Hermetic root
			GID:      0, // Hermetic root
			NLink:    1,
			MTime:    mtimeSec,
			FileSize: uint32(e.info.Size()),
			Name:     e.relPath,
		}

		inoCounter++

		if e.info.IsDir() {
			hdr.NLink = 2
			hdr.FileSize = 0
			if err := WriteCPIOEntry(destWriter, hdr, nil); err != nil {
				return fmt.Errorf("failed writing dir %s to cpio: %w", e.relPath, err)
			}
		} else if e.info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(e.fullPath)
			if err != nil {
				return fmt.Errorf("failed reading symlink %s: %w", e.fullPath, err)
			}
			hdr.FileSize = uint32(len(target))
			if err := WriteCPIOEntry(destWriter, hdr, strings.NewReader(target)); err != nil {
				return fmt.Errorf("failed writing symlink %s to cpio: %w", e.relPath, err)
			}
		} else {
			f, err := os.Open(e.fullPath)
			if err != nil {
				return fmt.Errorf("failed opening %s: %w", e.fullPath, err)
			}
			err = WriteCPIOEntry(destWriter, hdr, f)
			f.Close()
			if err != nil {
				return fmt.Errorf("failed writing file %s to cpio: %w", e.relPath, err)
			}
		}
	}

	// 3. Write trailer
	if err := WriteCPIOTrailer(destWriter); err != nil {
		return fmt.Errorf("failed writing cpio trailer: %w", err)
	}

	return nil
}
