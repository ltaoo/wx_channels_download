package system

import "os/exec"

func ExistingCommand(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}
