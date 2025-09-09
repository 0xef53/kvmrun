package task

type TaskState int32

const (
	StateUnknown TaskState = iota
	StateRunning
	StateCompleted
	StateFailed
)

type TaskStat struct {
	ID          string      `json:"id"`
	ShortID     string      `json:"short_id"`
	State       TaskState   `json:"state"`
	StateDesc   string      `json:"state_desc"`
	Interrupted bool        `json:"interrupted"`
	Progress    int         `json:"progress"`
	Details     interface{} `json:"details"`

	Metadata interface{} `json:"metadata"`
}
