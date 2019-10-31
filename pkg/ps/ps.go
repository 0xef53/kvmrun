package ps

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const SC_CLK_TCK int64 = 100 // C.sysconf(C._SC_CLK_TCK)

// GetCmdline returns the command line arguments of the process
// with the specified pid as a slice.
func GetCmdline(pid int) ([]string, error) {
	c, err := ioutil.ReadFile(filepath.Join("/proc", fmt.Sprintf("%d", pid), "cmdline"))
	if err != nil {
		return nil, err
	}

	ret := strings.FieldsFunc(string(c), func(r rune) bool {
		if r == '\u0000' {
			return true
		}
		return false
	})

	return ret, nil
}

// GetLifeTime returns the life time of the specified pid in seconds.
func GetLifeTime(pid int) (time.Duration, error) {
	sysinfo := syscall.Sysinfo_t{}
	if err := syscall.Sysinfo(&sysinfo); err != nil {
		return 0, err
	}

	c, err := ioutil.ReadFile(filepath.Join("/proc", fmt.Sprintf("%d", pid), "stat"))
	if err != nil {
		return 0, err
	}

	fields := strings.Fields(string(c))
	ticks, err := strconv.ParseInt(fields[21], 10, 64)
	if err != nil {
		return 0, err
	}

	t := int64(sysinfo.Uptime) - ticks/SC_CLK_TCK

	return time.Duration(t) * time.Second, err
}

// GetCreatedTime returns the creation time of the specified pid in seconds.
func GetCreateTime(pid int) (*time.Time, error) {
	lifeTime, err := GetLifeTime(pid)
	if err != nil {
		return nil, err
	}

	createTime := time.Now().Add(-lifeTime)

	return &createTime, nil
}
