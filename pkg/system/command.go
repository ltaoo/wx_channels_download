package system

import "os/exec"

func RunCommand(cmd string, args ...string) error {
	command := exec.Command(cmd, args...)
	return command.Run()
}
