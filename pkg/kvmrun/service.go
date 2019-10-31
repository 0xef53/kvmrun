package kvmrun

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
)

func CreateService(name string) error {
	if err := os.MkdirAll(VMCONFDIR, 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(LOGSDIR, 0755); err != nil {
		return err
	}

	maindir := filepath.Join(VMCONFDIR, name)

	if _, err := os.Stat(filepath.Join(maindir, "config")); err == nil {
		return fmt.Errorf("Already exists: %s", maindir)
	}

	dirs := []string{
		maindir,
		filepath.Join(maindir, "control"),
		filepath.Join(maindir, "log"),
		filepath.Join(LOGSDIR, name),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	symlinks := map[string]string{
		filepath.Join(maindir, "log/main"):  filepath.Join(LOGSDIR, name),
		filepath.Join(maindir, "run"):       "/usr/lib/kvmrun/launcher",
		filepath.Join(maindir, "finish"):    "/usr/lib/kvmrun/finisher",
		filepath.Join(maindir, "log/run"):   "/usr/lib/kvmrun/svlog_run",
		filepath.Join(maindir, "control/x"): "/usr/lib/kvmrun/control",
		filepath.Join(maindir, "control/d"): "/usr/lib/kvmrun/control",
		filepath.Join(maindir, "control/t"): "/usr/lib/kvmrun/control",
		filepath.Join(maindir, "control/p"): "/usr/lib/kvmrun/control",
		filepath.Join(maindir, "control/c"): "/usr/lib/kvmrun/control",
	}
	for dst, src := range symlinks {
		if err := os.Symlink(src, dst); err != nil && !os.IsExist(err) {
			return err
		}
	}
	if err := ioutil.WriteFile(filepath.Join(maindir, "down"), []byte(""), 0644); err != nil {
		return err
	}
	return nil
}

func RemoveService(name string) error {
	files := []string{
		filepath.Join("/etc/service", name),
		filepath.Join(VMCONFDIR, name),
		filepath.Join(SVDATADIR, name),
		filepath.Join(SVDATADIR, fmt.Sprintf("%s.log", name)),
		filepath.Join(CHROOTDIR, name),
		filepath.Join(LOGSDIR, name),
	}
	for _, f := range files {
		if err := os.RemoveAll(f); err != nil {
			return err
		}
	}
	return nil
}
