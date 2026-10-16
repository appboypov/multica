package processtree

import (
	"errors"
	"syscall"

	"golang.org/x/sys/unix"
)

// Darwin's SZOMB and P_WEXIT from sys/proc.h are not exported by x/sys/unix.
const (
	darwinZombieState = 5
	darwinExitingFlag = 0x00002000
)

func processGroupAlive(pid int) (bool, error) {
	err := syscall.Kill(-pid, 0)
	if errors.Is(err, syscall.ESRCH) {
		return false, nil
	}
	if err != nil && !errors.Is(err, syscall.EPERM) {
		return false, err
	}

	// Darwin keeps unreaped members in the group but excludes them from
	// signal delivery. A zombie-only group returns EPERM, not ESRCH.
	members, probeErr := unix.SysctlKinfoProcSlice("kern.proc.pgrp", pid)
	if probeErr != nil {
		return false, errors.Join(err, probeErr)
	}
	for _, member := range members {
		if member.Proc.P_stat != darwinZombieState {
			return true, nil
		}
	}
	return false, nil
}

func signalProcessGroup(pid int, signal syscall.Signal) error {
	err := syscall.Kill(-pid, signal)
	if err == nil || errors.Is(err, syscall.ESRCH) {
		return nil
	}
	if !errors.Is(err, syscall.EPERM) {
		return err
	}
	members, probeErr := unix.SysctlKinfoProcSlice("kern.proc.pgrp", pid)
	if probeErr != nil {
		return errors.Join(err, probeErr)
	}
	for _, member := range members {
		if member.Proc.P_stat != darwinZombieState && member.Proc.P_flag&darwinExitingFlag == 0 {
			return err
		}
	}
	// Exiting members cannot receive signals. finish still waits for them
	// to become zombies or disappear before releasing repository ownership.
	return nil
}
