package kvmrun

// ExtKernel represents the guest kernel configuration structure.
type ExtKernel struct {
	Image   string `json:"image,omitempty"`
	Cmdline string `json:"cmdline,omitempty"`
	Initrd  string `json:"initrd,omitempty"`
	Modiso  string `json:"modiso,omitempty"`
}
