package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// Robot Framework Remote Library Protocol structures
type RPCRequest struct {
	Method string                 `json:"method"`
	Args   []interface{}          `json:"args"`
	Kwargs map[string]interface{} `json:"kwargs"`
}

type RPCResponse struct {
	Status    string      `json:"status"` // "PASS" or "FAIL"
	Return    interface{} `json:"return"`
	Error     string      `json:"error,omitempty"`
	Output    string      `json:"output,omitempty"`
	Traceback string      `json:"traceback,omitempty"`
}

// NetworkLibrary holds active SSH connections
type NetworkLibrary struct {
	connections map[string]*SSHConnection
}

// SSHConnection represents a device connection
type SSHConnection struct {
	client     *ssh.Client
	hostname   string
	deviceType string
	username   string
	password   string
	config     *ssh.ClientConfig
}

func NewNetworkLibrary() *NetworkLibrary {
	return &NetworkLibrary{
		connections: make(map[string]*SSHConnection),
	}
}

// Connect to network device
func (lib *NetworkLibrary) ConnectToDevice(hostname, deviceType, username, password string) (string, error) {
	log.Printf("Connecting to %s (%s)...", hostname, deviceType)

	config := &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         30 * time.Second,
	}

	// Create persistent connection
	client, err := ssh.Dial("tcp", hostname+":22", config)
	if err != nil {
		return "", fmt.Errorf("SSH connection failed: %v", err)
	}

	// Store connection (client stays open)
	conn := &SSHConnection{
		client:     client,
		hostname:   hostname,
		deviceType: deviceType,
		username:   username,
		password:   password,
		config:     config,
	}

	// Generate unique handle
	handle := fmt.Sprintf("conn_%d", len(lib.connections)+1)
	lib.connections[handle] = conn

	log.Printf("✓ Connected to %s (handle: %s)", hostname, handle)
	return handle, nil
}

// ExecuteCommand - Interactive shell version for IOS-XR/IOS-XE
func (lib *NetworkLibrary) ExecuteCommand(handle, command string) (string, error) {
	conn, ok := lib.connections[handle]
	if !ok {
		return "", fmt.Errorf("invalid connection handle: %s", handle)
	}

	log.Printf("Executing on %s: %s", conn.hostname, command)

	// Try to create session with existing client
	session, err := conn.client.NewSession()
	if err != nil {
		// If session creation failed, reconnect
		log.Printf("Session creation failed, reconnecting...")

		client, dialErr := ssh.Dial("tcp", conn.hostname+":22", conn.config)
		if dialErr != nil {
			return "", fmt.Errorf("SSH reconnect failed: %v", dialErr)
		}

		// Update stored client
		if conn.client != nil {
			conn.client.Close()
		}
		conn.client = client

		// Try session again
		session, err = conn.client.NewSession()
		if err != nil {
			return "", fmt.Errorf("failed to create session after reconnect: %v", err)
		}
	}
	defer session.Close()

	// Request PTY for interactive shell (required for IOS-XR)
	modes := ssh.TerminalModes{
		ssh.ECHO:          0,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	if err := session.RequestPty("vt100", 80, 40, modes); err != nil {
		return "", fmt.Errorf("PTY request failed: %v", err)
	}

	// Set up stdin/stdout pipes
	stdin, err := session.StdinPipe()
	if err != nil {
		return "", fmt.Errorf("stdin pipe error: %v", err)
	}

	stdout, err := session.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("stdout pipe error: %v", err)
	}

	// Start interactive shell
	if err := session.Shell(); err != nil {
		return "", fmt.Errorf("shell start error: %v", err)
	}

	// Wait for initial prompt (2 seconds for device to be ready)
	time.Sleep(2 * time.Second)

	// Disable pagination
	fmt.Fprintf(stdin, "terminal length 0\n")
	time.Sleep(500 * time.Millisecond)

	// Send the actual command
	fmt.Fprintf(stdin, "%s\n", command)

	// Wait for command to complete
	time.Sleep(2 * time.Second)

	// Send exit to close cleanly
	fmt.Fprintf(stdin, "exit\n")

	// Read all output
	var outputBuf bytes.Buffer
	io.Copy(&outputBuf, stdout)

	// Wait for session to finish
	session.Wait()

	result := outputBuf.String()

	// Clean the output - remove prompts and command echoes
	result = cleanOutput(result, command)

	log.Printf("✓ Command completed, output length: %d bytes", len(result))
	return result, nil
}

