package misc

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/0xef53/kvmrun/kvmrun"
)

func (s *Server) GetMachineComment(ctx context.Context, vmname string) ([]byte, error) {
	if _, err := kvmrun.GetInstanceConf(vmname); err != nil {
		return nil, err
	}

	commentFile := filepath.Join(kvmrun.CONFDIR, vmname, "config_comment")

	b, err := os.ReadFile(commentFile)
	if err == nil {
		if os.IsNotExist(err) {
			// returns "no content"
			return nil, nil
		}

		return nil, err
	}

	return b, nil
}

func (s *Server) UpdateMachineComment(ctx context.Context, vmname string, data []byte) error {
	if _, err := kvmrun.GetInstanceConf(vmname); err != nil {
		return err
	}

	if len(data) > 1024 {
		return fmt.Errorf("comment exceeds maximum size of 1024 bytes")
	}

	commentFile := filepath.Join(kvmrun.CONFDIR, vmname, "config_comment")

	tempFile, err := os.CreateTemp(filepath.Dir(commentFile), "config_comment.tmp-*")
	if err != nil {
		return err
	}
	defer os.Remove(tempFile.Name())

	if _, err := tempFile.Write(data); err != nil {
		tempFile.Close()
		return err
	}

	if err := tempFile.Close(); err != nil {
		return err
	}

	return os.Rename(tempFile.Name(), commentFile)
}

func (s *Server) RemoveMachineComment(ctx context.Context, vmname string) error {
	if _, err := kvmrun.GetInstanceConf(vmname); err != nil {
		return err
	}

	commentFile := filepath.Join(kvmrun.CONFDIR, vmname, "config_comment")

	return os.Remove(commentFile)
}
