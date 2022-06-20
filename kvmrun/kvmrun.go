package kvmrun

const (
	FIRST_INCOMING_PORT = 30000
	FIRST_WS_PORT       = 10700
	FIRST_NBD_PORT      = 60000

	CONFDIR = "/etc/kvmrun"

	VMNETINIT = "/usr/lib/kvmrun/netinit"

	QMPMONDIR  = "/var/run/kvm-monitor"
	CHROOTDIR  = "/var/lib/kvmrun/chroot"
	KERNELSDIR = "/var/lib/kvmrun/kernels"
	MODULESDIR = "/var/lib/kvmrun/modules"
	LOGDIR     = "/var/log/kvmrun"

	CGROOTPATH = "kvmrun"
)

type DevDriver struct {
	Name         string `json:"name"`
	HotPluggable bool   `json:"hotplugguble"`
}

type DevDrivers []DevDriver

func (d DevDrivers) Exists(name string) bool {
	for _, v := range d {
		if v.Name == name {
			return true
		}
	}

	return false
}

func (d DevDrivers) HotPluggable(name string) bool {
	for _, v := range d {
		if v.Name == name && v.HotPluggable {
			return true
		}
	}

	return false
}
