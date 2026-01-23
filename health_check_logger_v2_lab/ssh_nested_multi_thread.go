/*
================================================================================
MERALCO Network Migration Health Check Logger v2.0
SSH Nested Multi-Thread Execution Tool - IMPROVED
================================================================================
Description: Automated show command collection tool for network devices
             Supports IOS-XR (ASR9K), IOS-XE (ASR903/920), and IOS (Switches)

Improvements in v2.0:
  - Single SSH session per device (login once, run all commands, logout)
  - Proper PTY allocation for IOS-XR devices
  - OS-aware command execution based on device type
  - Reduced login/logout log entries on network devices
  - Better output parsing and cleanup

Author: Network Engineering Team
Version: 2.0.0
Build:   go build -o ssh_health_check ssh_nested_multi_thread.go
================================================================================
*/

package main

import (
	"archive/zip"
	"bufio"
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ============================================================================
// CONFIGURATION CONSTANTS
// ============================================================================

const (
	DefaultSSHPort      = 22
	DefaultSSHTimeout   = 30 * time.Second
	DefaultCmdTimeout   = 120 * time.Second
	DefaultMaxWorkers   = 5
	DefaultOutputDir    = "output"
	DefaultCommandFile  = "command.txt"
	DefaultTargetFile   = "target.txt"
	DefaultHostFile     = "host_info.xlsx"
	Version             = "2.0.0"
	Banner              = `
================================================================================
  MERALCO Network Health Check Logger v%s
  Core Migration: ASR9010 -> ASR9906 (MPLS-SR Migration)
  Improved: Single SSH session per device
================================================================================
`
)

// ============================================================================
// DATA STRUCTURES
// ============================================================================

// DeviceInfo holds device connection information
type DeviceInfo struct {
	Hostname   string
	IPAddress  string
	DeviceType string
	Site       string
	Role       string
}

// ExecutionResult holds the result of command execution
type ExecutionResult struct {
	Hostname  string
	IPAddress string
	Command   string
	Output    string
	Error     error
	StartTime time.Time
	EndTime   time.Time
	Duration  time.Duration
}

// Config holds application configuration
type Config struct {
	Username        string
	Password        string
	CommandFile     string
	CommandFileXR   string
	CommandFileXE   string
	CommandFileL2   string
	TargetFile      string
	HostFile        string
	OutputDir       string
	MaxWorkers      int
	SSHPort         int
	SSHTimeout      time.Duration
	CmdTimeout      time.Duration
	Verbose         bool
	DryRun          bool
	Phase           string
	UseOSCommands   bool
}

// ============================================================================
// XLSX PARSER
// ============================================================================

type SharedStrings struct {
	XMLName xml.Name `xml:"sst"`
	Count   int      `xml:"count,attr"`
	Strings []struct {
		T string `xml:"t"`
	} `xml:"si"`
}

type SheetData struct {
	XMLName xml.Name `xml:"worksheet"`
	Rows    []struct {
		R     string `xml:"r,attr"`
		Cells []struct {
			R string `xml:"r,attr"`
			T string `xml:"t,attr"`
			V string `xml:"v"`
		} `xml:"c"`
	} `xml:"sheetData>row"`
}

func parseXLSX(filename string) (map[string]DeviceInfo, error) {
	devices := make(map[string]DeviceInfo)

	r, err := zip.OpenReader(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open xlsx file: %v", err)
	}
	defer r.Close()

	var sharedStrings []string
	for _, f := range r.File {
		if f.Name == "xl/sharedStrings.xml" {
			rc, err := f.Open()
			if err != nil {
				continue
			}
			defer rc.Close()

			var ss SharedStrings
			decoder := xml.NewDecoder(rc)
			if err := decoder.Decode(&ss); err != nil {
				continue
			}
			for _, s := range ss.Strings {
				sharedStrings = append(sharedStrings, s.T)
			}
			break
		}
	}

	for _, f := range r.File {
		if f.Name == "xl/worksheets/sheet1.xml" {
			rc, err := f.Open()
			if err != nil {
				return nil, fmt.Errorf("failed to open sheet1: %v", err)
			}
			defer rc.Close()

			var sheet SheetData
			decoder := xml.NewDecoder(rc)
			if err := decoder.Decode(&sheet); err != nil {
				return nil, fmt.Errorf("failed to decode sheet1: %v", err)
			}

			for i, row := range sheet.Rows {
				if i == 0 {
					continue
				}

				var hostname, ipAddress, deviceType, site, role string
				for _, cell := range row.Cells {
					col := extractColumn(cell.R)
					val := cell.V

					if cell.T == "s" {
						idx, _ := strconv.Atoi(val)
						if idx < len(sharedStrings) {
							val = sharedStrings[idx]
						}
					}

					switch col {
					case "A":
						hostname = strings.TrimSpace(val)
					case "B":
						ipAddress = strings.TrimSpace(val)
					case "C":
						deviceType = strings.TrimSpace(val)
					case "D":
						site = strings.TrimSpace(val)
					case "E":
						role = strings.TrimSpace(val)
					}
				}

				if hostname != "" && ipAddress != "" {
					devices[strings.ToUpper(hostname)] = DeviceInfo{
						Hostname:   hostname,
						IPAddress:  ipAddress,
						DeviceType: deviceType,
						Site:       site,
						Role:       role,
					}
				}
			}
			break
		}
	}

	return devices, nil
}

func extractColumn(cellRef string) string {
	re := regexp.MustCompile(`^([A-Z]+)`)
	match := re.FindString(cellRef)
	return match
}

// ============================================================================
// FILE READERS
// ============================================================================

func readLines(filename string) ([]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "#") && !strings.HasPrefix(line, "//") {
			lines = append(lines, line)
		}
	}
	return lines, scanner.Err()
}

