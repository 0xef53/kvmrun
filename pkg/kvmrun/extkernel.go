package kvmrun

// ExtKernel represents the guest kernel configuration structure.
type ExtKernel struct {
	Image   string `json:"image"`
	Cmdline string `json:"cmdline"`
	Initrd  string `json:"initrd"`
	Modiso  string `json:"modiso"`
}
