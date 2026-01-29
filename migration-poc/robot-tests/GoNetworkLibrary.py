"""
GoNetworkLibrary - Direct JSON-RPC client for Go Remote Library

This bypasses Robot Framework's Remote library which has connection issues.
"""

import socket
import json
import time


class GoNetworkLibrary:
    """Custom Robot Framework library for Go Remote Library"""
    
    ROBOT_LIBRARY_SCOPE = 'GLOBAL'
    
    def __init__(self, host='localhost', port=8270):
        """Initialize connection to Go server"""
        self.host = host
        self.port = port
        self.timeout = 30
        
    def _send_request(self, method, args=None, kwargs=None):
        """Send JSON-RPC request to Go server"""
        if args is None:
            args = []
        if kwargs is None:
            kwargs = {}
            
        # Create request
        request = {
            "method": method,
            "args": args,
            "kwargs": kwargs
        }
        
        # Connect and send
        sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        sock.settimeout(self.timeout)
        
        try:
            sock.connect((self.host, self.port))
            sock.sendall(json.dumps(request).encode() + b'\n')
            
            # Receive response
            response_data = b''
            while True:
                chunk = sock.recv(4096)
                if not chunk:
                    break
                response_data += chunk
                # Check if we have complete JSON
                try:
                    json.loads(response_data.decode())
                    break
                except json.JSONDecodeError:
                    continue
                    
            sock.close()
            
            # Parse response
            response = json.loads(response_data.decode())
            
            if response.get('status') == 'PASS':
                return response.get('return')
            else:
                error = response.get('error', 'Unknown error')
                raise Exception(f"Go server error: {error}")
                
        except socket.timeout:
            raise Exception(f"Timeout connecting to {self.host}:{self.port}")
        except Exception as e:
            raise Exception(f"Connection error: {e}")
        finally:
            try:
                sock.close()
            except:
                pass
    
    def connect_to_device(self, hostname, device_type, username, password):
        """Connect to network device
        
        Returns connection handle string
        """
        result = self._send_request(
            "run_keyword",
            ["Connect To Device", [hostname, device_type, username, password]],
            {}
        )
        return result
    
    def execute_command(self, handle, command):
        """Execute command on device
        
        Returns command output string
        """
        result = self._send_request(
            "run_keyword",
            ["Execute Command", [handle, command]],
            {}
        )
        return result
    
    def get_ospf_neighbors(self, handle):
        """Get OSPF neighbors from device
        
        Returns list of neighbor dictionaries
        """
        result = self._send_request(
            "run_keyword",
            ["Get OSPF Neighbors", [handle]],
            {}
        )
        return result
    
    def get_bgp_summary(self, handle, vrf="default"):
        """Get BGP summary from device
        
        Returns dictionary with peer information
        """
        result = self._send_request(
            "run_keyword",
            ["Get BGP Summary", [handle, vrf]],
            {}
        )
        return result
    
    def get_interface_status(self, handle, interface):
        """Get interface status
        
        Returns dictionary with interface state
        """
        result = self._send_request(
            "run_keyword",
            ["Get Interface Status", [handle, interface]],
            {}
        )
        return result
    
    def ping_test(self, handle, target, vrf="default", count=5):
        """Perform ping test
        
        Returns dictionary with ping results
        """
        result = self._send_request(
            "run_keyword",
            ["Ping Test", [handle, target, vrf, count]],
            {}
        )
        return result
    
    def close_connection(self, handle):
        """Close device connection"""
        result = self._send_request(
            "run_keyword",
            ["Close Connection", [handle]],
            {}
        )
        return result


if __name__ == "__main__":
    # Test the library
    print("Testing GoNetworkLibrary...")
    
    lib = GoNetworkLibrary()
    
    try:
        # Test connection to server
        print("✓ Library initialized")
        
        # Try a simple connection test
        print("Testing connection to Go server...")
        handle = lib.connect_to_device("172.10.1.1", "ASR9906", "admin", "admin")
        print(f"✓ Connected, handle: {handle}")
        
        print("\nLibrary is working!")
        
    except Exception as e:
        print(f"✗ Error: {e}")
