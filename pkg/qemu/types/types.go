package types

// IntValue is an integer representation of the Value argument.
type IntValue struct {
	Value int `json:"value"`
}

// Uint64Value is a uint64 representation of the Value argument.
type Uint64Value struct {
	Value uint64 `json:"value"`
}

// IntID is an integer representation of the ID argument.
type IntID struct {
	ID int `json:"id"`
}

// StrID is a string representation of the ID argument.
type StrID struct {
	ID string `json:"id"`
}

// StatusInfo represents a guest running status.
type StatusInfo struct {
	Running    bool   `json:"running"`
	Singlestep bool   `json:"singlestep"`
	Status     string `json:"status"`
}

// QomQuery is a query struct to get a property value of QOM by the path.
type QomQuery struct {
	Path     string `json:"path"`
	Property string `json:"property"`
}

// PCIInfo describes the PCI bus and all its devices.
type PCIInfo struct {
	Bus     int `json:"bus"`
	Devices []struct {
		QdevID    string `json:"qdev_id"`
		Slot      int    `json:"slot"`
		ClassInfo struct {
			Class int `json:"class"`
		} `json:"class_info"`
		ID struct {
			Device int `json:"device"`
			Vendor int `json:"vendor"`
		} `json:"id"`
	} `json:"devices"`
}

// NetdevTapOptions describes a TAP based guest networking device
type NetdevTapOptions struct {
	Type       string `json:"type"`
	ID         string `json:"id"`
	Ifname     string `json:"ifname"`
	Vhost      string `json:"vhost"`
	Script     string `json:"script"`
	Downscript string `json:"downscript"`
}

// BlockInfo describes a virtual device and the backing device associated with it.
type BlockInfo struct {
	Device       string `json:"device"`
	DirtyBitmaps []struct {
		Name string `json:"name"`
	} `json:"dirty-bitmaps"`
	Inserted struct {
		File             string `json:"file"`
		BackingFile      string `json:"backing_file"`
		BackingFileDepth int    `json:"backing_file_depth"`
		ReadOnly         bool   `json:"ro"`
		IopsRd           int    `json:"iops_rd"`
		IopsWr           int    `json:"iops_wr"`
		Image            struct {
			Filename        string `json:"filename"`
			Format          string `json:"format"`
			ActualSize      uint64 `json:"actual-size"`
			VirtualSize     uint64 `json:"virtual-size"`
			BackingFilename string `json:"backing-filename"`
			BackingImage    struct {
				Filename    string `json:"filename"`
				VirtualSize uint64 `json:"virtual-size"`
			} `json:"backing-image"`
		}
	} `json:"inserted"`
}

type InsertedFileOptions struct {
	Driver string `json:"driver"`
	File   struct {
		Driver string `json:"driver"`
		// iSCSI specific options
		InitiatorName string `json:"initiator-name"`
		Portal        string `json:"portal"`
		Target        string `json:"target"`
		Lun           string `json:"lun"`
		User          string `json:"user"`
		Password      string `json:"password"`
	} `json:"file"`
}

// BlockIOThrottle represents a set of parameters describing block device throttling.
type BlockIOThrottle struct {
	Device string `json:"device"`
	Iops   int    `json:"iops"`
	IopsRd int    `json:"iops_rd"`
	IopsWr int    `json:"iops_wr"`
	Bps    int    `json:"bps"`
	BpsWr  int    `json:"bps_wr"`
	BpsRd  int    `json:"bps_rd"`
}

// BlockResizeQuery is a query struct for the block_resize command.
// The size value should be in bytes.
type BlockResizeQuery struct {
	Device string `json:"device"`
	Size   int    `json:"size"`
}

// CPUInfo describes the properties of a virtual CPU.
type CPUInfo struct {
	CPU      int    `json:"CPU"`
	QomPath  string `json:"qom_path"`
	ThreadID int    `json:"thread_id"`
}

// CPUInfoFast describes the properties of a virtual CPU.
type CPUInfoFast struct {
	CPUIndex int `json:"cpu-index"`
	ThreadID int `json:"thread-id"`
}

// HotpluggableCPU describes a hot-pluggable CPU.
type HotpluggableCPU struct {
	Props struct {
		SocketID int `json:"socket-id"`
	} `json:"props"`
	QomPath string `json:"qom-path"`
	Type    string `json:"type"`
}

