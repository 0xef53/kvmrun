package block

/*
#include <linux/fs.h>    // present only for BLKGETSIZE64
*/
import "C"

import (
	"os"
	"syscall"
	"unsafe"
)

const BLKGETSIZE64 = C.BLKGETSIZE64

// BlkGetSize64 returns the current size of a block device in bytes.
//
// If there is an error, it will be of type *os.SyscallError.
func GetSize64(devpath string) (uint64, error) {
	var size uint64

	f, err := os.Open(devpath)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	if _, _, err := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), BLKGETSIZE64, uintptr(unsafe.Pointer(&size))); err != 0 {
		return 0, os.NewSyscallError("ioctl: BLKGETSIZE64", err)
	}

	return size, nil
}
