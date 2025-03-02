package main

import (
    "flag"
    "fmt"
    "log"
    "os"
    "os/exec"
    "strings"

    "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

func ssmConnectCmd() {
    fs := flag.NewFlagSet("connect", flag.ExitOnError)
    verbose := fs.Bool("verbose", false, "Enable verbose logging")
    target := fs.String("target", "", "Target instance ID (e.g., i-1234567890abcdef0)")
    tags := fs.String("tags", "", "Comma-separated tags to find instance (e.g., Role=bastion,Function=port-forward)")
    region := fs.String("region", "", "AWS region (default: AWS_DEFAULT_REGION env var)")

    fs.Usage = func() {
        fmt.Fprintf(os.Stderr, `
==============================================================
AWS SSM Session Connect
==============================================================

Start an interactive SSM session to a target EC2 instance by instance ID or tags.

Usage:
  %s connect [options]

Flags (at least one of -target or -tags is required):
  -target       Target instance ID (e.g., i-1234567890abcdef0)
  -tags         Comma-separated tags to find instance (e.g., Role=bastion,Function=port-forward)

Optional Flags:
`, os.Args[0])
        fs.PrintDefaults()
        fmt.Fprintf(os.Stderr, `
Examples:
  %s connect -target=i-1234567890abcdef0 -verbose
  %s connect -tags=Role=bastion,Function=port-forward -region=us-west-2 -verbose

Notes:
  - Ensure AWS credentials are configured (via AWS_PROFILE or AWS_ACCESS_KEY_ID/AWS_SECRET_ACCESS_KEY).
  - If both -target and -tags are provided, -target takes precedence.
`, os.Args[0], os.Args[0])
    }

    fs.Parse(os.Args[2:])

    if *target == "" && *tags == "" {
        fs.Usage()
        fmt.Fprintf(os.Stderr, "Error: At least one of -target or -tags must be provided\n")
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

    var instanceID string
    if *target != "" {
        instanceID = *target
    } else {
        cfg, err := loadAWSConfig(nil, regionVal)
        if err != nil {
            logger.Printf("Configuration error: %v", err)
            os.Exit(1)
        }
        ec2Client := ec2NewFromConfig(cfg)
        expectedTags := map[string]string{}
        for _, pair := range strings.Split(*tags, ",") {
            kv := strings.SplitN(pair, "=", 2)
            if len(kv) == 2 {
                expectedTags[kv[0]] = kv[1]
            }
        }
        filters := append([]types.Filter{
            {Name: awsString("instance-state-name"), Values: []string{"running"}},
        }, parseTags(*tags)...)

        instanceID, err = findBastionInstance(ec2Client, filters, expectedTags, *verbose, verboseLogger)
        if err != nil {
            logger.Printf("Error finding instance by tags: %v", err)
            os.Exit(1)
        }
    }

    args := []string{"ssm", "start-session", "--target", instanceID, "--region", regionVal}
    verboseLog("Starting SSM session: aws %s", strings.Join(args, " "))

    binary, err := exec.LookPath("aws")
    if err != nil {
        logger.Printf("Failed to find aws CLI: %v", err)
        os.Exit(1)
    }

    // Delegate to platform-specific execution
    err = executeSSM(binary, args, os.Environ(), logger)
    if err != nil {
        logger.Printf("Failed to start SSM session: %v", err)
        os.Exit(1)
    }

    logger.Println("Session completed successfully")
}
