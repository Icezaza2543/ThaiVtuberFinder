//go:build linux || darwin || freebsd

package app

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

type Lock struct{ f *os.File }

func AcquireLock(path string) (*Lock, error) {
	if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		return nil, e
	}
	f, e := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		return nil, e
	}
	if e = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); e != nil {
		f.Close()
		return nil, errors.New("another finder process owns this database; stop it before running this command")
	}
	return &Lock{f}, nil
}
func (l *Lock) Close() error {
	if l.f == nil {
		return nil
	}
	_ = syscall.Flock(int(l.f.Fd()), syscall.LOCK_UN)
	e := l.f.Close()
	l.f = nil
	return e
}