// cleanOutput removes command echoes and prompts from output
func cleanOutput(output, command string) string {
	lines := strings.Split(output, "\n")
	var cleanLines []string
	inOutput := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Skip empty lines at start
		if !inOutput && trimmed == "" {
			continue
		}

		// Skip terminal length command
		if strings.Contains(line, "terminal length") {
			continue
		}

		// Skip the command echo itself
		if strings.Contains(line, command) && !inOutput {
			inOutput = true
			continue
		}

		// Skip prompt lines (RP/0/0/CPU0:hostname#)
		if strings.Contains(trimmed, "RP/") && strings.Contains(trimmed, "#") && len(trimmed) < 50 {
			continue
		}

		// Skip router> or router# prompts
		if (strings.HasSuffix(trimmed, "#") || strings.HasSuffix(trimmed, ">")) && len(trimmed) < 30 {
			continue
		}

		// Skip exit command
		if trimmed == "exit" {
			break
		}

		if inOutput {
			cleanLines = append(cleanLines, line)
		}
	}

	return strings.Join(cleanLines, "\n")
}

// Get OSPF neighbors
func (lib *NetworkLibrary) GetOSPFNeighbors(handle string) ([]map[string]string, error) {
	output, err := lib.ExecuteCommand(handle, "show ospf neighbor")
	if err != nil {
		return nil, err
	}

	return parseOSPFNeighbors(output), nil
}

// Parse OSPF neighbor output
func parseOSPFNeighbors(output string) []map[string]string {
	neighbors := []map[string]string{}
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		if strings.Contains(line, "FULL") && strings.Contains(line, ".") {
			fields := strings.Fields(line)
			if len(fields) >= 6 {
				neighbor := map[string]string{
					"neighbor_id": fields[0],
					"priority":    fields[1],
					"state":       fields[2],
					"dead_time":   fields[3],
					"address":     fields[4],
					"interface":   fields[5],
				}
				neighbors = append(neighbors, neighbor)
			}
		}
	}

	return neighbors
}

// Get BGP summary
func (lib *NetworkLibrary) GetBGPSummary(handle, vrf string) (map[string]interface{}, error) {
	var command string
	if vrf == "" || vrf == "default" {
		command = "show bgp summary"
	} else {
		command = fmt.Sprintf("show bgp vrf %s summary", vrf)
	}

	output, err := lib.ExecuteCommand(handle, command)
	if err != nil {
		return nil, err
	}

	return parseBGPSummary(output), nil
}

// Parse BGP summary output
func parseBGPSummary(output string) map[string]interface{} {
	result := map[string]interface{}{
		"peers":       []map[string]string{},
		"established": 0,
	}

	lines := strings.Split(output, "\n")
	established := 0

	for _, line := range lines {
		if strings.Contains(line, ".") && len(strings.Fields(line)) >= 9 {
			fields := strings.Fields(line)
			state := fields[len(fields)-1]

			peer := map[string]string{
				"neighbor": fields[0],
				"asn":      fields[1],
				"state":    state,
			}

			if strings.Contains(state, "Established") || isNumeric(state) {
				established++
				peer["state"] = "Established"
			}

			result["peers"] = append(result["peers"].([]map[string]string), peer)
		}
	}

	result["established"] = established
	return result
}

func isNumeric(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return len(s) > 0
}

