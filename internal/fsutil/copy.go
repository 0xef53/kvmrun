package fsutil

import (
	"io"
	"os"
	"path/filepath"
)

// Copy copies srcname to dstname, doesn't matter if srcname is a directory or a file
func Copy(srcname, dstname string) error {
	info, err := os.Lstat(srcname)
	if err != nil {
		return err
	}

	return copy(srcname, dstname, info)
}

// copy dispatches copy-funcs according to the mode.
// Because this "copy" could be called recursively,
// "info" MUST be given here, NOT nil.
func copy(srcname, dstname string, info os.FileInfo) error {
	if info.Mode()&os.ModeSymlink != 0 {
		return lcopy(srcname, dstname, info)
	}

	if info.IsDir() {
		return dcopy(srcname, dstname, info)
	}

	return fcopy(srcname, dstname, info)
}

// fcopy is for just a file,
// with considering existence of parent directory
// and file permission.
func fcopy(srcname, dstname string, info os.FileInfo) error {
	if err := os.MkdirAll(filepath.Dir(dstname), os.ModePerm); err != nil {
		return err
	}

	dst, err := os.Create(dstname)
	if err != nil {
		return err
	}
	defer dst.Close()

	if err = os.Chmod(dst.Name(), info.Mode()); err != nil {
		return err
	}

	src, err := os.Open(srcname)
	if err != nil {
		return err
	}
	defer src.Close()

	_, err = io.Copy(dst, src)

	return err
}

// dcopy is for a directory,
// with scanning contents inside the directory
// and pass everything to "copy" recursively.
func dcopy(srcdir, dstdir string, info os.FileInfo) error {
	if err := os.MkdirAll(dstdir, info.Mode()); err != nil {
		return err
	}

	contents, err := os.ReadDir(srcdir)
	if err != nil {
		return err
	}

	for _, content := range contents {
		cs, cd := filepath.Join(srcdir, content.Name()), filepath.Join(dstdir, content.Name())

		info, err := content.Info()
		if err != nil {
			return err
		}

		if err := copy(cs, cd, info); err != nil {
			// If any error, exit immediately
			return err
		}
	}

	return nil
}

// lcopy is for a symlink,
// with just creating a new symlink by replicating src symlink.
func lcopy(srcname, dstname string, info os.FileInfo) error {
	srcname, err := os.Readlink(srcname)
	if err != nil {
		return err
	}

	return os.Symlink(srcname, dstname)
}
