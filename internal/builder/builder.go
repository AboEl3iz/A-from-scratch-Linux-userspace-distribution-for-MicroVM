package builder

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"karim-microvm-os/internal/pkgd"
)

// BuildConfig defines the paths and options for building a hermetic Karim MicroVM OS image.
type BuildConfig struct {
	InitBinPath     string
	SvcdBinPath     string
	ServicesDir     string
	LayersFile      string
	KernelPath      string
	OutputDir       string
	SourceDateEpoch int64
}

// DefaultBuildConfig initializes build parameters based on standard repository layout.
func DefaultBuildConfig(workDir string) BuildConfig {
	return BuildConfig{
		InitBinPath:     filepath.Join(workDir, "build", "init"),
		SvcdBinPath:     filepath.Join(workDir, "build", "karim-svcd"),
		ServicesDir:     filepath.Join(workDir, "config", "services"),
		LayersFile:      filepath.Join(workDir, "config", "layers.toml"),
		KernelPath:      filepath.Join(workDir, "dist", "bzImage"),
		OutputDir:       filepath.Join(workDir, "dist"),
		SourceDateEpoch: 0,
	}
}

// BuildImagePipeline executes the complete hermetic image assembly process.
func BuildImagePipeline(cfg BuildConfig) (*BuildManifest, error) {
	if err := os.MkdirAll(cfg.OutputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed creating output dir: %w", err)
	}

	// 1. Create temporary staging directories
	tmpBuild, err := os.MkdirTemp("", "karim-build-*")
	if err != nil {
		return nil, fmt.Errorf("failed creating temp build dir: %w", err)
	}
	defer os.RemoveAll(tmpBuild)

	initramfsStaging := filepath.Join(tmpBuild, "initramfs_root")
	rootfsStaging := filepath.Join(tmpBuild, "rootfs_tree")

	// 2. Stage Initramfs Directory Structure
	dirsToCreateInit := []string{
		"dev",
		"proc",
		"sys",
		"etc",
		"bin",
		"sbin",
		"etc/karim/services",
	}
	for _, d := range dirsToCreateInit {
		if err := os.MkdirAll(filepath.Join(initramfsStaging, d), 0755); err != nil {
			return nil, fmt.Errorf("failed staging initramfs dir %s: %w", d, err)
		}
	}

	// Copy static init binary to initramfs root
	if err := copyFileExecutable(cfg.InitBinPath, filepath.Join(initramfsStaging, "init")); err != nil {
		return nil, fmt.Errorf("failed staging init binary: %w", err)
	}

	// Copy guest daemons into initramfs sbin
	buildDir := filepath.Dir(cfg.InitBinPath)
	daemons := []string{"karim-svcd", "karim-secd", "karim-vsockd", "karim-obsd"}
	for _, d := range daemons {
		srcD := filepath.Join(buildDir, d)
		if d == "karim-svcd" && cfg.SvcdBinPath != "" {
			srcD = cfg.SvcdBinPath
		}
		if _, err := os.Stat(srcD); err == nil {
			_ = copyFileExecutable(srcD, filepath.Join(initramfsStaging, "sbin", d))
		}
	}

	// Copy service definitions into initramfs etc/karim/services
	if _, err := os.Stat(cfg.ServicesDir); err == nil {
		_ = filepath.Walk(cfg.ServicesDir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return err
			}
			rel, _ := filepath.Rel(cfg.ServicesDir, path)
			dest := filepath.Join(initramfsStaging, "etc/karim/services", rel)
			return copyFileRegular(path, dest)
		})
	}

	// 3. Stage RootFS Directory Structure
	dirsToCreateRoot := []string{
		"sbin",
		"etc/karim/services",
		"run/karim",
		"var",
		"tmp",
		"proc",
		"sys",
		"dev",
	}
	for _, d := range dirsToCreateRoot {
		if err := os.MkdirAll(filepath.Join(rootfsStaging, d), 0755); err != nil {
			return nil, fmt.Errorf("failed staging rootfs dir %s: %w", d, err)
		}
	}

	// Copy karim-svcd binary
	if err := copyFileExecutable(cfg.SvcdBinPath, filepath.Join(rootfsStaging, "sbin", "karim-svcd")); err != nil {
		return nil, fmt.Errorf("failed staging karim-svcd binary: %w", err)
	}

	// Copy workload C binaries (sample_app, httpd, kv_store)
	buildDir = filepath.Dir(cfg.InitBinPath)
	for _, binName := range []string{"sample_app", "httpd", "kv_store"} {
		srcBin := filepath.Join(buildDir, binName)
		if _, err := os.Stat(srcBin); err == nil {
			_ = copyFileExecutable(srcBin, filepath.Join(initramfsStaging, "bin", binName))
			_ = copyFileExecutable(srcBin, filepath.Join(rootfsStaging, "bin", binName))
		}
	}

	// Copy service definitions if present
	if _, err := os.Stat(cfg.ServicesDir); err == nil {
		err = filepath.Walk(cfg.ServicesDir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return err
			}
			rel, _ := filepath.Rel(cfg.ServicesDir, path)
			dest := filepath.Join(rootfsStaging, "etc/karim/services", rel)
			return copyFileRegular(path, dest)
		})
		if err != nil {
			return nil, fmt.Errorf("failed copying service configs: %w", err)
		}
	}

	// Unpack & merge declared package layers into rootfs staging tree
	if cfg.LayersFile != "" {
		if _, err := os.Stat(cfg.LayersFile); err == nil {
			layerCfg, err := pkgd.LoadLayerConfig(cfg.LayersFile)
			if err != nil {
				return nil, fmt.Errorf("failed loading layers config: %w", err)
			}
			baseDir := filepath.Dir(cfg.LayersFile)
			if err := pkgd.MergeLayerConfig(layerCfg, baseDir, rootfsStaging); err != nil {
				return nil, fmt.Errorf("failed merging package layers into rootfs: %w", err)
			}
		}
	}

	fixedTime := time.Unix(cfg.SourceDateEpoch, 0).UTC()

	// 4. Pack Deterministic CPIO Initramfs
	cpioPath := filepath.Join(cfg.OutputDir, "initramfs.cpio")
	cpioFile, err := os.Create(cpioPath)
	if err != nil {
		return nil, fmt.Errorf("failed creating cpio output file: %w", err)
	}
	err = PackCPIOInitramfs(initramfsStaging, cpioFile, fixedTime)
	cpioFile.Close()
	if err != nil {
		return nil, fmt.Errorf("failed packing CPIO initramfs: %w", err)
	}

	// 5. Pack Hermetic SquashFS RootFS
	squashPath := filepath.Join(cfg.OutputDir, "rootfs.sqsh")
	sqOpts := SquashFSOptions{
		Compression:     "xz",
		SourceDir:       rootfsStaging,
		OutputFile:      squashPath,
		FixedTime:       cfg.SourceDateEpoch,
		EnableFragments: false,
	}
	if err := BuildHermeticSquashFS(sqOpts); err != nil {
		return nil, fmt.Errorf("failed packing SquashFS rootfs: %w", err)
	}

	// 6. Generate Build Manifest
	manifest := &BuildManifest{
		ProjectName:     "karim-microvm-os",
		Version:         "1.0.0-hermetic",
		BuildTimestamp:  FormatFixedTimestamp(cfg.SourceDateEpoch),
		Hermetic:        true,
		SourceDateEpoch: cfg.SourceDateEpoch,
		InputArtifacts:  make(map[string]ArtifactChecksum),
		OutputArtifacts: make(map[string]ArtifactChecksum),
	}

	// Calculate Input Checksums
	inputs := map[string]string{
		"init_binary": cfg.InitBinPath,
		"svcd_binary": cfg.SvcdBinPath,
	}
	if _, err := os.Stat(cfg.KernelPath); err == nil {
		inputs["kernel_bzimage"] = cfg.KernelPath
	}

	for key, p := range inputs {
		chk, err := NewArtifactChecksum(key, p)
		if err == nil {
			manifest.InputArtifacts[key] = chk
		}
	}

	// Calculate Output Checksums
	cpioChk, err := NewArtifactChecksum("initramfs.cpio", cpioPath)
	if err != nil {
		return nil, fmt.Errorf("failed computing initramfs checksum: %w", err)
	}
	manifest.OutputArtifacts["initramfs.cpio"] = cpioChk

	sqshChk, err := NewArtifactChecksum("rootfs.sqsh", squashPath)
	if err != nil {
		return nil, fmt.Errorf("failed computing rootfs checksum: %w", err)
	}
	manifest.OutputArtifacts["rootfs.sqsh"] = sqshChk

	manifestPath := filepath.Join(cfg.OutputDir, "manifest.json")
	if err := SaveManifest(manifest, manifestPath); err != nil {
		return nil, fmt.Errorf("failed saving build manifest: %w", err)
	}

	return manifest, nil
}

