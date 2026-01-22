/*
================================================================================
MERALCO Network Migration Health Check Logger
SSH Nested Multi-Thread Execution Tool
================================================================================
Description: Automated show command collection tool for network devices
             Supports IOS-XR (ASR9K), IOS-XE (ASR903/920, Cat9300), and IOS

Features:
  - Multi-threaded SSH execution for parallel command collection
  - Reads device inventory from Excel (host_info.xlsx)
  - Reads target devices from target.txt
  - Reads show commands from command.txt
  - Generates timestamped output logs per device
  - Supports both pre-migration and post-migration health checks
  - Uses sshpass/ssh for connections (no external Go dependencies)

Author: Network Engineering Team
Version: 1.0.0
Build:   go build -o ssh_health_check ssh_nested_multi_thread.go

Requirements:
  - sshpass (apt-get install sshpass)
  - ssh client (standard on Linux/macOS)
================================================================================
*/

package main

import (
	"archive/zip"
	"bufio"
	"encoding/xml"
	"flag"
	"fmt"
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
	DefaultCmdTimeout   = 60 * time.Second
	DefaultMaxWorkers   = 5
	DefaultOutputDir    = "output"
	DefaultCommandFile  = "command.txt"
	DefaultTargetFile   = "target.txt"
	DefaultHostFile     = "host_info.xlsx"
	Version             = "1.0.0"
	Banner              = `
================================================================================
  MERALCO Network Health Check Logger v%s
  Core Migration: ASR9010 -> ASR9906 (MPLS-SR Migration)
================================================================================
`
)

// ============================================================================
// DATA STRUCTURES
// ============================================================================

// DeviceInfo holds device connection information
type DeviceInfo struct {
	Hostname  string
	IPAddress string
	DeviceType string
	Site      string
	Role      string
}

// ExecutionResult holds the result of command execution
type ExecutionResult struct {
	Hostname    string
	IPAddress   string
	Command     string
	Output      string
	Error       error
	StartTime   time.Time
	EndTime     time.Time
	Duration    time.Duration
}

// Config holds application configuration
type Config struct {
	Username     string
	Password     string
	CommandFile  string
	TargetFile   string
	HostFile     string
	OutputDir    string
	MaxWorkers   int
	SSHPort      int
	SSHTimeout   time.Duration
	CmdTimeout   time.Duration
	Verbose      bool
	DryRun       bool
	Phase        string // pre-migration, post-migration, health-check
}

// ============================================================================
// XLSX PARSER (Minimal implementation for host_info.xlsx)
// ============================================================================

// SharedStrings represents the shared strings in XLSX
type SharedStrings struct {
	XMLName xml.Name `xml:"sst"`
	Count   int      `xml:"count,attr"`
	Strings []struct {
		T string `xml:"t"`
	} `xml:"si"`
}

// SheetData represents worksheet data
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

