package main

import (
	"bufio"
	"fmt"
	"log"
	"os"

	"golang.org/x/crypto/ssh"
)

func main() {
	// Replace with your IOS XR device details
	host := "172.10.1.1:22"
	user := "meralco"
	password := "meralco"

	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	// Connect
	client, err := ssh.Dial("tcp", host, config)
	if err != nil {
		log.Fatalf("Failed to dial: %s", err)
	}
	defer client.Close()

	// Start interactive session
	session, err := client.NewSession()
	if err != nil {
		log.Fatalf("Failed to create session: %s", err)
	}
	defer session.Close()

	// Request PTY (important for IOS XR CLI)
	if err := session.RequestPty("vt100", 80, 40, ssh.TerminalModes{}); err != nil {
		log.Fatalf("Request for PTY failed: %s", err)
	}

	// Create pipes
	stdin, _ := session.StdinPipe()
	stdout, _ := session.StdoutPipe()

	// Start shell
	if err := session.Shell(); err != nil {
		log.Fatalf("Failed to start shell: %s", err)
	}

	// Commands to run
	commands := []string{
		"show platform",
		"show interface brief",
		"exit",
	}

	// Send commands
	for _, cmd := range commands {
		fmt.Fprintf(stdin, "%s\n", cmd)
	}

	// Capture output
	file, err := os.Create("iosxr_output.txt")
	if err != nil {
		log.Fatalf("Failed to create file: %s", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		fmt.Fprintln(file, line)
	}

	if err := scanner.Err(); err != nil {
		log.Fatalf("Error reading output: %s", err)
	}

	fmt.Println("Output saved to iosxr_output.txt")
}
