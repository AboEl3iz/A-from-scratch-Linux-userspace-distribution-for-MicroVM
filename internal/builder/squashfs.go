package builder

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// SquashFSOptions configures deterministic mksquashfs invocation.
type SquashFSOptions struct {
	Compression     string // e.g. "xz" or "gzip"
	SourceDir       string
	OutputFile      string
	FixedTime       int64 // e.g. 0 for epoch
	EnableFragments bool  // false for byte-exact reproducible layout
}

// DefaultSquashFSOptions returns hermetic production defaults.
func DefaultSquashFSOptions(sourceDir, outputFile string) SquashFSOptions {
	return SquashFSOptions{
		Compression:     "xz",
		SourceDir:       sourceDir,
		OutputFile:      outputFile,
		FixedTime:       0,
		EnableFragments: false,
	}
}

// BuildHermeticSquashFS wraps system mksquashfs with strict reproducible flags.
func BuildHermeticSquashFS(opts SquashFSOptions) error {
	mksquashfsPath, err := exec.LookPath("mksquashfs")
	if err != nil {
		return fmt.Errorf("mksquashfs utility not found in PATH: %w", err)
	}

	// Remove existing target file if present
	_ = os.Remove(opts.OutputFile)
	if err := os.MkdirAll(filepath.Dir(opts.OutputFile), 0755); err != nil {
		return fmt.Errorf("failed creating output directory: %w", err)
	}

	args := []string{
		opts.SourceDir,
		opts.OutputFile,
		"-noappend",
		"-comp", opts.Compression,
		"-all-root",    // Force UID/GID 0 for all entries
		"-no-recovery", // Disable recovery log file creation
		"-no-exports",  // Disable NFS export lookup table
	}

	if !opts.EnableFragments {
		args = append(args, "-no-fragments")
	}

	cmd := exec.Command(mksquashfsPath, args...)

	// Inject SOURCE_DATE_EPOCH environment variable for sub-tooling reproducibility
	env := os.Environ()
	env = append(env, fmt.Sprintf("SOURCE_DATE_EPOCH=%d", opts.FixedTime))
	cmd.Env = env

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("mksquashfs failed (%w): %s", err, string(out))
	}

	return nil
}
