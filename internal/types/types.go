package types

type DataTransferStat struct {
	Total       uint64
	Remaining   uint64
	Transferred uint64
	Progress    int32
	Speed       int32
}

type MachineMigrationDetails struct {
	DstServer string
	VMState   *DataTransferStat
	Disks     map[string]*DataTransferStat
}

type DiskBackupDetails struct {
	Disk *DataTransferStat
}
