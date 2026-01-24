/*
================================================================================
  IOS-XR SSH Test Script v3
  Purpose: Execute using a temp shell script file (like test_iosxr.sh)
================================================================================
*/

package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
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
		fmt.Println("Usage: ./test_iosxr_ssh3 -host <IP> -u <username> -p <password>")
		fmt.Println("\nExample:")
		fmt.Println("  ./test_iosxr_ssh3 -host 172.10.1.1 -u meralco -p meralco")
		return
	}

	fmt.Println("================================================================================")
	fmt.Println("  IOS-XR SSH Test v3 - Using Shell Script File Method")
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

	// Create temp shell script - send all commands in a single SSH session
	scriptContent := fmt.Sprintf(`#!/bin/bash
HOST="%s"
USER="%s"
PASS="%s"
PORT="%d"

`, *host, *user, *pass, *port)

	// Send all commands in one SSH session to avoid connection resets
	// Add IOS-XR compatible SSH options
	scriptContent += fmt.Sprintf(`sshpass -p "$PASS" ssh \
  -o StrictHostKeyChecking=no \
  -o UserKnownHostsFile=/dev/null \
  -o ConnectTimeout=10 \
  -o ServerAliveInterval=60 \
  -o ServerAliveCountMax=3 \
  -o KexAlgorithms=diffie-hellman-group14-sha1,diffie-hellman-group1-sha1 \
  -o Ciphers=aes128-cbc,aes192-cbc,aes256-cbc,3des-cbc \
  -o MACs=hmac-sha1,hmac-md5 \
  -o HostKeyAlgorithms=ssh-rsa \
  -p $PORT "$USER@$HOST" << 'EOF'
`)

	// Add each command
	for _, cmd := range commands {
		scriptContent += fmt.Sprintf(`echo "=== %s ==="
%s
echo ""
`, cmd, cmd)
	}

	scriptContent += `EOF`

	// Write script to temp file
	scriptPath := "/tmp/test_iosxr_temp.sh"
	err := os.WriteFile(scriptPath, []byte(scriptContent), 0755)
	if err != nil {
		fmt.Printf("ERROR: Failed to write script: %v\n", err)
		return
	}

	fmt.Println("Generated script:")
	fmt.Println("--------------------------------------------------------------------------------")
	fmt.Println(scriptContent)
	fmt.Println("--------------------------------------------------------------------------------")
	fmt.Println()
	fmt.Println("Executing script...")
	fmt.Println("================================================================================")
	fmt.Println()

	// Execute the script
	cmd := exec.Command("bash", scriptPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = cmd.Run()
	if err != nil {
		fmt.Printf("\nERROR: Script execution failed: %v\n", err)
	}

	// Cleanup
	os.Remove(scriptPath)

	fmt.Println()
	fmt.Println("================================================================================")
	fmt.Println("  Test Complete")
	fmt.Println("================================================================================")
}
