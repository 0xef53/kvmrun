package machine

type DataTransferStat struct {
	Total       uint64
	Remaining   uint64
	Transferred uint64
	Progress    int
	Speed       int
}
