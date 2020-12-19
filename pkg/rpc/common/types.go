package rpccommon

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/0xef53/kvmrun/pkg/kvmrun"
)

type NewInstanceRequest struct {
	Name       string
	MemTotal   int
	MemActual  int
	CPUTotal   int
	CPUActual  int
	CPUModel   string
	CPUQuota   int
	ExtraFiles map[string][]byte
}

type NewManifestInstanceRequest struct {
	Name       string
	Manifest   []byte
	ExtraFiles map[string][]byte
}

type BriefRequest struct {
	Names []string
}

type VMSummary struct {
	Name      string
	Pid       int
	MemActual int
	MemTotal  int
	CPUActual int
	CPUTotal  int
	CPUQuota  int
	State     string
	HasError  bool
	NotFound  bool
	LifeTime  time.Duration
}

type InstanceRequest struct {
	Name    string
	Live    bool
	Data    interface{}
	DataRaw json.RawMessage
	VM      *kvmrun.VirtMachine
}

type MigrationOverrides struct {
	Disks map[string]string
}

type MigrationParams struct {
	DstServer string
	Disks     []string
	Overrides MigrationOverrides
}

type NetifParams struct {
	Ifname string
	Driver string
	HwAddr string
	Ifup   string
	Ifdown string
}

type DiskParams struct {
	Path   string
	Driver string
	IopsRd int
	IopsWr int
	Index  int
}

type VNCParams struct {
	Password string
}

type VNCRequisites struct {
	Password string `json:"password"`
	Display  int    `json:"display"`
	Port     int    `json:"port"`
	WSPort   int    `json:"websock_port"`
}

type MemLimitsParams struct {
	Actual int
	Total  int
}

type CPUCountParams struct {
	Actual int
	Total  int
}

type KernelParams struct {
	Image      string
	Initrd     string
	Cmdline    string
	Modiso     string
	RemoveConf bool
}

type ChannelParams struct {
	ID   string
	Name string
}

type QemuInitRequest struct {
	Name      string
	Pid       int
	MemActual uint64
}

type NBDParams struct {
	ListenAddr string
	Disks      []string
}

type VMNameRequest struct {
	Name string
}

type DiskJobRequest struct {
	VMName   string
	DiskName string
}

type VMShutdownRequest struct {
	Name    string
	Timeout time.Duration
	Wait    bool
}

type CreateDisksRequest struct {
	Disks     map[string]uint64
	DstServer string
}

type CheckDisksRequest struct {
	Disks map[string]uint64
}

type DiskCopyingParams struct {
	SrcName     string
	TargetURI   string
	Incremental bool
	ClearBitmap bool
}

type MigrationTaskStat struct {
	DstServer string
	Status    string
	Qemu      *StatInfo
	Disks     map[string]*StatInfo
	Desc      string
}

type DiskCopyingTaskStat struct {
	Status  string
	QemuJob *StatInfo
	Desc    string
}

type StatInfo struct {
	Total       uint64 `json:"total"`
	Remaining   uint64 `json:"remaining"`
	Transferred uint64 `json:"transferred"`
	Percent     uint   `json:"percent"`
	Speed       uint   `json:"speed"`
}

type MigrationError struct {
	Err error
}

func (e *MigrationError) Error() string {
	return "migration error: " + e.Err.Error()
}

func NewMigrationError(format string, a ...interface{}) error {
	return &MigrationError{fmt.Errorf(format, a...)}
}

func IsMigrationError(err error) bool {
	if _, ok := err.(*MigrationError); ok {
		return true
	}
	return false
}
