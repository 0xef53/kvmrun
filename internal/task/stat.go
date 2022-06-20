package task

type TaskState int32

const (
	StateUnknown TaskState = iota
	StateRunning
	StateCompleted
	StateFailed
)

type TaskStat struct {
	Key       string      `json:"key"`
	State     TaskState   `json:"state"`
	StateDesc string      `json:"state_desc"`
	Progress  int32       `json:"progress"`
	Details   interface{} `json:"details"`
}