// ChardevInfo describes a character device.
type ChardevInfo struct {
	Label        string `json:"label"`
	Filename     string `json:"filename"`
	FrontendOpen bool   `json:"frontend-open"`
}

// ChardevOptions represents a set of parameters for the new character device.
type ChardevOptions struct {
	ID      string         `json:"id"`
	Backend ChardevBackend `json:"backend"`
}

// ChardevBackend represents a set of parameters for the new chardev backend.
type ChardevBackend struct {
	Type string        `json:"type"`
	Data ChardevSocket `json:"data"`
}

//  ChardevSocket describes a (stream) socket character device.
type ChardevSocket struct {
	Addr   SocketAddressLegacy `json:"addr"`
	Server bool                `json:"server"`
	Wait   bool                `json:"wait"`
}

type SocketAddressLegacy struct {
	Type string            `json:"type"`
	Data UnixSocketAddress `json:"data"`
}

type InetSocketAddressLegacy struct {
	Type string                `json:"type"`
	Data InetSocketAddressBase `json:"data"`
}

type UnixSocketAddress struct {
	Path string `json:"path"`
}

type InetSocketAddressBase struct {
	Host string `json:"host"`
	Port string `json:"port"`
}

// DeviceOptions is a common structure for different types of devices.
type DeviceOptions struct {
	Driver       string `json:"driver"`
	Id           string `json:"id"`
	Bus          string `json:"bus,omitempty"`
	Addr         string `json:"addr,omitempty"`
	Drive        string `json:"drive,omitempty"`
	Netdev       string `json:"netdev,omitempty"`
	Mac          string `json:"mac,omitempty"`
	Bootindex    int    `json:"bootindex,omitempty"`
	Chardev      string `json:"chardev,omitempty"`
	Name         string `json:"name,omitempty"`
	SCSI_Channel int    `json:"channel,omitempty"`
	SCSI_ID      int    `json:"scsi-id,omitempty"`
	SCSI_Lun     int    `json:"lun,omitempty"`
}

// MigrationCapabilityStatus describes the state (enabled/disabled) of migration capability.
type MigrationCapabilityStatus struct {
	Capability string `json:"capability"`
	State      bool   `json:"state"`
}

// MigrateSetParameters represents a set of various migration parameters.
type MigrateSetParameters struct {
	MaxBandwidth    int `json:"max-bandwidth"`
	XbzrleCacheSize int `json:"xbzrle-cache-size"`
}

// MigrationInfo describes a running migration process.
type MigrationInfo struct {
	Status string `json:"status"`
	Ram    struct {
		Total     uint64  `json:"total"`
		Remaining uint64  `json:"remaining"`
		Speed     float64 `json:"mbps"`
	} `json:"ram"`
	ErrDesc string `json:"error-desc"`
}

// DriveMirrorOptions is a set of parameters for setting up a new mirroring process.
type DriveMirrorOptions struct {
	JobID  string `json:"job-id"`
	Device string `json:"device"`
	Target string `json:"target"`
	Format string `json:"format"`
	Sync   string `json:"sync"`
	Mode   string `json:"mode"`
}

// DriveBackupOptions is a set of parameters for setting up a new drive backup process.
type DriveBackupOptions struct {
	JobID  string `json:"job-id,omitempty"`
	Device string `json:"device"`
	Target string `json:"target"`
	Format string `json:"format,omitempty"`
	Sync   string `json:"sync"`
	Bitmap string `json:"bitmap,omitempty"`
	Mode   string `json:"mode,omitempty"`
	Speed  uint64 `json:"speed,omitempty"`
}

// BlockJobInfo describes a long-running block device operation.
type BlockJobInfo struct {
	Type   string `json:"type"`
	Device string `json:"device"`
	Len    uint64 `json:"len"`
	Offset uint64 `json:"offset"`
	Busy   bool   `json:"busy"`
	Paused bool   `json:"paused"`
	Speed  uint64 `json:"speed"`
	Ready  bool   `json:"ready"`
}

// BlockDirtyBitmapOptions is a common structure for operations with dirty bitmaps.
type BlockDirtyBitmapOptions struct {
	Node string `json:"node"`
	Name string `json:"name"`
}
