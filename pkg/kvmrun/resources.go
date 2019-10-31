package kvmrun

// CPU represents the guest virtual CPU configuration structure.
type CPU struct {
	Actual int    `json:"actual"`
	Total  int    `json:"total"`
	Quota  int    `json:"quota,omitempty"`
	Model  string `json:"model,omitempty"`
}

// Memory represents the guest memory configuration structure.
type Memory struct {
	Actual int `json:"actual"`
	Total  int `json:"total"`
}
