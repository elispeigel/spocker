package network

import (
	"fmt"
	"net"
	"os"
	"testing"

	"github.com/vishvananda/netlink"
)

func TestCreateNetwork(t *testing.T) {
	// Test case 1: valid network configuration with static IP
	config1 := &NetworkConfig{
		Name:    "testnet1",
		IPNet:   &net.IPNet{IP: net.ParseIP("192.168.0.0"), Mask: net.CIDRMask(24, 32)},
		Gateway: net.ParseIP("192.168.0.1"),
		DNS:     []net.IP{net.ParseIP("8.8.8.8")},
		DHCP:    false,
	}
	handler1 := DefaultNetworkHandler{}
	net1, err1 := CreateNetwork(config1, handler1)
	if err1 != nil {
		t.Errorf("Test case 1 failed: %v", err1)
	}
	if net1.Name != "testnet1" || net1.IPNet.String() != "192.168.0.0/24" || net1.Gateway.String() != "192.168.0.1" || len(net1.DNS) != 1 || net1.DNS[0].String() != "8.8.8.8" || net1.DHCP {
		t.Errorf("Test case 1 failed: incorrect network configuration")
	}

	// Test case 2: valid network configuration with DHCP
	config2 := &NetworkConfig{
		Name:     "testnet2",
		IPNet:    &net.IPNet{IP: net.ParseIP("192.168.1.0"), Mask: net.CIDRMask(24, 32)},
		Gateway:  net.ParseIP("192.168.1.1"),
		DNS:      []net.IP{net.ParseIP("8.8.8.8")},
		DHCP:     true,
		DHCPArgs: []string{},
	}
	handler2 := DefaultNetworkHandler{}
	net2, err2 := CreateNetwork(config2, handler2)
	if err2 != nil {
		t.Errorf("Test case 2 failed: %v", err2)
	}
	if net2.Name != "testnet2" || net2.IPNet.String() != "192.168.1.0/24" || net2.Gateway.String() != "192.168.1.1" || len(net2.DNS) != 1 || net2.DNS[0].String() != "8.8.8.8" || !net2.DHCP {
		t.Errorf("Test case 2 failed: incorrect network configuration")
	}

	// Test case 3: invalid network configuration
	config3 := &NetworkConfig{
		Name: "testnet3",
		IPNet: &net.IPNet{
			IP:   net.ParseIP("192.168.2.0"),
			Mask: net.CIDRMask(24, 32),
		},
	}
	handler3 := DefaultNetworkHandler{}
	_, err3 := CreateNetwork(config3, handler3)
	if err3 == nil {
		t.Errorf("Test case 3 failed: expected error but got nil")
	}
}

func TestGetAvailableIP(t *testing.T) {
	// Create a new IPNet with the 192.168.1.0/24 subnet range
	_, ipNet, _ := net.ParseCIDR("192.168.1.0/24")

	// Set up some IP addresses that are already in use
	inUseIPs := []string{"192.168.1.1", "192.168.1.100", "192.168.1.200"}
	for _, ip := range inUseIPs {
		if err := os.Setenv(fmt.Sprintf("IP_IN_USE_%s", ip), "1"); err != nil {
			t.Fatalf("failed to set environment variable: %v", err)
		}
		defer os.Unsetenv(fmt.Sprintf("IP_IN_USE_%s", ip))
	}

	// Call GetAvailableIP and make sure it returns a valid IP address
	handler := DefaultNetworkHandler{}
	ip, err := GetAvailableIP(ipNet, handler)
	if err != nil {
		t.Fatalf("GetAvailableIP returned an error: %v", err)
	}
	if ip == nil {
		t.Fatal("GetAvailableIP returned nil")
	}
	if !ipNet.Contains(ip) {
		t.Fatalf("GetAvailableIP returned an IP outside of the subnet range: %v", ip)
	}
	if IsIPInUse(ip, handler) {
		t.Fatalf("GetAvailableIP returned an IP that is already in use: %v", ip)
	}
}

func TestIsIPInUse(t *testing.T) {
	// Set up an IP address that is in use
	inUseIP := "192.168.1.1"
	inUseAddr := net.ParseIP(inUseIP)
	if err := os.Setenv(fmt.Sprintf("IP_IN_USE_%s", inUseIP), "1"); err != nil {
		t.Fatalf("failed to set environment variable: %v", err)
	}
	defer os.Unsetenv(fmt.Sprintf("IP_IN_USE_%s", inUseIP))

	// Test that the in-use IP address is detected as being in use
	handler := DefaultNetworkHandler{}
	if !IsIPInUse(inUseAddr, handler) {
		t.Fatalf("IsIPInUse failed to detect an in-use IP address: %v", inUseAddr)
	}

	// Test that a different IP address is detected as not being in use
	unusedIP := "192.168.1.2"
	unusedAddr := net.ParseIP(unusedIP)
	if IsIPInUse(unusedAddr, handler) {
		t.Fatalf("IsIPInUse incorrectly detected an unused IP address as being in use: %v", unusedAddr)
	}
}

