/*
================================================================================
MERALCO Network Health Check Logger v2.3
Fixed: ASR903/920 detection, Added comparison summary
================================================================================
*/

package main

import (
	"archive/zip"
	"bufio"
	"encoding/csv"
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

const (
	Version = "2.3.0"
	Banner  = `
================================================================================
  MERALCO Network Health Check Logger v%s
  Fixed: ASR903/920 detection, Added Pre/Post comparison
================================================================================
`
)

// ============================================================================
// DATA STRUCTURES
// ============================================================================

type DeviceInfo struct {
	Hostname   string
	IPAddress  string
	DeviceType string
	Site       string
	Role       string
	DetectedOS string
}

type ExecutionResult struct {
	Hostname  string
	IPAddress string
	Command   string
	Output    string
	Error     error
	Duration  time.Duration
}

type Config struct {
	Username      string
	Password      string
	CommandFile   string
	CommandFileXR string
	CommandFileXE string
	CommandFileL2 string
	TargetFile    string
	HostFile      string
	OutputDir     string
	MaxWorkers    int
	SSHPort       int
	CmdTimeout    time.Duration
	Verbose       bool
	DryRun        bool
	Phase         string
	CompareDir    string // For pre/post comparison
}

// ============================================================================
// OS DETECTION - FIXED ORDER (specific patterns first!)
// ============================================================================

func detectDeviceOS(deviceType string) string {
	dt := strings.ToUpper(strings.TrimSpace(deviceType))

	// =====================================================
	// CHECK SPECIFIC PATTERNS FIRST (before generic ones!)
	// =====================================================

	// IOS-XE: ASR903, ASR920 (must check BEFORE ASR9 pattern!)
	if strings.Contains(dt, "ASR903") || strings.Contains(dt, "ASR-903") ||
		strings.Contains(dt, "ASR920") || strings.Contains(dt, "ASR-920") {
		return "IOS-XE"
	}

	// IOS-XR: ASR9000 series (ASR9K, ASR9006, ASR9010, ASR9906, etc.)
	// Only match ASR9 followed by 0 or K (not ASR903/ASR920)
	if strings.Contains(dt, "ASR9K") || strings.Contains(dt, "ASR-9K") ||
		strings.Contains(dt, "ASR90") || strings.Contains(dt, "ASR91") ||
		strings.Contains(dt, "ASR99") || // ASR9006, ASR9010, ASR9901, ASR9906, etc.
		strings.Contains(dt, "XRV") || strings.Contains(dt, "IOS-XR") ||
		strings.Contains(dt, "IOSXR") || strings.Contains(dt, "NCS") ||
		strings.Contains(dt, "CRS") {
		return "IOS-XR"
	}

	// IOS-XE: Other patterns
	if strings.Contains(dt, "ASR1") || strings.Contains(dt, "ASR-1") ||
		strings.Contains(dt, "ISR") || strings.Contains(dt, "CSR") ||
		strings.Contains(dt, "IOS-XE") || strings.Contains(dt, "IOSXE") ||
		strings.Contains(dt, "IOSV") || strings.Contains(dt, "IOS-V") ||
		strings.Contains(dt, "VIOS") || strings.Contains(dt, "C8") ||
		strings.Contains(dt, "C11") || strings.Contains(dt, "C12") {
		return "IOS-XE"
	}

	// L2 Switch patterns
	if strings.Contains(dt, "SWITCH") || strings.Contains(dt, "SW") ||
		strings.Contains(dt, "CAT") || strings.Contains(dt, "CATALYST") ||
		strings.Contains(dt, "C9300") || strings.Contains(dt, "C9200") ||
		strings.Contains(dt, "C9400") || strings.Contains(dt, "C9500") ||
		strings.Contains(dt, "C3850") || strings.Contains(dt, "C3750") ||
		strings.Contains(dt, "C2960") || strings.Contains(dt, "9300") ||
		strings.Contains(dt, "9200") || strings.Contains(dt, "3850") ||
		strings.Contains(dt, "3750") || strings.Contains(dt, "2960") ||
		strings.Contains(dt, "L2") || strings.Contains(dt, "IOL") ||
		strings.Contains(dt, "I86BI") {
		return "L2-SWITCH"
	}

	// Default
	return "IOS-XE"
}

// ============================================================================
// FILE PARSERS
// ============================================================================

func parseCSV(filename string) (map[string]DeviceInfo, error) {
	devices := make(map[string]DeviceInfo)

	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.TrimLeadingSpace = true
	reader.FieldsPerRecord = -1

	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	for i, record := range records {
		if i == 0 {
			continue
		}
		if len(record) < 2 {
			continue
		}

		hostname := strings.TrimSpace(record[0])
		ipAddress := strings.TrimSpace(record[1])
		var deviceType, site, role string
		if len(record) > 2 {
			deviceType = strings.TrimSpace(record[2])
		}
		if len(record) > 3 {
			site = strings.TrimSpace(record[3])
		}
		if len(record) > 4 {
			role = strings.TrimSpace(record[4])
		}

		if hostname != "" && ipAddress != "" {
			detectedOS := detectDeviceOS(deviceType)
			devices[strings.ToUpper(hostname)] = DeviceInfo{
				Hostname:   hostname,
				IPAddress:  ipAddress,
				DeviceType: deviceType,
				Site:       site,
				Role:       role,
				DetectedOS: detectedOS,
			}
		}
	}

	return devices, nil
}

type xlsxSST struct {
	SI []struct {
		T string `xml:"t"`
	} `xml:"si"`
}

type xlsxWorksheet struct {
	SheetData struct {
		Rows []struct {
			Cells []struct {
				R string `xml:"r,attr"`
				T string `xml:"t,attr"`
				V string `xml:"v"`
			} `xml:"c"`
		} `xml:"row"`
	} `xml:"sheetData"`
}

func parseXLSX(filename string) (map[string]DeviceInfo, error) {
	devices := make(map[string]DeviceInfo)

	r, err := zip.OpenReader(filename)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	var sharedStrings []string
	for _, f := range r.File {
		if f.Name == "xl/sharedStrings.xml" {
			rc, _ := f.Open()
			content, _ := io.ReadAll(rc)
			rc.Close()
			var sst xlsxSST
			xml.Unmarshal(content, &sst)
			for _, si := range sst.SI {
				sharedStrings = append(sharedStrings, si.T)
			}
			break
		}
	}

	for _, f := range r.File {
		if strings.Contains(f.Name, "worksheets/sheet") {
			rc, _ := f.Open()
			content, _ := io.ReadAll(rc)
			rc.Close()

			var ws xlsxWorksheet
			xml.Unmarshal(content, &ws)

			for rowIdx, row := range ws.SheetData.Rows {
				if rowIdx == 0 {
					continue
				}

				rowData := make(map[string]string)
				for _, cell := range row.Cells {
					col := ""
					for _, c := range cell.R {
						if c >= 'A' && c <= 'Z' {
							col += string(c)
						} else {
							break
						}
					}
					val := cell.V
					if cell.T == "s" {
						idx, _ := strconv.Atoi(val)
						if idx < len(sharedStrings) {
							val = sharedStrings[idx]
						}
					}
					rowData[col] = strings.TrimSpace(val)
				}

				hostname := rowData["A"]
				ipAddress := rowData["B"]
				deviceType := rowData["C"]

				if hostname != "" && ipAddress != "" {
					detectedOS := detectDeviceOS(deviceType)
					devices[strings.ToUpper(hostname)] = DeviceInfo{
						Hostname:   hostname,
						IPAddress:  ipAddress,
						DeviceType: deviceType,
						Site:       rowData["D"],
						Role:       rowData["E"],
						DetectedOS: detectedOS,
					}
				}
			}
			break
		}
	}

	return devices, nil
}

func loadHostInventory(filename string) (map[string]DeviceInfo, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	if ext == ".csv" {
		return parseCSV(filename)
	}
	if ext == ".xlsx" {
		devices, err := parseXLSX(filename)
		if err != nil || len(devices) == 0 {
			csvFile := strings.TrimSuffix(filename, ext) + ".csv"
			if _, e := os.Stat(csvFile); e == nil {
				return parseCSV(csvFile)
			}
		}
		return devices, err
	}
	return parseCSV(filename)
}

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
		if line != "" && !strings.HasPrefix(line, "#") {
			lines = append(lines, line)
		}
	}
	return lines, scanner.Err()
}

