package kvmrun

// Processor represents the guest virtual CPU configuration structure.
type Processor struct {
	Actual  int    `json:"actual"`
	Total   int    `json:"total"`
	Sockets int    `json:"sockets,omitempty"`
	Quota   int    `json:"quota,omitempty"`
	Model   string `json:"model,omitempty"`
}
