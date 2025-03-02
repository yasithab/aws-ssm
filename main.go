package main

import (
    "fmt"
    "os"
)

func main() {
    if len(os.Args) < 2 {
        usage()
        os.Exit(1)
    }

    switch os.Args[1] {
    case "port-forward":
        portForwardCmd()
    case "connect":
        ssmConnectCmd()
    default:
        usage()
        os.Exit(1)
    }
}

func usage() {
    fmt.Fprintf(os.Stderr, `
==============================================================
AWS SSM Utility
==============================================================

A utility for interacting with AWS Systems Manager (SSM).

Usage:
  %s <command> [options]

Available Commands:
  port-forward    Establish a port forwarding session to a remote host
  connect         Start an SSM session to a target instance

Run '%s <command> -h' for more information on a command.

`, os.Args[0], os.Args[0])
}
