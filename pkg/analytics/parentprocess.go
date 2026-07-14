package analytics

import (
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// getParentProcessInfo returns the name and full command line of the parent process.
// Times out after 100ms to avoid blocking the CLI.
func getParentProcessInfo() (name, cmdline string) {
	type result struct {
		name, cmdline string
	}
	ch := make(chan result, 1)
	go func() {
		n, c := getParentProcessInfoSync()
		ch <- result{name: n, cmdline: c}
	}()
	select {
	case r := <-ch:
		return r.name, r.cmdline
	case <-time.After(100 * time.Millisecond):
		return "", ""
	}
}

func getParentProcessInfoSync() (name, cmdline string) {
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
