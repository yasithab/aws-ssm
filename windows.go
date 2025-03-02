//go:build windows
// +build windows

package main

import (
    "log"
    "os"
    "os/exec"
)

// configureProcess is a no-op on Windows
func configureProcess(cmd *exec.Cmd) {
    // No process group setup needed on Windows
}

// terminateProcess terminates the command on Windows
func terminateProcess(cmd *exec.Cmd) {
    if cmd.Process != nil {
        cmd.Process.Kill()
    }
}

// executeSSM runs the AWS SSM command as a subprocess on Windows
func executeSSM(binary string, args, env []string, logger *log.Logger) error {
    cmd := exec.Command(binary, args...)
    cmd.Env = env
    cmd.Stdin = os.Stdin
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    return cmd.Run()
}