// ============================================================================
// DEVICE TYPE DETECTION
// ============================================================================

func getDeviceOS(deviceType string) string {
	deviceType = strings.ToUpper(deviceType)
	
	// IOS-XR devices
	if strings.Contains(deviceType, "ASR9") || strings.Contains(deviceType, "XRV") ||
		strings.Contains(deviceType, "NCS") || strings.Contains(deviceType, "IOS-XR") ||
		strings.Contains(deviceType, "IOSXR") {
		return "IOS-XR"
	}
	
	// IOS-XE devices
	if strings.Contains(deviceType, "ASR1") || strings.Contains(deviceType, "ASR-1") ||
		strings.Contains(deviceType, "ASR903") || strings.Contains(deviceType, "ASR920") ||
		strings.Contains(deviceType, "ISR") || strings.Contains(deviceType, "CSR") ||
		strings.Contains(deviceType, "IOS-XE") || strings.Contains(deviceType, "IOSXE") ||
		strings.Contains(deviceType, "CAT") || strings.Contains(deviceType, "C9") {
		return "IOS-XE"
	}
	
	// L2 Switch
	if strings.Contains(deviceType, "SWITCH") || strings.Contains(deviceType, "L2") ||
		strings.Contains(deviceType, "2960") || strings.Contains(deviceType, "3750") ||
		strings.Contains(deviceType, "3850") || strings.Contains(deviceType, "9200") ||
		strings.Contains(deviceType, "9300") {
		return "L2-SWITCH"
	}
	
	// Default to IOS-XE
	return "IOS-XE"
}

// ============================================================================
// SSH CLIENT - SINGLE SESSION EXECUTION
// ============================================================================

type SSHClient struct {
	host       string
	port       int
	username   string
	password   string
	cmdTimeout time.Duration
	deviceOS   string
}

func NewSSHClient(host string, port int, username, password string, timeout, cmdTimeout time.Duration, deviceOS string) *SSHClient {
	return &SSHClient{
		host:       host,
		port:       port,
		username:   username,
		password:   password,
		cmdTimeout: cmdTimeout,
		deviceOS:   deviceOS,
	}
}

