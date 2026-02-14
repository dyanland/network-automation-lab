/*
================================================================================
  IOS-XR SSH Test Script v4 - Native Go SSH
  Purpose: Test SSH using golang.org/x/crypto/ssh package
  
  Build: go mod init test_ssh && go mod tidy && go build -o test_iosxr_ssh4 test_iosxr_ssh4.go
================================================================================
*/

package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"golang.org/x/crypto/ssh"
)

func main() {
	// Command line flags
	host := flag.String("host", "", "IOS-XR device IP address (required)")
	user := flag.String("u", "", "SSH username (required)")
	pass := flag.String("p", "", "SSH password (required)")
	port := flag.Int("port", 22, "SSH port")
	flag.Parse()

	// Validate required flags
	if *host == "" || *user == "" || *pass == "" {
		fmt.Println("Usage: ./test_iosxr_ssh4 -host <IP> -u <username> -p <password>")
		fmt.Println("\nBuild first:")
		fmt.Println("  go mod init test_ssh && go mod tidy && go build -o test_iosxr_ssh4 test_iosxr_ssh4.go")
		fmt.Println("\nExample:")
		fmt.Println("  ./test_iosxr_ssh4 -host 172.10.1.1 -u meralco -p meralco")
		return
	}

	fmt.Println("================================================================================")
	fmt.Println("  IOS-XR SSH Test v4 - Native Go SSH with PTY")
	fmt.Println("================================================================================")
	fmt.Printf("  Host: %s\n", *host)
	fmt.Printf("  User: %s\n", *user)
	fmt.Printf("  Port: %d\n", *port)
	fmt.Println("================================================================================")
	fmt.Println()

	commands := []string{
		"show version",
		"show platform",
		"show interfaces brief",
	}

	results, err := executeIOSXRCommands(*host, *user, *pass, *port, commands)
	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
		os.Exit(1)
	}

	// Print results
	for _, cmd := range commands {
		fmt.Printf("[Command] %s\n", cmd)
		fmt.Println("--------------------------------------------------------------------------------")
		output := results[cmd]
		if output == "" {
			fmt.Println("(no output)")
		} else {
			// Print first 30 lines
			lines := 0
			for _, ch := range output {
				fmt.Print(string(ch))
				if ch == '\n' {
					lines++
					if lines >= 30 {
						fmt.Println("... (truncated)")
						break
					}
				}
			}
		}
		fmt.Println()
	}

	fmt.Println("================================================================================")
	fmt.Println("  Test Complete")
	fmt.Println("================================================================================")
}

func executeIOSXRCommands(host, username, password string, port int, commands []string) (map[string]string, error) {
	results := make(map[string]string)

	// SSH client configuration with timeout
	config := &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         30 * time.Second,
	}

	// Dial IOS XR
	addr := fmt.Sprintf("%s:%d", host, port)
	fmt.Printf("Connecting to %s...\n", addr)
	
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, fmt.Errorf("failed to dial: %w", err)
	}
	defer client.Close()
	fmt.Println("Connected!")

	// Start interactive session
	session, err := client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()
	fmt.Println("Session created!")

	// Request PTY (needed for IOS XR CLI)
	modes := ssh.TerminalModes{
		ssh.ECHO:          0,     // disable echo
		ssh.TTY_OP_ISPEED: 14400, // input speed
		ssh.TTY_OP_OSPEED: 14400, // output speed
	}
	if err := session.RequestPty("xterm", 80, 200, modes); err != nil {
		return nil, fmt.Errorf("failed to request PTY: %w", err)
	}
	fmt.Println("PTY allocated!")

	stdin, err := session.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get stdin pipe: %w", err)
	}

	stdout, err := session.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	// Start shell
	if err := session.Shell(); err != nil {
		return nil, fmt.Errorf("failed to start shell: %w", err)
	}
	fmt.Println("Shell started!")

	// Give shell time to initialize and show banner
	fmt.Println("Waiting for shell initialization...")
	time.Sleep(3 * time.Second)

	// Read initial banner/prompt
	initialBuf := make([]byte, 65535)
	n, _ := stdout.Read(initialBuf)
	fmt.Printf("Initial banner (%d bytes):\n%s\n", n, string(initialBuf[:n]))

	// Send terminal length 0 to disable paging
	fmt.Println("\nSending: terminal length 0")
	fmt.Fprintf(stdin, "terminal length 0\n")
	time.Sleep(1 * time.Second)

	// Clear output from terminal length command
	n, _ = stdout.Read(initialBuf)
	fmt.Printf("After terminal length 0 (%d bytes):\n%s\n", n, string(initialBuf[:n]))

	fmt.Println("\n--- Executing commands ---\n")

	// Execute each command and capture output
	for _, cmdStr := range commands {
		fmt.Printf("Sending: %s\n", cmdStr)
		fmt.Fprintf(stdin, "%s\n", cmdStr)
		
		// Wait for command to execute
		time.Sleep(2 * time.Second)

		// Read output
		outputBuf := make([]byte, 1048576) // 1MB buffer
		n, _ := stdout.Read(outputBuf)
		
		cmdOutput := string(outputBuf[:n])
		fmt.Printf("Received %d bytes\n", n)
		
		// Store output
		results[cmdStr] = cmdOutput
	}

	// Exit session cleanly
	fmt.Println("\nSending: exit")
	fmt.Fprintf(stdin, "exit\n")
	time.Sleep(500 * time.Millisecond)

	return results, nil
}
