package backend

type DiskBackend interface {
	FullPath() string
	BaseName() string
	QdevID() string
	Size() (uint64, error)
	IsLocal() bool
	IsAvailable() (bool, error)

	Copy() DiskBackend
}