// parseXLSX parses an Excel file and returns device information
func parseXLSX(filename string) (map[string]DeviceInfo, error) {
	devices := make(map[string]DeviceInfo)

	r, err := zip.OpenReader(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open xlsx file: %v", err)
	}
	defer r.Close()

	// Read shared strings
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

	// Read sheet1
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

			// Parse rows (skip header row)
			for i, row := range sheet.Rows {
				if i == 0 {
					continue // Skip header
				}

				var hostname, ipAddress, deviceType, site, role string
				for _, cell := range row.Cells {
					col := extractColumn(cell.R)
					val := cell.V

					// If it's a shared string reference
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

// extractColumn extracts column letter from cell reference (e.g., "A1" -> "A")
func extractColumn(cellRef string) string {
	re := regexp.MustCompile(`^([A-Z]+)`)
	match := re.FindString(cellRef)
	return match
}

// ============================================================================
// FILE READERS
// ============================================================================

// readLines reads a file and returns non-empty, non-comment lines
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
// SSH CLIENT (uses sshpass + ssh commands)
// ============================================================================

// SSHClient handles SSH connections using system ssh/sshpass
type SSHClient struct {
	host       string
	port       int
	username   string
	password   string
	cmdTimeout time.Duration
}

// NewSSHClient creates a new SSH client
func NewSSHClient(host string, port int, username, password string, timeout, cmdTimeout time.Duration) *SSHClient {
	return &SSHClient{
		host:       host,
		port:       port,
		username:   username,
		password:   password,
		cmdTimeout: cmdTimeout,
	}
}

// Connect is a no-op for the sshpass implementation (connection happens per command)
func (c *SSHClient) Connect() error {
	return nil
}

// Close is a no-op for the sshpass implementation
func (c *SSHClient) Close() {
}

// ExecuteCommand executes a command via sshpass/ssh and returns the output
func (c *SSHClient) ExecuteCommand(command string) (string, error) {
	// Build the command to execute
	// We'll send commands via stdin to the ssh session
	sshCmd := fmt.Sprintf(`sshpass -p '%s' ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o ConnectTimeout=30 -p %d %s@%s`,
		c.password, c.port, c.username, c.host)

	// Create the full command with terminal length 0 and the actual command
	fullCommand := fmt.Sprintf("terminal length 0\n%s\nexit\n", command)

	// Execute using bash
	cmd := exec.Command("bash", "-c", fmt.Sprintf("echo '%s' | %s", fullCommand, sshCmd))

	// Set timeout
	done := make(chan error)
	var output []byte
	var err error

	go func() {
		output, err = cmd.CombinedOutput()
		done <- err
	}()

	select {
	case <-time.After(c.cmdTimeout):
		cmd.Process.Kill()
		return "", fmt.Errorf("command timed out after %v", c.cmdTimeout)
	case err = <-done:
		if err != nil {
			return string(output), fmt.Errorf("command failed: %v - output: %s", err, string(output))
		}
	}

	return cleanOutput(string(output), command), nil
}

// ExecuteCommands executes multiple commands and returns all outputs
func (c *SSHClient) ExecuteCommands(commands []string) (map[string]string, error) {
	results := make(map[string]string)
	for _, cmd := range commands {
		output, err := c.ExecuteCommand(cmd)
		if err != nil {
			results[cmd] = fmt.Sprintf("ERROR: %v", err)
		} else {
			results[cmd] = output
		}
	}
	return results, nil
}

// ============================================================================
// OUTPUT HELPERS
// ============================================================================

// cleanOutput removes echoed commands and prompts from output
func cleanOutput(output, command string) string {
	lines := strings.Split(output, "\n")
	var cleaned []string
	skipNext := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		
		// Skip empty lines at start/end
		if line == "" {
			continue
		}
		
		// Skip echoed command
		if strings.Contains(line, command) {
			skipNext = true
			continue
		}
		
		// Skip terminal length command
		if strings.Contains(line, "terminal length") {
			continue
		}
		
		// Skip prompt lines
		if strings.HasSuffix(line, "#") || strings.HasSuffix(line, ">") {
			continue
		}

		// Skip SSH warnings
		if strings.Contains(line, "Warning:") || strings.Contains(line, "permanently added") {
			continue
		}

		if skipNext {
			skipNext = false
			continue
		}

		cleaned = append(cleaned, line)
	}

	return strings.Join(cleaned, "\n")
}

// ============================================================================
// WORKER POOL
// ============================================================================

// Worker represents a worker that processes devices
type Worker struct {
	id       int
	config   *Config
	devices  <-chan DeviceInfo
	results  chan<- *DeviceResult
	commands []string
	wg       *sync.WaitGroup
}

// DeviceResult holds all results for a device
type DeviceResult struct {
	Device       DeviceInfo
	Results      []ExecutionResult
	Success      bool
	ErrorMessage string
}

// NewWorker creates a new worker
func NewWorker(id int, config *Config, devices <-chan DeviceInfo, results chan<- *DeviceResult, commands []string, wg *sync.WaitGroup) *Worker {
	return &Worker{
		id:       id,
		config:   config,
		devices:  devices,
		results:  results,
		commands: commands,
		wg:       wg,
	}
}

// Start begins processing devices
func (w *Worker) Start() {
	defer w.wg.Done()

	for device := range w.devices {
		result := w.processDevice(device)
		w.results <- result
	}
}

// processDevice connects to a device and executes commands
func (w *Worker) processDevice(device DeviceInfo) *DeviceResult {
	result := &DeviceResult{
		Device:  device,
		Results: make([]ExecutionResult, 0),
		Success: true,
	}

	if w.config.Verbose {
		log.Printf("[Worker %d] Processing %s (%s)", w.id, device.Hostname, device.IPAddress)
	}

	if w.config.DryRun {
		log.Printf("[Worker %d] DRY RUN: Would connect to %s (%s)", w.id, device.Hostname, device.IPAddress)
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
	)

	// Execute commands
	for _, cmd := range w.commands {
		startTime := time.Now()
		output, err := client.ExecuteCommand(cmd)
		endTime := time.Now()

		execResult := ExecutionResult{
			Hostname:  device.Hostname,
			IPAddress: device.IPAddress,
			Command:   cmd,
			Output:    output,
			Error:     err,
			StartTime: startTime,
			EndTime:   endTime,
			Duration:  endTime.Sub(startTime),
		}

		result.Results = append(result.Results, execResult)

		if err != nil {
			log.Printf("[Worker %d] ERROR: %s - Command '%s' failed: %v", w.id, device.Hostname, cmd, err)
		} else if w.config.Verbose {
			log.Printf("[Worker %d] SUCCESS: %s - Command '%s' completed in %v", w.id, device.Hostname, cmd, execResult.Duration)
		}
	}

	return result
}

// ============================================================================
// OUTPUT WRITER
// ============================================================================

// OutputWriter writes results to files
type OutputWriter struct {
	outputDir string
	phase     string
	timestamp string
}

// NewOutputWriter creates a new output writer
func NewOutputWriter(outputDir, phase string) *OutputWriter {
	timestamp := time.Now().Format("20060102_150405")
	return &OutputWriter{
		outputDir: outputDir,
		phase:     phase,
		timestamp: timestamp,
	}
}

// WriteDeviceResults writes results for a single device
func (w *OutputWriter) WriteDeviceResults(result *DeviceResult) error {
	// Create output directory structure
	deviceDir := filepath.Join(w.outputDir, w.phase, w.timestamp)
	if err := os.MkdirAll(deviceDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %v", err)
	}

	// Create device output file
	filename := filepath.Join(deviceDir, fmt.Sprintf("%s_%s.log", result.Device.Hostname, w.timestamp))
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create output file: %v", err)
	}
	defer file.Close()

	// Write header
	fmt.Fprintf(file, "================================================================================\n")
	fmt.Fprintf(file, " MERALCO Network Health Check Logger\n")
	fmt.Fprintf(file, " Phase: %s\n", w.phase)
	fmt.Fprintf(file, "================================================================================\n")
	fmt.Fprintf(file, " Hostname:    %s\n", result.Device.Hostname)
	fmt.Fprintf(file, " IP Address:  %s\n", result.Device.IPAddress)
	fmt.Fprintf(file, " Device Type: %s\n", result.Device.DeviceType)
	fmt.Fprintf(file, " Site:        %s\n", result.Device.Site)
	fmt.Fprintf(file, " Role:        %s\n", result.Device.Role)
	fmt.Fprintf(file, " Timestamp:   %s\n", time.Now().Format("2006-01-02 15:04:05 MST"))
	fmt.Fprintf(file, "================================================================================\n\n")

	if !result.Success {
		fmt.Fprintf(file, "ERROR: %s\n\n", result.ErrorMessage)
	}

	// Write command outputs
	for _, execResult := range result.Results {
		fmt.Fprintf(file, "--------------------------------------------------------------------------------\n")
		fmt.Fprintf(file, " Command: %s\n", execResult.Command)
		fmt.Fprintf(file, " Duration: %v\n", execResult.Duration)
		fmt.Fprintf(file, "--------------------------------------------------------------------------------\n")
		
		if execResult.Error != nil {
			fmt.Fprintf(file, "ERROR: %v\n", execResult.Error)
		} else {
			fmt.Fprintf(file, "%s\n", execResult.Output)
		}
		fmt.Fprintf(file, "\n")
	}

	// Write footer
	fmt.Fprintf(file, "================================================================================\n")
	fmt.Fprintf(file, " End of Health Check Log\n")
	fmt.Fprintf(file, "================================================================================\n")

	return nil
}

// WriteSummary writes a summary of all results
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

	// Count successes and failures
	successCount := 0
	failCount := 0
	for _, r := range results {
		if r.Success {
			successCount++
		} else {
			failCount++
		}
	}

	// Write summary header
	fmt.Fprintf(file, "================================================================================\n")
	fmt.Fprintf(file, " MERALCO Network Health Check - Execution Summary\n")
	fmt.Fprintf(file, " Phase: %s\n", w.phase)
	fmt.Fprintf(file, " Timestamp: %s\n", time.Now().Format("2006-01-02 15:04:05 MST"))
	fmt.Fprintf(file, "================================================================================\n\n")

	fmt.Fprintf(file, "EXECUTION STATISTICS:\n")
	fmt.Fprintf(file, "  Total Devices:    %d\n", len(results))
	fmt.Fprintf(file, "  Successful:       %d\n", successCount)
	fmt.Fprintf(file, "  Failed:           %d\n", failCount)
	fmt.Fprintf(file, "  Success Rate:     %.1f%%\n\n", float64(successCount)/float64(len(results))*100)

	// Write detailed results
	fmt.Fprintf(file, "DEVICE STATUS:\n")
	fmt.Fprintf(file, "--------------------------------------------------------------------------------\n")
	fmt.Fprintf(file, "%-20s %-15s %-10s %s\n", "HOSTNAME", "IP ADDRESS", "STATUS", "NOTES")
	fmt.Fprintf(file, "--------------------------------------------------------------------------------\n")

	for _, r := range results {
		status := "SUCCESS"
		notes := ""
		if !r.Success {
			status = "FAILED"
			notes = r.ErrorMessage
		}
		fmt.Fprintf(file, "%-20s %-15s %-10s %s\n", r.Device.Hostname, r.Device.IPAddress, status, notes)
	}

	fmt.Fprintf(file, "--------------------------------------------------------------------------------\n")
	fmt.Fprintf(file, "\n")

	// Write failed devices separately
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
// MAIN APPLICATION
// ============================================================================

func main() {
	// Parse command line arguments
	config := parseFlags()

	// Print banner
	fmt.Printf(Banner, Version)

	// Validate configuration
	if err := validateConfig(config); err != nil {
		log.Fatalf("Configuration error: %v", err)
	}

	// Read host inventory from Excel
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

	// Read commands
	log.Printf("Reading commands from %s...", config.CommandFile)
	commands, err := readLines(config.CommandFile)
	if err != nil {
		log.Fatalf("Failed to read command file: %v", err)
	}
	log.Printf("Loaded %d commands to execute", len(commands))

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

	log.Printf("Processing %d devices with %d workers...", len(targetDevices), config.MaxWorkers)

	// Create output writer
	writer := NewOutputWriter(config.OutputDir, config.Phase)

	// Create channels
	deviceChan := make(chan DeviceInfo, len(targetDevices))
	resultChan := make(chan *DeviceResult, len(targetDevices))

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < config.MaxWorkers; i++ {
		wg.Add(1)
		worker := NewWorker(i+1, config, deviceChan, resultChan, commands, &wg)
		go worker.Start()
	}

	// Send devices to workers
	go func() {
		for _, device := range targetDevices {
			deviceChan <- device
		}
		close(deviceChan)
	}()

	// Collect results
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Process results
	var allResults []*DeviceResult
	for result := range resultChan {
		allResults = append(allResults, result)
		
		// Write individual device results
		if err := writer.WriteDeviceResults(result); err != nil {
			log.Printf("ERROR: Failed to write results for %s: %v", result.Device.Hostname, err)
		} else {
			status := "SUCCESS"
			if !result.Success {
				status = "FAILED"
			}
			log.Printf("Completed: %s [%s]", result.Device.Hostname, status)
		}
	}

	// Write summary
	if err := writer.WriteSummary(allResults); err != nil {
		log.Printf("ERROR: Failed to write summary: %v", err)
	}

	// Print final statistics
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

// parseFlags parses command line flags
func parseFlags() *Config {
	config := &Config{}

	flag.StringVar(&config.Username, "u", "", "SSH username (required)")
	flag.StringVar(&config.Username, "username", "", "SSH username (required)")
	flag.StringVar(&config.Password, "p", "", "SSH password (required)")
	flag.StringVar(&config.Password, "password", "", "SSH password (required)")
	flag.StringVar(&config.CommandFile, "c", DefaultCommandFile, "Command file path")
	flag.StringVar(&config.CommandFile, "commands", DefaultCommandFile, "Command file path")
	flag.StringVar(&config.TargetFile, "t", DefaultTargetFile, "Target devices file path")
	flag.StringVar(&config.TargetFile, "targets", DefaultTargetFile, "Target devices file path")
	flag.StringVar(&config.HostFile, "h", DefaultHostFile, "Host inventory Excel file path")
	flag.StringVar(&config.HostFile, "hosts", DefaultHostFile, "Host inventory Excel file path")
	flag.StringVar(&config.OutputDir, "o", DefaultOutputDir, "Output directory")
	flag.StringVar(&config.OutputDir, "output", DefaultOutputDir, "Output directory")
	flag.IntVar(&config.MaxWorkers, "w", DefaultMaxWorkers, "Maximum concurrent workers")
	flag.IntVar(&config.MaxWorkers, "workers", DefaultMaxWorkers, "Maximum concurrent workers")
	flag.IntVar(&config.SSHPort, "port", DefaultSSHPort, "SSH port")
	flag.BoolVar(&config.Verbose, "v", false, "Verbose output")
	flag.BoolVar(&config.Verbose, "verbose", false, "Verbose output")
	flag.BoolVar(&config.DryRun, "dry-run", false, "Dry run (no actual connections)")
	flag.StringVar(&config.Phase, "phase", "health-check", "Migration phase (pre-migration, post-migration, health-check)")

	// Custom timeout flags
	var sshTimeoutSec, cmdTimeoutSec int
	flag.IntVar(&sshTimeoutSec, "ssh-timeout", 30, "SSH connection timeout in seconds")
	flag.IntVar(&cmdTimeoutSec, "cmd-timeout", 60, "Command execution timeout in seconds")

	flag.Parse()

	config.SSHTimeout = time.Duration(sshTimeoutSec) * time.Second
	config.CmdTimeout = time.Duration(cmdTimeoutSec) * time.Second

	return config
}

// validateConfig validates the configuration
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
