//go:build !windows

package processtree

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"
)

const processTreeFinishTimeout = 5 * time.Second

type controller struct{}

func newController(cmd *exec.Cmd) (*controller, error) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
	return &controller{}, nil
}

func (*controller) attach(_ *exec.Cmd) error { return nil }

func (*controller) interrupt(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return os.ErrProcessDone
	}
	return signalProcessGroup(cmd.Process.Pid, syscall.SIGTERM)
}

func (*controller) stop(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return os.ErrProcessDone
	}
	return signalProcessGroup(cmd.Process.Pid, syscall.SIGKILL)
}

func (c *controller) finish(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	pid := cmd.Process.Pid
	alive, err := processGroupAlive(pid)
	if err != nil || !alive {
		return err
	}
	// A normally-exited leader can still leave a descendant holding inherited
	// pipes or repository locks. Kill the remaining group before returning
	// ownership of the repository to another operation.
	if err := c.stop(cmd); err != nil {
		return err
	}
	deadline := time.Now().Add(processTreeFinishTimeout)
	for time.Now().Before(deadline) {
		alive, err := processGroupAlive(pid)
		if err != nil || !alive {
			return err
		}
		time.Sleep(10 * time.Millisecond)
	}
	return fmt.Errorf("process group %d still active after %s", pid, processTreeFinishTimeout)
}

func (*controller) close() {}
