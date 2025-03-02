//go:build !windows
// +build !windows

package main

import (
    "log"
    "os/exec"
    "syscall"
)

// configureProcess sets up the command for Unix systems
func configureProcess(cmd *exec.Cmd) {
    cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// terminateProcess terminates the command on Unix systems
func terminateProcess(cmd *exec.Cmd) {
    if cmd.Process != nil {
        syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
    }
}

// executeSSM replaces the current process with the AWS SSM command on Unix
func executeSSM(binary string, args, env []string, logger *log.Logger) error {
    return syscall.Exec(binary, append([]string{"aws"}, args...), env)
}