// ExecuteAllCommands runs all commands in a SINGLE SSH session
func (c *SSHClient) ExecuteAllCommands(commands []string) (map[string]string, error) {
	results := make(map[string]string)

	// Build command script based on device OS
	var cmdScript strings.Builder
	
	// Terminal length command varies by OS
	switch c.deviceOS {
	case "IOS-XR":
		cmdScript.WriteString("terminal length 0\n")
		cmdScript.WriteString("terminal width 512\n")
	case "IOS-XE", "L2-SWITCH":
		cmdScript.WriteString("terminal length 0\n")
		cmdScript.WriteString("terminal width 512\n")
	}

	// Add markers and commands
	for i, cmd := range commands {
		marker := fmt.Sprintf("===CMD_%d_START===", i)
		endMarker := fmt.Sprintf("===CMD_%d_END===", i)
		
		// Echo markers for parsing (works on all Cisco platforms)
		cmdScript.WriteString(fmt.Sprintf("echo %s\n", marker))
		cmdScript.WriteString(cmd + "\n")
		cmdScript.WriteString(fmt.Sprintf("echo %s\n", endMarker))
	}
	
	cmdScript.WriteString("exit\n")

	// Build SSH command with proper PTY allocation
	// -t -t forces PTY allocation even when stdin is not a terminal (fixes IOS-XR issue)
	sshArgs := []string{
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "ConnectTimeout=30",
		"-o", "ServerAliveInterval=10",
		"-o", "ServerAliveCountMax=3",
		"-o", "LogLevel=ERROR",
		"-p", strconv.Itoa(c.port),
		"-t", "-t", // Force PTY allocation (critical for IOS-XR)
		fmt.Sprintf("%s@%s", c.username, c.host),
	}

	// Use sshpass for password authentication
	fullCmd := exec.Command("sshpass", append([]string{"-p", c.password, "ssh"}, sshArgs...)...)
	
	// Create pipes
	stdin, err := fullCmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %v", err)
	}

	// Capture output
	var outputBuf strings.Builder
	fullCmd.Stdout = &outputBuf
	fullCmd.Stderr = &outputBuf

	// Start the command
	if err := fullCmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start SSH: %v", err)
	}

	// Send commands
	io.WriteString(stdin, cmdScript.String())
	stdin.Close()

	// Wait with timeout
	done := make(chan error)
	go func() {
		done <- fullCmd.Wait()
	}()

	select {
	case err := <-done:
		if err != nil {
			// Check if it's just the exit status (normal for SSH)
			if !strings.Contains(err.Error(), "exit status") {
				return nil, fmt.Errorf("SSH failed: %v", err)
			}
		}
	case <-time.After(c.cmdTimeout):
		fullCmd.Process.Kill()
		return nil, fmt.Errorf("timeout after %v", c.cmdTimeout)
	}

	// Parse output to extract individual command results
	fullOutput := outputBuf.String()
	
	for i, cmd := range commands {
		marker := fmt.Sprintf("===CMD_%d_START===", i)
		endMarker := fmt.Sprintf("===CMD_%d_END===", i)
		
		output := extractBetweenMarkers(fullOutput, marker, endMarker)
		results[cmd] = cleanCommandOutput(output, cmd, c.deviceOS)
	}

	return results, nil
}

// extractBetweenMarkers extracts text between two markers
func extractBetweenMarkers(text, startMarker, endMarker string) string {
	startIdx := strings.Index(text, startMarker)
	if startIdx == -1 {
		return ""
	}
	startIdx += len(startMarker)

	endIdx := strings.Index(text[startIdx:], endMarker)
	if endIdx == -1 {
		return text[startIdx:]
	}

	return text[startIdx : startIdx+endIdx]
}

