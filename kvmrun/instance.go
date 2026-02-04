package kvmrun

import (
	"github.com/0xef53/kvmrun/internal/version"
)

type Instance interface {
	Name() string
	UID() int
	PID() int

	QemuVersion() *version.Version

	IsIncoming() bool

	Save() error

	Status() (InstanceState, error)

	MachineTypeGet() *MachineType
	MachineTypeSet(string) error

	FirmwareGet() *Firmware
	FirmwareGetImage() string
	FirmwareGetFlash() *Disk
	FirmwareSetImage(string) error
	FirmwareSetFlash(string) error
	FirmwareRemoveConf() error

	MemoryGetActual() int
	MemoryGetTotal() int
	MemorySetActual(int) error
	MemorySetTotal(int) error

	CPUGetActual() int
	CPUGetTotal() int
	CPUGetSockets() int
	CPUGetQuota() int
	CPUGetModel() string
	CPUSetActual(int) error
	CPUSetTotal(int) error
	CPUSetSockets(int) error
	CPUSetModel(string) error
	CPUSetQuota(int) error

	InputDeviceGet(string) *InputDevice
	InputDeviceGetList(...string) []*InputDevice
	InputDeviceAppend(InputDeviceProperties) error
	InputDeviceRemove(string) error

	CdromGet(string) *Cdrom
	CdromGetList(...string) []*Cdrom
	CdromAppend(CdromProperties) error
	CdromInsert(CdromProperties, int) error
	CdromRemove(string) error
	CdromGetMedia(string) (string, error)
	CdromChangeMedia(string, string) error
	CdromRemoveMedia(string) error

	DiskGet(string) *Disk
	DiskGetList(...string) []*Disk
	DiskAppend(DiskProperties) error
	DiskInsert(DiskProperties, int) error
	DiskRemove(string) error
	DiskSetReadIops(string, int) error
	DiskSetWriteIops(string, int) error
	DiskResizeQemuBlockdev(string) error
	DiskRemoveQemuBitmap(string) error

	NetIfaceGet(string) *NetIface
	NetIfaceGetList(...string) []*NetIface
	NetIfaceAppend(NetIfaceProperties) error
	NetIfaceRemove(string) error
	NetIfaceSetQueues(string, int) error
	NetIfaceSetUpScript(string, string) error
	NetIfaceSetDownScript(string, string) error
	NetIfaceSetLinkUp(string) error
	NetIfaceSetLinkDown(string) error

	VSockDeviceGet() *ChannelVSock
	VSockDeviceAppend(ChannelVSockProperties) error
	VSockDeviceRemove() error

	CloudInitGetDrive() *CloudInitDrive
	CloudInitSetMedia(string) error
	CloudInitSetDriver(string) error
	CloudInitRemoveConf() error

	KernelGetImage() string
	KernelGetCmdline() string
	KernelGetInitrd() string
	KernelGetModiso() string
	KernelSetImage(string) error
	KernelSetCmdline(string) error
	KernelSetInitrd(string) error
	KernelSetModiso(string) error
	KernelRemoveConf() error

	HostDeviceGet(string) *HostDevice
	HostDeviceGetList(...string) []*HostDevice
	HostDeviceAppend(HostDeviceProperties) error
	HostDeviceRemove(string) error
	HostDeviceSetMultifunctionOption(string, bool) error
	HostDeviceSetPrimaryGPUOption(string, bool) error

	VNCSetPassword(string) error
}
