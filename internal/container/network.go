// Package container provides functions for creating a container.
package container

import (
	"bufio"
	"fmt"
	"log"
	"math/big"
	"net"
	"os"
	"strings"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv6"
	"github.com/insomniacslk/dhcp/dhcpv6/server6"
	"github.com/vishvananda/netlink"
)

// NetworkConfig represents the configuration for a container network.
type NetworkConfig struct {
	Name     string
	IPNet    *net.IPNet
	Gateway  net.IP
	DNS      []net.IP
	DHCP     bool
	DHCPArgs []string
}

// Network is an abstraction over a container network.
type Network struct {
	Name    string
	IPNet   *net.IPNet
	Gateway net.IP
	DNS     []net.IP
	DHCP    bool
}

// CreateNetwork creates a new container network.
func CreateNetwork(config *NetworkConfig) (*Network, error) {
	if config == nil || config.IPNet == nil {
		return nil, fmt.Errorf("invalid network configuration")
	}

	if _, err := net.InterfaceByName(config.Name); err == nil {
		return nil, fmt.Errorf("network already exists")
	}

	if config.DHCP {
		laddr := &net.UDPAddr{
			IP:   net.ParseIP("::1"),
			Port: dhcpv6.DefaultServerPort,
		}
		server, err := server6.NewServer("", laddr, handler)
		if err != nil {
			log.Fatal(err)
		}

		if err := server.Serve(); err != nil {
			return nil, fmt.Errorf("failed to start DHCP server: %v", err)
		}
	} else {
		ip, err := GetAvailableIP(config.IPNet)
		if err != nil {
			return nil, fmt.Errorf("failed to assign IP address to container: %v", err)
		}
		config.IPNet.IP = ip
	}

	gateway := config.Gateway
	if gateway == nil {
		gateway = GetDefaultGateway(config.IPNet)
	}

	dns := config.DNS
	if dns == nil {
		dns = []net.IP{GetDefaultDNS()}
	}

	network := &Network{
		Name:    config.Name,
		IPNet:   config.IPNet,
		Gateway: gateway,
		DNS:     dns,
		DHCP:    config.DHCP,
	}

	return network, nil
}

func handler(conn net.PacketConn, peer net.Addr, m dhcpv6.DHCPv6) {
	// this function will just print the received DHCPv6 message, without replying
	log.Print(m.Summary())
}

// GetAvailableIP finds and returns an available IP address in the given IPNet subnet range.
func GetAvailableIP(ipNet *net.IPNet) (net.IP, error) {
	ipRange := ipNet.IP.Mask(ipNet.Mask)

	start := big.NewInt(0).SetBytes(ipRange)
	mask := big.NewInt(0).SetBytes(ipNet.Mask)
	end := big.NewInt(0).Add(start, big.NewInt(0).Not(mask))

	for ip := start; ip.Cmp(end) <= 0; ip.Add(ip, big.NewInt(1)) {
		ipAddr := net.IP(ip.Bytes())
		if !IsIPInUse(ipAddr) {
			return ipAddr, nil
		}
	}

	return nil, fmt.Errorf("no available IP address in subnet range")
}

// IsIPInUse checks if the given IP address is already in use.
func IsIPInUse(ip net.IP) bool {
	conn, err := net.DialTimeout("ip4:icmp", ip.String(), time.Second)
	if err != nil {
		return true
	}
	err = conn.Close()
	if err != nil {
		log.Printf("Failed to close connection for IP %v: %v", ip, err)
	}
	return false
}