// cleanCommandOutput removes echoed commands, prompts, and other noise
func cleanCommandOutput(output, command, deviceOS string) string {
	lines := strings.Split(output, "\n")
	var cleaned []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		
		// Skip empty lines
		if trimmed == "" {
			continue
		}
		
		// Skip the echoed command itself
		if strings.Contains(trimmed, command) && len(trimmed) < len(command)+20 {
			continue
		}
		
		// Skip echo commands
		if strings.HasPrefix(trimmed, "echo ===CMD_") {
			continue
		}
		
		// Skip marker lines
		if strings.Contains(trimmed, "===CMD_") && strings.Contains(trimmed, "===") {
			continue
		}
		
		// Skip terminal commands
		if strings.HasPrefix(trimmed, "terminal length") || strings.HasPrefix(trimmed, "terminal width") {
			continue
		}
		
		// Skip common prompts (hostname#, hostname>, etc.)
		if regexp.MustCompile(`^[A-Za-z0-9_-]+[#>]\s*$`).MatchString(trimmed) {
			continue
		}
		
		// Skip SSH warnings
		if strings.Contains(trimmed, "Warning:") && strings.Contains(trimmed, "known hosts") {
			continue
		}
		
		// Skip "Pseudo-terminal" message
		if strings.Contains(trimmed, "Pseudo-terminal") {
			continue
		}
		
		// Skip connection messages
		if strings.Contains(trimmed, "Connection to") && strings.Contains(trimmed, "closed") {
			continue
		}

		cleaned = append(cleaned, line)
	}

	return strings.Join(cleaned, "\n")
}

// ============================================================================
// WORKER POOL
// ============================================================================

type Worker struct {
	id       int
	config   *Config
	devices  <-chan DeviceInfo
	results  chan<- *DeviceResult
	commands map[string][]string // OS -> commands mapping
	wg       *sync.WaitGroup
}

type DeviceResult struct {
	Device       DeviceInfo
	Results      []ExecutionResult
	Success      bool
	ErrorMessage string
}

func NewWorker(id int, config *Config, devices <-chan DeviceInfo, results chan<- *DeviceResult, commands map[string][]string, wg *sync.WaitGroup) *Worker {
	return &Worker{
		id:       id,
		config:   config,
		devices:  devices,
		results:  results,
		commands: commands,
		wg:       wg,
	}
}

func (w *Worker) Start() {
	defer w.wg.Done()

	for device := range w.devices {
		result := w.processDevice(device)
		w.results <- result
	}
}

func (w *Worker) processDevice(device DeviceInfo) *DeviceResult {
	result := &DeviceResult{
		Device:  device,
		Results: make([]ExecutionResult, 0),
		Success: true,
	}

	// Detect device OS
	deviceOS := getDeviceOS(device.DeviceType)
	
	if w.config.Verbose {
		log.Printf("[Worker %d] Processing %s (%s) - OS: %s", w.id, device.Hostname, device.IPAddress, deviceOS)
	}

	if w.config.DryRun {
		log.Printf("[Worker %d] DRY RUN: Would connect to %s (%s)", w.id, device.Hostname, device.IPAddress)
		return result
	}

	// Get commands for this device OS
	commands, ok := w.commands[deviceOS]
	if !ok {
		// Fallback to default commands
		commands = w.commands["DEFAULT"]
	}
	
	if len(commands) == 0 {
		result.Success = false
		result.ErrorMessage = fmt.Sprintf("No commands defined for OS: %s", deviceOS)
		return result
	}

	// Create SSH client
	client := NewSSHClient(
		device.IPAddress,
		w.config.SSHPort,
		w.config.Username,
		w.config.Password,
		w.config.SSHTimeout,
		w.config.CmdTimeout,
		deviceOS,
	)

	// Execute ALL commands in a single session
	startTime := time.Now()
	outputs, err := client.ExecuteAllCommands(commands)
	endTime := time.Now()

	if err != nil {
		result.Success = false
		result.ErrorMessage = fmt.Sprintf("SSH execution failed: %v", err)
		log.Printf("[Worker %d] ERROR: %s - %s", w.id, device.Hostname, result.ErrorMessage)
		return result
	}

	// Store results for each command
	for _, cmd := range commands {
		output, exists := outputs[cmd]
		var cmdErr error
		if !exists {
			cmdErr = fmt.Errorf("no output captured")
			output = ""
		}

		execResult := ExecutionResult{
			Hostname:  device.Hostname,
			IPAddress: device.IPAddress,
			Command:   cmd,
			Output:    output,
			Error:     cmdErr,
			StartTime: startTime,
			EndTime:   endTime,
			Duration:  endTime.Sub(startTime) / time.Duration(len(commands)),
		}

		result.Results = append(result.Results, execResult)
	}

	if w.config.Verbose {
		log.Printf("[Worker %d] SUCCESS: %s - %d commands in single session (%v total)", 
			w.id, device.Hostname, len(commands), endTime.Sub(startTime))
	}

	return result
}

