package server

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/0xef53/kvmrun/kvmrun"

	grpcutils "github.com/0xef53/go-grpc/utils"

	"github.com/0xef53/go-task"
	"github.com/0xef53/go-task/classifiers"
	"github.com/0xef53/go-task/metadata"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

const (
	ModeBlockConf task.OperationMode = 1 << (8 - 1 - iota)
	ModeBlockBackup

	ModeNoBlock  = task.OperationMode(0)
	ModeBlockAll = ^task.OperationMode(0)
)

func NoBlockOperations(name string) map[string]task.OperationMode {
	return map[string]task.OperationMode{name: ModeNoBlock}
}

func BlockAnyOperations(main string, names ...string) map[string]task.OperationMode {
	targets := map[string]task.OperationMode{main: ModeBlockAll}

	for _, n := range names {
		targets[main+":"+n] = ModeBlockAll
	}

	return targets
}

func BlockConfOperations(vmname string) map[string]task.OperationMode {
	return map[string]task.OperationMode{vmname: ModeBlockConf}
}

func BlockBackupOperations(vmname, diskname string) map[string]task.OperationMode {
	return map[string]task.OperationMode{
		vmname:                  ModeBlockConf,
		vmname + ":" + diskname: ModeBlockAll,
	}
}

func WithUniqueLabel(label string) *task.TaskClassifierDefinition {
	return &task.TaskClassifierDefinition{
		Name: "unique-labels",
		Opts: &classifiers.UniqueLabelOptions{Label: label},
	}
}

func WithGroupLabel(label string) *task.TaskClassifierDefinition {
	return &task.TaskClassifierDefinition{
		Name: "group-labels",
		Opts: &classifiers.GroupLabelOptions{Label: label},
	}
}

func WithHostnetGroupLabel() *task.TaskClassifierDefinition {
	return &task.TaskClassifierDefinition{
		Name: "hostnet-group",
		Opts: &classifiers.LimitedGroupOptions{},
	}
}

var longRunningTasks = map[string]error{
	"MachineMigrationTask":         fmt.Errorf("machine is locked because the migration process is currently in progress"),
	"MachineIncomingMigrationTask": fmt.Errorf("machine is locked because the incoming-migration process is currently in progress"),
	"DiskBackupTask":               fmt.Errorf("resource is locked because the backup process is currently in progress"),
}

func (s *Server) taskStart(fn func() (string, error)) (string, error) {
	var tid string
	var err error

	for attempt := 0; attempt < 10; attempt++ {
		tid, err = fn()

		if _err, ok := err.(*task.ConcurrentRunningError); ok {
			labels := make([]string, 0, len(_err.Targets))

			for object := range _err.Targets {
				labels = append(labels, object+"/long-running")
			}

			if stats := s.Tasks.StatByLabel(labels...); len(stats) > 0 {
				if md, ok := stats[0].Metadata.(*TaskMetadata); ok && md != nil {
					if descErr, ok := longRunningTasks[md.Kind]; ok {
						err = descErr

						break
					}
				}
			}
		} else {
			break
		}

		time.Sleep(time.Second)
	}

	return tid, err
}

type TaskMetadata struct {
	Kind string
}

// Всегда делается context.WithoutCancel для переданного контекста
func (s *Server) TaskStart(ctx context.Context, t task.Task, resp interface{}, opts ...task.TaskOption) (string, error) {
	if _, ok := metadata.FromContext(ctx); ok {
		// do nothing, some metadata already set
	} else {
		ff := strings.Split(strings.TrimLeft(fmt.Sprintf("%T", t), "*"), ".")

		ctx = metadata.AppendToContext(ctx, &TaskMetadata{Kind: ff[len(ff)-1]})
	}

	return s.taskStart(func() (string, error) {
		return s.Tasks.StartTask(context.WithoutCancel(ctx), t, resp, opts...)
	})
}

// Всегда делается context.WithoutCancel для переданного контекста
func (s *Server) TaskRunFunc(ctx context.Context, tgt map[string]task.OperationMode, wait bool, opts []task.TaskOption, fn func(*log.Entry) error) error {
	if _, ok := metadata.FromContext(ctx); ok {
		// do nothing, some metadata already set
	} else {
		ctx = metadata.AppendToContext(ctx, &TaskMetadata{Kind: "FuncTask"})
	}

	if !wait {
		// Switch to blocking mode if request-id is not set
		reqID := grpcutils.ExtractRequestID(ctx)

		if len(reqID) == 0 {
			log.Info("Switch to blocking mode because request-id is not set")

			wait = true
		}
	}

	_, err := s.taskStart(func() (string, error) {
		return s.Tasks.RunFunc(context.WithoutCancel(ctx), tgt, wait, opts, fn)
	})

	return err
}

func (s *Server) TaskCancel(labels ...string) error {
	s.Tasks.CancelByLabel(labels...)

	return nil
}

func (s *Server) TaskGetStats(labels ...string) ([]*task.TaskStat, error) {
	m := make(map[string]*task.TaskStat)

	for _, st := range s.Tasks.StatByLabel(labels...) {
		m[st.ID] = st
	}

	if len(labels) == 0 {
		for _, tid := range s.Tasks.List() {
			if st := s.Tasks.Stat(tid); st != nil {
				m[st.ID] = st
			}
		}
	} else {
		// Сheck if there are task IDs in the label list
		for _, tid := range labels {
			if err := uuid.Validate(tid); err == nil {
				if st := s.Tasks.Stat(tid); st != nil {
					m[st.ID] = st
				}
			}
		}
	}

	if len(labels) == 0 {
		entries, err := os.ReadDir(kvmrun.CHROOTDIR)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}

		for _, v := range entries {
			if v.IsDir() {
				taskDir := filepath.Join(kvmrun.CHROOTDIR, v.Name(), ".tasks")

				files, err := os.ReadDir(taskDir)
				if err != nil {
					if os.IsNotExist(err) {
						continue
					}
					return nil, err
				}

				for _, f := range files {
					if st, err := s.readStatFile(filepath.Join(taskDir, f.Name())); err == nil {
						m[st.ID] = st
					} else if !errors.Is(err, os.ErrNotExist) {
						return nil, err
					}
				}
			}
		}
	}

	for _, label := range labels {
		if st, err := s.readTaskStatFile(label); err == nil {
			m[st.ID] = st
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	}

	stats := make([]*task.TaskStat, 0, len(m))

	for _, st := range m {
		stats = append(stats, st)
	}

	return stats, nil
}

type TaskStatFile struct {
	Label string         `json:"label"`
	Stat  *task.TaskStat `json:"stat"`
}

func (s *Server) readTaskStatFile(label string) (*task.TaskStat, error) {
	hashname := fmt.Sprintf("%x", md5.Sum([]byte(label)))

	/*
		COMMENT:
			Метка всегда записана в формате: vmname/part1/part2/.../partN
	*/

	vmname := strings.TrimSpace(strings.SplitN(label, "/", 2)[0])

	if len(vmname) > 0 {
		vmChrootDir := filepath.Join(kvmrun.CHROOTDIR, vmname)

		if info, err := os.Stat(vmChrootDir); err == nil && info.IsDir() {
			return s.readStatFile(filepath.Join(vmChrootDir, ".tasks", hashname))
		}
	}

	return nil, os.ErrNotExist
}

func (s *Server) readStatFile(fname string) (*task.TaskStat, error) {
	b, err := os.ReadFile(fname)
	if err != nil {
		return nil, err
	}

	st := TaskStatFile{}

	if err := json.Unmarshal(b, &st); err != nil {
		return nil, err
	}

	return st.Stat, nil
}
