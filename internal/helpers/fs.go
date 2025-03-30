package helpers

import (
	"fmt"
	"os"
	"path/filepath"
)

func ResolveExecutable(fname string) (string, error) {
	st, err := os.Stat(fname)
	if err != nil {
		return "", err
	}

	if !st.Mode().IsRegular() {
		return "", fmt.Errorf("not a file: %s", fname)
	}

	if st.Mode()&0100 == 0 {
		return "", fmt.Errorf("not executable by root: %s", fname)
	}

	return filepath.Abs(fname)
}

func LookForFile(basename string, dirs ...string) (string, string, error) {
	for _, d := range dirs {
		fullname := filepath.Join(d, basename)

		switch fi, err := os.Stat(fullname); {
		case err == nil:
			if fi.IsDir() {
				return "", "", fmt.Errorf("not a file: %s", fullname)
			}
			return d, filepath.Clean(fullname), nil
		case os.IsNotExist(err):
			continue
		default:
			return "", "", err
		}
	}

	return "", "", &os.PathError{"stat", basename, os.ErrNotExist}
}
