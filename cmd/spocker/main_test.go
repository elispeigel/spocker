package main

import (
	"net"
	"testing"
)

func TestDefaultNetworkCIDR(t *testing.T) {
	// Test that getDefaultNetworkCIDR returns a valid CIDR
	cidr := getDefaultNetworkCIDR()

	if cidr == "" {
		t.Error("Default CIDR should not be empty")
	}

	// Verify it's a valid CIDR
	_, _, err := net.ParseCIDR(cidr)
	if err != nil {
		t.Errorf("Default CIDR is invalid: %v", err)
	}

	// Verify it's in the expected range (10.100.x.x/24)
	ip, ipNet, _ := net.ParseCIDR(cidr)
	if !ipNet.Contains(ip) {
		t.Error("IP should be contained in the network")
	}

	// Check that the IP is in the 10.100.x.x range
	// Convert to IPv4 to get correct byte representation
	ipv4 := ip.To4()
	if ipv4 == nil {
		t.Fatal("IP should be IPv4")
	}
	if ipv4[0] != 10 || ipv4[1] != 100 {
		t.Errorf("IP should be in 10.100.x.x range, got %v", ipv4)
	}
}
