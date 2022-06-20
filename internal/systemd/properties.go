package systemd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"
)

type ExecCode int

const (
	CodeUndefined ExecCode = iota
	CodeExited
	CodeKilled
	CodeDumped
)

type UnitProperties struct {
	Name            string `json:"-"` // The primary unit name as string
	LoadState       string // The load state (i.e. whether the unit file has been loaded successfully)
	ActiveState     string // The active state (i.e. whether the unit is currently started or not)
	SubState        string // The sub state (a more fine-grained version of the active state that is specific to the unit type, which the active state is not)
	ExecMainPID     int    // The main PID is the current main PID of the service and is 0 when the service currently has no main PID
	ActiveEnterTime time.Time
	ActiveExitTime  time.Time
	StateChangeTime time.Time
	ExecStart       []ExecCommandState
	ExecStartPost   []ExecCommandState
	ExecStartPre    []ExecCommandState
	ExecStop        []ExecCommandState
	ExecStopPost    []ExecCommandState
}

type ExecCommandState struct {
	Path          string    // The binary path to execute
	Arguments     []string  // An array with all arguments to pass to the executed command, starting with argument 0
	Fail          bool      // A boolean whether it should be considered a failure if the process exits uncleanly
	EnterTime     time.Time // CLOCK_REALTIME time when the process began running the last time
	EnterTimeMono int64     // CLOCK_MONOTONIC usec timestamps when the process began running the last time
	ExitTime      time.Time // CLOCK_REALTIME time when the process finished running the last time
	ExitTimeMono  int64     // CLOCK_MONOTONIC usec timestamps when the process finished running the last time
	ExecPID       int       // The PID of the process (or 0 if it has not run yet)
	ExecCode      ExecCode  // The exit code of the last run
	ExecStatus    int       // The exit status
}

func (s *ExecCommandState) UnmarshalJSON(b []byte) error {
	if bytes.Equal(b, []byte("null")) {
		return nil
	}

	if len(b) == 0 {
		return fmt.Errorf("input is too short")
	}

	dst := struct {
		Path          string
		Arguments     []string
		Fail          bool
		EnterTime     int64
		EnterTimeMono int64
		ExitTime      int64
		ExitTimeMono  int64
		ExecPID       int
		ExecCode      ExecCode
		ExecStatus    int
	}{}

	tmp := []interface{}{
		&dst.Path,
		&dst.Arguments,
		&dst.Fail,
		&dst.EnterTime,
		&dst.EnterTimeMono,
		&dst.ExitTime,
		&dst.ExitTimeMono,
		&dst.ExecPID,
		&dst.ExecCode,
		&dst.ExecStatus,
	}

	if err := json.Unmarshal(b, &tmp); err != nil {
		return err
	}

	s.Path = dst.Path
	s.Arguments = dst.Arguments
	s.EnterTime = time.Unix(0, dst.EnterTime*int64(time.Microsecond))
	s.ExitTime = time.Unix(0, dst.ExitTime*int64(time.Microsecond))
	s.ExecPID = dst.ExecPID
	s.ExecCode = dst.ExecCode
	s.ExecStatus = dst.ExecStatus

	return nil
}
