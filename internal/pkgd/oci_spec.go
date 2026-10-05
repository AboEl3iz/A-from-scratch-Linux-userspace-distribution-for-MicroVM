package pkgd

// OCI Runtime Spec structures matching opencontainers/runtime-spec (config.json)

type Spec struct {
	Version *string `json:"ociVersion,omitempty"`
	Root    *Root   `json:"root,omitempty"`
	Process *Process `json:"process,omitempty"`
	Hostname string  `json:"hostname,omitempty"`
	Mounts  []Mount `json:"mounts,omitempty"`
	Linux   *Linux  `json:"linux,omitempty"`
}

type Root struct {
	Path     string `json:"path"`
	Readonly bool   `json:"readonly,omitempty"`
}

type Process struct {
	Terminal        bool               `json:"terminal,omitempty"`
	User            User               `json:"user"`
	Args            []string           `json:"args"`
	Env             []string           `json:"env,omitempty"`
	Cwd             string             `json:"cwd"`
	Capabilities    *LinuxCapabilities `json:"capabilities,omitempty"`
	Rlimits         []POSIXRlimit      `json:"rlimits,omitempty"`
	NoNewPrivileges bool               `json:"noNewPrivileges,omitempty"`
}

type User struct {
	UID uint32 `json:"uid"`
	GID uint32 `json:"gid"`
}

type POSIXRlimit struct {
	Type string `json:"type"`
	Hard uint64 `json:"hard"`
	Soft uint64 `json:"soft"`
}

type LinuxCapabilities struct {
	Bounding    []string `json:"bounding,omitempty"`
	Effective   []string `json:"effective,omitempty"`
	Inheritable []string `json:"inheritable,omitempty"`
	Permitted   []string `json:"permitted,omitempty"`
	Ambient     []string `json:"ambient,omitempty"`
}

type Mount struct {
	Destination string   `json:"destination"`
	Type        string   `json:"type,omitempty"`
	Source      string   `json:"source,omitempty"`
	Options     []string `json:"options,omitempty"`
}

type Linux struct {
	Namespaces []LinuxNamespace `json:"namespaces,omitempty"`
	Seccomp    *LinuxSeccomp    `json:"seccomp,omitempty"`
	Resources  *LinuxResources  `json:"resources,omitempty"`
}

type LinuxNamespace struct {
	Type string `json:"type"`
	Path string `json:"path,omitempty"`
}

type LinuxSeccomp struct {
	DefaultAction string `json:"defaultAction"`
	ProfileName   string `json:"profileName,omitempty"`
}

type LinuxResources struct {
	Memory *LinuxMemory `json:"memory,omitempty"`
	CPU    *LinuxCPU    `json:"cpu,omitempty"`
}

type LinuxMemory struct {
	Limit *int64 `json:"limit,omitempty"`
}

type LinuxCPU struct {
	Quota *int64 `json:"quota,omitempty"`
}

// OCIContainerBundle represents an incoming dynamic bundle request payload over VSOCK/RPC.
type OCIContainerBundle struct {
	BundleID   string   `json:"bundle_id"`
	Spec       *Spec    `json:"spec"`
	LowerDirs  []string `json:"lower_dirs,omitempty"`
	RootfsTar  []byte   `json:"rootfs_tar,omitempty"`
	Restart    string   `json:"restart,omitempty"`
}
