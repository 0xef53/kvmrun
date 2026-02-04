package kvmrun

const (
	FIRST_INCOMING_PORT = 30000
	FIRST_WS_PORT       = 10700
	FIRST_NBD_PORT      = 60000

	CONFDIR = "/etc/kvmrun"

	QEMU_BINARY = "/usr/lib/kvmrun/qemu.wrapper"

	VMNETINIT = "/usr/lib/kvmrun/netinit"

	QMPMONDIR  = "/var/run/kvm-monitor"
	CHROOTDIR  = "/var/lib/kvmrun/chroot"
	KERNELSDIR = "/var/lib/kvmrun/kernels"
	MODULESDIR = "/var/lib/kvmrun/modules"
	LOGDIR     = "/var/log/kvmrun"

	DEFAULT_QEMU_ROOTDIR = "/"
)

const DriverType_UNKNOWN = 0