// ============================================================================
// SSH CLIENT - Fixed for IOS-XR (no echo command available)
// ============================================================================

type SSHClient struct {
	host       string
	port       int
	username   string
	password   string
	cmdTimeout time.Duration
	deviceOS   string
}

func (c *SSHClient) ExecuteCommands(commands []string) (map[string]string, error) {
	results := make(map[string]string)

	// For IOS-XR: Execute each command separately using SSH command mode
	// This is because IOS-XR closes stdin-based sessions
	if c.deviceOS == "IOS-XR" {
		return c.executeIOSXRCommands(commands)
	}

	// For IOS-XE/L2: Use stdin with echo markers
	var script strings.Builder
	script.WriteString("terminal length 0\n")
	script.WriteString("terminal width 512\n")

	for i, cmd := range commands {
		script.WriteString(fmt.Sprintf("echo ===START_%d===\n", i))
		script.WriteString(cmd + "\n")
		script.WriteString(fmt.Sprintf("echo ===END_%d===\n", i))
	}
	script.WriteString("exit\n")

	sshArgs := []string{
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "ConnectTimeout=30",
		"-o", "LogLevel=ERROR",
		"-p", strconv.Itoa(c.port),
		"-t", "-t",
		fmt.Sprintf("%s@%s", c.username, c.host),
	}

	cmd := exec.Command("sshpass", append([]string{"-p", c.password, "ssh"}, sshArgs...)...)

	stdin, _ := cmd.StdinPipe()
	var output strings.Builder
	cmd.Stdout = &output
	cmd.Stderr = &output

	cmd.Start()
	io.WriteString(stdin, script.String())
	stdin.Close()

	done := make(chan error)
	go func() { done <- cmd.Wait() }()

	select {
	case <-done:
	case <-time.After(c.cmdTimeout):
		cmd.Process.Kill()
		return nil, fmt.Errorf("timeout")
	}

	fullOutput := output.String()

	// DEBUG: Save raw output to file for troubleshooting
	debugFile := fmt.Sprintf("/tmp/debug_%s_%s.txt", c.deviceOS, c.host)
	os.WriteFile(debugFile, []byte(fullOutput), 0644)

	// Parse using echo markers
	for i, cmdStr := range commands {
		start := fmt.Sprintf("===START_%d===", i)
		end := fmt.Sprintf("===END_%d===", i)

		startIdx := strings.Index(fullOutput, start)
		if startIdx == -1 {
			results[cmdStr] = "(no output)"
			continue
		}
		startIdx += len(start)

		endIdx := strings.Index(fullOutput[startIdx:], end)
		if endIdx == -1 {
			results[cmdStr] = strings.TrimSpace(fullOutput[startIdx:])
		} else {
			results[cmdStr] = cleanOutput(fullOutput[startIdx : startIdx+endIdx])
		}
	}

	return results, nil
}

