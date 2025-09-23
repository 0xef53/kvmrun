package classifiers

// Options defines the common interface for classifier options.
type Options interface {
	GetLabel() string
	Validate() error
}
