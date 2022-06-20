package backend

type DiskBackend interface {
	BaseName() string
	QdevID() string
	Size() (uint64, error)
	IsLocal() bool
	IsAvailable() (bool, error)
}
