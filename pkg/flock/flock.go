package flock

import (
	"errors"
	"os"
	"syscall"
	"time"
)

var (
	ErrAcquireLock = errors.New("Could not acquire lock")
)

// FileLocker is the structure that wraps exclusive file locking functionality.
type FileLocker struct {
	f *os.File
}

// NewLocker creates new locker instance for a file path.
func NewLocker(filepath string) (*FileLocker, error) {
	f, err := os.Create(filepath)
	if err != nil {
		return nil, err
	}

	return &FileLocker{f}, nil
}

// Acquire tries to lock the file for writing (exclusive lock) using flock(2).
// If the function cannot obtain a lock during the timeout it will return an error.
func (l *FileLocker) Acquire(timeout time.Duration) error {
	success := make(chan struct{})
	timedOut := make(chan struct{})

	go func() {
		for {
			select {
			case <-timedOut:
				return
			default:
			}
			if err := syscall.Flock(int(l.f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
				time.Sleep(time.Second * 1)
				continue
			}
			break
		}
		close(success)
	}()

	select {
	case <-success:
		return nil
	case <-time.After(timeout):
		close(timedOut)
		return ErrAcquireLock
	}

	return nil
}

// Release releases the lock on the file.
func (l *FileLocker) Release() {
	l.f.Close()
}
