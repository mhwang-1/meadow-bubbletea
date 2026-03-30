package store

import (
	"os"
	"path/filepath"
	"syscall"
)

// WithLock acquires an exclusive file lock at path + ".lock", calls fn, then
// releases the lock. Parent directories are created if they do not exist.
func WithLock(path string, fn func() error) error {
	lockPath := path + ".lock"

	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		return err
	}

	f, err := os.Create(lockPath)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}

	return fn()
}
