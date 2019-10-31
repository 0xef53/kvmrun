package runsv

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/0xef53/go-fsnotify"
)

var (
	REPOSITORY = "/etc/sv"
	SERVICEDIR = "/etc/service"
	SVDATADIR  = "/var/lib/supervise"

	ErrTimedOut        = errors.New("Timeout error")
	ErrStateChanged    = errors.New("State was changed")
	ErrRunsvNotRunning = errors.New("Runsv is not running")
)

func Enable(name string, start bool) error {
	maindir := filepath.Join(REPOSITORY, name)

	symlinks := map[string]string{
		maindir:                        filepath.Join(SERVICEDIR, name),
		filepath.Join(SVDATADIR, name): filepath.Join(maindir, "supervise"),
		filepath.Join(SVDATADIR, fmt.Sprintf("%s.log", name)): filepath.Join(maindir, "log/supervise"),
	}

	for src, dst := range symlinks {
		if err := os.Symlink(src, dst); err != nil {
			return err
		}
	}

	if start {
		if err := os.Remove(filepath.Join(maindir, "down")); err != nil && !os.IsNotExist(err) {
			return err
		}
	}

	return nil
}

func EnableWaitPid(name string, start bool, t uint) error {
	if err := Enable(name, start); err != nil {
		return err
	}

	statfile := filepath.Join(SERVICEDIR, name, "supervise/stat")
	done := make(chan struct{})
	finish := make(chan struct{})
	go func() {
		for {
			select {
			case <-finish:
				return
			default:
			}
			if _, err := os.Stat(statfile); err == nil {
				break
			}
			time.Sleep(time.Second * 1)
		}
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-time.After(time.Second * time.Duration(t)):
		close(finish)
		return ErrTimedOut
	}

	return nil
}

func IsEnabled(name string) bool {
	serviceDir := filepath.Join(SERVICEDIR, name, "supervise")
	if _, err := os.Stat(serviceDir); err != nil {
		return false
	}

	return true
}

func CheckState(name string, t uint) error {
	pidfile := filepath.Join(SERVICEDIR, name, "supervise/pid")

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()

	if err := watcher.Add(filepath.Dir(pidfile)); err != nil {
		return err
	}

	timer := time.NewTimer(time.Second * time.Duration(t))
	defer timer.Stop()

	for {
		select {
		case event := <-watcher.Events:
			if event.Op&fsnotify.Create == fsnotify.Create && event.Name == pidfile {
				return ErrStateChanged
			}
		case err := <-watcher.Errors:
			return err
		case <-timer.C:
			return nil
		}
	}

	return nil
}

func GetPid(name string) (int, error) {
	c, err := ioutil.ReadFile(filepath.Join(SERVICEDIR, name, "supervise/pid"))
	if err != nil {
		return 0, err
	}

	pid, err := strconv.Atoi(string(c))
	if err != nil {
		return 0, err
	}

	return pid, nil
}

func GetState(name string) (string, error) {
	c, err := ioutil.ReadFile(filepath.Join(SERVICEDIR, name, "supervise/stat"))
	if err != nil {
		return "", err
	}

	state := strings.Split(string(c), ", ")

	return strings.TrimSpace(state[0]), nil
}

func GetWantState(name string) (string, error) {
	c, err := ioutil.ReadFile(filepath.Join(SERVICEDIR, name, "supervise/stat"))
	if err != nil {
		return "", err
	}

	state := strings.Split(string(c), ", ")

	if len(state) != 2 {
		return strings.TrimSpace(state[0]), nil
	}

	return strings.TrimSpace(strings.Replace(state[1], " ", "_", -1)), nil
}

func SendSignal(name, s string) error {
	pipe := filepath.Join(SERVICEDIR, name, "supervise/control")

	f, err := os.OpenFile(pipe, os.O_WRONLY|syscall.O_NONBLOCK, os.ModeNamedPipe)
	if err != nil {
		if _err, ok := err.(*os.PathError); ok {
			if _err.Err == syscall.ENXIO {
				return ErrRunsvNotRunning
			}
		}
		return err
	}
	defer f.Close()

	if _, err := fmt.Fprintf(f, s); err != nil {
		return err
	}

	return nil
}
