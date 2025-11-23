//go:build darwin

package platform

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func is_admin() bool {
	return os.Geteuid() == 0
}

func need_admin_permission() bool {
	// args := os.Args[1:]
	// if len(args) == 0 {
	// 	return true
	// }
	// if strings.Contains(args[0], "--") {
	// 	return true
	// }
	return false
}

func request_admin_permission() bool {
	exe, err := os.Executable()
	if err != nil {
		return false
	}

	params := strings.Join(os.Args[1:], " ")
	
	// Escape backslashes and double quotes for the AppleScript string
	cmdStr := fmt.Sprintf("%s %s", exe, params)
	cmdStr = strings.ReplaceAll(cmdStr, "\\", "\\\\")
	cmdStr = strings.ReplaceAll(cmdStr, "\"", "\\\"")

	script := fmt.Sprintf("do shell script \"%s\" with administrator privileges", cmdStr)

	cmd := exec.Command("osascript", "-e", script)
	err = cmd.Run()
	return err == nil
}