// ============================================================================
// OUTPUT WRITER
// ============================================================================

type OutputWriter struct {
	outputDir string
	phase     string
	timestamp string
}

func NewOutputWriter(outputDir, phase string) *OutputWriter {
	timestamp := time.Now().Format("20060102_150405")
	return &OutputWriter{
		outputDir: outputDir,
		phase:     phase,
		timestamp: timestamp,
	}
}

func (w *OutputWriter) WriteDeviceResults(result *DeviceResult) error {
	deviceDir := filepath.Join(w.outputDir, w.phase, w.timestamp)
	if err := os.MkdirAll(deviceDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %v", err)
	}

	filename := filepath.Join(deviceDir, fmt.Sprintf("%s_%s.log", result.Device.Hostname, w.timestamp))
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create output file: %v", err)
	}
	defer file.Close()

	// Detect OS for display
	deviceOS := getDeviceOS(result.Device.DeviceType)

	fmt.Fprintf(file, "================================================================================\n")
	fmt.Fprintf(file, " MERALCO Network Health Check Logger v2.0\n")
	fmt.Fprintf(file, " Phase: %s\n", w.phase)
	fmt.Fprintf(file, "================================================================================\n")
	fmt.Fprintf(file, " Hostname:    %s\n", result.Device.Hostname)
	fmt.Fprintf(file, " IP Address:  %s\n", result.Device.IPAddress)
	fmt.Fprintf(file, " Device Type: %s\n", result.Device.DeviceType)
	fmt.Fprintf(file, " Device OS:   %s\n", deviceOS)
	fmt.Fprintf(file, " Site:        %s\n", result.Device.Site)
	fmt.Fprintf(file, " Role:        %s\n", result.Device.Role)
	fmt.Fprintf(file, " Timestamp:   %s\n", time.Now().Format("2006-01-02 15:04:05 MST"))
	fmt.Fprintf(file, " Session:     Single SSH session (all commands)\n")
	fmt.Fprintf(file, "================================================================================\n\n")

	if !result.Success {
		fmt.Fprintf(file, "ERROR: %s\n\n", result.ErrorMessage)
	}

	for _, execResult := range result.Results {
		fmt.Fprintf(file, "--------------------------------------------------------------------------------\n")
		fmt.Fprintf(file, " Command: %s\n", execResult.Command)
		fmt.Fprintf(file, "--------------------------------------------------------------------------------\n")
		
		if execResult.Error != nil {
			fmt.Fprintf(file, "ERROR: %v\n", execResult.Error)
		} else if execResult.Output == "" {
			fmt.Fprintf(file, "(No output or command not supported on this platform)\n")
		} else {
			fmt.Fprintf(file, "%s\n", execResult.Output)
		}
		fmt.Fprintf(file, "\n")
	}

	fmt.Fprintf(file, "================================================================================\n")
	fmt.Fprintf(file, " End of Health Check Log\n")
	fmt.Fprintf(file, "================================================================================\n")

	return nil
}

