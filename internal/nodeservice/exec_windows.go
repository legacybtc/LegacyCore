//go:build windows

package nodeservice

import (
	"os/exec"
	"syscall"
)

const createNoWindow = 0x08000000

func runCommandOutput(name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow,
	}
	return cmd.Output()
}
