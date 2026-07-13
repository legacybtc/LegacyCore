package fsutil

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
)

// WriteFileAtomic writes data to path with the provided permissions using a
// temp file, fsync, close, rename, and parent-directory fsync. This prevents
// partially written JSON/index files from becoming the canonical state after a
// crash or power loss.
func WriteFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	committed := false
	defer func() {
		if !committed {
			_ = os.Remove(tmpName)
		}
	}()
	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	committed = true
	return syncDir(dir)
}

func syncDir(dir string) error {
	d, err := os.Open(dir) // #nosec
	if err != nil {
		return err
	}
	defer d.Close()
	err = d.Sync()
	if err == nil {
		return nil
	}
	if runtime.GOOS == "windows" {
		// Directory fsync semantics are inconsistent on Windows filesystems.
		// File data has already been fsynced before rename, so treat this as best-effort.
		if errors.Is(err, syscall.EACCES) || errors.Is(err, syscall.EPERM) || errors.Is(err, syscall.ENOTSUP) || errors.Is(err, syscall.EINVAL) {
			return nil
		}
		return nil
	}
	return err
}
