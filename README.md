# AWS SSM Utility

The `AWS SSM Utility` is a Go-based command-line tool for interacting with AWS Systems Manager (SSM). It provides two main subcommands:

- **`connect`**: Starts an interactive SSM session to an EC2 instance, either by specifying an instance ID or finding an instance by tags.
- **`port-forward`**: Establishes a port forwarding session via SSM to a remote host through a bastion instance, with automatic TCP pinging to keep the session alive.

## Features
- **Port Forwarding**: Forward local ports to remote hosts through a bastion instance, with configurable TCP ping intervals to prevent session timeouts.
- **Interactive Sessions**: Connect to EC2 instances for interactive shell access, supporting both instance ID and tag-based lookups.
- **Tag-Based Instance Selection**: Efficiently finds the first running instance matching specified tags using a worker pool.
- **Verbose Logging**: Detailed logs for debugging and monitoring.

## Prerequisites
- **Go**: Version 1.13 or later (modules are used).
- **AWS CLI**: Installed and configured with credentials (via `AWS_PROFILE` or `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY`).
- **AWS Permissions**: IAM permissions for `ssm:StartSession` and `ec2:DescribeInstances`.

## Building the Tool
Follow these steps to build the `ssm` binary:

1. **Clone or Create the Project Directory**:
   ```bash
   mkdir ssm
   cd ssm
   ```

2. **Copy the Source Files**:
    - Ensure you have the following files in the ssm directory:
      - main.go
      - portForward.go
      - ssmConnect.go
      - utils.go
    - Use the latest versions from our previous discussions.

3. **Initialize Go Module**:
   ```bash
   go mod init ssm
   ```

4. **Fetch Dependencies**:
  ```bash
  go get github.com/aws/aws-sdk-go-v2/...
  ```

5. **Build the Binary**:
  ```bash
  go build -o ssm
  ```

  - This creates an executable named ssm in the current directory.

 ## Usage
The tool supports two subcommands: `port-forward` and `connect`. Run `ssm <command> -h` for detailed help on each.

### Subcommands

#### port-forward

  - Establishes a port forwarding session to a remote host via a bastion instance.

    **Syntax**:
    ```bash
    ssm port-forward -remote=<host>:<port> [options]
    ```

    **Required Flags**:
    
    `-remote`: Remote host and port (e.g., example.com:3306).
    
    **Optional Flags**:
    
    `-verbose`: Enable verbose logging.
    
    `-local-port`: Local port number (default: same as remote port).
    
    `-tags`: Tags to find the bastion instance (default: Role=bastion,Function=port-forward).
    
    `-region`: AWS region (default: AWS_DEFAULT_REGION env var).
    
    `-ping-interval`: TCP ping interval (e.g., 5m, default: 5m).
    
    
    **Example**:
    
    ```bash
    ssm port-forward -remote=example.local:3306 -verbose
    ```

  - Forwards local port 3306 to the remote MySQL host through a bastion instance.




#### connect

  - Starts an interactive SSM session to an EC2 instance by instance ID or tags.

    **Syntax**:
    ```bash
    ssm connect [options]
    ```

    **Flags (at least one of `-target` or `-tags` is required)**:
    
    `-target`: Instance ID (e.g., i-1234567890abcdef0).
    `-tags`: Tags to find the instance (e.g., Role=bastion,Function=port-forward).
    
    **Optional Flags**:

    `-verbose`: Enable verbose logging.
    
    `-region`: AWS region (default: AWS_DEFAULT_REGION env var).
    
    
    **Example**:

    - Connect by instance ID:
    
    ```bash
    ssm connect -target=i-0443eca53cfe5a925 -verbose
    ```

    - Connect by tags:
    
    ```bash
    ssm connect -tags=Role=bastion,Function=port-forward -verbose
    ```

### Interacting with Sessions

- For `connect`:
  - After connecting, you’ll see a shell prompt.
  - Type commands like `whoami` or `exit`.
  - Use `Ctrl+C` or `exit` to terminate.

- For `port-forward`:
  - The session runs until interrupted with `Ctrl+C`.

### Cleanup
Note: After using connect with `Ctrl+C`, the session-manager-plugin process might linger. To clean up manually:

```bash
pkill -f session-manager-plugin
```

## Project Structure

```text
ssm
├── main.go           # Main entry point and CLI dispatcher
├── portForward.go    # Port forwarding subcommand logic
├── ssmConnect.go     # Connect subcommand logic
├── utils.go          # Shared utility functions (AWS config, instance lookup)
├── go.mod            # Go module file
└── go.sum            # Dependency checksums
```

## How It Works

- `port-forward`: Finds a bastion instance by tags, sets up port forwarding via SSM, and pings the local port periodically to keep the session alive.
- `connect`: Connects to an instance by ID or finds one by tags, then launches an interactive SSM session using the AWS CLI.
- **Tag Lookup**: Uses a worker pool (size 8) to efficiently search EC2 instances by tags, connecting to the first running match.

## Troubleshooting
- AWS Credentials: Ensure `aws configure` is set up or environment variables are defined.
- Permissions: Verify IAM permissions for SSM and EC2 operations.
- Lingering Processes: Use `ps aux | grep ssm` to check for `session-manager-plugin` after exiting; kill them if needed with `pkill`.

## Contributing
Feel free to fork this project, modify the source files, and submit pull requests with improvements!

## License
This project is unlicensed—use it freely!

## Notes
- **Building**: The instructions assume a fresh setup. If you already have a go.mod, skip go mod init and just run go build.
