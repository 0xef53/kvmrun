package osuser

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/0xef53/kvmrun/internal/flock"
)

const (
	MIN_UID = 1024
	MAX_UID = 65534
)

var (
	passwdFile   = "/etc/passwd"
	lockfileName = "/var/lock/passwd.lock"

	ErrEmptyPasswd     = errors.New("empty passwd file")
	ErrNoAvailableUids = errors.New("no available UIDs in passwd file")
)

func contains(uids []int, x int) bool {
	for _, i := range uids {
		if i == x {
			return true
		}
	}
	return false
}

func getVacantUid() (int, error) {
	uids := []int{MIN_UID - 1}

	f, err := os.Open(passwdFile)
	if err != nil {
		return -1, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		// root:x:0:0:root:/root:/bin/bash
		strUid := strings.Split(scanner.Text(), ":")[2]
		uid, err := strconv.Atoi(strUid)
		if err != nil {
			return -1, err
		}
		if uid >= MIN_UID && !contains(uids, uid) {
			uids = append(uids, uid)
		}
	}
	if err := scanner.Err(); err != nil {
		return -1, err
	}

	sort.Ints(uids)

	switch len(uids) {
	case 0:
		return -1, ErrEmptyPasswd
	case 1:
		return uids[0] + 1, nil
	}

	for i := 0; i < len(uids)-1; i++ {
		if (uids[i] + 1) > MAX_UID {
			return -1, ErrNoAvailableUids
		}
		if uids[i]+1 != uids[i+1] {
			return uids[i] + 1, nil
		}
	}

	if uids[len(uids)-1] >= MAX_UID {
		return -1, ErrNoAvailableUids
	}

	return (uids[len(uids)-1] + 1), nil
}

// CreateUser creates a new user with the first vacant UID.
func CreateUser(name string) (int, error) {
	lock, err := flock.NewLocker(lockfileName)
	if err != nil {
		return -1, err
	}
	defer lock.Release()

	if err := lock.Acquire(time.Second * 10); err != nil {
		return -1, err
	}

	uid, err := getVacantUid()
	if err != nil {
		return -1, err
	}

	out, err := exec.Command(
		"/usr/sbin/useradd",
		"--shell=/bin/false",
		fmt.Sprintf("--uid=%d", uid),
		"--gid=nogroup",
		name,
	).CombinedOutput()
	if err != nil {
		return -1, fmt.Errorf("%s: %s", err, out)
	}

	return uid, nil
}

// RemoveUser deletes an existing user.
func RemoveUser(name string) error {
	out, err := exec.Command(
		"/usr/sbin/userdel",
		"--force",
		name,
	).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", err, out)
	}

	return nil
}