// executeIOSXRCommands runs commands on IOS-XR using native Go SSH
func (c *SSHClient) executeIOSXRCommands(commands []string) (map[string]string, error) {
	results := make(map[string]string)
	var allOutput strings.Builder

	// SSH client configuration
	config := &ssh.ClientConfig{
		User: c.username,
		Auth: []ssh.AuthMethod{
			ssh.Password(c.password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	// Dial IOS XR
	addr := fmt.Sprintf("%s:%d", c.host, c.port)
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, fmt.Errorf("failed to dial: %w", err)
	}
	defer client.Close()

	// Start interactive session
	session, err := client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	// Request PTY (needed for IOS XR CLI)
	if err := session.RequestPty("vt100", 80, 40, ssh.TerminalModes{}); err != nil {
		return nil, fmt.Errorf("failed to request PTY: %w", err)
	}

	stdin, _ := session.StdinPipe()
	stdout, _ := session.StdoutPipe()

	// Start shell
	if err := session.Shell(); err != nil {
		return nil, fmt.Errorf("failed to start shell: %w", err)
	}

	// Send commands
	for _, cmdStr := range commands {
		fmt.Fprintf(stdin, "%s\n", cmdStr)
	}

	// Exit session cleanly
	fmt.Fprintln(stdin, "exit")

	// Capture output
	scanner := bufio.NewScanner(stdout)
	var currentCmd string
	for scanner.Scan() {
		line := scanner.Text()
		allOutput.WriteString(line + "\n")

		// crude detection: if line contains command string, switch context
		for _, cmdStr := range commands {
			if strings.Contains(line, cmdStr) {
				currentCmd = cmdStr
				results[currentCmd] = ""
			}
		}
		if currentCmd != "" {
			results[currentCmd] += line + "\n"
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading output: %w", err)
	}

	// DEBUG: Save all raw output
	debugFile := fmt.Sprintf("/tmp/debug_%s_%s.txt", c.deviceOS, c.host)
	_ = os.WriteFile(debugFile, []byte(allOutput.String()), 0644)

	// Clean outputs
	for cmdStr, out := range results {
		results[cmdStr] = cleanIOSXRCommandOutput(out)
	}

	return results, nil
}

// cleanIOSXRCommandOutput cleans output from IOS-XR SSH command execution
func cleanIOSXRCommandOutput(output string) string {
	lines := strings.Split(output, "\n")
	var clean []string

	// Compile prompt pattern once
	promptPattern := regexp.MustCompile(`^(RP/\d+/(RSP)?\d+/CPU\d+:)?[A-Za-z0-9_-]+#`)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Skip empty lines at start
		if len(clean) == 0 && trimmed == "" {
			continue
		}

		// Skip markers
		if strings.Contains(trimmed, "__MARKER_START_") || strings.Contains(trimmed, "__MARKER_END_") {
			continue
		}

		// Skip echo commands
		if strings.HasPrefix(trimmed, "echo __MARKER") {
			continue
		}

		// Skip IOS-XR prompts
		if promptPattern.MatchString(trimmed) {
			continue
		}

		// Skip terminal settings
		if strings.HasPrefix(trimmed, "terminal length") || strings.HasPrefix(trimmed, "terminal width") {
			continue
		}

		// Skip SSH warnings
		if strings.Contains(trimmed, "Warning:") && strings.Contains(trimmed, "known hosts") {
			continue
		}
		if strings.Contains(trimmed, "Pseudo-terminal") {
			continue
		}
		if strings.Contains(trimmed, "Connection to") && strings.Contains(trimmed, "closed") {
			continue
		}

		// Skip the IMPORTANT license banner (common on XRv)
		if strings.Contains(trimmed, "IMPORTANT:") && strings.Contains(trimmed, "READ CAREFULLY") {
			continue
		}
		if strings.Contains(trimmed, "Demo Version") || strings.Contains(trimmed, "XRv") && strings.Contains(trimmed, "Software") {
			continue
		}
		if strings.Contains(trimmed, "End User License") || strings.Contains(trimmed, "License Agreement") {
			continue
		}
		if strings.Contains(trimmed, "cisco.com/go/terms") {
			continue
		}
		if strings.Contains(trimmed, "demonstration and evaluation") {
			continue
		}
		if strings.Contains(trimmed, "non-production environment") {
			continue
		}
		if strings.Contains(trimmed, "Downloading, installing") {
			continue
		}
		if strings.Contains(trimmed, "binding yourself") {
			continue
		}
		if strings.Contains(trimmed, "unwilling to license") {
			continue
		}
		if strings.Contains(trimmed, "return the Software") {
			continue
		}
		if strings.Contains(trimmed, "Please login with") {
			continue
		}
		if strings.Contains(trimmed, "configured user/password") {
			continue
		}

		clean = append(clean, line)
	}

	return strings.TrimSpace(strings.Join(clean, "\n"))
}

func cleanOutput(s string) string {
	lines := strings.Split(s, "\n")
	var clean []string
	for _, line := range lines {
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, "terminal ") || strings.Contains(t, "===") {
			continue
		}
		if strings.Contains(t, "Pseudo-terminal") {
			continue
		}
		// Skip IOS.sh shell warning messages
		if strings.Contains(t, "IOS.sh") ||
			strings.Contains(t, "shell is currently disabled") ||
			strings.Contains(t, "term shell") ||
			strings.Contains(t, "shell processing full") ||
			strings.Contains(t, "man command") ||
			strings.Contains(t, "man IOS.sh") ||
			strings.Contains(t, "The command you have entered") ||
			strings.Contains(t, "You can enable") ||
			strings.Contains(t, "You can also enable") ||
			strings.Contains(t, "For more information") ||
			strings.Contains(t, "However, the shell") ||
			strings.Contains(t, "There is additional information") {
			continue
		}
		// Skip prompts with commands echoed (e.g., "Switch1#show ip interface brief")
		if regexp.MustCompile(`^[A-Za-z0-9_-]+[#>]`).MatchString(t) && strings.Contains(t, "show ") {
			continue
		}
		// Skip standalone prompts
		if regexp.MustCompile(`^[A-Za-z0-9_-]+[#>]$`).MatchString(t) {
			continue
		}
		clean = append(clean, line)
	}
	return strings.TrimSpace(strings.Join(clean, "\n"))
}

// ============================================================================
// COMMAND LOADER
// ============================================================================

type CommandSet struct {
	IOSXR    []string
	IOSXE    []string
	L2Switch []string
	Default  []string
}

func loadAllCommands(config *Config) (*CommandSet, error) {
	cs := &CommandSet{}

	if cmds, err := readLines(config.CommandFileXR); err == nil {
		cs.IOSXR = cmds
		log.Printf("✓ Loaded %d IOS-XR commands from %s", len(cmds), config.CommandFileXR)
	} else {
		log.Printf("✗ IOS-XR commands not found: %s", config.CommandFileXR)
	}

	if cmds, err := readLines(config.CommandFileXE); err == nil {
		cs.IOSXE = cmds
		log.Printf("✓ Loaded %d IOS-XE commands from %s", len(cmds), config.CommandFileXE)
	} else {
		log.Printf("✗ IOS-XE commands not found: %s", config.CommandFileXE)
	}

	if cmds, err := readLines(config.CommandFileL2); err == nil {
		cs.L2Switch = cmds
		log.Printf("✓ Loaded %d L2-Switch commands from %s", len(cmds), config.CommandFileL2)
	} else {
		log.Printf("✗ L2-Switch commands not found: %s", config.CommandFileL2)
	}

	if cmds, err := readLines(config.CommandFile); err == nil {
		cs.Default = cmds
		log.Printf("✓ Loaded %d default commands from %s", len(cmds), config.CommandFile)
	} else {
		return nil, fmt.Errorf("default commands required: %s", config.CommandFile)
	}

	return cs, nil
}

func (cs *CommandSet) GetCommandsForOS(osType string) []string {
	switch osType {
	case "IOS-XR":
		if len(cs.IOSXR) > 0 {
			return cs.IOSXR
		}
	case "IOS-XE":
		if len(cs.IOSXE) > 0 {
			return cs.IOSXE
		}
	case "L2-SWITCH":
		if len(cs.L2Switch) > 0 {
			return cs.L2Switch
		}
	}
	return cs.Default
}

// ============================================================================
// DEVICE RESULT & PROCESSING
// ============================================================================

type DeviceResult struct {
	Device       DeviceInfo
	Results      []ExecutionResult
	Success      bool
	ErrorMessage string
	CommandFile  string
}

func processDevice(device DeviceInfo, config *Config, commands *CommandSet) *DeviceResult {
	result := &DeviceResult{
		Device:  device,
		Results: []ExecutionResult{},
		Success: true,
	}

	osType := device.DetectedOS
	cmds := commands.GetCommandsForOS(osType)

	switch osType {
	case "IOS-XR":
		result.CommandFile = config.CommandFileXR
	case "IOS-XE":
		result.CommandFile = config.CommandFileXE
	case "L2-SWITCH":
		result.CommandFile = config.CommandFileL2
	default:
		result.CommandFile = config.CommandFile
	}

	if config.Verbose {
		log.Printf("  → %s (%s) | Type: %s | OS: %s | Cmds: %d",
			device.Hostname, device.IPAddress, device.DeviceType, osType, len(cmds))
	}

	if config.DryRun {
		return result
	}

	client := &SSHClient{
		host:       device.IPAddress,
		port:       config.SSHPort,
		username:   config.Username,
		password:   config.Password,
		cmdTimeout: config.CmdTimeout,
		deviceOS:   osType,
	}

	startTime := time.Now()
	outputs, err := client.ExecuteCommands(cmds)
	duration := time.Since(startTime)

	if err != nil {
		result.Success = false
		result.ErrorMessage = err.Error()
		return result
	}

	for _, cmd := range cmds {
		result.Results = append(result.Results, ExecutionResult{
			Hostname:  device.Hostname,
			IPAddress: device.IPAddress,
			Command:   cmd,
			Output:    outputs[cmd],
			Duration:  duration / time.Duration(len(cmds)),
		})
	}

	return result
}

// ============================================================================
// OUTPUT WRITER WITH COMPARISON SUMMARY
// ============================================================================

type OutputWriter struct {
	dir       string
	phase     string
	timestamp string
}

func NewOutputWriter(outputDir, phase string) *OutputWriter {
	ts := time.Now().Format("20060102_150405")
	dir := filepath.Join(outputDir, phase, ts)
	os.MkdirAll(dir, 0755)
	return &OutputWriter{dir: dir, phase: phase, timestamp: ts}
}

func (w *OutputWriter) WriteDevice(result *DeviceResult) error {
	filename := filepath.Join(w.dir, fmt.Sprintf("%s_%s.log", result.Device.Hostname, w.timestamp))
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	fmt.Fprintf(file, "================================================================================\n")
	fmt.Fprintf(file, " MERALCO Network Health Check Logger v%s\n", Version)
	fmt.Fprintf(file, " Phase: %s\n", w.phase)
	fmt.Fprintf(file, "================================================================================\n")
	fmt.Fprintf(file, " Hostname:     %s\n", result.Device.Hostname)
	fmt.Fprintf(file, " IP Address:   %s\n", result.Device.IPAddress)
	fmt.Fprintf(file, " Device Type:  %s\n", result.Device.DeviceType)
	fmt.Fprintf(file, " Detected OS:  %s\n", result.Device.DetectedOS)
	fmt.Fprintf(file, " Command File: %s\n", result.CommandFile)
	fmt.Fprintf(file, " Timestamp:    %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Fprintf(file, "================================================================================\n\n")

	if !result.Success {
		fmt.Fprintf(file, "ERROR: %s\n\n", result.ErrorMessage)
	}

	for _, r := range result.Results {
		fmt.Fprintf(file, "--------------------------------------------------------------------------------\n")
		fmt.Fprintf(file, " Command: %s\n", r.Command)
		fmt.Fprintf(file, "--------------------------------------------------------------------------------\n")
		if r.Output == "" {
			fmt.Fprintf(file, "(no output)\n")
		} else {
			fmt.Fprintf(file, "%s\n", r.Output)
		}
		fmt.Fprintf(file, "\n")
	}

	fmt.Fprintf(file, "================================================================================\n")
	return nil
}

// WriteSummaryCSV creates a CSV summary of key metrics per device/command
func (w *OutputWriter) WriteSummaryCSV(results []*DeviceResult) error {
	filename := filepath.Join(w.dir, fmt.Sprintf("SUMMARY_%s.csv", w.timestamp))
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	// Write CSV header
	fmt.Fprintf(file, "Phase,Timestamp,Hostname,IP,DeviceType,OS,Command,MetricName,MetricValue\n")

	for _, r := range results {
		if !r.Success {
			fmt.Fprintf(file, "%s,%s,%s,%s,%s,%s,CONNECTION,Status,FAILED\n",
				w.phase, w.timestamp, r.Device.Hostname, r.Device.IPAddress,
				r.Device.DeviceType, r.Device.DetectedOS)
			continue
		}

		for _, exec := range r.Results {
			metrics := extractMetrics(exec.Command, exec.Output)
			for metricName, metricValue := range metrics {
				fmt.Fprintf(file, "%s,%s,%s,%s,%s,%s,%s,%s,%s\n",
					w.phase, w.timestamp, r.Device.Hostname, r.Device.IPAddress,
					r.Device.DeviceType, r.Device.DetectedOS,
					exec.Command, metricName, metricValue)
			}
		}
	}

	return nil
}

// extractMetrics parses command output and extracts key metrics
func extractMetrics(command, output string) map[string]string {
	metrics := make(map[string]string)
	lines := strings.Split(output, "\n")

	switch {
	case strings.Contains(command, "show version"):
		for _, line := range lines {
			if strings.Contains(line, "uptime is") {
				metrics["Uptime"] = extractAfter(line, "uptime is")
			}
			if strings.Contains(line, "Version") || strings.Contains(line, "version") {
				if strings.Contains(line, "IOS") || strings.Contains(line, "Cisco") {
					metrics["Version"] = strings.TrimSpace(line)
				}
			}
		}

	case strings.Contains(command, "ospf neighbor"):
		count := 0
		fullCount := 0
		for _, line := range lines {
			if strings.Contains(line, "FULL") {
				fullCount++
			}
			// Count lines with IP addresses (neighbor entries)
			if regexp.MustCompile(`\d+\.\d+\.\d+\.\d+`).MatchString(line) {
				count++
			}
		}
		metrics["OSPF_Neighbors_Total"] = strconv.Itoa(count)
		metrics["OSPF_Neighbors_FULL"] = strconv.Itoa(fullCount)

	case strings.Contains(command, "bgp summary") || strings.Contains(command, "bgp vpnv4"):
		established := 0
		total := 0
		for _, line := range lines {
			fields := strings.Fields(line)
			if len(fields) >= 3 && regexp.MustCompile(`^\d+\.\d+\.\d+\.\d+$`).MatchString(fields[0]) {
				total++
				// Check if state/pfxrcd is a number (established) not a state string
				lastField := fields[len(fields)-1]
				if _, err := strconv.Atoi(lastField); err == nil {
					established++
				}
			}
		}
		metrics["BGP_Neighbors_Total"] = strconv.Itoa(total)
		metrics["BGP_Neighbors_Established"] = strconv.Itoa(established)

	case strings.Contains(command, "mpls ldp neighbor"):
		count := 0
		for _, line := range lines {
			if regexp.MustCompile(`\d+\.\d+\.\d+\.\d+`).MatchString(line) {
				count++
			}
		}
		metrics["LDP_Neighbors"] = strconv.Itoa(count)

	case strings.Contains(command, "interface") && strings.Contains(command, "brief"):
		up := 0
		down := 0
		admin_down := 0
		for _, line := range lines {
			lineLower := strings.ToLower(line)
			if strings.Contains(lineLower, "up") && strings.Contains(lineLower, "up") {
				up++
			} else if strings.Contains(lineLower, "administratively") || strings.Contains(lineLower, "admin") {
				admin_down++
			} else if strings.Contains(lineLower, "down") {
				down++
			}
		}
		metrics["Interfaces_Up"] = strconv.Itoa(up)
		metrics["Interfaces_Down"] = strconv.Itoa(down)
		metrics["Interfaces_AdminDown"] = strconv.Itoa(admin_down)

	case strings.Contains(command, "show vrf"):
		count := 0
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" && !strings.HasPrefix(line, "Name") && !strings.HasPrefix(line, "VRF") {
				if !strings.Contains(line, "-----") {
					count++
				}
			}
		}
		metrics["VRF_Count"] = strconv.Itoa(count)

	case strings.Contains(command, "bfd"):
		up := 0
		down := 0
		for _, line := range lines {
			if strings.Contains(strings.ToLower(line), "up") {
				up++
			}
			if strings.Contains(strings.ToLower(line), "down") {
				down++
			}
		}
		metrics["BFD_Sessions_Up"] = strconv.Itoa(up)
		metrics["BFD_Sessions_Down"] = strconv.Itoa(down)

	case strings.Contains(command, "xconnect") || strings.Contains(command, "l2vpn"):
		up := 0
		down := 0
		for _, line := range lines {
			if strings.Contains(strings.ToUpper(line), "UP") {
				up++
			}
			if strings.Contains(strings.ToUpper(line), "DOWN") {
				down++
			}
		}
		metrics["L2VPN_Up"] = strconv.Itoa(up)
		metrics["L2VPN_Down"] = strconv.Itoa(down)
	}

	// If no specific metrics extracted, just note it was captured
	if len(metrics) == 0 {
		metrics["Captured"] = "Yes"
		metrics["OutputLines"] = strconv.Itoa(len(lines))
	}

	return metrics
}

