//go:build !windows && !darwin

package processtree

import (
	"errors"
	"syscall"
)

func processGroupAlive(pid int) (bool, error) {
	err := syscall.Kill(-pid, 0)
	if errors.Is(err, syscall.ESRCH) {
		return false, nil
	}
	return err == nil, err
}

func signalProcessGroup(pid int, signal syscall.Signal) error {
	err := syscall.Kill(-pid, signal)
	if errors.Is(err, syscall.ESRCH) {
		return nil
	}
	return err
}