// Get interface status
func (lib *NetworkLibrary) GetInterfaceStatus(handle, interfaceName string) (map[string]string, error) {
	command := fmt.Sprintf("show interface %s", interfaceName)
	output, err := lib.ExecuteCommand(handle, command)
	if err != nil {
		return nil, err
	}

	status := map[string]string{
		"interface": interfaceName,
		"status":    "unknown",
		"protocol":  "unknown",
	}

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "line protocol") {
			if strings.Contains(line, "up") {
				if strings.Contains(strings.ToLower(line), "line protocol is up") {
					status["status"] = "up"
					status["protocol"] = "up"
				} else {
					status["status"] = "up"
					status["protocol"] = "down"
				}
			} else if strings.Contains(line, "down") {
				status["status"] = "down"
				status["protocol"] = "down"
			}
			break
		}
	}

	return status, nil
}

// Ping test
func (lib *NetworkLibrary) PingTest(handle, target, vrf string, count int) (map[string]interface{}, error) {
	var command string
	if vrf == "" || vrf == "default" {
		command = fmt.Sprintf("ping %s count %d", target, count)
	} else {
		command = fmt.Sprintf("ping vrf %s %s count %d", vrf, target, count)
	}

	output, err := lib.ExecuteCommand(handle, command)
	if err != nil {
		return nil, err
	}

	return parsePingOutput(output, count), nil
}

// Parse ping output
func parsePingOutput(output string, count int) map[string]interface{} {
	result := map[string]interface{}{
		"sent":        count,
		"received":    0,
		"success_pct": 0.0,
	}

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "Success rate") {
			parts := strings.Split(line, "percent")
			if len(parts) > 0 {
				percentPart := strings.TrimSpace(parts[0])
				fields := strings.Fields(percentPart)
				if len(fields) > 0 {
					lastField := fields[len(fields)-1]
					var pct float64
					fmt.Sscanf(lastField, "%f", &pct)
					result["success_pct"] = pct
					result["received"] = int(float64(count) * pct / 100.0)
				}
			}
		}
	}

	return result
}

// Close connection
func (lib *NetworkLibrary) CloseConnection(handle string) error {
	conn, ok := lib.connections[handle]
	if !ok {
		return fmt.Errorf("invalid connection handle: %s", handle)
	}

	// Close SSH client if still open
	if conn.client != nil {
		conn.client.Close()
	}

	delete(lib.connections, handle)
	log.Printf("✓ Closed connection handle: %s", handle)
	return nil
}

// Handle RPC request
func (lib *NetworkLibrary) HandleRequest(req RPCRequest) RPCResponse {
	log.Printf("→ Method: %s, Args: %v", req.Method, req.Args)

	switch req.Method {
	case "get_keyword_names":
		return RPCResponse{
			Status: "PASS",
			Return: []string{
				"Connect To Device",
				"Execute Command",
				"Get OSPF Neighbors",
				"Get BGP Summary",
				"Get Interface Status",
				"Ping Test",
				"Close Connection",
			},
		}

	case "run_keyword":
		if len(req.Args) < 1 {
			return RPCResponse{Status: "FAIL", Error: "No keyword specified"}
		}
		keyword := req.Args[0].(string)
		args := []interface{}{}
		if len(req.Args) > 1 {
			args = req.Args[1].([]interface{})
		}
		return lib.runKeyword(keyword, args, req.Kwargs)

	default:
		return RPCResponse{Status: "FAIL", Error: fmt.Sprintf("Unknown method: %s", req.Method)}
	}
}