func extractAfter(line, marker string) string {
	idx := strings.Index(line, marker)
	if idx == -1 {
		return ""
	}
	return strings.TrimSpace(line[idx+len(marker):])
}

func (w *OutputWriter) WriteSummary(results []*DeviceResult) error {
	filename := filepath.Join(w.dir, fmt.Sprintf("SUMMARY_%s.log", w.timestamp))
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	success, fail := 0, 0
	for _, r := range results {
		if r.Success {
			success++
		} else {
			fail++
		}
	}

	fmt.Fprintf(file, "================================================================================\n")
	fmt.Fprintf(file, " MERALCO Health Check Summary v%s\n", Version)
	fmt.Fprintf(file, " Phase: %s | Time: %s\n", w.phase, time.Now().Format("2006-01-02 15:04:05"))
	fmt.Fprintf(file, "================================================================================\n\n")
	fmt.Fprintf(file, " Total: %d | Success: %d | Failed: %d | Rate: %.1f%%\n\n",
		len(results), success, fail, float64(success)/float64(len(results))*100)

	fmt.Fprintf(file, "%-12s %-15s %-10s %-10s %-10s %s\n",
		"HOSTNAME", "IP", "TYPE", "OS", "STATUS", "CMD_FILE")
	fmt.Fprintf(file, "--------------------------------------------------------------------------------\n")

	for _, r := range results {
		status := "SUCCESS"
		if !r.Success {
			status = "FAILED"
		}
		fmt.Fprintf(file, "%-12s %-15s %-10s %-10s %-10s %s\n",
			r.Device.Hostname, r.Device.IPAddress, r.Device.DeviceType,
			r.Device.DetectedOS, status, filepath.Base(r.CommandFile))
	}

	fmt.Fprintf(file, "--------------------------------------------------------------------------------\n")
	fmt.Fprintf(file, "\nOutput: %s\n", w.dir)
	fmt.Fprintf(file, "CSV Summary: SUMMARY_%s.csv\n", w.timestamp)
	return nil
}

