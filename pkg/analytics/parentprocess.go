package analytics

import (
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

// getParentProcessInfo returns the name and full command line of the parent process.
func getParentProcessInfo() (name, cmdline string) {
	ppid := os.Getppid()
	if ppid <= 0 {
		return "", ""
	}

	switch runtime.GOOS {
	case "linux":
		return getParentProcessLinux(ppid)
	case "darwin":
		return getParentProcessDarwin(ppid)
	default:
		return "", ""
	}
}

func getParentProcessLinux(ppid int) (name, cmdline string) {
	pidStr := strconv.Itoa(ppid)

	commBytes, err := os.ReadFile("/proc/" + pidStr + "/comm")
	if err == nil {
		name = strings.TrimSpace(string(commBytes))
	}

	cmdlineBytes, err := os.ReadFile("/proc/" + pidStr + "/cmdline")
	if err == nil {
		// /proc cmdline uses null bytes as separators
		cmdline = strings.ReplaceAll(string(cmdlineBytes), "\x00", " ")
		cmdline = strings.TrimSpace(cmdline)
	}

	return name, cmdline
}

func getParentProcessDarwin(ppid int) (name, cmdline string) {
	pidStr := strconv.Itoa(ppid)

	out, err := exec.Command("ps", "-p", pidStr, "-o", "comm=").Output() // #nosec G204
	if err == nil {
		name = strings.TrimSpace(string(out))
	}

	out, err = exec.Command("ps", "-p", pidStr, "-o", "args=").Output() // #nosec G204
	if err == nil {
		cmdline = strings.TrimSpace(string(out))
	}

	return name, cmdline
}