func (lib *NetworkLibrary) runKeyword(keyword string, args []interface{}, kwargs map[string]interface{}) RPCResponse {
	switch keyword {
	case "Connect To Device":
		if len(args) < 4 {
			return RPCResponse{Status: "FAIL", Error: "Expected 4 arguments: hostname, device_type, username, password"}
		}
		handle, err := lib.ConnectToDevice(
			args[0].(string),
			args[1].(string),
			args[2].(string),
			args[3].(string),
		)
		if err != nil {
			return RPCResponse{Status: "FAIL", Error: err.Error()}
		}
		return RPCResponse{
			Status: "PASS",
			Return: handle,
			Output: fmt.Sprintf("Connected to %s", args[0]),
		}

	case "Execute Command":
		if len(args) < 2 {
			return RPCResponse{Status: "FAIL", Error: "Expected 2 arguments: handle, command"}
		}
		output, err := lib.ExecuteCommand(args[0].(string), args[1].(string))
		if err != nil {
			return RPCResponse{Status: "FAIL", Error: err.Error()}
		}
		return RPCResponse{Status: "PASS", Return: output}

	case "Get OSPF Neighbors":
		if len(args) < 1 {
			return RPCResponse{Status: "FAIL", Error: "Expected 1 argument: handle"}
		}
		neighbors, err := lib.GetOSPFNeighbors(args[0].(string))
		if err != nil {
			return RPCResponse{Status: "FAIL", Error: err.Error()}
		}
		return RPCResponse{
			Status: "PASS",
			Return: neighbors,
			Output: fmt.Sprintf("Found %d OSPF neighbors", len(neighbors)),
		}

	case "Get BGP Summary":
		if len(args) < 1 {
			return RPCResponse{Status: "FAIL", Error: "Expected at least 1 argument: handle"}
		}
		vrf := ""
		if len(args) > 1 {
			vrf = args[1].(string)
		}
		summary, err := lib.GetBGPSummary(args[0].(string), vrf)
		if err != nil {
			return RPCResponse{Status: "FAIL", Error: err.Error()}
		}
		return RPCResponse{Status: "PASS", Return: summary}

	case "Get Interface Status":
		if len(args) < 2 {
			return RPCResponse{Status: "FAIL", Error: "Expected 2 arguments: handle, interface"}
		}
		status, err := lib.GetInterfaceStatus(args[0].(string), args[1].(string))
		if err != nil {
			return RPCResponse{Status: "FAIL", Error: err.Error()}
		}
		return RPCResponse{Status: "PASS", Return: status}

	case "Ping Test":
		if len(args) < 3 {
			return RPCResponse{Status: "FAIL", Error: "Expected 3+ arguments: handle, target, vrf, [count]"}
		}
		count := 5
		if len(args) > 3 {
			count = int(args[3].(float64))
		}
		result, err := lib.PingTest(args[0].(string), args[1].(string), args[2].(string), count)
		if err != nil {
			return RPCResponse{Status: "FAIL", Error: err.Error()}
		}
		return RPCResponse{Status: "PASS", Return: result}

	case "Close Connection":
		if len(args) < 1 {
			return RPCResponse{Status: "FAIL", Error: "Expected 1 argument: handle"}
		}
		err := lib.CloseConnection(args[0].(string))
		if err != nil {
			return RPCResponse{Status: "FAIL", Error: err.Error()}
		}
		return RPCResponse{Status: "PASS", Output: "Connection closed"}

	default:
		return RPCResponse{Status: "FAIL", Error: fmt.Sprintf("Unknown keyword: %s", keyword)}
	}
}

// Handle TCP connection
func handleConnection(conn net.Conn, lib *NetworkLibrary) {
	defer conn.Close()

	decoder := json.NewDecoder(conn)
	encoder := json.NewEncoder(conn)

	for {
		var req RPCRequest
		if err := decoder.Decode(&req); err != nil {
			return
		}

		resp := lib.HandleRequest(req)
		if err := encoder.Encode(resp); err != nil {
			log.Printf("Error encoding response: %v", err)
			return
		}
	}
}

func main() {
	lib := NewNetworkLibrary()

	listener, err := net.Listen("tcp", ":8270")
	if err != nil {
		log.Fatal("Failed to start server:", err)
	}
	defer listener.Close()

	log.Println("=" + strings.Repeat("=", 78))
	log.Println("  Network Migration Go Remote Library")
	log.Println("  Listening on port 8270")
	log.Println("  Ready for Robot Framework connections")
	log.Println("  Using Interactive Shell Mode for IOS-XR/IOS-XE")
	log.Println("=" + strings.Repeat("=", 78))

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println("Error accepting connection:", err)
			continue
		}
		go handleConnection(conn, lib)
	}
}