// ============================================================================
// COMPARISON FUNCTION - Pre vs Post Migration
// ============================================================================

func comparePhases(preDir, postDir, outputFile string) error {
	// Find CSV files
	preCSV := findCSVFile(preDir)
	postCSV := findCSVFile(postDir)

	if preCSV == "" || postCSV == "" {
		return fmt.Errorf("could not find CSV summary files in pre/post directories")
	}

	log.Printf("Comparing:\n  Pre:  %s\n  Post: %s", preCSV, postCSV)

	// Load data
	preData, err := loadCSVData(preCSV)
	if err != nil {
		return fmt.Errorf("failed to load pre-migration data: %v", err)
	}

	postData, err := loadCSVData(postCSV)
	if err != nil {
		return fmt.Errorf("failed to load post-migration data: %v", err)
	}

	// Create comparison report
	file, err := os.Create(outputFile)
	if err != nil {
		return err
	}
	defer file.Close()

	fmt.Fprintf(file, "================================================================================\n")
	fmt.Fprintf(file, " MERALCO Pre/Post Migration Comparison Report\n")
	fmt.Fprintf(file, " Generated: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Fprintf(file, "================================================================================\n\n")

	// Get all unique hostnames
	hostnames := make(map[string]bool)
	for key := range preData {
		parts := strings.Split(key, "|")
		if len(parts) > 0 {
			hostnames[parts[0]] = true
		}
	}
	for key := range postData {
		parts := strings.Split(key, "|")
		if len(parts) > 0 {
			hostnames[parts[0]] = true
		}
	}

	// Sort hostnames
	var sortedHosts []string
	for h := range hostnames {
		sortedHosts = append(sortedHosts, h)
	}
	sort.Strings(sortedHosts)

	// Compare each host
	for _, hostname := range sortedHosts {
		fmt.Fprintf(file, "\n=== %s ===\n", hostname)
		fmt.Fprintf(file, "%-40s %-15s %-15s %-10s\n", "Metric", "Pre-Migration", "Post-Migration", "Status")
		fmt.Fprintf(file, "--------------------------------------------------------------------------------\n")

		// Get all metrics for this host
		metrics := make(map[string]bool)
		for key := range preData {
			if strings.HasPrefix(key, hostname+"|") {
				parts := strings.Split(key, "|")
				if len(parts) >= 2 {
					metrics[parts[1]] = true
				}
			}
		}
		for key := range postData {
			if strings.HasPrefix(key, hostname+"|") {
				parts := strings.Split(key, "|")
				if len(parts) >= 2 {
					metrics[parts[1]] = true
				}
			}
		}

		for metric := range metrics {
			key := hostname + "|" + metric
			preVal := preData[key]
			postVal := postData[key]

			status := "OK"
			if preVal != postVal {
				// Check if it's a numeric comparison
				preNum, preErr := strconv.Atoi(preVal)
				postNum, postErr := strconv.Atoi(postVal)
				if preErr == nil && postErr == nil {
					if postNum < preNum {
						status = "⚠ DECREASED"
					} else if postNum > preNum {
						status = "↑ INCREASED"
					} else {
						status = "OK"
					}
				} else {
					status = "CHANGED"
				}
			}

			fmt.Fprintf(file, "%-40s %-15s %-15s %-10s\n", metric, preVal, postVal, status)
		}
	}

	fmt.Fprintf(file, "\n================================================================================\n")
	fmt.Fprintf(file, " End of Comparison Report\n")
	fmt.Fprintf(file, "================================================================================\n")

	return nil
}

func findCSVFile(dir string) string {
	files, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, f := range files {
		if strings.HasPrefix(f.Name(), "SUMMARY_") && strings.HasSuffix(f.Name(), ".csv") {
			return filepath.Join(dir, f.Name())
		}
	}
	// Check subdirectories
	for _, f := range files {
		if f.IsDir() {
			subFiles, _ := os.ReadDir(filepath.Join(dir, f.Name()))
			for _, sf := range subFiles {
				if strings.HasPrefix(sf.Name(), "SUMMARY_") && strings.HasSuffix(sf.Name(), ".csv") {
					return filepath.Join(dir, f.Name(), sf.Name())
				}
			}
		}
	}
	return ""
}

func loadCSVData(filename string) (map[string]string, error) {
	data := make(map[string]string)

	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	for i, record := range records {
		if i == 0 {
			continue // Skip header
		}
		if len(record) >= 9 {
			// Key: Hostname|Command|MetricName
			hostname := record[2]
			command := record[6]
			metricName := record[7]
			metricValue := record[8]
			key := fmt.Sprintf("%s|%s_%s", hostname, command, metricName)
			data[key] = metricValue
		}
	}

	return data, nil
}

// ============================================================================
// MAIN
// ============================================================================

func main() {
	config := parseFlags()
	fmt.Printf(Banner, Version)

	// Handle comparison mode
	if config.CompareDir != "" {
		parts := strings.Split(config.CompareDir, ",")
		if len(parts) != 2 {
			log.Fatal("Compare requires: -compare pre_dir,post_dir")
		}
		outputFile := filepath.Join(config.OutputDir, "COMPARISON_REPORT.txt")
		if err := comparePhases(parts[0], parts[1], outputFile); err != nil {
			log.Fatalf("Comparison failed: %v", err)
		}
		log.Printf("Comparison report: %s", outputFile)
		return
	}

	if config.Username == "" || config.Password == "" {
		log.Fatal("Username (-u) and password (-p) required")
	}

	log.Printf("Loading inventory from %s...", config.HostFile)
	devices, err := loadHostInventory(config.HostFile)
	if err != nil {
		log.Fatalf("Failed to load inventory: %v", err)
	}
	log.Printf("Loaded %d devices\n", len(devices))

	fmt.Println("\n--- Device OS Detection ---")
	fmt.Printf("%-12s %-15s %-10s → %-10s\n", "HOSTNAME", "IP", "TYPE", "DETECTED_OS")
	fmt.Println(strings.Repeat("-", 55))
	for _, d := range devices {
		fmt.Printf("%-12s %-15s %-10s → %-10s\n", d.Hostname, d.IPAddress, d.DeviceType, d.DetectedOS)
	}
	fmt.Println()

	targets, err := readLines(config.TargetFile)
	if err != nil {
		log.Fatalf("Failed to read targets: %v", err)
	}

	commands, err := loadAllCommands(config)
	if err != nil {
		log.Fatalf("Failed to load commands: %v", err)
	}
	fmt.Println()

	var targetDevices []DeviceInfo
	for _, t := range targets {
		if d, ok := devices[strings.ToUpper(t)]; ok {
			targetDevices = append(targetDevices, d)
		} else {
			log.Printf("WARNING: %s not in inventory", t)
		}
	}

	if len(targetDevices) == 0 {
		log.Fatal("No valid devices")
	}

	log.Printf("Processing %d devices with %d workers...\n", len(targetDevices), config.MaxWorkers)

	writer := NewOutputWriter(config.OutputDir, config.Phase)

	deviceChan := make(chan DeviceInfo, len(targetDevices))
	resultChan := make(chan *DeviceResult, len(targetDevices))

	var wg sync.WaitGroup
	for i := 0; i < config.MaxWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for d := range deviceChan {
				resultChan <- processDevice(d, config, commands)
			}
		}()
	}

	go func() {
		for _, d := range targetDevices {
			deviceChan <- d
		}
		close(deviceChan)
	}()

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	var allResults []*DeviceResult
	for r := range resultChan {
		allResults = append(allResults, r)
		writer.WriteDevice(r)
		status := "✓ SUCCESS"
		if !r.Success {
			status = "✗ FAILED"
		}
		log.Printf("%s: %s [%s] using %s", status, r.Device.Hostname, r.Device.DetectedOS, filepath.Base(r.CommandFile))
	}

	writer.WriteSummary(allResults)
	writer.WriteSummaryCSV(allResults)

	success := 0
	for _, r := range allResults {
		if r.Success {
			success++
		}
	}

	fmt.Printf("\n================================================================================\n")
	fmt.Printf(" COMPLETE: %d/%d successful\n", success, len(allResults))
	fmt.Printf(" Output:   %s\n", writer.dir)
	fmt.Printf(" Summary:  SUMMARY_%s.log\n", writer.timestamp)
	fmt.Printf(" CSV:      SUMMARY_%s.csv (for comparison)\n", writer.timestamp)
	fmt.Printf("================================================================================\n")
}

