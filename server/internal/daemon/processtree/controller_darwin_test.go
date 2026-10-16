package processtree

import (
	"os/exec"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestFinishGroupWithUnreapedMember(t *testing.T) {
	for _, liveMember := range []bool{false, true} {
		name := "zombie only"
		if liveMember {
			name = "zombie and live descendant"
		}
		t.Run(name, func(t *testing.T) {
			leader := exec.Command("/bin/sleep", "60")
			controller, err := newController(leader)
			if err != nil {
				t.Fatal(err)
			}
			defer controller.close()
			if err := leader.Start(); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				_ = leader.Process.Kill()
				_ = leader.Wait()
			})
			if err := controller.attach(leader); err != nil {
				t.Fatal(err)
			}

			// Keep an exited member unreaped after the leader's Wait, just as
			// a descendant's parent outside the group can do.
			zombie := exec.Command("/bin/sh", "-c", "exit 0")
			zombie.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pgid: leader.Process.Pid}
			if err := zombie.Start(); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				_ = zombie.Process.Kill()
				_ = zombie.Wait()
			})
			deadline := time.Now().Add(5 * time.Second)
			for {
				info, err := unix.SysctlKinfoProc("kern.proc.pid", zombie.Process.Pid)
				if err != nil {
					t.Fatal(err)
				}
				if info.Proc.P_stat == 5 { // SZOMB from Darwin's sys/proc.h.
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("member did not exit")
				}
				time.Sleep(time.Millisecond)
			}

			var live *exec.Cmd
			if liveMember {
				live = exec.Command("/bin/sleep", "60")
				live.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pgid: leader.Process.Pid}
				if err := live.Start(); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					_ = live.Process.Kill()
					_ = live.Wait()
				})
			}

			if err := leader.Process.Kill(); err != nil {
				t.Fatal(err)
			}
			_ = leader.Wait()
			if err := controller.finish(leader); err != nil {
				t.Fatalf("finish exited leader's group: %v", err)
			}
			if live != nil {
				info, err := unix.SysctlKinfoProc("kern.proc.pid", live.Process.Pid)
				if err != nil {
					t.Fatal(err)
				}
				if info.Proc.P_stat != 5 {
					t.Fatalf("live descendant survived finish: state %d", info.Proc.P_stat)
				}
			}
		})
	}
}
