package tasks

import (
	"github.com/0xef53/kvmrun/internal/task"
	"github.com/0xef53/kvmrun/server/machine"

	pb_types "github.com/0xef53/kvmrun/api/types/v2"
)

func taskStatToProto(st *task.TaskStat) *pb_types.TaskInfo {
	info := pb_types.TaskInfo{
		TaskID:    st.ID,
		State:     pb_types.TaskInfo_TaskState(st.State),
		StateDesc: st.StateDesc,
		Progress:  uint32(st.Progress),
	}

	switch d := st.Details.(type) {
	case *machine.MachineMigrationStatDetails:
		if d != nil {
			mi := pb_types.TaskInfo_MigrationInfo{
				DstServer: d.DstServer,
			}

			if d.VMState != nil {
				mi.Qemu = &pb_types.TaskInfo_MigrationInfo_Stat{
					Total:       d.VMState.Total,
					Remaining:   d.VMState.Remaining,
					Transferred: d.VMState.Transferred,
					Progress:    uint32(d.VMState.Progress),
					Speed:       uint32(d.VMState.Speed),
				}
			}

			if d.Disks != nil {
				mi.Disks = make(map[string]*pb_types.TaskInfo_MigrationInfo_Stat)
				for n, v := range d.Disks {
					mi.Disks[n] = &pb_types.TaskInfo_MigrationInfo_Stat{
						Total:       v.Total,
						Remaining:   v.Remaining,
						Transferred: v.Transferred,
						Progress:    uint32(v.Progress),
						Speed:       uint32(v.Speed),
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
