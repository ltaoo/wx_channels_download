//go:build windows

package system

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

func signals() []os.Signal {
	return []os.Signal{os.Interrupt}
}

func set_sys_proc_attr_for_daemon() *syscall.SysProcAttr {
	return nil
}

func terminate_process(pid int) error {
	p, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return p.Kill()
}

func is_process_running(pid int) bool {
	if pid <= 0 {
		return false
	}
	cmd := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/FO", "CSV")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return false
	}
	// CSV output has header line and optionally a data line
	// If only header exists, the process is not found
	lines := bytes.Split(out.Bytes(), []byte("\n"))
	for _, line := range lines {
		trim := bytes.TrimSpace(line)
		if len(trim) == 0 {
			continue
		}
		// Skip header line starting with "Image Name"
		if bytes.HasPrefix(trim, []byte("\"Image Name\"")) {
			continue
		}
		// Any non-header non-empty line indicates a match
		return true
	}
	return false
}
