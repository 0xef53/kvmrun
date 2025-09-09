package classifiers

type Options interface {
	GetLabel() string
	Validate() error
}
