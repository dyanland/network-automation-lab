/*
================================================================================
MERALCO Network Health Check Logger v2.2
Fixed: Proper OS-specific command selection
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
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	Version = "2.2.0"
	Banner  = `
================================================================================
  MERALCO Network Health Check Logger v%s
  Fixed: Proper OS-specific command selection
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
	DetectedOS string // NEW: Store detected OS
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
}

// ============================================================================
// OS DETECTION - IMPROVED WITH EXPLICIT MAPPING
// ============================================================================

func detectDeviceOS(deviceType string) string {
	dt := strings.ToUpper(strings.TrimSpace(deviceType))
	
	// IOS-XR patterns (ASR9K family, NCS, XRv)
	iosxrPatterns := []string{
		"ASR9", "ASR-9", "ASR 9",  // ASR9000 series
		"XRV", "IOS-XR", "IOSXR",  // XRv and explicit IOS-XR
		"NCS",                      // NCS series
		"CRS",                      // CRS series
	}
	for _, pattern := range iosxrPatterns {
		if strings.Contains(dt, pattern) {
			return "IOS-XR"
		}
	}
	
	// IOS-XE patterns (ASR1K, ASR903/920, ISR, CSR, Cat9K)
	iosxePatterns := []string{
		"ASR1", "ASR-1", "ASR 1",      // ASR1000 series
		"ASR903", "ASR-903", "ASR 903", // ASR903
		"ASR920", "ASR-920", "ASR 920", // ASR920
		"ISR", "CSR",                   // ISR/CSR routers
		"IOS-XE", "IOSXE",              // Explicit IOS-XE
		"IOSV", "IOS-V", "VIOS",        // IOSv (virtual)
		"C8", "C11", "C12",             // Catalyst 8000/ISR
	}
	for _, pattern := range iosxePatterns {
		if strings.Contains(dt, pattern) {
			return "IOS-XE"
		}
	}
	
	// L2 Switch patterns
	switchPatterns := []string{
		"SWITCH", "SW",
		"CAT", "CATALYST",
		"C9300", "C9200", "C9400", "C9500", "C9600",
		"C3850", "C3750", "C3650", "C3560",
		"C2960", "C2950",
		"9300", "9200", "9400", "3850", "3750", "2960",
		"L2", "L3-SW",
		"IOL", "I86BI", // IOS-on-Linux (lab switches)
	}
	for _, pattern := range switchPatterns {
		if strings.Contains(dt, pattern) {
			return "L2-SWITCH"
		}
	}
	
	// Default to IOS-XE if unknown
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
			continue // Skip header
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

// XLSX parser structures
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

	// Read shared strings
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

	// Read worksheet
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
			// Try CSV fallback
			csvFile := strings.TrimSuffix(filename, ext) + ".csv"
			if _, e := os.Stat(csvFile); e == nil {
				log.Printf("XLSX failed, using CSV: %s", csvFile)
				return parseCSV(csvFile)
			}
		}
		return devices, err
	}
	return parseCSV(filename) // Default to CSV parsing
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
// SSH CLIENT - SINGLE SESSION
// ============================================================================

type SSHClient struct {
	host       string
	port       int
	username   string
	password   string
	cmdTimeout time.Duration
}

func (c *SSHClient) ExecuteCommands(commands []string) (map[string]string, error) {
	results := make(map[string]string)

	// Build script with markers
	var script strings.Builder
	script.WriteString("terminal length 0\n")
	script.WriteString("terminal width 512\n")

	for i, cmd := range commands {
		script.WriteString(fmt.Sprintf("echo ===START_%d===\n", i))
		script.WriteString(cmd + "\n")
		script.WriteString(fmt.Sprintf("echo ===END_%d===\n", i))
	}
	script.WriteString("exit\n")

	// SSH with forced PTY
	sshArgs := []string{
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "ConnectTimeout=30",
		"-o", "LogLevel=ERROR",
		"-p", strconv.Itoa(c.port),
		"-t", "-t", // Force PTY
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

	// Parse output
	fullOutput := output.String()
	for i, cmdStr := range commands {
		start := fmt.Sprintf("===START_%d===", i)
		end := fmt.Sprintf("===END_%d===", i)

		startIdx := strings.Index(fullOutput, start)
		if startIdx == -1 {
			results[cmdStr] = "(no output captured)"
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
		if regexp.MustCompile(`^[A-Za-z0-9_-]+[#>]$`).MatchString(t) {
			continue
		}
		clean = append(clean, line)
	}
	return strings.TrimSpace(strings.Join(clean, "\n"))
}

// ============================================================================
// COMMAND LOADER - WITH OS MAPPING
// ============================================================================

type CommandSet struct {
	IOSXR    []string
	IOSXE    []string
	L2Switch []string
	Default  []string
}

func loadAllCommands(config *Config) (*CommandSet, error) {
	cs := &CommandSet{}

	// Load IOS-XR commands
	if cmds, err := readLines(config.CommandFileXR); err == nil {
		cs.IOSXR = cmds
		log.Printf("✓ Loaded %d IOS-XR commands from %s", len(cmds), config.CommandFileXR)
	} else {
		log.Printf("✗ IOS-XR command file not found: %s", config.CommandFileXR)
	}

	// Load IOS-XE commands
	if cmds, err := readLines(config.CommandFileXE); err == nil {
		cs.IOSXE = cmds
		log.Printf("✓ Loaded %d IOS-XE commands from %s", len(cmds), config.CommandFileXE)
	} else {
		log.Printf("✗ IOS-XE command file not found: %s", config.CommandFileXE)
	}

	// Load L2 Switch commands
	if cmds, err := readLines(config.CommandFileL2); err == nil {
		cs.L2Switch = cmds
		log.Printf("✓ Loaded %d L2-Switch commands from %s", len(cmds), config.CommandFileL2)
	} else {
		log.Printf("✗ L2-Switch command file not found: %s", config.CommandFileL2)
	}

	// Load default commands
	if cmds, err := readLines(config.CommandFile); err == nil {
		cs.Default = cmds
		log.Printf("✓ Loaded %d default commands from %s", len(cmds), config.CommandFile)
	} else {
		return nil, fmt.Errorf("default command file required: %s", config.CommandFile)
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
// WORKER
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

	// Get commands for this device's OS
	osType := device.DetectedOS
	cmds := commands.GetCommandsForOS(osType)

	// Determine which file was used
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
		log.Printf("  → %s (%s) | Type: %s | OS: %s | Commands: %d from %s",
			device.Hostname, device.IPAddress, device.DeviceType, osType, len(cmds), result.CommandFile)
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
// OUTPUT WRITER
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

	fmt.Fprintf(file, "%-12s %-15s %-12s %-10s %-10s %s\n",
		"HOSTNAME", "IP", "TYPE", "OS", "STATUS", "CMD_FILE")
	fmt.Fprintf(file, "--------------------------------------------------------------------------------\n")

	for _, r := range results {
		status := "SUCCESS"
		if !r.Success {
			status = "FAILED"
		}
		fmt.Fprintf(file, "%-12s %-15s %-12s %-10s %-10s %s\n",
			r.Device.Hostname, r.Device.IPAddress, r.Device.DeviceType,
			r.Device.DetectedOS, status, filepath.Base(r.CommandFile))
	}

	fmt.Fprintf(file, "--------------------------------------------------------------------------------\n")
	fmt.Fprintf(file, "\nOutput: %s\n", w.dir)
	return nil
}

// ============================================================================
// MAIN
// ============================================================================

func main() {
	config := parseFlags()
	fmt.Printf(Banner, Version)

	if config.Username == "" || config.Password == "" {
		log.Fatal("Username (-u) and password (-p) required")
	}

	// Load host inventory
	log.Printf("Loading inventory from %s...", config.HostFile)
	devices, err := loadHostInventory(config.HostFile)
	if err != nil {
		log.Fatalf("Failed to load inventory: %v", err)
	}
	log.Printf("Loaded %d devices\n", len(devices))

	// Show OS detection results
	fmt.Println("\n--- Device OS Detection ---")
	fmt.Printf("%-12s %-15s %-12s → %-10s\n", "HOSTNAME", "IP", "TYPE", "DETECTED_OS")
	fmt.Println(strings.Repeat("-", 60))
	for _, d := range devices {
		fmt.Printf("%-12s %-15s %-12s → %-10s\n", d.Hostname, d.IPAddress, d.DeviceType, d.DetectedOS)
	}
	fmt.Println()

	// Load targets
	targets, err := readLines(config.TargetFile)
	if err != nil {
		log.Fatalf("Failed to read targets: %v", err)
	}

	// Load commands
	commands, err := loadAllCommands(config)
	if err != nil {
		log.Fatalf("Failed to load commands: %v", err)
	}
	fmt.Println()

	// Match targets
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

	// Process devices
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

	success := 0
	for _, r := range allResults {
		if r.Success {
			success++
		}
	}

	fmt.Printf("\n================================================================================\n")
	fmt.Printf(" COMPLETE: %d/%d successful\n", success, len(allResults))
	fmt.Printf(" Output: %s\n", writer.dir)
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
	flag.StringVar(&config.Phase, "phase", "health-check", "Phase")
	var timeout int
	flag.IntVar(&timeout, "timeout", 180, "Timeout (seconds)")
	flag.Parse()
	config.CmdTimeout = time.Duration(timeout) * time.Second
	return config
}
