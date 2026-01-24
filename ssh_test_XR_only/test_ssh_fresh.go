package main

import (
	"fmt"
	"log"
	"os"

	"golang.org/x/crypto/ssh"
)

func main() {
	// Replace with your IOS XR device details
	host := "172.10.1.1"
	user := "meralco"
	password := "meralco"

	// SSH client configuration
	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // for testing only
	}

	// Connect to the IOS XR device
	client, err := ssh.Dial("tcp", host, config)
	if err != nil {
		log.Fatalf("Failed to dial: %s", err)
	}
	defer client.Close()

	// Commands to run
	commands := []string{
		"show platform",
		"show interface brief",
	}

	// Create output file
	file, err := os.Create("iosxr_output.txt")
	if err != nil {
		log.Fatalf("Failed to create file: %s", err)
	}
	defer file.Close()

	// Run each command and save output
	for _, cmd := range commands {
		session, err := client.NewSession()
		if err != nil {
			log.Fatalf("Failed to create session: %s", err)
		}

		output, err := session.CombinedOutput(cmd)
		if err != nil {
			log.Printf("Command failed: %s", err)
		}

		// Write command and output to file
		fmt.Fprintf(file, ">>> %s\n%s\n\n", cmd, string(output))

		session.Close()
	}

	fmt.Println("Output saved to iosxr_output.txt")
}
