"""
GoNetworkLibrary - Enhanced Robot Framework Library for Network Automation
Supports both IOS-XR (ASR9906) and IOS-XE (ASR903) devices
"""

import socket
import json
import re


class GoNetworkLibrary:
    """Robot Framework library for network device automation via Go server"""
    
    ROBOT_LIBRARY_SCOPE = 'GLOBAL'
    
    def __init__(self, host='127.0.0.1', port=8270):
        """Initialize library with Go server connection details"""
        self.host = host
        self.port = port
        self.active_connections = {}
    
    def _send_request(self, method, *args, **kwargs):
        """Send JSON-RPC request to Go server"""
        request = {
            'method': method,
            'args': args,
            'kwargs': kwargs
        }
        
        try:
            # Create socket connection
            sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
            sock.settimeout(60)  # 60 second timeout
            sock.connect((self.host, self.port))
            
            # Send request
            request_json = json.dumps(request)
            sock.sendall((request_json + '\n').encode('utf-8'))
            
            # Receive response
            response_data = b''
            while True:
                chunk = sock.recv(4096)
                if not chunk:
                    break
                response_data += chunk
                if b'\n' in chunk:
                    break
            
            sock.close()
            
            # Parse response
            response = json.loads(response_data.decode('utf-8'))
            
            if response.get('status') == 'FAIL':
                error_msg = response.get('error', 'Unknown error')
                raise Exception(f"Remote execution failed: {error_msg}")
            
            return response.get('return')
            
        except socket.timeout:
            raise Exception(f"Connection to Go server timed out")
        except ConnectionRefusedError:
            raise Exception(f"Cannot connect to Go server at {self.host}:{self.port}. Is the server running?")
        except Exception as e:
            raise Exception(f"Communication with Go server failed: {str(e)}")
    
    # ========================================
    # BASIC CONNECTION KEYWORDS
    # ========================================
    
    def connect_to_device(self, hostname, device_type, username, password):
        """
        Connect to a network device
        
        Args:
            hostname: IP address or hostname
            device_type: Device type (ASR9906, ASR903, ASR9010)
            username: SSH username
            password: SSH password
        
        Returns:
            Connection handle string
        """
        handle = self._send_request('run_keyword', 'Connect To Device',
                                   [hostname, device_type, username, password])
        
        # Store connection info
        self.active_connections[handle] = {
            'hostname': hostname,
            'device_type': device_type
        }
        
        return handle
    
    def close_connection(self, handle):
        """Close connection to device"""
        result = self._send_request('run_keyword', 'Close Connection', [handle])
        
        if handle in self.active_connections:
            del self.active_connections[handle]
        
        return result
    
    def execute_command(self, handle, command):
        """
        Execute command on device
        
        Args:
            handle: Connection handle
            command: Command to execute
        
        Returns:
            Command output as string
        """
        output = self._send_request('run_keyword', 'Execute Command', [handle, command])
        return output
    
    # ========================================
    # ROUTING PROTOCOL KEYWORDS
    # ========================================
    
    def get_ospf_neighbors(self, handle):
        """
        Get OSPF neighbors
        
        Returns:
            List of neighbor dictionaries with keys:
            - neighbor_id
            - priority
            - state
            - dead_time
            - address
            - interface
        """
        neighbors = self._send_request('run_keyword', 'Get OSPF Neighbors', [handle])
        return neighbors if neighbors else []
    
    def get_bgp_summary(self, handle, vrf='default'):
        """
        Get BGP summary
        
        Args:
            handle: Connection handle
            vrf: VRF name (default: 'default')
        
        Returns:
            Dictionary with keys:
            - peers: List of peer dictionaries
            - established: Number of established peers
        """
        summary = self._send_request('run_keyword', 'Get BGP Summary', [handle, vrf])
        return summary if summary else {'peers': [], 'established': 0}
    
    # ========================================
    # INTERFACE KEYWORDS
    # ========================================
    
    def get_interface_status(self, handle, interface):
        """
        Get interface status
        
        Args:
            handle: Connection handle
            interface: Interface name
        
        Returns:
            Dictionary with keys:
            - interface: Interface name
            - status: up/down
            - protocol: up/down
        """
        status = self._send_request('run_keyword', 'Get Interface Status', 
                                    [handle, interface])
        return status if status else {'interface': interface, 'status': 'unknown', 'protocol': 'unknown'}
    
    # ========================================
    # CONNECTIVITY TESTING KEYWORDS
    # ========================================
    
    def ping_test(self, handle, target, vrf='default', count=5):
        """
        Execute ping test
        
        Args:
            handle: Connection handle
            target: Target IP address
            vrf: VRF name (default: 'default')
            count: Number of pings (default: 5)
        
        Returns:
            Dictionary with keys:
            - sent: Number of packets sent
            - received: Number of packets received
            - success_pct: Success percentage
        """
        result = self._send_request('run_keyword', 'Ping Test', 
                                    [handle, target, vrf, count])
        return result if result else {'sent': count, 'received': 0, 'success_pct': 0.0}
    
    # ========================================
    # DEVICE-TYPE-AWARE KEYWORDS (NEW)
    # ========================================
    
    def get_bgp_summary_ios_xe(self, handle, vrf='default'):
        """
        Get BGP summary using IOS-XE command syntax
        
        For ASR903 devices, use 'show bgp vpnv4 vrf X summary'
        
        Args:
            handle: Connection handle
            vrf: VRF name
        
        Returns:
            Dictionary with BGP summary info
        """
        if vrf == 'default':
            command = 'show bgp vpnv4 unicast all summary'
        else:
            command = f'show bgp vpnv4 vrf {vrf} summary'
        
        output = self.execute_command(handle, command)
        
        # Parse output (basic parsing - can be enhanced)
        peers = []
        established_count = 0
        
        lines = output.split('\n')
        for line in lines:
            # Look for neighbor lines (simplified)
            if '.' in line and len(line.split()) >= 9:
                fields = line.split()
                neighbor_ip = fields[0]
                
                # Check if established (look for numeric state or "Established")
                state = fields[-1]
                if state.isdigit() or 'Established' in state:
                    established_count += 1
                    peer_state = 'Established'
                else:
                    peer_state = state
                
                peers.append({
                    'neighbor': neighbor_ip,
                    'asn': fields[1] if len(fields) > 1 else 'unknown',
                    'state': peer_state
                })
        
        return {
            'peers': peers,
            'established': established_count
        }
    
    # ========================================
    # PARSING HELPER KEYWORDS (NEW)
    # ========================================
    
    def parse_interface_status_ios_xr(self, output):
        """
        Parse IOS-XR interface status output
        
        Example input: "GigabitEthernet0/1/0/44 is down, line protocol is down"
        
        Args:
            output: Command output string
        
        Returns:
            Dictionary with status and protocol keys
        """
        status = {'status': 'unknown', 'protocol': 'unknown'}
        
        # Look for "is <status>, line protocol is <protocol>"
        match = re.search(r'is (\w+), line protocol is (\w+)', output)
        if match:
            status['status'] = match.group(1)
            status['protocol'] = match.group(2)
        
        return status
    
    def extract_ping_success_rate(self, ping_output):
        """
        Extract ping success rate from output
        
        Example: "Success rate is 100 percent (10/10)"
        
        Args:
            ping_output: Ping command output
        
        Returns:
            Success rate as integer (0-100)
        """
        # Look for "Success rate is X percent"
        match = re.search(r'Success rate is (\d+) percent', ping_output)
        if match:
            return int(match.group(1))
        
        # Alternative: look for (X/Y) pattern
        match = re.search(r'\((\d+)/(\d+)\)', ping_output)
        if match:
            received = int(match.group(1))
            sent = int(match.group(2))
            if sent > 0:
                return int((received / sent) * 100)
        
        return 0
    
    def extract_route_count(self, route_output):
        """
        Extract route count from route summary output
        
        Args:
            route_output: Output from 'show route vrf X summary'
        
        Returns:
            Number of routes as integer
        """
        # Look for "Total" line with route count
        match = re.search(r'Total\s+(\d+)', route_output, re.IGNORECASE)
        if match:
            return int(match.group(1))
        
        # Alternative patterns
        match = re.search(r'(\d+)\s+routes', route_output, re.IGNORECASE)
        if match:
            return int(match.group(1))
        
        return 0
    
    def parse_cpu_utilization(self, cpu_output):
        """
        Parse CPU utilization from output
        
        Args:
            cpu_output: Output from 'show processes cpu'
        
        Returns:
            CPU percentage as integer
        """
        # Look for "CPU utilization: X%"
        match = re.search(r'CPU utilization.*?(\d+)%', cpu_output)
        if match:
            return int(match.group(1))
        
        # Alternative: look for just percentage
        match = re.search(r'(\d+)%', cpu_output)
        if match:
            return int(match.group(1))
        
        return 0
    
    def parse_memory_utilization(self, memory_output):
        """
        Parse memory utilization from output
        
        Args:
            memory_output: Output from 'show memory summary' or similar
        
        Returns:
            Memory usage percentage as integer
        """
        # Look for percentage pattern
        match = re.search(r'(\d+)%', memory_output)
        if match:
            return int(match.group(1))
        
        # Alternative: calculate from Used/Total
        match = re.search(r'(\d+)K.*?(\d+)K', memory_output)
        if match:
            used = int(match.group(1))
            total = int(match.group(2))
            if total > 0:
                return int((used / total) * 100)
        
        return 0
    
    # ========================================
    # PLACEHOLDER KEYWORDS FOR PARSING
    # These return dummy data and should be replaced with actual parsing
    # ========================================
    
    def extract_ios_version(self, version_output):
        """Extract IOS version from show version output"""
        # Look for version pattern
        match = re.search(r'Version\s+([\d\.]+)', version_output, re.IGNORECASE)
        if match:
            return match.group(1)
        return "Unknown"
    
    def extract_uptime(self, version_output):
        """Extract uptime from show version output"""
        # Look for uptime pattern
        match = re.search(r'uptime is (.+)', version_output, re.IGNORECASE)
        if match:
            return match.group(1)
        return "Unknown"
    
    def extract_interface_errors(self, interface_output):
        """Extract interface error counters"""
        errors = {
            'input_errors': 0,
            'output_errors': 0,
            'crc_errors': 0
        }
        
        # Look for error patterns
        match = re.search(r'(\d+) input errors', interface_output)
        if match:
            errors['input_errors'] = int(match.group(1))
        
        match = re.search(r'(\d+) output errors', interface_output)
        if match:
            errors['output_errors'] = int(match.group(1))
        
        match = re.search(r'(\d+) CRC', interface_output)
        if match:
            errors['crc_errors'] = int(match.group(1))
        
        return errors
    
    # ========================================
    # UTILITY METHODS
    # ========================================
    
    def get_connection_info(self, handle):
        """Get information about a connection"""
        return self.active_connections.get(handle, {})
    
    def is_ios_xr_device(self, device_type):
        """Check if device is IOS-XR (ASR9906 or ASR9010)"""
        return 'ASR9906' in device_type or 'ASR9010' in device_type
    
    def is_ios_xe_device(self, device_type):
        """Check if device is IOS-XE (ASR903)"""
        return 'ASR903' in device_type


# For direct testing
if __name__ == '__main__':
    lib = GoNetworkLibrary()
    print("GoNetworkLibrary initialized successfully")
    print(f"Available keywords: {dir(lib)}")
