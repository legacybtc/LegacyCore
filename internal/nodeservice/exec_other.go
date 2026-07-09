//go:build !windows

package nodeservice

import "os/exec"

func runCommandOutput(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).Output()
}
