package network

import (
	"fmt"
	"log"
	"net"
	"net/netip"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv6"
	"github.com/mdlayher/arp"
	"github.com/vishvananda/netlink"
)

func dhcpHandler(conn net.PacketConn, peer net.Addr, m dhcpv6.DHCPv6) {
	// this function will just print the received DHCPv6 message, without replying
	log.Print(m.Summary())
}

// IsIPInUse checks if the given IP address is already in use.
func IsIPInUse(ip net.IP) bool {
	iface, err := net.InterfaceByIndex(1) // You may need to change this to the appropriate network interface index
	if err != nil {
		log.Printf("Error getting network interface: %v", err)
		return true
	}

	// Get the source IP and hardware address for the network interface
	sourceIP, sourceHardwareAddr := getSourceIPAndHardwareAddr(iface)

	// Create an ARP client
	client, err := arp.Dial(iface)
	if err != nil {
		log.Printf("Error creating ARP client: %v", err)
		return true
	}
	defer client.Close()

	// Create an ARP request
	arpRequest, err := arp.NewPacket(
		arp.OperationRequest,
		sourceHardwareAddr,
		netIPToNetIPAddr(sourceIP), // Use helper function to convert net.IP to netip.Addr
		net.HardwareAddr{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
		netIPToNetIPAddr(ip), // Use helper function to convert net.IP to netip.Addr
	)
	if err != nil {
		log.Printf("Error creating ARP request: %v", err)
		return true
	}

	// Send the ARP request
	err = client.WriteTo(arpRequest, net.HardwareAddr{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF})
	if err != nil {
		log.Printf("Error sending ARP request: %v", err)
		return true
	}

	// Set a one-second timeout
	timeout := time.After(time.Second)

	for {
		select {
		case <-timeout:
			// Timeout reached, no ARP reply received
			return false
		default:
			// Read ARP replies
			arpReply, _, err := client.Read()
			if err != nil {
				continue
			}

			// Check if the ARP reply is for the target IP address
			if arpReply.Operation == arp.OperationReply && arpReply.TargetIP == (netIPToNetIPAddr(ip)) { // Use helper function to convert net.IP to netip.Addr
				return true
			}
		}
	}
}

func getSourceIPAndHardwareAddr(iface *net.Interface) (net.IP, net.HardwareAddr) {
	addrs, err := iface.Addrs()
	if err != nil {
		log.Printf("Error getting addresses for interface: %v", err)
		return nil, nil
	}

	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok {
			if ip4 := ipNet.IP.To4(); ip4 != nil {
				return ip4, iface.HardwareAddr
			}
		}
	}

	return nil, nil
}

func netIPToNetIPAddr(ip net.IP) netip.Addr {
	ipBytes := ip.To4()
	if ipBytes != nil {
		var ipv4 [4]byte
		copy(ipv4[:], ipBytes)
		return netip.AddrFrom4(ipv4)
	}
	ipBytes = ip.To16()
	if ipBytes != nil {
		var ipv6 [16]byte
		copy(ipv6[:], ipBytes)
		return netip.AddrFrom16(ipv6)
	}
	return netip.Addr{}
}

// GetDefaultGateway returns the default gateway IP address for the given IPNet subnet.
func GetDefaultGateway(ipNet *net.IPNet, handler NetworkHandler) (net.IP, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("failed to get interfaces: %w", err)
	}

	var defaultIface *net.Interface
	for _, iface := range interfaces {
		if defaultIface == nil || iface.Index < defaultIface.Index {
			defaultIface = &iface
		}
	}

	addrs, err := handler.Addrs(defaultIface)
	if err != nil {
		return nil, fmt.Errorf("failed to get interface address: %w", err)
	}

	for _, addr := range addrs {
		switch addr := addr.(type) {
		case *net.IPNet:
			if addr.Contains(ipNet.IP) {
				routes, err := handler.RouteList(nil, netlink.FAMILY_ALL)
				if err != nil {
					return nil, fmt.Errorf("failed to get routes: %w", err)
				}

				for _, route := range routes {
					if route.Dst == nil {
						continue
					}

					_, dstNet, err := net.ParseCIDR(route.Dst.String())
					if err != nil {
						return nil, fmt.Errorf("failed to get destination net: %w", err)
					}

					if dstNet.Contains(ipNet.IP) {
						return route.Gw, nil
					}
				}
			}
		}
	}

	return nil, nil
}