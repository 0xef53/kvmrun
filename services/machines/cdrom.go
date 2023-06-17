package machines

import (
	"context"
	"fmt"
	"strings"

	pb "github.com/0xef53/kvmrun/api/services/machines/v1"
	"github.com/0xef53/kvmrun/internal/helpers"
	"github.com/0xef53/kvmrun/kvmrun"

	empty "github.com/golang/protobuf/ptypes/empty"
	log "github.com/sirupsen/logrus"
)

func (s *ServiceServer) AttachCdrom(ctx context.Context, req *pb.AttachCdromRequest) (*empty.Empty, error) {
	cdrom, err := kvmrun.NewCdrom(req.DeviceName, req.DeviceMedia)
	if err != nil {
		return nil, err
	}

	cdrom.Driver = strings.ReplaceAll(strings.ToLower(req.Driver.String()), "_", "-")
	cdrom.ReadOnly = req.ReadOnly
	cdrom.Bootindex = uint(req.Bootindex)

	if len(req.ProxyCommand) > 0 {
		if v, err := helpers.ResolveExecutable(req.ProxyCommand); err == nil {
			req.ProxyCommand = v
		} else {
			return nil, err
		}
	}

	taskKey := req.Name + ":cdrom_" + cdrom.Name + ":"

	err = s.RunFuncTask(ctx, taskKey, func(l *log.Entry) error {
		vm, err := s.GetMachine(req.Name)
		if err != nil {
			return err
		}

		if req.Live && vm.R != nil {
			if req.Index >= 0 {
				return fmt.Errorf("unable to set cdrom index in relation to the running QEMU instance")
			}

			if vm.R.GetCdroms().Exists(req.DeviceName) {
				if len(req.ProxyCommand) > 0 {
					return fmt.Errorf("cdrom is already connected, unable to add a proxy for it")
				}
			} else {
				var success bool

				if len(req.ProxyCommand) > 0 {
					proxy := kvmrun.Proxy{
						Path:    req.DeviceMedia,
						Command: req.ProxyCommand,
						Envs:    req.ProxyEnvs,
					}

					defer func() {
						if !success {
							vm.R.RemoveProxy(proxy.Path)
							s.DeactivateDiskBackendProxy(req.Name, cdrom.Backend.BaseName())
						}
					}()

					if err := vm.R.AppendProxy(proxy); err != nil {
						return err
					}

					if err := s.ActivateDiskBackendProxy(req.Name, cdrom.Backend.BaseName()); err != nil {
						return err
					}
				}

				if err := vm.R.AppendCdrom(*cdrom); err != nil {
					return err
				}

				success = true
			}
		}

		addToConf := func() error {
			// In the config file we are able to add a proxy server
			// for an already connected cdrom
			if len(req.ProxyCommand) > 0 {
				proxy := kvmrun.Proxy{
					Path:    req.DeviceMedia,
					Command: req.ProxyCommand,
					Envs:    req.ProxyEnvs,
				}
				if err := vm.C.AppendProxy(proxy); err != nil {
					return err
				}
			}
			if req.Index >= 0 {
				return vm.C.InsertCdrom(*cdrom, int(req.Index))
			}
			return vm.C.AppendCdrom(*cdrom)
		}

		if err := addToConf(); err != nil && !kvmrun.IsAlreadyConnectedError(err) {
			return err
		}

		return vm.C.Save()
	})

	if err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}

func (s *ServiceServer) DetachCdrom(ctx context.Context, req *pb.DetachCdromRequest) (*empty.Empty, error) {
	taskKey := req.Name + ":cdrom_" + req.DeviceName + ":"

	err := s.RunFuncTask(ctx, taskKey, func(l *log.Entry) error {
		vm, err := s.GetMachine(req.Name)
		if err != nil {
			return err
		}

		if req.Live && vm.R != nil {
			if cdrom := vm.R.GetCdroms().Get(req.DeviceName); cdrom != nil {
				if err := vm.R.RemoveCdrom(req.DeviceName); err != nil {
					return err
				}
				if err := vm.R.RemoveProxy(cdrom.Media); err != nil && !kvmrun.IsNotConnectedError(err) {
					return err
				}
				if err := s.DeactivateDiskBackendProxy(req.Name, cdrom.Backend.BaseName()); err != nil {
					return err
				}
			}
		}

		if cdrom := vm.C.GetCdroms().Get(req.DeviceName); cdrom != nil {
			if err := vm.C.RemoveCdrom(req.DeviceName); err != nil {
				return err
			}
			if err := vm.C.RemoveProxy(cdrom.Media); err != nil && !kvmrun.IsNotConnectedError(err) {
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

func (s *ServiceServer) ChangeCdromMedia(ctx context.Context, req *pb.ChangeCdromMediaRequest) (*empty.Empty, error) {
	if len(req.ProxyCommand) > 0 {
		if v, err := helpers.ResolveExecutable(req.ProxyCommand); err == nil {
			req.ProxyCommand = v
		} else {
			return nil, err
		}
	}

	err := s.RunFuncTask(ctx, req.Name, func(l *log.Entry) error {
		vm, err := s.GetMachine(req.Name)
		if err != nil {
			return err
		}

		var savingRequire bool

		if req.Live && vm.R != nil {
			if cdrom := vm.R.GetCdroms().Get(req.DeviceName); cdrom != nil {
				if cdrom.Media != req.DeviceMedia {
					var success bool

					if len(req.ProxyCommand) > 0 {
						// Configure proxy for new media
						proxy := kvmrun.Proxy{
							Path:    req.DeviceMedia,
							Command: req.ProxyCommand,
							Envs:    req.ProxyEnvs,
						}

						be, err := kvmrun.NewDiskBackend(req.DeviceMedia)
						if err != nil {
							return err
						}

						defer func() {
							if !success {
								vm.R.RemoveProxy(req.DeviceMedia)
								s.DeactivateDiskBackendProxy(req.Name, be.BaseName())
							}
						}()

						if err := vm.R.AppendProxy(proxy); err != nil {
							return err
						}
						if err := s.ActivateDiskBackendProxy(req.Name, be.BaseName()); err != nil {
							return err
						}
					}

					oldMedia := cdrom.Media
					oldMediaBaseName := cdrom.Backend.BaseName()

					// Change media
					if err := vm.R.ChangeCdromMedia(req.DeviceName, req.DeviceMedia); err != nil {
						return err
					}

					success = true

					// Deconfigure old media proxy
					if err := vm.R.RemoveProxy(oldMedia); err != nil && !kvmrun.IsNotConnectedError(err) {
						return err
					}
					if err := s.DeactivateDiskBackendProxy(req.Name, oldMediaBaseName); err != nil {
						return err
					}
				}
			} else {
				return &kvmrun.NotConnectedError{"instance_qemu", req.DeviceName}
			}
		}

		if cdrom := vm.C.GetCdroms().Get(req.DeviceName); cdrom != nil {
			switch err := vm.C.ChangeCdromMedia(req.DeviceName, req.DeviceMedia); {
			case err == nil:
				savingRequire = true
			case kvmrun.IsAlreadyConnectedError(err):
			default:
				return err
			}

			proxy := kvmrun.Proxy{
				Path:    req.DeviceMedia,
				Command: req.ProxyCommand,
				Envs:    req.ProxyEnvs,
			}

			switch err := vm.C.AppendProxy(proxy); {
			case err == nil:
				savingRequire = true
			case kvmrun.IsAlreadyConnectedError(err):
			default:
				return err
			}
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