// GetDefaultGateway returns the default gateway IP address for the given IPNet subnet.
func GetDefaultGateway(ipNet *net.IPNet) net.IP {
	iface, err := net.InterfaceByIndex(1) // assuming the first interface is the default one
	if err != nil {
		log.Fatal(err)
	}

	addrs, err := iface.Addrs()
	if err != nil {
		log.Fatal(err)
	}

	for _, addr := range addrs {
		switch addr := addr.(type) {
		case *net.IPNet:
			if addr.Contains(ipNet.IP) {
				routes, err := netlink.RouteList(nil, netlink.FAMILY_ALL)
				if err != nil {
					log.Fatal(err)
				}

				for _, route := range routes {
					if route.Dst == nil {
						continue
					}

					_, dstNet, err := net.ParseCIDR(route.Dst.String())
					if err != nil {
						log.Fatal(err)
					}

					if dstNet.Contains(ipNet.IP) {
						return route.Gw
					}
				}
			}
		}
	}

	return nil
}

// GetDefaultDNS returns the default DNS IP address.
func GetDefaultDNS() net.IP {
	// Open the resolv.conf file
	file, err := os.Open("/etc/resolv.conf")
	if err != nil {
		log.Printf("Error opening resolv.conf: %v", err)
		return nil
	}
	defer file.Close()

	// Read the file line by line
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)

		// Look for the nameserver directive
		if len(fields) >= 2 && fields[0] == "nameserver" {
			ip := net.ParseIP(fields[1])
			if ip != nil {
				return ip
			}
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Error reading resolv.conf: %v", err)
	}

	return nil
}

// DeleteNetwork deletes an existing container network.
func DeleteNetwork(networkName string) error {
	iface, err := net.InterfaceByName(networkName)
	if err != nil {
		return err
	}

	link, err := netlink.LinkByIndex(iface.Index)
	if err != nil {
		return err
	}

	err = netlink.LinkDel(link)
	if err != nil {
		return err
	}

	log.Printf("Deleted network %s\n", networkName)

	return nil
}

// ConnectToNetwork connects the container to an existing network.
func ConnectToNetwork(containerID string, network *Network) error {
	if network == nil {
		return fmt.Errorf("invalid network configuration")
	}

	iface, err := net.InterfaceByName(network.Name)
	if err != nil {
		return fmt.Errorf("network not found: %v", err)
	}

	link, err := netlink.LinkByIndex(iface.Index)
	if err != nil {
		return fmt.Errorf("failed to get network link: %v", err)
	}

	ipAddr := &netlink.Addr{
		IPNet: network.IPNet,
	}
	if err := netlink.AddrAdd(link, ipAddr); err != nil {
		return fmt.Errorf("failed to assign IP address to container: %v", err)
	}

	if network.Gateway != nil {
		defaultRoute := &netlink.Route{
			Dst: nil,
			Gw:  network.Gateway,
		}
		if err := netlink.RouteAdd(defaultRoute); err != nil {
			return fmt.Errorf("failed to add default route: %v", err)
		}
	}

	if network.DNS != nil {
		udpAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", network.DNS[0].String(), 53))
		if err != nil {
			return fmt.Errorf("failed to resolve DNS address: %v", err)
		}

		udpConn, err := net.DialUDP("udp", nil, udpAddr)
		if err != nil {
			return fmt.Errorf("failed to create UDP connection to DNS server: %v", err)
		}
		defer udpConn.Close()

		message := []byte("Hello DNS server!")
		if _, err := udpConn.Write(message); err != nil {
			return fmt.Errorf("failed to send DNS message: %v", err)
		}
	}

	log.Printf("Container %s connected to network %s", containerID, network.Name)

	return nil
}

// DisconnectFromNetwork disconnects a container from a network.
func DisconnectFromNetwork(containerID, networkName string) error {
	iface, err := net.InterfaceByName(networkName)
	if err != nil {
		return fmt.Errorf("network not found: %v", err)
	}

	link, err := netlink.LinkByIndex(iface.Index)
	if err != nil {
		return fmt.Errorf("failed to get network link: %v", err)
	}

	if err := netlink.LinkSetDown(link); err != nil {
		return fmt.Errorf("failed to bring down network link: %v", err)
	}

	log.Printf("Container %s disconnected from network %s", containerID, networkName)

	return nil
}
