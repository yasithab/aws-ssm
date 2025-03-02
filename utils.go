package main

import (
    "context"
    "fmt"
    "log"
    "net"
    "os"
    "strings"
    "sync"

    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/credentials"
    "github.com/aws/aws-sdk-go-v2/service/ec2"
    ec2Types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

const (
    workerPoolSize      = 8
    defaultTags         = "Role=bastion,Function=port-forward"
)

func loadAWSConfig(ctx context.Context, region string) (aws.Config, error) {
    if ctx == nil {
        ctx = context.Background()
    }
    if profile := os.Getenv("AWS_PROFILE"); profile != "" {
        return config.LoadDefaultConfig(ctx,
            config.WithSharedConfigProfile(profile),
            config.WithRegion(region),
        )
    }

    accessKeyID := os.Getenv("AWS_ACCESS_KEY_ID")
    secretAccessKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
    if accessKeyID == "" || secretAccessKey == "" {
        return aws.Config{}, fmt.Errorf("AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY required when AWS_PROFILE is not set")
    }

    return config.LoadDefaultConfig(ctx,
        config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
            accessKeyID,
            secretAccessKey,
            os.Getenv("AWS_SESSION_TOKEN"),
        )),
        config.WithRegion(region),
    )
}

func parseRemote(remote string) (string, string, error) {
    parts := strings.SplitN(remote, ":", 2)
    if len(parts) != 2 {
        return "", "", fmt.Errorf("use host:port format")
    }
    return parts[0], parts[1], nil
}

func isPortAvailable(port int) bool {
    ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
    if err != nil {
        return false
    }
    ln.Close()
    return true
}

func parseTags(tags string) []ec2Types.Filter {
    var filters []ec2Types.Filter
    for _, pair := range strings.Split(tags, ",") {
        kv := strings.SplitN(pair, "=", 2)
        if len(kv) == 2 {
            filters = append(filters, ec2Types.Filter{
                Name:   awsString("tag:" + kv[0]),
                Values: []string{kv[1]},
            })
        }
    }
    return filters
}

func findBastionInstance(client *ec2.Client, filters []ec2Types.Filter, expectedTags map[string]string, verbose bool, logger *log.Logger) (string, error) {
    result, err := client.DescribeInstances(context.Background(), &ec2.DescribeInstancesInput{Filters: filters})
    if err != nil {
        return "", fmt.Errorf("failed to describe instances: %w", err)
    }

    sem := make(chan struct{}, workerPoolSize)
    var wg sync.WaitGroup
    done := make(chan struct{})
    var mu sync.Mutex
    var instanceID string

outer:
    for _, reservation := range result.Reservations {
        for _, instance := range reservation.Instances {
            select {
            case <-done:
                break outer
            default:
                wg.Add(1)
                sem <- struct{}{}
                go func(inst ec2Types.Instance) {
                    defer wg.Done()
                    defer func() { <-sem }()

                    if checkInstanceTags(inst, expectedTags, verbose, logger) {
                        mu.Lock()
                        if instanceID == "" {
                            instanceID = awsToString(inst.InstanceId)
                            printInstanceDetails(inst, verbose, logger)
                            close(done)
                        }
                        mu.Unlock()
                    }
                }(instance)
            }
        }
    }

    wg.Wait()
    if instanceID == "" {
        return "", fmt.Errorf("no instance found with tags: %v", expectedTags)
    }
    return instanceID, nil
}

func checkInstanceTags(instance ec2Types.Instance, expectedTags map[string]string, verbose bool, logger *log.Logger) bool {
    instanceTagMap := make(map[string]string)
    for _, tag := range instance.Tags {
        if tag.Key != nil && tag.Value != nil {
            instanceTagMap[awsToString(tag.Key)] = awsToString(tag.Value)
        }
    }

    for expectedKey, expectedValue := range expectedTags {
        if actualValue, exists := instanceTagMap[expectedKey]; !exists {
            if verbose {
                logger.Printf("Tag %s is missing", expectedKey)
            }
            return false
        } else if actualValue != expectedValue {
            if verbose {
                logger.Printf("Tag %s: expected %s, got %s", expectedKey, expectedValue, actualValue)
            }
            return false
        }
    }
    return true
}

func printInstanceDetails(instance ec2Types.Instance, verbose bool, logger *log.Logger) {
    if !verbose {
        return
    }
    logger.Printf("Instance ID: %s, State: %s, Private IP: %s, Public IP: %s",
        awsToString(instance.InstanceId),
        instance.State.Name,
        awsToString(instance.PrivateIpAddress),
        awsToString(instance.PublicIpAddress))
    logger.Println("Tags:")
    for _, tag := range instance.Tags {
        if tag.Key != nil && tag.Value != nil {
            logger.Printf("  %s: %s", awsToString(tag.Key), awsToString(tag.Value))
        }
    }
}

// Helper functions to handle AWS pointer dereferencing
func awsString(s string) *string {
    return &s
}

func awsToString(ptr *string) string {
    if ptr == nil {
        return ""
    }
    return *ptr
}

// Wrapper for ec2.NewFromConfig to satisfy staticcheck
func ec2NewFromConfig(cfg aws.Config) *ec2.Client {
    return ec2.NewFromConfig(cfg)
}
