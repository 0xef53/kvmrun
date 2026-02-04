package system

import (
	"time"

	"github.com/0xef53/kvmrun/internal/appconf"
	"github.com/0xef53/kvmrun/server/system"

	pb "github.com/0xef53/kvmrun/api/services/system/v2"
	pb_types "github.com/0xef53/kvmrun/api/types/v2"
)

func appConfToProto(appConf *appconf.Config) *pb_types.AppConf {
	return &pb_types.AppConf{
		Kvmrun: &pb_types.AppConf_KvmrunConf{
			QemuRootdir: appConf.Kvmrun.QemuRootDir,
			CertDir:     appConf.Kvmrun.CertDir,
		},
	}
}

func optsFromStartIncomingMigrationRequest(req *pb.StartIncomingMigrationRequest) *system.IncomingMigrationOptions {
	return &system.IncomingMigrationOptions{
		Manifest:     req.Manifest,
		Disks:        req.Disks,
		ExtraFiles:   req.ExtraFiles,
		ListenAddr:   req.ListenAddr,
		CreateDisks:  req.CreateDisks,
		TurnOffAfter: req.TurnOffAfter,
	}
}

func incomingRequisitesToProto(requisites *system.IncomingRequisites) *pb_types.IncomingMigrationRequisites {
	return &pb_types.IncomingMigrationRequisites{
		IncomingPort: uint32(requisites.IncomingPort),
		NBDPort:      uint32(requisites.NBDPort),
		PID:          uint32(requisites.PID),
	}
}

func optsFromQemuInstanceRegisterRequest(req *pb.QemuInstanceRegisterRequest) *system.InstanceRegistrationOptions {
	return &system.InstanceRegistrationOptions{
		MemActual: uint32(req.MemActual),
		PID:       uint32(req.PID),
	}
}

func optsFromStopQemuInstanceRequest(req *pb.QemuInstanceStopRequest) *system.InstanceTerminationOptions {
	return &system.InstanceTerminationOptions{
		GracefulTimeout: time.Duration(req.GracefulTimeout) * time.Second,
	}
}