func TestGetDefaultGateway(t *testing.T) {
	expectedGateway := net.ParseIP("192.168.1.1")
	ipNet := &net.IPNet{
		IP:   net.ParseIP("192.168.1.10"),
		Mask: net.IPv4Mask(255, 255, 255, 0),
	}

	handler := DefaultNetworkHandler{}
	gateway := GetDefaultGateway(ipNet, handler)

	if gateway == nil {
		t.Errorf("GetDefaultGateway returned nil, expected %v", expectedGateway)
	}

	if !gateway.Equal(expectedGateway) {
		t.Errorf("GetDefaultGateway returned %v, expected %v", gateway, expectedGateway)
	}
}

func TestGetDefaultDNS(t *testing.T) {
	// Create a temporary file with sample data
	content := []byte("nameserver 8.8.8.8\nnameserver 8.8.4.4\n")
	tmpfile, err := os.CreateTemp("", "resolv.conf")
	if err != nil {
		t.Fatalf("Failed to create temporary file: %v", err)
	}
	defer os.Remove(tmpfile.Name())
	if _, err := tmpfile.Write(content); err != nil {
		t.Fatalf("Failed to write to temporary file: %v", err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatalf("Failed to close temporary file: %v", err)
	}

	// Test the GetDefaultDNS function with the temporary file
	expected := net.ParseIP("8.8.8.8")
	actual := GetDefaultDNS()
	if actual == nil {
		t.Errorf("GetDefaultDNS returned nil")
	} else if !actual.Equal(expected) {
		t.Errorf("GetDefaultDNS returned %v, expected %v", actual, expected)
	}
}

func TestDeleteNetwork(t *testing.T) {
	// Create a virtual network interface to use for the test
	ifName := "testnet"
	err := createTestNetwork(ifName)
	if err != nil {
		t.Fatalf("Failed to create test network: %v", err)
	}
	defer func() {
		// Clean up the test network after the test
		err := DeleteNetwork(ifName)
		if err != nil {
			t.Fatalf("Failed to delete test network: %v", err)
		}
	}()

	// Call the function to be tested
	err = DeleteNetwork(ifName)

	// Check that the error returned is nil
	if err != nil {
		t.Errorf("DeleteNetwork(%s) returned error: %v", ifName, err)
	}

	// Check that the network interface was actually deleted
	_, err = net.InterfaceByName(ifName)
	if err == nil {
		t.Errorf("Network interface %s still exists after calling DeleteNetwork", ifName)
	}
}

// Helper function to create a virtual network interface for testing
func createTestNetwork(ifName string) error {
	link := &netlink.Dummy{
		LinkAttrs: netlink.LinkAttrs{
			Name: ifName,
		},
	}
	err := netlink.LinkAdd(link)
	if err != nil {
		return err
	}
	return nil
}

func TestConnectToNetwork(t *testing.T) {
	networkName := "test_network"
	err := createTestNetwork(networkName)
	if err != nil {
		t.Fatalf("Failed to create test network: %v", err)
	}

	containerID := "test_container"
	ipNet := &net.IPNet{
		IP:   net.IPv4(192, 168, 0, 2),
		Mask: net.CIDRMask(24, 32),
	}
	network := &Network{
		Name:    networkName,
		IPNet:   ipNet,
		Gateway: net.ParseIP("192.168.0.1"),
		DNS:     []net.IP{net.ParseIP("8.8.8.8")},
	}

	err = ConnectToNetwork(containerID, network)
	if err != nil {
		t.Fatalf("Failed to connect container %s to network %s: %v", containerID, networkName, err)
	}

	// Check that the container is assigned the correct IP address
	addrs, err := netlink.AddrList(nil, netlink.FAMILY_ALL)
	if err != nil {
		t.Fatalf("Failed to get address list: %v", err)
	}
	var found bool
	for _, addr := range addrs {
		if addr.IPNet.String() == network.IPNet.String() {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("IP address %s not found in address list after connecting to network", network.IPNet.String())
	}

	// Check that the default route is set up correctly
	routes, err := netlink.RouteList(nil, netlink.FAMILY_ALL)
	if err != nil {
		t.Fatalf("Failed to get route list: %v", err)
	}
	var gwFound bool
	for _, route := range routes {
		if route.Dst == nil && route.Gw != nil && route.Gw.String() == network.Gateway.String() {
			gwFound = true
			break
		}
	}
	if !gwFound {
		t.Fatalf("Default route to gateway %s not found in route list after connecting to network", network.Gateway.String())
	}

	// Check that DNS is reachable
	conn, err := net.Dial("udp", fmt.Sprintf("%s:53", network.DNS[0].String()))
	if err != nil {
		t.Fatalf("Failed to connect to DNS %s: %v", network.DNS[0].String(), err)
	}
	conn.Close()

	err = DisconnectFromNetwork(containerID, network.Name)
	if err != nil {
		t.Fatalf("Failed to disconnect container %s from network %s: %v", containerID, networkName, err)
	}
}

func TestDisconnectFromNetwork(t *testing.T) {
	networkName := "test_network"
	err := createTestNetwork(networkName)
	if err != nil {
		t.Fatalf("Failed to create test network: %v", err)
	}

	iface, err := net.InterfaceByName(networkName)
	if err != nil {
		t.Fatalf("Failed to get interface %s: %v", networkName, err)
	}

	containerID := "test_container"
	ipNet := &net.IPNet{
		IP:   net.IPv4(192, 168, 0, 2),
		Mask: net.CIDRMask(24, 32),
	}
	network := &Network{
		Name:    networkName,
		IPNet:   ipNet,
		Gateway: net.ParseIP("192.168.0.1"),
		DNS:     []net.IP{net.ParseIP("8.8.8.8")},
	}

	err = ConnectToNetwork(containerID, network)
	if err != nil {
		t.Fatalf("Failed to connect container %s to network %s: %v", containerID, networkName, err)
	}

	err = DisconnectFromNetwork(containerID, network.Name)
	if err != nil {
		t.Fatalf("Failed to disconnect container %s from network %s: %v", containerID, networkName, err)
	}

	link, err := netlink.LinkByIndex(iface.Index)
	if err != nil {
		t.Fatalf("Failed to get link by index %d: %v", iface.Index, err)
	}

	// Check that the IP address was removed from the interface
	addrs, err := netlink.AddrList(link, netlink.FAMILY_ALL)
	if err != nil {
		t.Fatalf("Failed to get address list for link %s: %v", link.Attrs().Name, err)
	}
	for _, addr := range addrs {
		if addr.IPNet.String() == network.IPNet.String() {
			t.Fatalf("IP address %s was not removed from interface %s", addr.IPNet.String(), link.Attrs().Name)
		}
	}

	// Check that the default route was removed
	routes, err := netlink.RouteList(link, netlink.FAMILY_ALL)
	if err != nil {
		t.Fatalf("Failed to get route list for link %s: %v", link.Attrs().Name, err)
	}
	for _, route := range routes {
		if route.Gw != nil && route.Gw.String() == network.Gateway.String() {
			t.Fatalf("Default route to gateway %s was not removed from interface %s", route.Gw.String(), link.Attrs().Name)
		}
	}

	// Check that DNS is no longer reachable
	udpAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", network.DNS[0].String(), 53))
	if err != nil {
		t.Fatalf("Failed to resolve UDP address for DNS %s: %v", network.DNS[0].String(), err)
	}

	_, err = net.DialUDP("udp", nil, udpAddr)
	if err == nil {
		t.Fatalf("DNS %s is still reachable after disconnection", network.DNS[0].String())
	}
}

func TestConnectToNetwork_NonExistentNetwork(t *testing.T) {
	containerID := "test_container"
	ipNet := &net.IPNet{
		IP:   net.IPv4(192, 168, 0, 2),
		Mask: net.CIDRMask(24, 32),
	}
	network := &Network{
		Name:    "non_existent_network",
		IPNet:   ipNet,
		Gateway: net.ParseIP("192.168.0.1"),
		DNS:     []net.IP{net.ParseIP("8.8.8.8")},
	}

	err := ConnectToNetwork(containerID, network)
	if err == nil {
		t.Fatalf("Expected error when connecting container to a non-existent network, but got no error")
	}
}

func TestConnectToNetwork_InvalidIPAddress(t *testing.T) {
	networkName := "test_network"
	err := createTestNetwork(networkName)
	if err != nil {
		t.Fatalf("Failed to create test network: %v", err)
	}

	containerID := "test_container"
	ipNet := &net.IPNet{
		IP:   net.ParseIP("256.168.0.2"), // Invalid IP address
		Mask: net.CIDRMask(24, 32),
	}
	network := &Network{
		Name:    networkName,
		IPNet:   ipNet,
		Gateway: net.ParseIP("192.168.0.1"),
		DNS:     []net.IP{net.ParseIP("8.8.8.8")},
	}

	err = ConnectToNetwork(containerID, network)
	if err == nil {
		t.Fatalf("Expected error when connecting container with an invalid IP address, but got no error")
	}
}

func TestConnectToNetwork_DuplicateIPAddress(t *testing.T) {
	networkName := "test_network"
	err := createTestNetwork(networkName)
	if err != nil {
		t.Fatalf("Failed to create test network: %v", err)
	}

	containerID := "test_container"
	ipNet := &net.IPNet{
		IP:   net.IPv4(192, 168, 0, 2),
		Mask: net.CIDRMask(24, 32),
	}
	network := &Network{
		Name:    networkName,
		IPNet:   ipNet,
		Gateway: net.ParseIP("192.168.0.1"),
		DNS:     []net.IP{net.ParseIP("8.8.8.8")},
	}

	// First connection attempt
	err = ConnectToNetwork(containerID, network)
	if err != nil {
		t.Fatalf("Failed to connect container %s to network %s: %v", containerID, networkName, err)
	}

	// Second connection attempt with the same IP address
	containerID2 := "test_container_2"
	err = ConnectToNetwork(containerID2, network)
	if err == nil {
		t.Fatalf("Expected error when connecting two containers with the same IP address, but got no error")
	}

	err = DisconnectFromNetwork(containerID, network.Name)
	if err != nil {
		t.Fatalf("Failed to disconnect container %s from network %s: %v", containerID, networkName, err)
	}
}
