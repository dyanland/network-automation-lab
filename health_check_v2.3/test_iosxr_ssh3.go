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

	// Create temp shell script (exactly like test_iosxr.sh)
	scriptContent := fmt.Sprintf(`#!/bin/bash
HOST="%s"
USER="%s"
PASS="%s"
PORT="%d"

`, *host, *user, *pass, *port)

	// Add each command - execute individually
	for _, cmd := range commands {
		scriptContent += fmt.Sprintf(`echo "=== %s ==="
sshpass -p "$PASS" ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o LogLevel=ERROR -p $PORT "$USER@$HOST" "%s"
EXIT_CODE=$?
if [ $EXIT_CODE -ne 0 ]; then
  echo "ERROR: Command failed with exit code $EXIT_CODE"
  exit $EXIT_CODE
fi
echo ""

`, cmd, cmd)
	}

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
