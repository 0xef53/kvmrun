package kvmrun

// Memory represents the guest memory configuration structure.
type Memory struct {
	Actual int `json:"actual"`
	Total  int `json:"total"`
}