func (w *OutputWriter) WriteSummary(results []*DeviceResult) error {
	summaryDir := filepath.Join(w.outputDir, w.phase, w.timestamp)
	if err := os.MkdirAll(summaryDir, 0755); err != nil {
		return fmt.Errorf("failed to create summary directory: %v", err)
	}

	filename := filepath.Join(summaryDir, fmt.Sprintf("SUMMARY_%s.log", w.timestamp))
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create summary file: %v", err)
	}
	defer file.Close()

	successCount := 0
	failCount := 0
	for _, r := range results {
		if r.Success {
			successCount++
		} else {
			failCount++
		}
	}

	fmt.Fprintf(file, "================================================================================\n")
	fmt.Fprintf(file, " MERALCO Network Health Check - Execution Summary v2.0\n")
	fmt.Fprintf(file, " Phase: %s\n", w.phase)
	fmt.Fprintf(file, " Timestamp: %s\n", time.Now().Format("2006-01-02 15:04:05 MST"))
	fmt.Fprintf(file, " Mode: Single SSH session per device (reduced login logs)\n")
	fmt.Fprintf(file, "================================================================================\n\n")

	fmt.Fprintf(file, "EXECUTION STATISTICS:\n")
	fmt.Fprintf(file, "  Total Devices:    %d\n", len(results))
	fmt.Fprintf(file, "  Successful:       %d\n", successCount)
	fmt.Fprintf(file, "  Failed:           %d\n", failCount)
	fmt.Fprintf(file, "  Success Rate:     %.1f%%\n\n", float64(successCount)/float64(len(results))*100)

	fmt.Fprintf(file, "DEVICE STATUS:\n")
	fmt.Fprintf(file, "--------------------------------------------------------------------------------\n")
	fmt.Fprintf(file, "%-15s %-15s %-10s %-10s %s\n", "HOSTNAME", "IP ADDRESS", "OS", "STATUS", "NOTES")
	fmt.Fprintf(file, "--------------------------------------------------------------------------------\n")

	for _, r := range results {
		status := "SUCCESS"
		notes := ""
		if !r.Success {
			status = "FAILED"
			notes = r.ErrorMessage
		}
		deviceOS := getDeviceOS(r.Device.DeviceType)
		fmt.Fprintf(file, "%-15s %-15s %-10s %-10s %s\n", r.Device.Hostname, r.Device.IPAddress, deviceOS, status, notes)
	}

	fmt.Fprintf(file, "--------------------------------------------------------------------------------\n")
	fmt.Fprintf(file, "\n")

	if failCount > 0 {
		fmt.Fprintf(file, "FAILED DEVICES (Require Manual Check):\n")
		for _, r := range results {
			if !r.Success {
				fmt.Fprintf(file, "  - %s (%s): %s\n", r.Device.Hostname, r.Device.IPAddress, r.ErrorMessage)
			}
		}
	}

	fmt.Fprintf(file, "\n================================================================================\n")
	fmt.Fprintf(file, " Output Directory: %s\n", summaryDir)
	fmt.Fprintf(file, "================================================================================\n")

	return nil
}

// ============================================================================
// COMMAND LOADING
// ============================================================================