// VerifyReproducibility performs two complete clean builds and asserts byte-for-byte SHA-256 equality.
func VerifyReproducibility(cfg BuildConfig) error {
	// Build 1
	m1, err := BuildImagePipeline(cfg)
	if err != nil {
		return fmt.Errorf("build 1 failed: %w", err)
	}

	// Wipe output build artifacts temporarily for fresh re-run
	cpioPath := filepath.Join(cfg.OutputDir, "initramfs.cpio")
	sqshPath := filepath.Join(cfg.OutputDir, "rootfs.sqsh")
	_ = os.Remove(cpioPath)
	_ = os.Remove(sqshPath)

	// Build 2
	m2, err := BuildImagePipeline(cfg)
	if err != nil {
		return fmt.Errorf("build 2 failed: %w", err)
	}

	// Compare manifests
	if err := CompareManifests(m1, m2); err != nil {
		return fmt.Errorf("reproducibility check failed: %w", err)
	}

	return nil
}

func copyFileExecutable(src, dst string) error {
	return copyFileWithPerm(src, dst, 0755)
}

func copyFileRegular(src, dst string) error {
	return copyFileWithPerm(src, dst, 0644)
}

func copyFileWithPerm(src, dst string, perm os.FileMode) error {
	sf, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sf.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	df, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	defer df.Close()

	if _, err := io.Copy(df, sf); err != nil {
		return err
	}

	return nil
}
