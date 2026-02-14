/*
================================================================================
  IOS-XR SSH Test Script v2
  Purpose: Test different SSH execution methods for IOS-XR devices
================================================================================
*/

package main

import (
	"flag"
	"fmt"
	"os/exec"
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
		fmt.Println("Usage: ./test_iosxr_ssh2 -host <IP> -u <username> -p <password>")
		fmt.Println("\nExample:")
		fmt.Println("  ./test_iosxr_ssh2 -host 172.10.1.1 -u meralco -p meralco")
		return
	}

	fmt.Println("================================================================================")
	fmt.Println("  IOS-XR SSH Test v2 - Testing Multiple Methods")
	fmt.Println("================================================================================")
	fmt.Printf("  Host: %s\n", *host)
	fmt.Printf("  User: %s\n", *user)
	fmt.Printf("  Port: %d\n", *port)
	fmt.Println("================================================================================")
	fmt.Println()

	testCommand := "show version"

	// Method 1: Using bash -c (exactly like shell)
	fmt.Println("[Method 1] bash -c with full command string")
	fmt.Println("--------------------------------------------------------------------------------")
	output1 := method1_bashC(*host, *user, *pass, *port, testCommand)
	printOutput(output1)
	time.Sleep(1 * time.Second)

	// Method 2: Using bash -c with escaped quotes
	fmt.Println("[Method 2] bash -c with escaped quotes")
	fmt.Println("--------------------------------------------------------------------------------")
	output2 := method2_bashCEscaped(*host, *user, *pass, *port, testCommand)
	printOutput(output2)
	time.Sleep(1 * time.Second)

	// Method 3: Using sh -c
	fmt.Println("[Method 3] sh -c")
	fmt.Println("--------------------------------------------------------------------------------")
	output3 := method3_shC(*host, *user, *pass, *port, testCommand)
	printOutput(output3)
	time.Sleep(1 * time.Second)

	// Method 4: Direct exec with shell expansion
	fmt.Println("[Method 4] Direct sshpass with command in quotes")
	fmt.Println("--------------------------------------------------------------------------------")
	output4 := method4_direct(*host, *user, *pass, *port, testCommand)
	printOutput(output4)

	fmt.Println()
	fmt.Println("================================================================================")
	fmt.Println("  Test Complete - Check which method worked!")
	fmt.Println("================================================================================")
}

// Method 1: bash -c "sshpass -p 'pass' ssh ... \"command\""
func method1_bashC(host, user, pass string, port int, command string) string {
	// Build the exact command string as you would type in terminal
	cmdStr := fmt.Sprintf(`sshpass -p '%s' ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o LogLevel=ERROR -p %d %s@%s "%s"`,
		pass, port, user, host, command)

	fmt.Printf("Executing: bash -c \"%s\"\n\n", cmdStr)

	cmd := exec.Command("bash", "-c", cmdStr)
	output, err := cmd.CombinedOutput()

	if err != nil {
		return fmt.Sprintf("ERROR: %v\nOutput: %s", err, string(output))
	}
	return string(output)
}

// Method 2: bash -c with different escaping
func method2_bashCEscaped(host, user, pass string, port int, command string) string {
	// Using different quote style
	cmdStr := fmt.Sprintf("sshpass -p %s ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o LogLevel=ERROR -p %d %s@%s '%s'",
		pass, port, user, host, command)

	fmt.Printf("Executing: bash -c \"%s\"\n\n", cmdStr)

	cmd := exec.Command("bash", "-c", cmdStr)
	output, err := cmd.CombinedOutput()

	if err != nil {
		return fmt.Sprintf("ERROR: %v\nOutput: %s", err, string(output))
	}
	return string(output)
}

// Method 3: sh -c
func method3_shC(host, user, pass string, port int, command string) string {
	cmdStr := fmt.Sprintf(`sshpass -p '%s' ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o LogLevel=ERROR -p %d %s@%s "%s"`,
		pass, port, user, host, command)

	fmt.Printf("Executing: sh -c \"%s\"\n\n", cmdStr)

	cmd := exec.Command("sh", "-c", cmdStr)
	output, err := cmd.CombinedOutput()

	if err != nil {
		return fmt.Sprintf("ERROR: %v\nOutput: %s", err, string(output))
	}
	return string(output)
}

// Method 4: Direct execution
func method4_direct(host, user, pass string, port int, command string) string {
	fmt.Printf("Executing: sshpass -p *** ssh ... %s@%s \"%s\"\n\n", user, host, command)

	cmd := exec.Command("sshpass",
		"-p", pass,
		"ssh",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "LogLevel=ERROR",
		"-p", fmt.Sprintf("%d", port),
		fmt.Sprintf("%s@%s", user, host),
		command)

	output, err := cmd.CombinedOutput()

	if err != nil {
		return fmt.Sprintf("ERROR: %v\nOutput: %s", err, string(output))
	}
	return string(output)
}

func printOutput(output string) {
	if output == "" {
		fmt.Println("(no output)")
	} else {
		// Print first 20 lines
		lines := 0
		for i, ch := range output {
			fmt.Print(string(ch))
			if ch == '\n' {
				lines++
				if lines >= 20 {
					fmt.Printf("\n... (truncated, total %d chars)\n", len(output))
					break
				}
			}
			if i > 2000 {
				fmt.Printf("\n... (truncated at 2000 chars)\n")
				break
			}
		}
	}
	fmt.Println()
}