func parseFlags() *Config {
	config := &Config{}
	flag.StringVar(&config.Username, "u", "", "SSH username")
	flag.StringVar(&config.Password, "p", "", "SSH password")
	flag.StringVar(&config.CommandFile, "c", "command.txt", "Default commands")
	flag.StringVar(&config.CommandFileXR, "cmd-xr", "command_iosxr.txt", "IOS-XR commands")
	flag.StringVar(&config.CommandFileXE, "cmd-xe", "command_iosxe.txt", "IOS-XE commands")
	flag.StringVar(&config.CommandFileL2, "cmd-l2", "command_l2switch.txt", "L2 Switch commands")
	flag.StringVar(&config.TargetFile, "t", "target.txt", "Target file")
	flag.StringVar(&config.HostFile, "hosts", "host_info.csv", "Host inventory")
	flag.StringVar(&config.OutputDir, "o", "output", "Output directory")
	flag.IntVar(&config.MaxWorkers, "w", 5, "Workers")
	flag.IntVar(&config.SSHPort, "port", 22, "SSH port")
	flag.BoolVar(&config.Verbose, "v", false, "Verbose")
	flag.BoolVar(&config.DryRun, "dry-run", false, "Dry run")
	flag.StringVar(&config.Phase, "phase", "health-check", "Phase (pre-migration/post-migration)")
	flag.StringVar(&config.CompareDir, "compare", "", "Compare pre,post directories")
	var timeout int
	flag.IntVar(&timeout, "timeout", 180, "Timeout (seconds)")
	flag.Parse()
	config.CmdTimeout = time.Duration(timeout) * time.Second
	return config
}