func loadCommands(config *Config) (map[string][]string, error) {
	commands := make(map[string][]string)

	// Try to load OS-specific command files if they exist
	if config.UseOSCommands {
		// IOS-XR commands
		if config.CommandFileXR != "" {
			if cmds, err := readLines(config.CommandFileXR); err == nil && len(cmds) > 0 {
				commands["IOS-XR"] = cmds
				log.Printf("Loaded %d IOS-XR commands from %s", len(cmds), config.CommandFileXR)
			}
		}

		// IOS-XE commands
		if config.CommandFileXE != "" {
			if cmds, err := readLines(config.CommandFileXE); err == nil && len(cmds) > 0 {
				commands["IOS-XE"] = cmds
				log.Printf("Loaded %d IOS-XE commands from %s", len(cmds), config.CommandFileXE)
			}
		}

		// L2 Switch commands
		if config.CommandFileL2 != "" {
			if cmds, err := readLines(config.CommandFileL2); err == nil && len(cmds) > 0 {
				commands["L2-SWITCH"] = cmds
				log.Printf("Loaded %d L2-Switch commands from %s", len(cmds), config.CommandFileL2)
			}
		}
	}

	// Load default command file
	defaultCmds, err := readLines(config.CommandFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read command file: %v", err)
	}
	commands["DEFAULT"] = defaultCmds

	// If no OS-specific files, use default for all
	if _, ok := commands["IOS-XR"]; !ok {
		commands["IOS-XR"] = defaultCmds
	}
	if _, ok := commands["IOS-XE"]; !ok {
		commands["IOS-XE"] = defaultCmds
	}
	if _, ok := commands["L2-SWITCH"]; !ok {
		commands["L2-SWITCH"] = defaultCmds
	}

	return commands, nil
}

// ============================================================================
// MAIN APPLICATION
// ============================================================================

func main() {
	config := parseFlags()

	fmt.Printf(Banner, Version)

	if err := validateConfig(config); err != nil {
		log.Fatalf("Configuration error: %v", err)
	}

	// Read host inventory
	log.Printf("Reading device inventory from %s...", config.HostFile)
	devices, err := parseXLSX(config.HostFile)
	if err != nil {
		log.Fatalf("Failed to read host inventory: %v", err)
	}
	log.Printf("Loaded %d devices from inventory", len(devices))

	// Read target devices
	log.Printf("Reading target devices from %s...", config.TargetFile)
	targets, err := readLines(config.TargetFile)
	if err != nil {
		log.Fatalf("Failed to read target file: %v", err)
	}
	log.Printf("Found %d target devices", len(targets))

	// Load commands (OS-specific if available)
	log.Printf("Loading commands...")
	commands, err := loadCommands(config)
	if err != nil {
		log.Fatalf("Failed to load commands: %v", err)
	}

	// Match targets to device info
	var targetDevices []DeviceInfo
	for _, target := range targets {
		targetUpper := strings.ToUpper(strings.TrimSpace(target))
		if device, ok := devices[targetUpper]; ok {
			targetDevices = append(targetDevices, device)
		} else {
			log.Printf("WARNING: Device '%s' not found in host_info.xlsx, skipping", target)
		}
	}

	if len(targetDevices) == 0 {
		log.Fatalf("No valid target devices found")
	}

	log.Printf("Processing %d devices with %d workers (single session per device)...", len(targetDevices), config.MaxWorkers)

	writer := NewOutputWriter(config.OutputDir, config.Phase)

	deviceChan := make(chan DeviceInfo, len(targetDevices))
	resultChan := make(chan *DeviceResult, len(targetDevices))

	var wg sync.WaitGroup
	for i := 0; i < config.MaxWorkers; i++ {
		wg.Add(1)
		worker := NewWorker(i+1, config, deviceChan, resultChan, commands, &wg)
		go worker.Start()
	}

	go func() {
		for _, device := range targetDevices {
			deviceChan <- device
		}
		close(deviceChan)
	}()

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	var allResults []*DeviceResult
	for result := range resultChan {
		allResults = append(allResults, result)
		
		if err := writer.WriteDeviceResults(result); err != nil {
			log.Printf("ERROR: Failed to write results for %s: %v", result.Device.Hostname, err)
		} else {
			status := "SUCCESS"
			if !result.Success {
				status = "FAILED"
			}
			log.Printf("Completed: %s [%s] - %d commands", result.Device.Hostname, status, len(result.Results))
		}
	}

	if err := writer.WriteSummary(allResults); err != nil {
		log.Printf("ERROR: Failed to write summary: %v", err)
	}

	successCount := 0
	for _, r := range allResults {
		if r.Success {
			successCount++
		}
	}

	fmt.Printf("\n")
	fmt.Printf("================================================================================\n")
	fmt.Printf(" EXECUTION COMPLETE\n")
	fmt.Printf("================================================================================\n")
	fmt.Printf(" Total Devices:  %d\n", len(allResults))
	fmt.Printf(" Successful:     %d\n", successCount)
	fmt.Printf(" Failed:         %d\n", len(allResults)-successCount)
	fmt.Printf(" Output Dir:     %s/%s/%s/\n", config.OutputDir, config.Phase, writer.timestamp)
	fmt.Printf("================================================================================\n")
}

