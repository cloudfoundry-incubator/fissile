// Copied from https://code.google.com/p/gopass/

// +build darwin freebsd linux netbsd openbsd

package termpassword

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/SUSE/termui/sigint"
)

const (
	sttyArg0  = "/bin/stty"
	execCwdir = ""
)

var (
	// sttyArgvEOff tells the terminal to turn echo off.
	sttyArgvEOff = []string{"stty", "-echo"}
	// sttyArgvEOn tells the terminal to turn echo on.
	sttyArgvEOn = []string{"stty", "echo"}
	// The wait status of the system call
	ws syscall.WaitStatus
)

// PromptForPassword displays a prompt and reads a password without showing input.
func (pr passwordReader) PromptForPassword(promptText string, args ...interface{}) (passwd string) {

	// Display the prompt.
	fmt.Printf(promptText+": ", args...)

	// File descriptors for stdin, stdout, and stderr.
	fd := []uintptr{os.Stdin.Fd(), os.Stdout.Fd(), os.Stderr.Fd()}

	sigint.DefaultHandler.Add(func() { echoOn(fd) })

	pid, err := echoOff(fd)
	defer echoOn(fd)
	if err != nil {
		return
	}

	passwd = readPassword(pid)

	fmt.Println("") // Carriage return after the user input.

	return
}

func readPassword(pid int) string {
	rd := bufio.NewReader(os.Stdin)
	syscall.Wait4(pid, &ws, 0, nil)

	line, err := rd.ReadString('\n')
	if err == nil {
		return strings.TrimSpace(line)
	}
	return ""
}

func echoOff(fd []uintptr) (int, error) {
	pid, err := syscall.ForkExec(sttyArg0, sttyArgvEOff, &syscall.ProcAttr{Dir: execCwdir, Files: fd})
	if err != nil {
		return 0, fmt.Errorf("failed turning off console echo for password entry:\n%s", err)
	}

	return pid, nil
}

func echoOn(fd []uintptr) {
	pid, e := syscall.ForkExec(sttyArg0, sttyArgvEOn, &syscall.ProcAttr{Dir: execCwdir, Files: fd})
	if e == nil {
		syscall.Wait4(pid, &ws, 0, nil)
	}
}
