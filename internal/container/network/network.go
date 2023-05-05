package network

import (
	"bufio"
	"crypto/rand"
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

type NetworkHandler interface {
	InterfaceByName(name string) (*net.Interface, error)
	RouteList(link netlink.Link, family int) ([]netlink.Route, error)
	DialTimeout(network, address string, timeout time.Duration) (net.Conn, error)
	ResolveUDPAddr(network, address string) (*net.UDPAddr, error)
}

type DefaultNetworkHandler struct{}

func (dnh DefaultNetworkHandler) InterfaceByName(name string) (*net.Interface, error) {
	return net.InterfaceByName(name)
}

func (dnh DefaultNetworkHandler) RouteList(link netlink.Link, family int) ([]netlink.Route, error) {
	return netlink.RouteList(link, family)
}

func (dnh DefaultNetworkHandler) DialTimeout(network, address string, timeout time.Duration) (net.Conn, error) {
	return net.DialTimeout(network, address, timeout)
}

func (dnh DefaultNetworkHandler) ResolveUDPAddr(network, address string) (*net.UDPAddr, error) {
	return net.ResolveUDPAddr(network, address)
}

// CreateNetwork creates a new container network.
func CreateNetwork(config *NetworkConfig, handler NetworkHandler) (*Network, error) {
	if config == nil || config.IPNet == nil {
		return nil, fmt.Errorf("invalid network configuration")
	}

	if _, err := handler.InterfaceByName(config.Name); err == nil {
		return nil, fmt.Errorf("network already exists: %w", err)
	}

	if config.DHCP {
		laddr := &net.UDPAddr{
			IP:   net.ParseIP("::1"),
			Port: dhcpv6.DefaultServerPort,
		}
		server, err := server6.NewServer("", laddr, dhcpHandler)
		if err != nil {
			log.Fatal(err)
		}

		if err := server.Serve(); err != nil {
			return nil, fmt.Errorf("failed to start DHCP server: %v", err)
		}
	} else {
		ip, err := GetAvailableIP(config.IPNet, handler)
		if err != nil {
			return nil, fmt.Errorf("failed to assign IP address to container: %v", err)
		}
		config.IPNet.IP = ip
	}

	gateway := config.Gateway
	if gateway == nil {
		gateway = GetDefaultGateway(config.IPNet, handler)
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

func dhcpHandler(conn net.PacketConn, peer net.Addr, m dhcpv6.DHCPv6) {
	// this function will just print the received DHCPv6 message, without replying
	log.Print(m.Summary())
}

// GetAvailableIP finds and returns an available IP address in the given IPNet subnet range.
func GetAvailableIP(ipNet *net.IPNet, handler NetworkHandler) (net.IP, error) {
	ipRange := ipNet.IP.Mask(ipNet.Mask)

	ones, bits := ipNet.Mask.Size()
	ipSpace := big.NewInt(1 << uint(bits-ones))

	// Try up to 10 random addresses
	for i := 0; i < 10; i++ {
		// Generate a random IP address within the subnet range
		randInt, err := rand.Int(rand.Reader, ipSpace)
		if err != nil {
			return nil, fmt.Errorf("failed to generate random IP address: %v", err)
		}
		ipInt := big.NewInt(0).Add(randInt, big.NewInt(0).SetBytes(ipRange.To4()))
		ip := net.IP(ipInt.Bytes())

		// Check if the IP address is available
		if !IsIPInUse(ip, handler) {
			return ip, nil
		}
	}

	return nil, fmt.Errorf("no available IP address in subnet range")
}

// IsIPInUse checks if the given IP address is already in use.
func IsIPInUse(ip net.IP, handler NetworkHandler) bool {
	conn, err := handler.DialTimeout("ip4:icmp", ip.String(), time.Second)
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
func GetDefaultGateway(ipNet *net.IPNet, handler NetworkHandler) net.IP {
	iface, err := handler.InterfaceByName("eth0") // assuming the first interface is the default one
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
				routes, err := handler.RouteList(nil, netlink.FAMILY_ALL)
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