func parseFlags() *Config {
	config := &Config{}

	flag.StringVar(&config.Username, "u", "", "SSH username (required)")
	flag.StringVar(&config.Username, "username", "", "SSH username (required)")
	flag.StringVar(&config.Password, "p", "", "SSH password (required)")
	flag.StringVar(&config.Password, "password", "", "SSH password (required)")
	flag.StringVar(&config.CommandFile, "c", DefaultCommandFile, "Default command file path")
	flag.StringVar(&config.CommandFile, "commands", DefaultCommandFile, "Default command file path")
	flag.StringVar(&config.CommandFileXR, "cmd-xr", "command_iosxr.txt", "IOS-XR specific command file")
	flag.StringVar(&config.CommandFileXE, "cmd-xe", "command_iosxe.txt", "IOS-XE specific command file")
	flag.StringVar(&config.CommandFileL2, "cmd-l2", "command_l2switch.txt", "L2 Switch specific command file")
	flag.StringVar(&config.TargetFile, "t", DefaultTargetFile, "Target devices file path")
	flag.StringVar(&config.TargetFile, "targets", DefaultTargetFile, "Target devices file path")
	flag.StringVar(&config.HostFile, "hosts", DefaultHostFile, "Host inventory Excel file path")
	flag.StringVar(&config.OutputDir, "o", DefaultOutputDir, "Output directory")
	flag.StringVar(&config.OutputDir, "output", DefaultOutputDir, "Output directory")
	flag.IntVar(&config.MaxWorkers, "w", DefaultMaxWorkers, "Maximum concurrent workers")
	flag.IntVar(&config.MaxWorkers, "workers", DefaultMaxWorkers, "Maximum concurrent workers")
	flag.IntVar(&config.SSHPort, "port", DefaultSSHPort, "SSH port")
	flag.BoolVar(&config.Verbose, "v", false, "Verbose output")
	flag.BoolVar(&config.Verbose, "verbose", false, "Verbose output")
	flag.BoolVar(&config.DryRun, "dry-run", false, "Dry run (no actual connections)")
	flag.StringVar(&config.Phase, "phase", "health-check", "Migration phase")
	flag.BoolVar(&config.UseOSCommands, "os-commands", true, "Use OS-specific command files if available")

	var sshTimeoutSec, cmdTimeoutSec int
	flag.IntVar(&sshTimeoutSec, "ssh-timeout", 30, "SSH connection timeout in seconds")
	flag.IntVar(&cmdTimeoutSec, "cmd-timeout", 180, "Total command execution timeout in seconds")

	flag.Parse()

	config.SSHTimeout = time.Duration(sshTimeoutSec) * time.Second
	config.CmdTimeout = time.Duration(cmdTimeoutSec) * time.Second

	return config
}

func validateConfig(config *Config) error {
	if config.Username == "" {
		return fmt.Errorf("username is required (-u or -username)")
	}
	if config.Password == "" {
		return fmt.Errorf("password is required (-p or -password)")
	}
	if _, err := os.Stat(config.CommandFile); os.IsNotExist(err) {
		return fmt.Errorf("command file not found: %s", config.CommandFile)
	}
	if _, err := os.Stat(config.TargetFile); os.IsNotExist(err) {
		return fmt.Errorf("target file not found: %s", config.TargetFile)
	}
	if _, err := os.Stat(config.HostFile); os.IsNotExist(err) {
		return fmt.Errorf("host inventory file not found: %s", config.HostFile)
	}
	return nil
}
