package tasks

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	pb_types "github.com/0xef53/kvmrun/api/types"
	"github.com/0xef53/kvmrun/internal/task"
	"github.com/0xef53/kvmrun/internal/types"
	"github.com/0xef53/kvmrun/kvmrun"
)

func taskStatToProto(st *task.TaskStat) *pb_types.TaskInfo {
	info := pb_types.TaskInfo{
		Key:       st.Key,
		State:     pb_types.TaskInfo_TaskState(st.State),
		StateDesc: st.StateDesc,
		Progress:  st.Progress,
	}

	switch d := st.Details.(type) {
	case *types.MachineMigrationDetails:
		if d != nil {
			mi := pb_types.TaskInfo_MigrationInfo{
				DstServer: d.DstServer,
			}

			if d.VMState != nil {
				mi.Qemu = &pb_types.TaskInfo_MigrationInfo_Stat{
					Total:       d.VMState.Total,
					Remaining:   d.VMState.Remaining,
					Transferred: d.VMState.Transferred,
					Progress:    d.VMState.Progress,
					Speed:       d.VMState.Speed,
				}
			}

			if d.Disks != nil {
				mi.Disks = make(map[string]*pb_types.TaskInfo_MigrationInfo_Stat)
				for n, v := range d.Disks {
					mi.Disks[n] = &pb_types.TaskInfo_MigrationInfo_Stat{
						Total:       v.Total,
						Remaining:   v.Remaining,
						Transferred: v.Transferred,
						Progress:    v.Progress,
						Speed:       v.Speed,
					}
				}
			}

			info.Stat = &pb_types.TaskInfo_Migration{
				Migration: &mi,
			}
		}
	}

	return &info
}

func readStatFromFile(key string) (*task.TaskStat, error) {
	ff := strings.SplitN(key, ":", 3)
	if len(ff) != 3 {
		// We cannot find the statfile using the wrong Key
		return nil, os.ErrNotExist
	}

	statfile := filepath.Join(kvmrun.CHROOTDIR, ff[1], ".tasks", key)

	b, err := ioutil.ReadFile(statfile)
	if err != nil {
		return nil, err
	}

	st := task.TaskStat{}

	if err := json.Unmarshal(b, &st); err != nil {
		return nil, err
	}

	return &st, nil
}

func getFileSystemKeys() ([]string, error) {
	chdirs, err := ioutil.ReadDir(kvmrun.CHROOTDIR)
	switch {
	case err == nil:
	case os.IsNotExist(err):
		return nil, nil
	default:
		return nil, err
	}

	keys := make([]string, 0)

	for _, x := range chdirs {
		if x.IsDir() {
			tasks, err := ioutil.ReadDir(filepath.Join(kvmrun.CHROOTDIR, x.Name(), ".tasks"))
			if err == nil {
				for _, key := range tasks {
					keys = append(keys, key.Name())
				}
			} else {
				if os.IsNotExist(err) {
					continue
				}
				return nil, err
			}
		}
	}

	return keys, nil
}
