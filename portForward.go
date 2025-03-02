package main

import (
    "context"
    "flag"
    "fmt"
    "log"
    "net"
    "os"
    "os/exec"
    "os/signal"
    "strconv"
    "strings"
    "time"

    "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

const (
    defaultPingInterval = 5 * time.Minute
    defaultPingTimeout  = 5 * time.Second
)

func portForwardCmd() {
    fs := flag.NewFlagSet("port-forward", flag.ExitOnError)
    verbose := fs.Bool("verbose", false, "Enable verbose logging")
    remote := fs.String("remote", "", "Remote host and port to forward (required, format: host:port)")
    localPort := fs.Int("local-port", 0, "Local port number (default: same as remote port)")
    tags := fs.String("tags", defaultTags, "Comma-separated tags for bastion instance (e.g., Role=bastion,Function=port-forward)")
    region := fs.String("region", "", "AWS region (default: AWS_DEFAULT_REGION env var)")
    pingInterval := fs.Duration("ping-interval", defaultPingInterval, "TCP ping interval to keep session alive (e.g., 5m, 300s)")

    fs.Usage = func() {
        fmt.Fprintf(os.Stderr, `
==============================================================
AWS SSM Port Forwarding
==============================================================

Establish a port forwarding session via AWS SSM to a remote host through a bastion instance.

Usage:
  %s port-forward -remote=<host>:<port> [options]

Required Flags:
  -remote       Remote host and port (e.g., example.com:3306)

Optional Flags:
`, os.Args[0])
        fs.PrintDefaults()
        fmt.Fprintf(os.Stderr, `
Examples:
  %s port-forward -remote=db.example.com:3306 -verbose
  %s port-forward -remote=app.internal:8080 -local-port=8081 -region=us-west-2 -ping-interval=2m

Notes:
  - Ensure AWS credentials are configured (via AWS_PROFILE or AWS_ACCESS_KEY_ID/AWS_SECRET_ACCESS_KEY).
  - The bastion instance must have the specified tags (default: Role=bastion,Function=port-forward).
`, os.Args[0], os.Args[0])
    }

    fs.Parse(os.Args[2:])

    if *remote == "" {
        fs.Usage()
        os.Exit(1)
    }

    logger := log.New(os.Stdout, "SSM: ", log.Ldate|log.Ltime|log.Lshortfile)
    verboseLogger := log.New(os.Stdout, "VERBOSE: ", log.Ldate|log.Ltime|log.Lshortfile)

    verboseLog := func(format string, v ...interface{}) {
        if *verbose {
            verboseLogger.Printf(format, v...)
        }
    }

    regionVal := *region
    if regionVal == "" {
        regionVal = os.Getenv("AWS_DEFAULT_REGION")
        if regionVal == "" {
            logger.Println("Error: AWS region required")
            os.Exit(1)
        }
    }

    cfg, err := loadAWSConfig(context.Background(), regionVal)
    if err != nil {
        logger.Printf("Configuration error: %v", err)
        os.Exit(1)
    }

    host, remotePort, err := parseRemote(*remote)
    if err != nil {
        logger.Printf("Invalid remote format: %v", err)
        os.Exit(1)
    }

    lPort := *localPort
    if lPort == 0 {
        lPort, err = strconv.Atoi(remotePort)
        if err != nil {
            logger.Printf("Invalid port number: %v", err)
            os.Exit(1)
        }
    }

    if !isPortAvailable(lPort) {
        logger.Printf("Error: Local port %d is already in use. Please stop the process using it or choose a different port with -local-port.", lPort)
        os.Exit(1)
    }

    ec2Client := ec2NewFromConfig(cfg)
    expectedTags := map[string]string{
        "Role":     "bastion",
        "Function": "port-forward",
    }
    filters := append([]types.Filter{
        {Name: awsString("instance-state-name"), Values: []string{"running"}},
    }, parseTags(*tags)...)

    if *verbose {
        verboseLogger.Printf("Config: Remote=%s:%s, LocalPort=%d, Region=%s, Tags=%v, PingInterval=%v",
            host, remotePort, lPort, regionVal, expectedTags, *pingInterval)
    }

    instanceID, err := findBastionInstance(ec2Client, filters, expectedTags, *verbose, verboseLogger)
    if err != nil {
        logger.Printf("Error finding bastion: %v", err)
        os.Exit(1)
    }

    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, os.Interrupt)
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    var cmd *exec.Cmd
    go func() {
        <-sigChan
        verboseLog("Received interrupt signal, terminating session...")
        cancel()
        if cmd != nil && cmd.Process != nil {
            terminateProcess(cmd) // Platform-specific termination
        }
    }()

    cmd, err = startPortForwardingSession(ctx, instanceID, host, remotePort, lPort, regionVal, *pingInterval, verboseLog)
    if err != nil {
        logger.Printf("Port forwarding failed: %v", err)
        os.Exit(1)
    }

    logger.Println("Port forwarding completed successfully")
}

// Shared logic for starting the port forwarding session
func startPortForwardingSession(ctx context.Context, instanceID, host, remotePort string, localPort int, region string, pingInterval time.Duration, verboseLog func(string, ...interface{})) (*exec.Cmd, error) {
    cmd := exec.CommandContext(ctx, "aws", "ssm", "start-session",
        "--target", instanceID,
        "--document-name", "AWS-StartPortForwardingSessionToRemoteHost",
        "--parameters", fmt.Sprintf(`{"host":["%s"],"portNumber":["%s"],"localPortNumber":["%d"]}`, host, remotePort, localPort),
        "--region", region,
    )

    cmd.Stdin = os.Stdin
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr

    configureProcess(cmd) // Platform-specific configuration

    verboseLog("Starting port forwarding session: %s", strings.Join(cmd.Args, " "))

    if err := cmd.Start(); err != nil {
        return nil, fmt.Errorf("failed to start port forwarding: %w", err)
    }

    go func() {
        time.Sleep(5 * time.Second)
        ticker := time.NewTicker(pingInterval)
        defer ticker.Stop()

        verboseLog("Started TCP ping goroutine with interval %v", pingInterval)
        for {
            select {
            case <-ctx.Done():
                verboseLog("TCP ping goroutine stopped")
                return
            case <-ticker.C:
                remoteAddr := fmt.Sprintf("localhost:%d", localPort)
                conn, err := net.DialTimeout("tcp", remoteAddr, defaultPingTimeout)
                if err != nil {
                    verboseLog("TCP ping to %s failed: %v", remoteAddr, err)
                } else {
                    conn.Close()
                    verboseLog("TCP ping to %s successful", remoteAddr)
                }
            }
        }
    }()

    errChan := make(chan error, 1)
    go func() {
        errChan <- cmd.Wait()
    }()

    select {
    case err := <-errChan:
        if err != nil {
            if exitErr, ok := err.(*exec.ExitError); ok {
                return nil, fmt.Errorf("port forwarding session failed with exit code %d: %s", exitErr.ExitCode(), string(exitErr.Stderr))
            }
            return nil, fmt.Errorf("port forwarding session failed: %w", err)
        }
        verboseLog("Port forwarding session completed successfully")
        return cmd, nil
    case <-ctx.Done():
        if cmd.Process != nil {
            terminateProcess(cmd) // Platform-specific termination
        }
        return nil, fmt.Errorf("port forwarding session terminated by signal")
    }
}
