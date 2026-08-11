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

	command_parts := make([]string, 0, len(os.Args))
	command_parts = append(command_parts, shell_quote(exe))
	for _, arg := range os.Args[1:] {
		command_parts = append(command_parts, shell_quote(arg))
	}
	command_text := strings.Join(command_parts, " ")
	escaped_command := strings.ReplaceAll(command_text, "\\", "\\\\")
	escaped_command = strings.ReplaceAll(escaped_command, "\"", "\\\"")
	script := fmt.Sprintf("do shell script \"%s\" with administrator privileges", escaped_command)

	cmd := exec.Command("osascript", "-e", script)
	err = cmd.Run()
	return err == nil
}

func shell_quote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func request_admin_permission_and_wait() (started bool, exited bool) {
	ok := request_admin_permission()
	return ok, ok
}
