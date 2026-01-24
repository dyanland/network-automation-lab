/*
================================================================================
  IOS-XR SSH Test Script
  Purpose: Test SSH command execution method for IOS-XR devices
================================================================================
*/

package main

import (
	"flag"
	"fmt"
	"os/exec"
	"strconv"
	"time"
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
		fmt.Println("Usage: ./test_iosxr_ssh -host <IP> -u <username> -p <password>")
		fmt.Println("\nExample:")
		fmt.Println("  ./test_iosxr_ssh -host 172.10.1.1 -u meralco -p meralco")
		return
	}

	fmt.Println("================================================================================")
	fmt.Println("  IOS-XR SSH Test")
	fmt.Println("================================================================================")
	fmt.Printf("  Host: %s\n", *host)
	fmt.Printf("  User: %s\n", *user)
	fmt.Printf("  Port: %d\n", *port)
	fmt.Println("================================================================================")
	fmt.Println()

	// Test commands
	commands := []string{
		"show version",
		"show platform",
		"show interfaces brief",
	}

	// Execute each command
	for i, cmd := range commands {
		fmt.Printf("[%d/%d] Executing: %s\n", i+1, len(commands), cmd)
		fmt.Println("--------------------------------------------------------------------------------")

		output, err := executeIOSXRCommand(*host, *user, *pass, *port, cmd)
		
		if err != nil {
			fmt.Printf("ERROR: %v\n", err)
		}
		
		if output == "" {
			fmt.Println("(no output)")
		} else {
			// Print first 30 lines max
			lines := splitLines(output)
			maxLines := 30
			if len(lines) < maxLines {
				maxLines = len(lines)
			}
			for j := 0; j < maxLines; j++ {
				fmt.Println(lines[j])
			}
			if len(lines) > 30 {
				fmt.Printf("... (%d more lines)\n", len(lines)-30)
			}
		}
		
		fmt.Println()
		
		// Small delay between commands
		if i < len(commands)-1 {
			time.Sleep(500 * time.Millisecond)
		}
	}

	fmt.Println("================================================================================")
	fmt.Println("  Test Complete")
	fmt.Println("================================================================================")
}

// executeIOSXRCommand executes a single command on IOS-XR device
// Uses: sshpass -p 'pass' ssh -o StrictHostKeyChecking=no user@host "command"
func executeIOSXRCommand(host, username, password string, port int, command string) (string, error) {
	
	// Build SSH arguments
	sshArgs := []string{
		"-p", password,
		"ssh",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "LogLevel=ERROR",
		"-p", strconv.Itoa(port),
		fmt.Sprintf("%s@%s", username, host),
		command,  // Command as last argument
	}

	// Execute command
	cmd := exec.Command("sshpass", sshArgs...)
	
	// Capture combined stdout and stderr
	output, err := cmd.CombinedOutput()
	
	return string(output), err
}

// splitLines splits string into lines
func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
