//go:build !linux && !darwin && !freebsd

package app

import "errors"

type Lock struct{}

func AcquireLock(string) (*Lock, error) {
	return nil, errors.New("this build requires POSIX file locking; use the provided Linux Docker image")
}
func (*Lock) Close() error { return nil }
