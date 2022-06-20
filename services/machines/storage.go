package machines

import (
	"context"
	"fmt"
	"strings"

	pb "github.com/0xef53/kvmrun/api/services/machines/v1"
	"github.com/0xef53/kvmrun/kvmrun"
	"github.com/0xef53/kvmrun/kvmrun/backend"

	empty "github.com/golang/protobuf/ptypes/empty"
	log "github.com/sirupsen/logrus"
)

func (s *ServiceServer) AttachDisk(ctx context.Context, req *pb.AttachDiskRequest) (*empty.Empty, error) {
	d, err := kvmrun.NewDisk(req.DiskPath)
	if err != nil {
		return nil, err
	}

	d.Driver = strings.ReplaceAll(strings.ToLower(req.Driver.String()), "_", "-")

	d.IopsRd = int(req.IopsRd)
	d.IopsWr = int(req.IopsWr)

	d.Bootindex = uint(req.Bootindex)

	if len(req.ProxyCommand) > 0 {
		if v, err := resolveExecutable(req.ProxyCommand); err == nil {
			req.ProxyCommand = v
		} else {
			return nil, err
		}
	}

	taskKey := req.Name + ":" + d.BaseName() + ":"

	err = s.RunFuncTask(ctx, taskKey, func(l *log.Entry) error {
		vm, err := s.GetMachine(req.Name)
		if err != nil {
			return err
		}

		if req.Live && vm.R != nil {
			if req.Index >= 0 {
				return fmt.Errorf("unable to set disk index in relation to the running QEMU instance")
			}

			if vm.R.GetDisks().Exists(req.DiskPath) {
				if len(req.ProxyCommand) > 0 {
					return fmt.Errorf("disk is already connected, unable to add a proxy for it")
				}

				// In this case just re-set the io limits
				if err := vm.R.SetDiskReadIops(req.DiskPath, int(req.IopsRd)); err != nil {
					return err
				}
				if err := vm.R.SetDiskWriteIops(req.DiskPath, int(req.IopsWr)); err != nil {
					return err
				}
			} else {
				var success bool

				if len(req.ProxyCommand) > 0 {
					proxy := kvmrun.Proxy{
						Path:    req.DiskPath,
						Command: req.ProxyCommand,
						Envs:    req.ProxyEnvs,
					}

					defer func() {
						if !success {
							vm.R.RemoveProxy(req.DiskPath)
							s.DeactivateDiskBackendProxy(req.Name, d.BaseName())
						}
					}()

					if err := vm.R.AppendProxy(proxy); err != nil {
						return err
					}

					if err := s.ActivateDiskBackendProxy(req.Name, d.BaseName()); err != nil {
						return err
					}
				}

				if err := vm.R.AppendDisk(*d); err != nil {
					return err
				}

				success = true
			}
		}

		addToConf := func() error {
			// In the config file we are able to add a proxy server
			// for an already connected disk
			if len(req.ProxyCommand) > 0 {
				proxy := kvmrun.Proxy{
					Path:    req.DiskPath,
					Command: req.ProxyCommand,
					Envs:    req.ProxyEnvs,
				}
				if err := vm.C.AppendProxy(proxy); err != nil {
					return err
				}
			}
			if req.Index >= 0 {
				return vm.C.InsertDisk(*d, int(req.Index))
			}
			return vm.C.AppendDisk(*d)
		}

		switch err := addToConf(); {
		case err == nil:
		case kvmrun.IsAlreadyConnectedError(err):
			// In this case just re-set the io limits
			if err := vm.C.SetDiskReadIops(req.DiskPath, int(req.IopsRd)); err != nil {
				return err
			}
			if err := vm.C.SetDiskWriteIops(req.DiskPath, int(req.IopsWr)); err != nil {
				return err
			}
		default:
			return err
		}

		return vm.C.Save()
	})

	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}

func (s *ServiceServer) DetachDisk(ctx context.Context, req *pb.DetachDiskRequest) (*empty.Empty, error) {
	var diskname string

	switch be, err := kvmrun.NewDiskBackend(req.DiskName); err.(type) {
	case nil:
		diskname = be.BaseName()
	case *backend.UnknownBackendError:
		// Try args.DiskName as a short disk name
		diskname = req.DiskName
	default:
		return nil, err
	}

	taskKey := req.Name + ":" + diskname + ":"

	err := s.RunFuncTask(ctx, taskKey, func(l *log.Entry) error {
		vm, err := s.GetMachine(req.Name)
		if err != nil {
			return err
		}

		if req.Live && vm.R != nil {
			if disk := vm.R.GetDisks().Get(diskname); disk != nil {
				if err := vm.R.RemoveDisk(diskname); err != nil {
					return err
				}
				if err := vm.R.RemoveProxy(disk.Path); err != nil && !kvmrun.IsNotConnectedError(err) {
					return err
				}
				if err := s.DeactivateDiskBackendProxy(req.Name, diskname); err != nil {
					return err
				}
			}
		}

		if disk := vm.C.GetDisks().Get(diskname); disk != nil {
			if err := vm.C.RemoveDisk(diskname); err != nil {
				return err
			}
			if err := vm.C.RemoveProxy(disk.Path); err != nil && !kvmrun.IsNotConnectedError(err) {
				return err
			}
		}

		return vm.C.Save()
	})

	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}

func (s *ServiceServer) SetDiskLimits(ctx context.Context, req *pb.SetDiskLimitsRequest) (*empty.Empty, error) {
	err := s.RunFuncTask(ctx, req.Name, func(l *log.Entry) error {
		vm, err := s.GetMachine(req.Name)
		if err != nil {
			return err
		}

		var savingRequire bool

		// "-1" means that the parameter should not be changed

		if req.IopsRd >= 0 {
			if req.Live && vm.R != nil {
				if err := vm.R.SetDiskReadIops(req.DiskName, int(req.IopsRd)); err != nil {
					return err
				}
			}
			if err := vm.C.SetDiskReadIops(req.DiskName, int(req.IopsRd)); err != nil {
				return err
			}
			savingRequire = true
		}

		if req.IopsWr >= 0 {
			if req.Live && vm.R != nil {
				if err := vm.R.SetDiskWriteIops(req.DiskName, int(req.IopsWr)); err != nil {
					return err
				}
			}
			if err := vm.C.SetDiskWriteIops(req.DiskName, int(req.IopsWr)); err != nil {
				return err
			}
			savingRequire = true
		}

		if savingRequire {
			return vm.C.Save()
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}

func (s *ServiceServer) RemoveDiskBitmap(ctx context.Context, req *pb.RemoveDiskBitmapRequest) (*empty.Empty, error) {
	err := s.RunFuncTask(ctx, req.Name, func(l *log.Entry) error {
		vm, err := s.GetMachine(req.Name)
		if err != nil {
			return err
		}

		if vm.R != nil {
			return vm.R.RemoveDiskBitmap(req.DiskName)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}

func (s *ServiceServer) ResizeQemuBlockdev(ctx context.Context, req *pb.ResizeQemuBlockdevRequest) (*empty.Empty, error) {
	err := s.RunFuncTask(ctx, req.Name, func(l *log.Entry) error {
		vm, err := s.GetMachine(req.Name)
		if err != nil {
			return err
		}

		if vm.R != nil {
			return vm.R.ResizeQemuBlockdev(req.DiskName)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}
