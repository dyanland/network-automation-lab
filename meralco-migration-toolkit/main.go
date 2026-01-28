// MERALCO Network Migration Validation Toolkit v2.0
// Cross-platform consolidated tool for ASR9010 to ASR9906 migration
// Build: go build -o meralco-toolkit main.go

package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

const AppVersion = "2.0.0"

// ============ Data Structures ============

type DeviceInfo struct {
	Hostname   string `json:"hostname"`
	IPAddress  string `json:"ip_address"`
	DeviceType string `json:"device_type"`
	Role       string `json:"role"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	SSHPort    int    `json:"ssh_port"`
}

type VRFInfo struct {
	Name        string `json:"name"`
	RD          string `json:"rd"`
	RTImport    string `json:"rt_import"`
	RTExport    string `json:"rt_export"`
	Description string `json:"description"`
	Priority    string `json:"priority"`
	Source      string `json:"source"` // "static" or device hostname
}

// DeviceVRF stores VRFs collected from a specific device
type DeviceVRF struct {
	DeviceHostname string    `json:"device_hostname"`
	VRFs           []VRFInfo `json:"vrfs"`
	CollectedAt    time.Time `json:"collected_at"`
}

var deviceVRFs = make(map[string]DeviceVRF) // VRFs per device

type ValidationResult struct {
	CheckName string    `json:"check_name"`
	Device    string    `json:"device"`
	Status    string    `json:"status"`
	Details   string    `json:"details"`
	Timestamp time.Time `json:"timestamp"`
}

type MigrationBaseline struct {
	Timestamp time.Time                 `json:"timestamp"`
	Devices   map[string]DeviceBaseline `json:"devices"`
}

type DeviceBaseline struct {
	Hostname      string         `json:"hostname"`
	BGPSessions   int            `json:"bgp_sessions"`
	BGPPrefixes   int            `json:"bgp_prefixes"`
	OSPFNeighbors int            `json:"ospf_neighbors"`
	LDPNeighbors  int            `json:"ldp_neighbors"`
	MPLSLabels    int            `json:"mpls_labels"`
	VRFRoutes     map[string]int `json:"vrf_routes"`
}

// ============ Global State ============

var (
	devices        []DeviceInfo
	vrfs           []VRFInfo
	baseline       *MigrationBaseline
	results        []ValidationResult
	outputDir      string
	sshClients     = make(map[string]*ssh.Client)
	sshMux         sync.Mutex
	drainThreshold = 5.0
)

// ============ Main ============

func main() {
	outputDir = filepath.Join(".", "output")
	os.MkdirAll(outputDir, 0755)
	printBanner()
	loadDefaults()
	mainMenu()
}

func printBanner() {
	fmt.Println(`
╔══════════════════════════════════════════════════════════════════════════════╗
║   ███╗   ███╗███████╗██████╗  █████╗ ██╗      ██████╗ ██████╗                ║
║   ████╗ ████║██╔════╝██╔══██╗██╔══██╗██║     ██╔════╝██╔═══██╗               ║
║   ██╔████╔██║█████╗  ██████╔╝███████║██║     ██║     ██║   ██║               ║
║   ██║╚██╔╝██║██╔══╝  ██╔══██╗██╔══██║██║     ██║     ██║   ██║               ║
║   ██║ ╚═╝ ██║███████╗██║  ██║██║  ██║███████╗╚██████╗╚██████╔╝               ║
║   ╚═╝     ╚═╝╚══════╝╚═╝  ╚═╝╚═╝  ╚═╝╚══════╝ ╚═════╝ ╚═════╝                ║
║            Network Migration Validation Toolkit v2.0                          ║
║            ASR9010 → ASR9906 | MPLS-LDP → MPLS-SR                             ║
╚══════════════════════════════════════════════════════════════════════════════╝`)
}

func loadDefaults() {
	vrfs = []VRFInfo{
		{Name: "VPN_ADMS", RD: "65000:100", RTImport: "RT:65000:100", RTExport: "RT:65000:101", Description: "MERALCO ADMS VRF", Priority: "critical"},
		{Name: "VPN_SCADA", RD: "65100:50", RTImport: "RT:65100:50", RTExport: "RT:65100:50", Description: "SCADA RTU VRF", Priority: "critical"},
		{Name: "VPN_Telepro", RD: "65000:240", RTImport: "RT:65000:240", RTExport: "RT:65000:240", Description: "Teleprotection VRF", Priority: "critical"},
		{Name: "VPN_Tetra", RD: "65100:90", RTImport: "RT:65100:90", RTExport: "RT:65100:90", Description: "Tetra Radio VRF", Priority: "high"},
		{Name: "VPN_CCTV", RD: "65000:200", RTImport: "RT:65000:200", RTExport: "RT:65000:200", Description: "CCTV VRF", Priority: "high"},
		{Name: "VPN_Metering", RD: "65000:270", RTImport: "RT:65000:270", RTExport: "RT:65000:270", Description: "AMI Metering VRF", Priority: "high"},
		{Name: "VPN_Transport_Mgt", RD: "65000:210", RTImport: "RT:65000:210", RTExport: "RT:65000:210", Description: "Transport Management", Priority: "medium"},
		{Name: "VPN_SCADA_SIP", RD: "65000:220", RTImport: "RT:65000:220", RTExport: "RT:65000:220", Description: "SCADA SIP VRF", Priority: "high"},
	}
	if data, err := os.ReadFile("devices.json"); err == nil {
		json.Unmarshal(data, &devices)
	}
}

// ============ SSH Functions ============

func connectSSH(d DeviceInfo) (*ssh.Client, error) {
	cfg := &ssh.ClientConfig{
		User:            d.Username,
		Auth:            []ssh.AuthMethod{ssh.Password(d.Password)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         30 * time.Second,
	}
	return ssh.Dial("tcp", fmt.Sprintf("%s:%d", d.IPAddress, d.SSHPort), cfg)
}

func getClient(d DeviceInfo) (*ssh.Client, error) {
	sshMux.Lock()
	defer sshMux.Unlock()
	if c, ok := sshClients[d.Hostname]; ok {
		if s, err := c.NewSession(); err == nil {
			s.Close()
			return c, nil
		}
		delete(sshClients, d.Hostname)
	}
	c, err := connectSSH(d)
	if err != nil {
		return nil, err
	}
	sshClients[d.Hostname] = c
	return c, nil
}

func execCmd(d DeviceInfo, cmd string) (string, error) {
	c, err := getClient(d)
	if err != nil {
		return "", fmt.Errorf("connection failed: %v", err)
	}

	// Use device-specific execution method
	switch d.DeviceType {
	case "ios-xr":
		return execCmdIOSXR(c, d, cmd)
	default:
		return execCmdStandard(c, cmd)
	}
}

// execCmdStandard for IOS/IOS-XE devices
func execCmdStandard(c *ssh.Client, cmd string) (string, error) {
	s, err := c.NewSession()
	if err != nil {
		return "", fmt.Errorf("session failed: %v", err)
	}
	defer s.Close()

	var stdout, stderr bytes.Buffer
	s.Stdout = &stdout
	s.Stderr = &stderr

	fullCmd := fmt.Sprintf("terminal length 0\n%s", cmd)
	err = s.Run(fullCmd)

	output := stdout.String()
	if stderr.Len() > 0 {
		output += "\n" + stderr.String()
	}

	if output != "" {
		return output, nil
	}

	if err != nil {
		return "", fmt.Errorf("command failed: %v", err)
	}

	return output, nil
}

// execCmdIOSXR uses interactive shell method that works with IOS-XR
// This matches the working method from the health_check tool
func execCmdIOSXR(c *ssh.Client, d DeviceInfo, cmd string) (string, error) {
	s, err := c.NewSession()
	if err != nil {
		return "", fmt.Errorf("session failed: %v", err)
	}
	defer s.Close()

	// Request PTY - required for IOS-XR
	modes := ssh.TerminalModes{
		ssh.ECHO:          0,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}

	if err := s.RequestPty("vt100", 80, 500, modes); err != nil {
		return "", fmt.Errorf("PTY failed: %v", err)
	}

	// Set up pipes
	stdin, err := s.StdinPipe()
	if err != nil {
		return "", fmt.Errorf("stdin pipe failed: %v", err)
	}

	var outputBuf bytes.Buffer
	s.Stdout = &outputBuf
	s.Stderr = &outputBuf

	// Start shell
	if err := s.Shell(); err != nil {
		return "", fmt.Errorf("shell failed: %v", err)
	}

	// Wait for initial prompt
	time.Sleep(1 * time.Second)

	// Send terminal length 0
	io.WriteString(stdin, "terminal length 0\n")
	time.Sleep(500 * time.Millisecond)

	// Send the command
	io.WriteString(stdin, cmd+"\n")

	// Wait time depends on command type
	if strings.HasPrefix(cmd, "ping") {
		// Ping needs more time based on count
		time.Sleep(8 * time.Second)
	} else if strings.HasPrefix(cmd, "show vrf") {
		time.Sleep(3 * time.Second)
	} else {
		time.Sleep(2 * time.Second)
	}

	// Send exit
	io.WriteString(stdin, "exit\n")
	time.Sleep(500 * time.Millisecond)

	// Close stdin to signal we're done
	stdin.Close()

	// Wait for session to complete
	s.Wait()

	// Get and clean output
	output := outputBuf.String()
	return cleanIOSXROutput(output, cmd), nil
}

// cleanIOSXROutput removes terminal artifacts from IOS-XR output
func cleanIOSXROutput(output, cmd string) string {
	// Remove ANSI escape codes
	ansiRegex := regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)
	output = ansiRegex.ReplaceAllString(output, "")

	// Remove carriage returns
	output = strings.ReplaceAll(output, "\r", "")

	// Split into lines and filter
	lines := strings.Split(output, "\n")
	var result []string
	skipNext := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Skip empty lines
		if trimmed == "" {
			continue
		}

		// Skip terminal length command
		if strings.Contains(trimmed, "terminal length") {
			skipNext = true
			continue
		}

		if skipNext {
			skipNext = false
			continue
		}

		// Skip command echo (but keep for ping to show what was executed)
		if strings.HasPrefix(trimmed, cmd[:min(len(cmd), 10)]) {
			continue
		}

		// Skip prompts (lines ending with # or >)
		if strings.HasSuffix(trimmed, "#") || strings.HasSuffix(trimmed, ">") {
			continue
		}

		// Skip "exit" command
		if trimmed == "exit" {
			continue
		}

		// Skip Building configuration...
		if strings.Contains(trimmed, "Building configuration") {
			continue
		}

		result = append(result, line)
	}

	return strings.Join(result, "\n")
}

// min returns the smaller of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func closeAll() {
	sshMux.Lock()
	defer sshMux.Unlock()
	for _, c := range sshClients {
		c.Close()
	}
	sshClients = make(map[string]*ssh.Client)
}

// ============ Menu System ============

func mainMenu() {
	r := bufio.NewReader(os.Stdin)
	for {
		fmt.Println("\n╔══════════════════════════════════════════════════════╗")
		fmt.Println("║               MAIN MENU                              ║")
		fmt.Println("╠══════════════════════════════════════════════════════╣")
		fmt.Println("║  1. Device Management                                ║")
		fmt.Println("║  2. Pre-Migration Validation                         ║")
		fmt.Println("║  3. During-Migration Monitoring                      ║")
		fmt.Println("║  4. Post-Migration Validation                        ║")
		fmt.Println("║  5. Connectivity Testing                             ║")
		fmt.Println("║  6. MTU Testing                                      ║")
		fmt.Println("║  7. Traffic Drain Monitor                            ║")
		fmt.Println("║  8. Generate Reports                                 ║")
		fmt.Println("║  9. Baseline Management                              ║")
		fmt.Println("║  0. Exit                                             ║")
		fmt.Println("╚══════════════════════════════════════════════════════╝")
		fmt.Print("\nSelect: ")
		in := readLine(r)
		switch in {
		case "1":
			deviceMenu(r)
		case "2":
			preMenu(r)
		case "3":
			duringMenu(r)
		case "4":
			postMenu(r)
		case "5":
			connMenu(r)
		case "6":
			mtuMenu(r)
		case "7":
			drainMenu(r)
		case "8":
			reportMenu(r)
		case "9":
			baselineMenu(r)
		case "0":
			fmt.Println("\n✓ Goodbye!")
			closeAll()
			os.Exit(0)
		}
	}
}

func deviceMenu(r *bufio.Reader) {
	for {
		fmt.Println("\n╔══════════════════════════════════════════════════════╗")
		fmt.Println("║           DEVICE MANAGEMENT                          ║")
		fmt.Println("╠══════════════════════════════════════════════════════╣")
		fmt.Println("║  1. Add Device          6. Export to JSON            ║")
		fmt.Println("║  2. Remove Device       7. List Static VRFs          ║")
		fmt.Println("║  3. List Devices        8. Collect VRFs from Device  ║")
		fmt.Println("║  4. Test Connectivity   9. List Device VRFs          ║")
		fmt.Println("║  5. Import from JSON    0. Back                      ║")
		fmt.Println("╚══════════════════════════════════════════════════════╝")
		fmt.Print("\nSelect: ")
		switch readLine(r) {
		case "1":
			addDevice(r)
		case "2":
			removeDevice(r)
		case "3":
			listDevices()
		case "4":
			testConn(r)
		case "5":
			importDev(r)
		case "6":
			exportDev(r)
		case "7":
			listVRFs()
		case "8":
			collectVRFsFromDevice(r)
		case "9":
			listDeviceVRFs(r)
		case "0":
			return
		}
	}
}

func preMenu(r *bufio.Reader) {
	for {
		fmt.Println("\n╔══════════════════════════════════════════════════════╗")
		fmt.Println("║         PRE-MIGRATION VALIDATION                     ║")
		fmt.Println("╠══════════════════════════════════════════════════════╣")
		fmt.Println("║  1. Run Full Pre-Migration Check                     ║")
		fmt.Println("║  2. Collect Baseline                                 ║")
		fmt.Println("║  3. Check OSPF         6. Check Interface Errors     ║")
		fmt.Println("║  4. Check BGP          7. Verify Config Backups      ║")
		fmt.Println("║  5. Check MPLS/LDP     8. Generate Go/No-Go Report   ║")
		fmt.Println("║  0. Back                                             ║")
		fmt.Println("╚══════════════════════════════════════════════════════╝")
		fmt.Print("\nSelect: ")
		switch readLine(r) {
		case "1":
			fullPreCheck(r)
		case "2":
			collectBaseline(r)
		case "3":
			checkOSPF(r)
		case "4":
			checkBGP(r)
		case "5":
			checkMPLS(r)
		case "6":
			checkErrors(r)
		case "7":
			checkBackups()
		case "8":
			genGoNoGo(r)
		case "0":
			return
		}
	}
}

func duringMenu(r *bufio.Reader) {
	for {
		fmt.Println("\n╔══════════════════════════════════════════════════════╗")
		fmt.Println("║       DURING-MIGRATION MONITORING                    ║")
		fmt.Println("╠══════════════════════════════════════════════════════╣")
		fmt.Println("║  1. Traffic Monitor     4. Service Health Check      ║")
		fmt.Println("║  2. BGP Monitor         5. Rollback Trigger Check    ║")
		fmt.Println("║  3. OSPF Monitor        6. Quick Snapshot            ║")
		fmt.Println("║  0. Back                                             ║")
		fmt.Println("╚══════════════════════════════════════════════════════╝")
		fmt.Print("\nSelect: ")
		switch readLine(r) {
		case "1":
			trafficMon(r)
		case "2":
			bgpMon(r)
		case "3":
			ospfMon(r)
		case "4":
			svcCheck()
		case "5":
			rollbackCheck()
		case "6":
			snapshot()
		case "0":
			return
		}
	}
}

func postMenu(r *bufio.Reader) {
	for {
		fmt.Println("\n╔══════════════════════════════════════════════════════╗")
		fmt.Println("║        POST-MIGRATION VALIDATION                     ║")
		fmt.Println("╠══════════════════════════════════════════════════════╣")
		fmt.Println("║  1. Run Full Post-Migration Check                    ║")
		fmt.Println("║  2. Compare with Baseline                            ║")
		fmt.Println("║  3. Check MPLS-SR Labels                             ║")
		fmt.Println("║  4. Check QoS Policies                               ║")
		fmt.Println("║  5. E2E Connectivity Test                            ║")
		fmt.Println("║  0. Back                                             ║")
		fmt.Println("╚══════════════════════════════════════════════════════╝")
		fmt.Print("\nSelect: ")
		switch readLine(r) {
		case "1":
			fullPostCheck(r)
		case "2":
			compareBase()
		case "3":
			checkSR(r)
		case "4":
			checkQoS(r)
		case "5":
			e2eTest(r)
		case "0":
			return
		}
	}
}

func connMenu(r *bufio.Reader) {
	for {
		fmt.Println("\n╔══════════════════════════════════════════════════════╗")
		fmt.Println("║          CONNECTIVITY TESTING                        ║")
		fmt.Println("╠══════════════════════════════════════════════════════╣")
		fmt.Println("║  1. Single Ping Test (manual VRF input)              ║")
		fmt.Println("║  2. Ping Test (select from device VRFs)              ║")
		fmt.Println("║  3. Batch Ping (all device VRFs)                     ║")
		fmt.Println("║  4. Traceroute                                       ║")
		fmt.Println("║  5. MPLS Traceroute                                  ║")
		fmt.Println("║  0. Back                                             ║")
		fmt.Println("╚══════════════════════════════════════════════════════╝")
		fmt.Print("\nSelect: ")
		switch readLine(r) {
		case "1":
			singlePing(r)
		case "2":
			pingWithDeviceVRF(r)
		case "3":
			batchPing(r)
		case "4":
			traceroute(r)
		case "5":
			mplsTrace(r)
		case "0":
			return
		}
	}
}

func mtuMenu(r *bufio.Reader) {
	for {
		fmt.Println("\n╔══════════════════════════════════════════════════════╗")
		fmt.Println("║              MTU TESTING                             ║")
		fmt.Println("╠══════════════════════════════════════════════════════╣")
		fmt.Println("║  1. MTU Discovery Test                               ║")
		fmt.Println("║  2. Jumbo Frame Validation                           ║")
		fmt.Println("║  0. Back                                             ║")
		fmt.Println("╚══════════════════════════════════════════════════════╝")
		fmt.Print("\nSelect: ")
		switch readLine(r) {
		case "1":
			mtuTest(r)
		case "2":
			jumboTest(r)
		case "0":
			return
		}
	}
}

func drainMenu(r *bufio.Reader) {
	for {
		fmt.Println("\n╔══════════════════════════════════════════════════════╗")
		fmt.Println("║         TRAFFIC DRAIN MONITOR                        ║")
		fmt.Println("╠══════════════════════════════════════════════════════╣")
		fmt.Println("║  1. Start Drain Monitor                              ║")
		fmt.Println("║  2. Check Interface Traffic                          ║")
		fmt.Println("║  3. Set Threshold (current: %.1f Mbps)               ║", drainThreshold)
		fmt.Println("║  0. Back                                             ║")
		fmt.Println("╚══════════════════════════════════════════════════════╝")
		fmt.Print("\nSelect: ")
		switch readLine(r) {
		case "1":
			drainMon(r)
		case "2":
			checkTraffic(r)
		case "3":
			setThresh(r)
		case "0":
			return
		}
	}
}

func reportMenu(r *bufio.Reader) {
	for {
		fmt.Println("\n╔══════════════════════════════════════════════════════╗")
		fmt.Println("║          REPORT GENERATION                           ║")
		fmt.Println("╠══════════════════════════════════════════════════════╣")
		fmt.Println("║  1. Generate HTML Report                             ║")
		fmt.Println("║  2. Export JSON Data                                 ║")
		fmt.Println("║  3. List Previous Reports                            ║")
		fmt.Println("║  0. Back                                             ║")
		fmt.Println("╚══════════════════════════════════════════════════════╝")
		fmt.Print("\nSelect: ")
		switch readLine(r) {
		case "1":
			fmt.Print("Report type (pre/post/final): ")
			genHTML(readLine(r))
		case "2":
			exportJSON()
		case "3":
			listReports()
		case "0":
			return
		}
	}
}

func baselineMenu(r *bufio.Reader) {
	for {
		fmt.Println("\n╔══════════════════════════════════════════════════════╗")
		fmt.Println("║         BASELINE MANAGEMENT                          ║")
		fmt.Println("╠══════════════════════════════════════════════════════╣")
		fmt.Println("║  1. Load Baseline File                               ║")
		fmt.Println("║  2. View Current Baseline                            ║")
		fmt.Println("║  0. Back                                             ║")
		fmt.Println("╚══════════════════════════════════════════════════════╝")
		fmt.Print("\nSelect: ")
		switch readLine(r) {
		case "1":
			loadBase(r)
		case "2":
			viewBase()
		case "0":
			return
		}
	}
}

// ============ Device Functions ============

func addDevice(r *bufio.Reader) {
	d := DeviceInfo{SSHPort: 22}
	fmt.Print("Hostname: ")
	d.Hostname = readLine(r)
	fmt.Print("IP Address: ")
	d.IPAddress = readLine(r)
	fmt.Print("Type (1=IOS-XR, 2=IOS-XE, 3=IOS): ")
	switch readLine(r) {
	case "1":
		d.DeviceType = "ios-xr"
	case "2":
		d.DeviceType = "ios-xe"
	default:
		d.DeviceType = "ios"
	}
	fmt.Print("Role (1=Core, 2=Agg, 3=Access): ")
	switch readLine(r) {
	case "1":
		d.Role = "core"
	case "2":
		d.Role = "aggregation"
	default:
		d.Role = "access"
	}
	fmt.Print("Username: ")
	d.Username = readLine(r)
	fmt.Print("Password: ")
	d.Password = readLine(r)
	fmt.Print("SSH Port [22]: ")
	if p := readLine(r); p != "" {
		if port, err := strconv.Atoi(p); err == nil {
			d.SSHPort = port
		}
	}
	devices = append(devices, d)
	fmt.Printf("✓ Device '%s' added\n", d.Hostname)
}

func removeDevice(r *bufio.Reader) {
	if len(devices) == 0 {
		fmt.Println("No devices.")
		return
	}
	listDevices()
	fmt.Print("Number to remove (0=cancel): ")
	n, _ := strconv.Atoi(readLine(r))
	if n > 0 && n <= len(devices) {
		name := devices[n-1].Hostname
		devices = append(devices[:n-1], devices[n:]...)
		fmt.Printf("✓ Removed '%s'\n", name)
	}
}

func listDevices() {
	if len(devices) == 0 {
		fmt.Println("\nNo devices configured.")
		return
	}
	fmt.Println("\n╔════╦══════════════════╦══════════════════╦══════════╦═══════════════╗")
	fmt.Println("║ #  ║ Hostname         ║ IP Address       ║ Type     ║ Role          ║")
	fmt.Println("╠════╬══════════════════╬══════════════════╬══════════╬═══════════════╣")
	for i, d := range devices {
		fmt.Printf("║ %-2d ║ %-16s ║ %-16s ║ %-8s ║ %-13s ║\n",
			i+1, trunc(d.Hostname, 16), trunc(d.IPAddress, 16), trunc(d.DeviceType, 8), trunc(d.Role, 13))
	}
	fmt.Println("╚════╩══════════════════╩══════════════════╩══════════╩═══════════════╝")
}

func listVRFs() {
	fmt.Println("\n╔════╦══════════════════════╦════════════╦══════════╦════════════════════════════╗")
	fmt.Println("║ #  ║ VRF Name             ║ RD         ║ Priority ║ Description                ║")
	fmt.Println("╠════╬══════════════════════╬════════════╬══════════╬════════════════════════════╣")
	for i, v := range vrfs {
		fmt.Printf("║ %-2d ║ %-20s ║ %-10s ║ %-8s ║ %-26s ║\n",
			i+1, trunc(v.Name, 20), trunc(v.RD, 10), trunc(v.Priority, 8), trunc(v.Description, 26))
	}
	fmt.Println("╚════╩══════════════════════╩════════════╩══════════╩════════════════════════════╝")
	fmt.Printf("\nTotal: %d static VRFs configured\n", len(vrfs))
}

// collectVRFsFromDevice - Auto-collect VRFs from a device
func collectVRFsFromDevice(r *bufio.Reader) {
	d := selectDev(r)
	if d == nil {
		return
	}

	fmt.Printf("\nCollecting VRFs from %s (%s)...\n", d.Hostname, d.IPAddress)

	// Try IOS-XR command first
	var out string
	var err error

	if d.DeviceType == "ios-xr" {
		out, err = execCmd(*d, "show vrf all")
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
	} else {
		// IOS-XE or IOS
		out, err = execCmd(*d, "show vrf")
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
	}

	fmt.Println("\n--- Raw VRF Output ---")
	fmt.Println(out)
	fmt.Println("--- End of Output ---")

	// Parse VRFs from output
	collectedVRFs := parseVRFOutput(out, d.DeviceType)

	if len(collectedVRFs) == 0 {
		fmt.Println("\n⚠ No VRFs found or unable to parse output")
		fmt.Println("Trying alternative command...")

		// Try alternative command
		out, err = execCmd(*d, "show vrf all detail")
		if err == nil {
			fmt.Println("\n--- Raw VRF Detail Output ---")
			fmt.Println(out)
			fmt.Println("--- End of Output ---")
			collectedVRFs = parseVRFDetailOutput(out)
		}
	}

	if len(collectedVRFs) > 0 {
		// Store in deviceVRFs map
		deviceVRFs[d.Hostname] = DeviceVRF{
			DeviceHostname: d.Hostname,
			VRFs:           collectedVRFs,
			CollectedAt:    time.Now(),
		}

		fmt.Printf("\n✓ Collected %d VRFs from %s:\n", len(collectedVRFs), d.Hostname)
		fmt.Println("╔════╦══════════════════════════════╦══════════════════════╗")
		fmt.Println("║ #  ║ VRF Name                     ║ RD                   ║")
		fmt.Println("╠════╬══════════════════════════════╬══════════════════════╣")
		for i, v := range collectedVRFs {
			fmt.Printf("║ %-2d ║ %-28s ║ %-20s ║\n", i+1, trunc(v.Name, 28), trunc(v.RD, 20))
		}
		fmt.Println("╚════╩══════════════════════════════╩══════════════════════╝")

		// Ask to save
		fmt.Print("\nSave collected VRFs to file? (y/n): ")
		if strings.ToLower(readLine(r)) == "y" {
			ts := time.Now().Format("20060102_150405")
			path := filepath.Join(outputDir, fmt.Sprintf("vrfs_%s_%s.json", d.Hostname, ts))
			data, _ := json.MarshalIndent(collectedVRFs, "", "  ")
			os.WriteFile(path, data, 0644)
			fmt.Printf("✓ Saved to %s\n", path)
		}
	} else {
		fmt.Println("\n⚠ Could not collect VRFs. Please check device connectivity and permissions.")
	}
}

// parseVRFOutput parses "show vrf all" output for IOS-XR
func parseVRFOutput(output, deviceType string) []VRFInfo {
	var vrfList []VRFInfo
	lines := strings.Split(output, "\n")

	// Skip header lines
	startParsing := false
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip empty lines
		if line == "" {
			continue
		}

		// Look for header line to start parsing
		if strings.Contains(line, "VRF") && (strings.Contains(line, "RD") || strings.Contains(line, "Name")) {
			startParsing = true
			continue
		}

		// Skip separator lines
		if strings.HasPrefix(line, "-") || strings.HasPrefix(line, "=") {
			continue
		}

		if !startParsing {
			continue
		}

		// Parse VRF line - format varies by device
		fields := strings.Fields(line)
		if len(fields) >= 1 {
			vrfName := fields[0]

			// Skip management VRFs and default
			if vrfName == "default" || vrfName == "management" || vrfName == "__bgp_vrf__" {
				continue
			}

			vrf := VRFInfo{
				Name:     vrfName,
				Priority: "medium", // Default priority
				Source:   "device",
			}

			// Try to extract RD if present
			if len(fields) >= 2 {
				// Check if second field looks like an RD (contains :)
				if strings.Contains(fields[1], ":") {
					vrf.RD = fields[1]
				}
			}

			// Check if this VRF is in our static list for priority
			for _, staticVRF := range vrfs {
				if strings.EqualFold(staticVRF.Name, vrfName) {
					vrf.Priority = staticVRF.Priority
					vrf.Description = staticVRF.Description
					break
				}
			}

			vrfList = append(vrfList, vrf)
		}
	}

	return vrfList
}

// parseVRFDetailOutput parses "show vrf all detail" output
func parseVRFDetailOutput(output string) []VRFInfo {
	var vrfList []VRFInfo
	lines := strings.Split(output, "\n")

	var currentVRF *VRFInfo
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Look for VRF name line
		if strings.HasPrefix(line, "VRF ") && !strings.Contains(line, "Table") {
			// Save previous VRF if exists
			if currentVRF != nil && currentVRF.Name != "" {
				vrfList = append(vrfList, *currentVRF)
			}

			// Extract VRF name
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				vrfName := strings.TrimSuffix(parts[1], ",")
				vrfName = strings.TrimSuffix(vrfName, ";")

				if vrfName != "default" && vrfName != "management" {
					currentVRF = &VRFInfo{
						Name:     vrfName,
						Priority: "medium",
						Source:   "device",
					}
				} else {
					currentVRF = nil
				}
			}
		}

		// Look for RD line
		if currentVRF != nil && strings.Contains(line, "RD") {
			parts := strings.Split(line, ":")
			if len(parts) >= 2 {
				rdPart := strings.TrimSpace(strings.Join(parts[1:], ":"))
				rdPart = strings.TrimSuffix(rdPart, ";")
				if rdPart != "not set" && rdPart != "" {
					currentVRF.RD = rdPart
				}
			}
		}

		// Look for RT import/export
		if currentVRF != nil {
			if strings.Contains(line, "Import") && strings.Contains(line, "RT") {
				re := regexp.MustCompile(`(\d+:\d+)`)
				if m := re.FindString(line); m != "" {
					currentVRF.RTImport = m
				}
			}
			if strings.Contains(line, "Export") && strings.Contains(line, "RT") {
				re := regexp.MustCompile(`(\d+:\d+)`)
				if m := re.FindString(line); m != "" {
					currentVRF.RTExport = m
				}
			}
		}
	}

	// Don't forget the last VRF
	if currentVRF != nil && currentVRF.Name != "" {
		vrfList = append(vrfList, *currentVRF)
	}

	return vrfList
}

// listDeviceVRFs - Show VRFs collected from a specific device
func listDeviceVRFs(r *bufio.Reader) {
	if len(deviceVRFs) == 0 {
		fmt.Println("\nNo VRFs collected from devices yet.")
		fmt.Println("Use option 8 'Collect VRFs from Device' first.")
		return
	}

	fmt.Println("\nDevices with collected VRFs:")
	i := 1
	var deviceList []string
	for hostname, dvrf := range deviceVRFs {
		fmt.Printf("  %d. %s (%d VRFs, collected %s)\n",
			i, hostname, len(dvrf.VRFs), dvrf.CollectedAt.Format("15:04:05"))
		deviceList = append(deviceList, hostname)
		i++
	}

	fmt.Print("\nSelect device # (0=show all): ")
	n, _ := strconv.Atoi(readLine(r))

	if n == 0 {
		// Show all
		for hostname, dvrf := range deviceVRFs {
			fmt.Printf("\n=== %s ===\n", hostname)
			for j, v := range dvrf.VRFs {
				fmt.Printf("  %d. %-25s RD: %-15s Priority: %s\n", j+1, v.Name, v.RD, v.Priority)
			}
		}
	} else if n > 0 && n <= len(deviceList) {
		hostname := deviceList[n-1]
		dvrf := deviceVRFs[hostname]
		fmt.Printf("\n=== VRFs on %s ===\n", hostname)
		fmt.Println("╔════╦══════════════════════════════╦══════════════════════╦══════════╗")
		fmt.Println("║ #  ║ VRF Name                     ║ RD                   ║ Priority ║")
		fmt.Println("╠════╬══════════════════════════════╬══════════════════════╬══════════╣")
		for j, v := range dvrf.VRFs {
			fmt.Printf("║ %-2d ║ %-28s ║ %-20s ║ %-8s ║\n",
				j+1, trunc(v.Name, 28), trunc(v.RD, 20), trunc(v.Priority, 8))
		}
		fmt.Println("╚════╩══════════════════════════════╩══════════════════════╩══════════╝")
	}
}

// getVRFsForDevice returns VRFs for a specific device (from device collection or static list)
func getVRFsForDevice(hostname string) []VRFInfo {
	if dvrf, ok := deviceVRFs[hostname]; ok {
		return dvrf.VRFs
	}
	return vrfs // fallback to static list
}

func testConn(r *bufio.Reader) {
	if len(devices) == 0 {
		fmt.Println("No devices.")
		return
	}
	listDevices()
	fmt.Print("Device # (0=all): ")
	n, _ := strconv.Atoi(readLine(r))
	var list []DeviceInfo
	if n == 0 {
		list = devices
	} else if n > 0 && n <= len(devices) {
		list = []DeviceInfo{devices[n-1]}
	} else {
		return
	}
	fmt.Println("\nTesting...")
	for _, d := range list {
		fmt.Printf("  %s (%s)... ", d.Hostname, d.IPAddress)
		if conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", d.IPAddress, d.SSHPort), 5*time.Second); err != nil {
			fmt.Printf("FAIL: %v\n", err)
		} else {
			conn.Close()
			if _, err := getClient(d); err != nil {
				fmt.Printf("TCP OK, SSH FAIL: %v\n", err)
			} else {
				fmt.Println("OK")
			}
		}
	}
}

func importDev(r *bufio.Reader) {
	fmt.Print("File [devices.json]: ")
	p := readLine(r)
	if p == "" {
		p = "devices.json"
	}
	data, err := os.ReadFile(p)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	json.Unmarshal(data, &devices)
	fmt.Printf("✓ Imported %d devices\n", len(devices))
}

func exportDev(r *bufio.Reader) {
	fmt.Print("File [devices.json]: ")
	p := readLine(r)
	if p == "" {
		p = "devices.json"
	}
	data, _ := json.MarshalIndent(devices, "", "  ")
	os.WriteFile(p, data, 0644)
	fmt.Printf("✓ Exported to %s\n", p)
}

// ============ Pre-Migration ============

func fullPreCheck(r *bufio.Reader) {
	if len(devices) == 0 {
		fmt.Println("No devices.")
		return
	}
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("          FULL PRE-MIGRATION VALIDATION")
	fmt.Println(strings.Repeat("=", 60))
	results = nil
	start := time.Now()
	fmt.Println("\n[1/4] Collecting baseline...")
	doCollectBaseline()
	fmt.Println("[2/4] Checking control plane...")
	checkCtrlPlane()
	fmt.Println("[3/4] Checking data plane...")
	checkDataPlane()
	fmt.Println("[4/4] Checking services...")
	checkSvcs()
	printSummary(time.Since(start))
	fmt.Print("\nGenerate HTML report? (y/n): ")
	if strings.ToLower(readLine(r)) == "y" {
		genHTML("pre_migration")
	}
}

func collectBaseline(r *bufio.Reader) {
	if len(devices) == 0 {
		fmt.Println("No devices.")
		return
	}
	doCollectBaseline()
	ts := time.Now().Format("20060102_150405")
	p := filepath.Join(outputDir, fmt.Sprintf("baseline_%s.json", ts))
	data, _ := json.MarshalIndent(baseline, "", "  ")
	os.WriteFile(p, data, 0644)
	fmt.Printf("✓ Baseline saved to %s\n", p)
}

func doCollectBaseline() {
	baseline = &MigrationBaseline{Timestamp: time.Now(), Devices: make(map[string]DeviceBaseline)}
	for _, d := range devices {
		fmt.Printf("  %s... ", d.Hostname)
		db := DeviceBaseline{Hostname: d.Hostname, VRFRoutes: make(map[string]int)}
		if out, _ := execCmd(d, "show bgp vpnv4 unicast all summary"); out != "" {
			db.BGPSessions = countMatch(out, `(?i)estab`)
			db.BGPPrefixes = sumPfx(out)
		}
		if out, _ := execCmd(d, "show ospf neighbor"); out != "" {
			db.OSPFNeighbors = countMatch(out, `(?i)FULL`)
		}
		if out, _ := execCmd(d, "show mpls ldp neighbor brief"); out != "" {
			db.LDPNeighbors = countMatch(out, `(?i)oper`)
		}
		if out, _ := execCmd(d, "show mpls forwarding"); out != "" {
			db.MPLSLabels = countLines(out)
		}
		for _, v := range vrfs {
			if out, _ := execCmd(d, fmt.Sprintf("show route vrf %s summary", v.Name)); out != "" {
				db.VRFRoutes[v.Name] = parseRoutes(out)
			}
		}
		baseline.Devices[d.Hostname] = db
		fmt.Println("done")
	}
}

func checkOSPF(r *bufio.Reader) {
	d := selectDev(r)
	if d == nil {
		return
	}
	out, _ := execCmd(*d, "show ospf neighbor detail")
	fmt.Println("\n" + out)
	fmt.Printf("✓ %d neighbors in FULL state\n", countMatch(out, `(?i)FULL`))
}

func checkBGP(r *bufio.Reader) {
	d := selectDev(r)
	if d == nil {
		return
	}
	out, _ := execCmd(*d, "show bgp vpnv4 unicast all summary")
	fmt.Println("\n" + out)
	fmt.Printf("✓ %d sessions established\n", countMatch(out, `(?i)estab`))
}

func checkMPLS(r *bufio.Reader) {
	d := selectDev(r)
	if d == nil {
		return
	}
	fmt.Println("\nLDP Neighbors:")
	ldp, _ := execCmd(*d, "show mpls ldp neighbor brief")
	fmt.Println(ldp)
	fmt.Println("MPLS Forwarding (sample):")
	mpls, _ := execCmd(*d, "show mpls forwarding | head 20")
	fmt.Println(mpls)
}

func checkErrors(r *bufio.Reader) {
	d := selectDev(r)
	if d == nil {
		return
	}
	out, _ := execCmd(*d, "show interfaces | include \"errors|drops|CRC\"")
	fmt.Println("\n" + out)
}

func checkBackups() {
	for _, d := range devices {
		fmt.Printf("  %s... ", d.Hostname)
		out, _ := execCmd(d, "dir disk0: | include backup")
		if strings.Contains(out, "backup") {
			fmt.Println("found")
		} else {
			fmt.Println("NOT FOUND")
		}
	}
}

func genGoNoGo(r *bufio.Reader) {
	if baseline == nil {
		fmt.Println("Collecting baseline first...")
		doCollectBaseline()
	}
	if len(results) == 0 {
		checkCtrlPlane()
		checkDataPlane()
	}
	genHTML("go_nogo")
}

func checkCtrlPlane() {
	for _, d := range devices {
		if out, _ := execCmd(d, "show ospf neighbor | include FULL"); countMatch(out, `(?i)FULL`) > 0 {
			addRes("OSPF Neighbors", d.Hostname, "PASS", "Neighbors in FULL state")
		} else {
			addRes("OSPF Neighbors", d.Hostname, "FAIL", "No OSPF neighbors")
		}
		if out, _ := execCmd(d, "show bgp vpnv4 unicast all summary | include Estab"); countMatch(out, `(?i)estab`) > 0 {
			addRes("BGP Sessions", d.Hostname, "PASS", "Sessions established")
		} else {
			addRes("BGP Sessions", d.Hostname, "FAIL", "No BGP sessions")
		}
		if out, _ := execCmd(d, "show mpls ldp neighbor brief | include oper"); countMatch(out, `(?i)oper`) > 0 {
			addRes("LDP Neighbors", d.Hostname, "PASS", "LDP operational")
		} else {
			addRes("LDP Neighbors", d.Hostname, "WARN", "No LDP neighbors")
		}
	}
}

func checkDataPlane() {
	for _, d := range devices {
		if out, _ := execCmd(d, "show mpls forwarding | include local"); countLines(out) > 0 {
			addRes("MPLS Forwarding", d.Hostname, "PASS", fmt.Sprintf("%d labels", countLines(out)))
		} else {
			addRes("MPLS Forwarding", d.Hostname, "FAIL", "No MPLS labels")
		}
	}
}

func checkSvcs() {
	for _, v := range vrfs {
		if v.Priority != "critical" && v.Priority != "high" {
			continue
		}
		for _, d := range devices {
			out, _ := execCmd(d, fmt.Sprintf("show route vrf %s summary", v.Name))
			routes := parseRoutes(out)
			if routes > 0 {
				addRes("VRF "+v.Name, d.Hostname, "PASS", fmt.Sprintf("%d routes", routes))
			} else {
				addRes("VRF "+v.Name, d.Hostname, "WARN", "No routes")
			}
			break
		}
	}
}

// ============ During Migration ============

func trafficMon(r *bufio.Reader) {
	d := selectDev(r)
	if d == nil {
		return
	}
	fmt.Print("Interface: ")
	intf := readLine(r)
	fmt.Println("\nMonitoring (20 samples)...")
	for i := 0; i < 20; i++ {
		out, _ := execCmd(*d, fmt.Sprintf("show interface %s | include rate", intf))
		in, ou := parseRates(out)
		fmt.Printf("[%s] IN: %.2f Mbps | OUT: %.2f Mbps\n", time.Now().Format("15:04:05"), in, ou)
		time.Sleep(5 * time.Second)
	}
}

func bgpMon(r *bufio.Reader) {
	d := selectDev(r)
	if d == nil {
		return
	}
	fmt.Println("\nMonitoring BGP (20 samples)...")
	last := 0
	for i := 0; i < 20; i++ {
		out, _ := execCmd(*d, "show bgp vpnv4 unicast all summary | include Estab")
		cnt := sumPfx(out)
		status := "STABLE"
		if cnt != last && last != 0 {
			if cnt > last {
				status = "CONVERGING ↑"
			} else {
				status = "DIVERGING ↓"
			}
		}
		fmt.Printf("[%s] Prefixes: %d | %s\n", time.Now().Format("15:04:05"), cnt, status)
		last = cnt
		time.Sleep(5 * time.Second)
	}
}

func ospfMon(r *bufio.Reader) {
	d := selectDev(r)
	if d == nil {
		return
	}
	fmt.Println("\nMonitoring OSPF (20 samples)...")
	for i := 0; i < 20; i++ {
		out, _ := execCmd(*d, "show ospf neighbor | include FULL")
		fmt.Printf("[%s] FULL neighbors: %d\n", time.Now().Format("15:04:05"), countMatch(out, `(?i)FULL`))
		time.Sleep(5 * time.Second)
	}
}

func svcCheck() {
	fmt.Println("\n--- Service Health Check ---")
	for _, v := range vrfs {
		if v.Priority == "critical" || v.Priority == "high" {
			fmt.Printf("%-20s (%s)... ", v.Name, v.Priority)
			for _, d := range devices {
				out, _ := execCmd(d, fmt.Sprintf("show route vrf %s summary", v.Name))
				routes := parseRoutes(out)
				if routes > 0 {
					fmt.Printf("OK (%d routes)\n", routes)
				} else {
					fmt.Println("NO ROUTES")
				}
				break
			}
		}
	}
}

func rollbackCheck() {
	fmt.Println("\n--- Rollback Trigger Check ---")
	var triggers []string
	for _, v := range vrfs {
		if v.Priority == "critical" {
			for _, d := range devices {
				out, _ := execCmd(d, fmt.Sprintf("show route vrf %s summary", v.Name))
				if parseRoutes(out) == 0 {
					triggers = append(triggers, fmt.Sprintf("VRF %s has no routes on %s", v.Name, d.Hostname))
				}
				break
			}
		}
	}
	for _, d := range devices {
		out, _ := execCmd(d, "show bgp vpnv4 unicast all summary")
		if countMatch(out, `(?i)estab`) == 0 {
			triggers = append(triggers, fmt.Sprintf("No BGP on %s", d.Hostname))
		}
	}
	if len(triggers) > 0 {
		fmt.Println("\n⚠️  ROLLBACK TRIGGERS:")
		for _, t := range triggers {
			fmt.Println("  • " + t)
		}
	} else {
		fmt.Println("\n✓ No rollback triggers")
	}
}

func snapshot() {
	fmt.Println("\n╔═══════════════════╦═══════╦═══════╦═══════╦═══════════╗")
	fmt.Println("║ Device            ║ OSPF  ║ BGP   ║ LDP   ║ Status    ║")
	fmt.Println("╠═══════════════════╬═══════╬═══════╬═══════╬═══════════╣")
	for _, d := range devices {
		ospf, bgp, ldp := "✗", "✗", "✗"
		if out, _ := execCmd(d, "show ospf neighbor | include FULL"); countMatch(out, `(?i)FULL`) > 0 {
			ospf = "✓"
		}
		if out, _ := execCmd(d, "show bgp vpnv4 unicast all summary | include Estab"); countMatch(out, `(?i)estab`) > 0 {
			bgp = "✓"
		}
		if out, _ := execCmd(d, "show mpls ldp neighbor brief | include oper"); countMatch(out, `(?i)oper`) > 0 {
			ldp = "✓"
		}
		status := "OK"
		if ospf == "✗" || bgp == "✗" {
			status = "ISSUES"
		}
		fmt.Printf("║ %-17s ║ %-5s ║ %-5s ║ %-5s ║ %-9s ║\n", trunc(d.Hostname, 17), ospf, bgp, ldp, status)
	}
	fmt.Println("╚═══════════════════╩═══════╩═══════╩═══════╩═══════════╝")
}

// ============ Post Migration ============

func fullPostCheck(r *bufio.Reader) {
	if len(devices) == 0 {
		fmt.Println("No devices.")
		return
	}
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("          FULL POST-MIGRATION VALIDATION")
	fmt.Println(strings.Repeat("=", 60))
	results = nil
	start := time.Now()
	fmt.Println("\n[1/3] Checking control plane...")
	checkCtrlPlane()
	fmt.Println("[2/3] Checking data plane...")
	checkDataPlane()
	fmt.Println("[3/3] Checking services...")
	checkSvcs()
	printSummary(time.Since(start))
	fmt.Print("\nGenerate HTML report? (y/n): ")
	if strings.ToLower(readLine(r)) == "y" {
		genHTML("post_migration")
	}
}

func compareBase() {
	if baseline == nil {
		fmt.Println("No baseline loaded.")
		return
	}
	fmt.Println("\n--- Baseline Comparison ---")
	for _, d := range devices {
		bl, ok := baseline.Devices[d.Hostname]
		if !ok {
			continue
		}
		out, _ := execCmd(d, "show bgp vpnv4 unicast all summary")
		curr := countMatch(out, `(?i)estab`)
		if curr != bl.BGPSessions {
			fmt.Printf("  %s BGP: %d → %d\n", d.Hostname, bl.BGPSessions, curr)
		}
		out, _ = execCmd(d, "show ospf neighbor")
		curr = countMatch(out, `(?i)FULL`)
		if curr != bl.OSPFNeighbors {
			fmt.Printf("  %s OSPF: %d → %d\n", d.Hostname, bl.OSPFNeighbors, curr)
		}
	}
	fmt.Println("Done")
}

func checkSR(r *bufio.Reader) {
	d := selectDev(r)
	if d == nil {
		return
	}
	fmt.Println("\nSegment Routing Labels:")
	out, _ := execCmd(*d, "show isis segment-routing label table")
	if out == "" {
		out, _ = execCmd(*d, "show ospf segment-routing local-block")
	}
	fmt.Println(out)
}

func checkQoS(r *bufio.Reader) {
	d := selectDev(r)
	if d == nil {
		return
	}
	out, _ := execCmd(*d, "show policy-map interface")
	fmt.Println("\n" + out)
	if strings.Contains(out, "tail drops") {
		fmt.Println("⚠ Tail drops detected")
	} else {
		fmt.Println("✓ No tail drops")
	}
}

func e2eTest(r *bufio.Reader) {
	d := selectDev(r)
	if d == nil {
		return
	}
	fmt.Print("VRF: ")
	vrf := readLine(r)
	fmt.Print("Destination IP: ")
	dst := readLine(r)
	fmt.Print("Count [100]: ")
	cnt := 100
	if c, _ := strconv.Atoi(readLine(r)); c > 0 {
		cnt = c
	}
	out, _ := execCmd(*d, fmt.Sprintf("ping vrf %s %s repeat %d", vrf, dst, cnt))
	fmt.Println("\n" + out)
	rate := pingRate(out)
	if rate == 100 {
		fmt.Println("✓ PASSED (100%)")
	} else {
		fmt.Printf("✗ %d%% success\n", rate)
	}
}

// ============ Connectivity ============

func singlePing(r *bufio.Reader) {
	d := selectDev(r)
	if d == nil {
		return
	}
	fmt.Print("VRF (blank=global/default): ")
	vrf := readLine(r)
	fmt.Print("Destination IP: ")
	dst := readLine(r)
	fmt.Print("Repeat count [5]: ")
	cnt := 5
	if c, _ := strconv.Atoi(readLine(r)); c > 0 {
		cnt = c
	}

	// Build command based on device type and VRF
	var cmd string
	if vrf == "" || vrf == "default" || vrf == "global" {
		if d.DeviceType == "ios-xr" {
			cmd = fmt.Sprintf("ping %s count %d", dst, cnt)
		} else {
			cmd = fmt.Sprintf("ping %s repeat %d", dst, cnt)
		}
	} else {
		if d.DeviceType == "ios-xr" {
			cmd = fmt.Sprintf("ping vrf %s %s count %d", vrf, dst, cnt)
		} else {
			cmd = fmt.Sprintf("ping vrf %s %s repeat %d", vrf, dst, cnt)
		}
	}

	fmt.Printf("\n--- Executing on %s ---\n", d.Hostname)
	fmt.Printf("Command: %s\n", cmd)
	fmt.Println(strings.Repeat("-", 50))

	out, err := execCmd(*d, cmd)
	if err != nil {
		fmt.Printf("\n✗ ERROR: %v\n", err)
		fmt.Println("Check SSH connectivity and device credentials.")
		return
	}

	// Show full output
	fmt.Println(out)
	fmt.Println(strings.Repeat("-", 50))

	// Parse and show result
	rate := pingRate(out)
	if rate == 100 {
		fmt.Println("\n✓ PING SUCCESSFUL (100%)")
	} else if rate > 0 {
		fmt.Printf("\n⚠ PARTIAL SUCCESS (%d%%)\n", rate)
	} else {
		// Check if it's a timeout or complete failure
		if strings.Contains(out, "timed out") || strings.Contains(out, "0 percent") || strings.Contains(out, "0/") {
			fmt.Println("\n✗ PING FAILED (0% - timeout/unreachable)")
		} else if strings.Contains(out, "Invalid") || strings.Contains(out, "error") || strings.Contains(out, "Error") {
			fmt.Println("\n✗ COMMAND ERROR - Check VRF name and destination")
		} else {
			fmt.Println("\n✗ PING FAILED - Could not determine success rate")
		}
	}
}

// pingWithDeviceVRF - Select VRF from device's collected VRFs
func pingWithDeviceVRF(r *bufio.Reader) {
	d := selectDev(r)
	if d == nil {
		return
	}

	// Get VRFs for this device
	vrfList := getVRFsForDevice(d.Hostname)

	if len(vrfList) == 0 {
		fmt.Println("\nNo VRFs available for this device.")
		fmt.Println("Use 'Device Management' → 'Collect VRFs from Device' first.")
		fmt.Print("\nEnter VRF name manually: ")
		vrf := readLine(r)
		if vrf != "" {
			doPingTest(r, d, vrf)
		}
		return
	}

	// Show available VRFs
	fmt.Printf("\n--- VRFs available on %s ---\n", d.Hostname)
	fmt.Println("╔════╦══════════════════════════════╦══════════╗")
	fmt.Println("║ #  ║ VRF Name                     ║ Priority ║")
	fmt.Println("╠════╬══════════════════════════════╬══════════╣")
	for i, v := range vrfList {
		fmt.Printf("║ %-2d ║ %-28s ║ %-8s ║\n", i+1, trunc(v.Name, 28), trunc(v.Priority, 8))
	}
	fmt.Println("╚════╩══════════════════════════════╩══════════╝")

	fmt.Print("\nSelect VRF # (0=manual input): ")
	n, _ := strconv.Atoi(readLine(r))

	var vrf string
	if n == 0 {
		fmt.Print("Enter VRF name: ")
		vrf = readLine(r)
	} else if n > 0 && n <= len(vrfList) {
		vrf = vrfList[n-1].Name
	} else {
		fmt.Println("Invalid selection")
		return
	}

	doPingTest(r, d, vrf)
}

// doPingTest performs the actual ping test
func doPingTest(r *bufio.Reader, d *DeviceInfo, vrf string) {
	fmt.Print("Destination IP: ")
	dst := readLine(r)
	if dst == "" {
		fmt.Println("No destination specified")
		return
	}

	fmt.Print("Repeat count [5]: ")
	cnt := 5
	if c, _ := strconv.Atoi(readLine(r)); c > 0 {
		cnt = c
	}

	// Build command
	var cmd string
	if d.DeviceType == "ios-xr" {
		cmd = fmt.Sprintf("ping vrf %s %s count %d", vrf, dst, cnt)
	} else {
		cmd = fmt.Sprintf("ping vrf %s %s repeat %d", vrf, dst, cnt)
	}

	fmt.Printf("\n--- Executing on %s ---\n", d.Hostname)
	fmt.Printf("VRF: %s\n", vrf)
	fmt.Printf("Command: %s\n", cmd)
	fmt.Println(strings.Repeat("-", 50))

	out, err := execCmd(*d, cmd)
	if err != nil {
		fmt.Printf("\n✗ ERROR: %v\n", err)
		return
	}

	// Show full output
	fmt.Println(out)
	fmt.Println(strings.Repeat("-", 50))

	// Parse result
	rate := pingRate(out)
	if rate == 100 {
		fmt.Printf("\n✓ VRF %s: PING SUCCESSFUL (100%%)\n", vrf)
	} else if rate > 0 {
		fmt.Printf("\n⚠ VRF %s: PARTIAL SUCCESS (%d%%)\n", vrf, rate)
	} else {
		fmt.Printf("\n✗ VRF %s: PING FAILED\n", vrf)
	}
}

func batchPing(r *bufio.Reader) {
	d := selectDev(r)
	if d == nil {
		return
	}

	// Get VRFs for this device
	vrfList := getVRFsForDevice(d.Hostname)

	if len(vrfList) == 0 {
		fmt.Println("\nNo VRFs available. Using static VRF list.")
		vrfList = vrfs
	}

	fmt.Printf("\n--- Batch Ping Test from %s ---\n", d.Hostname)
	fmt.Printf("Testing %d VRFs (ping to loopback 127.0.0.1 or specify destination)\n", len(vrfList))

	fmt.Print("Destination IP (blank=127.0.0.1): ")
	dst := readLine(r)
	if dst == "" {
		dst = "127.0.0.1"
	}

	fmt.Println("\n╔══════════════════════════════╦══════════╦════════════════════════════════╗")
	fmt.Println("║ VRF                          ║ Result   ║ Details                        ║")
	fmt.Println("╠══════════════════════════════╬══════════╬════════════════════════════════╣")

	passCount, failCount := 0, 0
	for _, v := range vrfList {
		var cmd string
		if d.DeviceType == "ios-xr" {
			cmd = fmt.Sprintf("ping vrf %s %s count 3", v.Name, dst)
		} else {
			cmd = fmt.Sprintf("ping vrf %s %s repeat 3", v.Name, dst)
		}

		out, err := execCmd(*d, cmd)
		result := "FAIL"
		details := "Error or timeout"

		if err != nil {
			details = fmt.Sprintf("Error: %v", err)
		} else {
			rate := pingRate(out)
			if rate == 100 {
				result = "OK"
				details = "100% success"
				passCount++
			} else if rate > 0 {
				result = "PARTIAL"
				details = fmt.Sprintf("%d%% success", rate)
			} else {
				if strings.Contains(out, "Invalid") {
					details = "VRF not found"
				} else if strings.Contains(out, "0 percent") {
					details = "0% - unreachable"
				}
				failCount++
			}
		}

		fmt.Printf("║ %-28s ║ %-8s ║ %-30s ║\n", trunc(v.Name, 28), result, trunc(details, 30))
	}

	fmt.Println("╚══════════════════════════════╩══════════╩════════════════════════════════╝")
	fmt.Printf("\nSummary: %d passed, %d failed out of %d VRFs\n", passCount, failCount, len(vrfList))
}

func traceroute(r *bufio.Reader) {
	d := selectDev(r)
	if d == nil {
		return
	}
	fmt.Print("VRF (blank=default): ")
	vrf := readLine(r)
	fmt.Print("Destination: ")
	dst := readLine(r)
	var cmd string
	if vrf == "" {
		cmd = fmt.Sprintf("traceroute %s", dst)
	} else {
		cmd = fmt.Sprintf("traceroute vrf %s %s", vrf, dst)
	}
	fmt.Println("\nRunning...")
	out, _ := execCmd(*d, cmd)
	fmt.Println("\n" + out)
}

func mplsTrace(r *bufio.Reader) {
	d := selectDev(r)
	if d == nil {
		return
	}
	fmt.Print("Destination FEC: ")
	fec := readLine(r)
	out, _ := execCmd(*d, fmt.Sprintf("traceroute mpls ipv4 %s", fec))
	fmt.Println("\n" + out)
}

// ============ MTU ============

func mtuTest(r *bufio.Reader) {
	d := selectDev(r)
	if d == nil {
		return
	}
	fmt.Print("VRF: ")
	vrf := readLine(r)
	fmt.Print("Source IP: ")
	src := readLine(r)
	fmt.Print("Destination IP: ")
	dst := readLine(r)
	fmt.Println("\nMTU Discovery (1500-9000)...")
	max := findMTU(*d, vrf, src, dst)
	fmt.Printf("\n✓ Max MTU: %d bytes\n", max)
}

func findMTU(d DeviceInfo, vrf, src, dst string) int {
	low, high, best := 1500, 9000, 1500
	for low <= high {
		mid := (low + high) / 2
		var cmd string
		if d.DeviceType == "ios-xr" {
			cmd = fmt.Sprintf("ping vrf %s %s source %s size %d df-bit count 3", vrf, dst, src, mid)
		} else {
			cmd = fmt.Sprintf("ping vrf %s %s source %s size %d df-bit repeat 3", vrf, dst, src, mid)
		}
		out, _ := execCmd(d, cmd)
		fmt.Printf("  %d bytes: ", mid)
		if pingRate(out) == 100 {
			fmt.Println("OK")
			best = mid
			low = mid + 1
		} else {
			fmt.Println("FAIL")
			high = mid - 1
		}
		time.Sleep(time.Second)
	}
	return best
}

func jumboTest(r *bufio.Reader) {
	d := selectDev(r)
	if d == nil {
		return
	}
	out, _ := execCmd(*d, "show interfaces | include MTU")
	fmt.Println("\n" + out)
}

// ============ Traffic Drain ============

func drainMon(r *bufio.Reader) {
	d := selectDev(r)
	if d == nil {
		return
	}
	fmt.Print("Interface: ")
	intf := readLine(r)
	fmt.Printf("\nMonitoring %s (threshold: %.1f Mbps)...\n", intf, drainThreshold)
	stable := 0
	for i := 0; i < 30; i++ {
		out, _ := execCmd(*d, fmt.Sprintf("show interface %s | include rate", intf))
		in, ou := parseRates(out)
		max := in
		if ou > max {
			max = ou
		}
		fmt.Printf("[%s] IN:%.2f OUT:%.2f MAX:%.2f Mbps", time.Now().Format("15:04:05"), in, ou, max)
		if max < drainThreshold {
			stable++
			fmt.Printf(" [%d/6]\n", stable)
			if stable >= 6 {
				fmt.Println("\n✓ DRAIN COMPLETE - Safe to proceed")
				return
			}
		} else {
			stable = 0
			fmt.Println()
		}
		time.Sleep(5 * time.Second)
	}
}

func checkTraffic(r *bufio.Reader) {
	d := selectDev(r)
	if d == nil {
		return
	}
	fmt.Print("Interface: ")
	intf := readLine(r)
	out, _ := execCmd(*d, fmt.Sprintf("show interface %s | include rate", intf))
	in, ou := parseRates(out)
	fmt.Printf("\nIN: %.2f Mbps | OUT: %.2f Mbps\n", in, ou)
}

func setThresh(r *bufio.Reader) {
	fmt.Printf("Current: %.1f Mbps\nNew: ", drainThreshold)
	if v, err := strconv.ParseFloat(readLine(r), 64); err == nil {
		drainThreshold = v
		fmt.Printf("✓ Set to %.1f Mbps\n", v)
	}
}

// ============ Reports ============

func genHTML(typ string) {
	if len(results) == 0 {
		fmt.Println("No results to report.")
		return
	}
	ts := time.Now().Format("20060102_150405")
	path := filepath.Join(outputDir, fmt.Sprintf("%s_%s.html", typ, ts))
	pass, fail, warn := 0, 0, 0
	for _, r := range results {
		switch r.Status {
		case "PASS":
			pass++
		case "FAIL":
			fail++
		case "WARN":
			warn++
		}
	}
	overall := "PASS"
	if fail > 0 {
		overall = "FAIL"
	} else if warn > 0 {
		overall = "WARNING"
	}
	html := genHTMLContent(typ, overall, pass, fail, warn)
	os.WriteFile(path, []byte(html), 0644)
	fmt.Printf("✓ Report: %s\n", path)
	openBrowser(path)
}

func genHTMLContent(typ, overall string, pass, fail, warn int) string {
	grouped := make(map[string][]ValidationResult)
	for _, r := range results {
		grouped[r.CheckName] = append(grouped[r.CheckName], r)
	}
	var rows strings.Builder
	for check, res := range grouped {
		for _, r := range res {
			class := "pass"
			if r.Status == "FAIL" {
				class = "fail"
			} else if r.Status == "WARN" {
				class = "warn"
			}
			rows.WriteString(fmt.Sprintf("<tr class='%s'><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>",
				class, check, r.Device, r.Status, r.Details))
		}
	}
	return fmt.Sprintf(`<!DOCTYPE html>
<html><head><meta charset="UTF-8"><title>MERALCO %s Report</title>
<style>
body{font-family:Arial,sans-serif;margin:20px;background:#f5f5f5}
.container{max-width:1200px;margin:0 auto;background:#fff;padding:30px;border-radius:10px;box-shadow:0 2px 10px rgba(0,0,0,0.1)}
h1{color:#1a365d;border-bottom:3px solid #3b82f6;padding-bottom:10px}
.stats{display:flex;gap:20px;margin:20px 0}
.stat{padding:20px;border-radius:8px;text-align:center;flex:1}
.stat.pass{background:#dcfce7;color:#166534}
.stat.fail{background:#fee2e2;color:#991b1b}
.stat.warn{background:#fef3c7;color:#92400e}
.stat .num{font-size:2.5em;font-weight:bold}
.overall{padding:15px 30px;border-radius:25px;font-size:1.2em;font-weight:bold;display:inline-block;margin:10px 0}
.overall.PASS{background:#22c55e;color:#fff}
.overall.FAIL{background:#ef4444;color:#fff}
.overall.WARNING{background:#f59e0b;color:#fff}
table{width:100%%;border-collapse:collapse;margin-top:20px}
th{background:#1a365d;color:#fff;padding:12px;text-align:left}
td{padding:10px;border-bottom:1px solid #e5e7eb}
tr.pass{border-left:4px solid #22c55e}
tr.fail{border-left:4px solid #ef4444}
tr.warn{border-left:4px solid #f59e0b}
tr:hover{background:#f9fafb}
.meta{color:#6b7280;font-size:0.9em}
</style></head>
<body><div class="container">
<h1>🔌 MERALCO %s Migration Report</h1>
<p class="meta">Generated: %s | ASR9010 → ASR9906 Migration</p>
<div><span class="overall %s">%s</span></div>
<div class="stats">
<div class="stat pass"><div class="num">%d</div>Passed</div>
<div class="stat fail"><div class="num">%d</div>Failed</div>
<div class="stat warn"><div class="num">%d</div>Warnings</div>
</div>
<table><tr><th>Check</th><th>Device</th><th>Status</th><th>Details</th></tr>%s</table>
<p class="meta" style="margin-top:30px">MERALCO Network Migration Toolkit v%s</p>
</div></body></html>`, typ, typ, time.Now().Format("2006-01-02 15:04:05"), overall, overall, pass, fail, warn, rows.String(), AppVersion)
}

func exportJSON() {
	ts := time.Now().Format("20060102_150405")
	path := filepath.Join(outputDir, fmt.Sprintf("export_%s.json", ts))
	data := map[string]interface{}{"timestamp": time.Now(), "devices": devices, "vrfs": vrfs, "baseline": baseline, "results": results}
	j, _ := json.MarshalIndent(data, "", "  ")
	os.WriteFile(path, j, 0644)
	fmt.Printf("✓ Exported: %s\n", path)
}

func listReports() {
	files, _ := filepath.Glob(filepath.Join(outputDir, "*.html"))
	if len(files) == 0 {
		fmt.Println("No reports.")
		return
	}
	fmt.Println("\nReports:")
	for i, f := range files {
		fmt.Printf("  %d. %s\n", i+1, filepath.Base(f))
	}
}

// ============ Baseline ============

func loadBase(r *bufio.Reader) {
	fmt.Print("File: ")
	p := readLine(r)
	data, err := os.ReadFile(p)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	baseline = &MigrationBaseline{}
	json.Unmarshal(data, baseline)
	fmt.Printf("✓ Loaded baseline from %s\n", p)
}

func viewBase() {
	if baseline == nil {
		fmt.Println("No baseline.")
		return
	}
	fmt.Printf("\nBaseline: %s\n", baseline.Timestamp.Format("2006-01-02 15:04:05"))
	fmt.Println("╔═══════════════════╦═══════╦═══════╦═══════╦═══════╗")
	fmt.Println("║ Device            ║ BGP   ║ OSPF  ║ LDP   ║ MPLS  ║")
	fmt.Println("╠═══════════════════╬═══════╬═══════╬═══════╬═══════╣")
	for n, d := range baseline.Devices {
		fmt.Printf("║ %-17s ║ %-5d ║ %-5d ║ %-5d ║ %-5d ║\n",
			trunc(n, 17), d.BGPSessions, d.OSPFNeighbors, d.LDPNeighbors, d.MPLSLabels)
	}
	fmt.Println("╚═══════════════════╩═══════╩═══════╩═══════╩═══════╝")
}

// ============ Helpers ============

func readLine(r *bufio.Reader) string {
	s, _ := r.ReadString('\n')
	return strings.TrimSpace(s)
}

func selectDev(r *bufio.Reader) *DeviceInfo {
	if len(devices) == 0 {
		fmt.Println("No devices.")
		return nil
	}
	listDevices()
	fmt.Print("Select device #: ")
	n, _ := strconv.Atoi(readLine(r))
	if n > 0 && n <= len(devices) {
		return &devices[n-1]
	}
	fmt.Println("Invalid.")
	return nil
}

func trunc(s string, n int) string {
	if len(s) > n {
		return s[:n-1] + "…"
	}
	return s
}

func countMatch(s, pattern string) int {
	re := regexp.MustCompile(pattern)
	return len(re.FindAllString(s, -1))
}

func countLines(s string) int {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	count := 0
	for _, l := range lines {
		if strings.TrimSpace(l) != "" {
			count++
		}
	}
	return count
}

func sumPfx(s string) int {
	re := regexp.MustCompile(`\s(\d+)\s*$`)
	total := 0
	for _, line := range strings.Split(s, "\n") {
		if m := re.FindStringSubmatch(line); len(m) > 1 {
			if n, _ := strconv.Atoi(m[1]); n > 0 {
				total += n
			}
		}
	}
	return total
}

func parseRoutes(s string) int {
	re := regexp.MustCompile(`Total\s+(\d+)`)
	if m := re.FindStringSubmatch(s); len(m) > 1 {
		n, _ := strconv.Atoi(m[1])
		return n
	}
	return countLines(s)
}

func parseRates(s string) (float64, float64) {
	inRe := regexp.MustCompile(`input rate\s+(\d+)\s+bits`)
	outRe := regexp.MustCompile(`output rate\s+(\d+)\s+bits`)
	var in, out float64
	if m := inRe.FindStringSubmatch(s); len(m) > 1 {
		if n, _ := strconv.ParseFloat(m[1], 64); n > 0 {
			in = n / 1000000
		}
	}
	if m := outRe.FindStringSubmatch(s); len(m) > 1 {
		if n, _ := strconv.ParseFloat(m[1], 64); n > 0 {
			out = n / 1000000
		}
	}
	return in, out
}

func pingRate(s string) int {
	// Try IOS-XR/IOS format: "Success rate is X percent"
	re1 := regexp.MustCompile(`Success rate is\s+(\d+)\s*percent`)
	if m := re1.FindStringSubmatch(s); len(m) > 1 {
		n, _ := strconv.Atoi(m[1])
		return n
	}

	// Try format: "X/Y" (X received out of Y sent)
	re2 := regexp.MustCompile(`(\d+)/(\d+)`)
	if m := re2.FindStringSubmatch(s); len(m) > 2 {
		received, _ := strconv.Atoi(m[1])
		sent, _ := strconv.Atoi(m[2])
		if sent > 0 {
			return (received * 100) / sent
		}
	}

	// Try format: "X% packet loss" (Linux style)
	re3 := regexp.MustCompile(`(\d+)%\s*packet loss`)
	if m := re3.FindStringSubmatch(s); len(m) > 1 {
		loss, _ := strconv.Atoi(m[1])
		return 100 - loss
	}

	// Try format: "round-trip" or "rtt" (indicates at least some success)
	if strings.Contains(s, "round-trip") || strings.Contains(s, "rtt min") {
		// If we see round-trip stats, at least some pings succeeded
		// Try to find the actual success count
		re4 := regexp.MustCompile(`(\d+)\s+packets received`)
		if m := re4.FindStringSubmatch(s); len(m) > 1 {
			received, _ := strconv.Atoi(m[1])
			if received > 0 {
				return 100 // Assume success if we see packets received
			}
		}
	}

	// Check for explicit success indicators
	if strings.Contains(s, "100 percent") || strings.Contains(s, "100%") {
		return 100
	}

	// Check for complete failure
	if strings.Contains(s, "0 percent") || strings.Contains(s, "100% packet loss") {
		return 0
	}

	return 0
}

func addRes(check, device, status, details string) {
	results = append(results, ValidationResult{
		CheckName: check, Device: device, Status: status, Details: details, Timestamp: time.Now(),
	})
}

func printSummary(dur time.Duration) {
	pass, fail, warn := 0, 0, 0
	for _, r := range results {
		switch r.Status {
		case "PASS":
			pass++
		case "FAIL":
			fail++
		case "WARN":
			warn++
		}
	}
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Printf("SUMMARY: %d Pass | %d Fail | %d Warn | Duration: %s\n", pass, fail, warn, dur.Round(time.Second))
	if fail > 0 {
		fmt.Println("Overall: FAIL")
	} else if warn > 0 {
		fmt.Println("Overall: WARNING")
	} else {
		fmt.Println("Overall: PASS")
	}
	fmt.Println(strings.Repeat("=", 60))
}

func openBrowser(path string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", path)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	cmd.Start()
}
