package utils

import (
	"errors"

	"golang.org/x/sys/unix"
)

func CheckIoUringSupport() bool {
	// Call io_uring_register with deliberate dummy parameters.
	_, _, errno := unix.Syscall6(
		unix.SYS_IO_URING_REGISTER,
		0, // fd (0 is safe to test)
		2, // opcode (using an arbitrary opcode or IORING_UNREGISTER_BUFFERS)
		0,
		0,
		0,
		0,
	)

	if errno == 0 {
		return true
	}

	// If the system call is completely missing or blocked.
	if errors.Is(errno, unix.ENOSYS) {
		return false
	}

	// If it returns a different error (like EBADF or EINVAL), it means the
	// kernel recognized the syscall but rejected our bad dummy parameters.
	// This confirms the kernel supports io_uring.
	return true
}
