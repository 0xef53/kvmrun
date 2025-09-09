package kvmrun

type InstanceState uint16

const (
	StateNoState InstanceState = iota
	StateStarting
	StateRunning
	StatePaused
	StateShutdown
	StateInactive
	StateCrashed
	StateIncoming
	StateMigrating
	StateMigrated
)
