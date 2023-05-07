package network

import (
	"bufio"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"log"
	"math/big"
	"net"
	"net/netip"
	"os"
	"strings"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv6"
	"github.com/insomniacslk/dhcp/dhcpv6/server6"
	"github.com/mdlayher/arp"
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
	Addrs(*net.Interface) ([]net.Addr, error)
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

func (dnh *DefaultNetworkHandler) Addrs(iface *net.Interface) ([]net.Addr, error) {
	return iface.Addrs()
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
			return nil, fmt.Errorf("failed to create DHCP server: %w", err)
		}

		if err := server.Serve(); err != nil {
			return nil, fmt.Errorf("failed to start DHCP server: %w", err)
		}
	} else {
		ip, err := GetAvailableIP(config.IPNet, handler)
		if err != nil {
			return nil, fmt.Errorf("failed to assign IP address to container: %w", err)
		}
		config.IPNet.IP = ip
	}

	gateway := config.Gateway
	if gateway == nil {
		defaultGateway, err := GetDefaultGateway(config.IPNet, handler)
		if err != nil {
			return nil, fmt.Errorf("failed to get default gateway: %w", err)
		}
		gateway = defaultGateway
	}

	dns := config.DNS
	if dns == nil {
		defaultDNS, err := GetDefaultDNS()
		if err != nil {
			return nil, fmt.Errorf("failed to get default DNS: %w", err)
		}
		dns = []net.IP{defaultDNS}
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
			return nil, fmt.Errorf("failed to generate random IP address: %w", err)
		}
		ipInt := big.NewInt(0).Add(randInt, big.NewInt(0).SetBytes(ipRange.To16()))
		ip := net.IP(ipInt.Bytes())

		// Check if the IP address is available
		if !IsIPInUse(ip) {
			return ip, nil
		}
	}

	return nil, fmt.Errorf("no available IP address in subnet range")
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

// GetDefaultDNS returns the default DNS IP address.
func GetDefaultDNS() (net.IP, error) {
	// Open the resolv.conf file
	file, err := os.Open("/etc/resolv.conf")
	if err != nil {
		log.Printf("Error opening resolv.conf: %v", err)
		return nil, err
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
				return ip, nil
			}
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Error reading resolv.conf: %v", err)
		return nil, err
	}

	return nil, nil
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
		return fmt.Errorf("network not found: %w", err)
	}

	link, err := netlink.LinkByIndex(iface.Index)
	if err != nil {
		return fmt.Errorf("failed to get network link: %w", err)
	}

	ipAddr := &netlink.Addr{
		IPNet: network.IPNet,
	}
	if err := netlink.AddrAdd(link, ipAddr); err != nil {
		return fmt.Errorf("failed to assign IP address to container: %w", err)
	}

	if network.Gateway != nil {
		defaultRoute := &netlink.Route{
			Dst: nil,
			Gw:  network.Gateway,
		}
		if err := netlink.RouteAdd(defaultRoute); err != nil {
			return fmt.Errorf("failed to add default route: %w", err)
		}
	}

	if network.DNS != nil && len(network.DNS) > 0 {
		dns := network.DNS[0].String()
		if err := configureDNS(containerID, dns); err != nil {
			return fmt.Errorf("failed to configure DNS: %w", err)
		}
	}

	log.Printf("Container %s connected to network %s", containerID, network.Name)

	return nil
}

func configureDNS(containerID, dns string) error {
	udpAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", dns, 53))
	if err != nil {
		return fmt.Errorf("failed to resolve DNS address: %w", err)
	}

	udpConn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		return fmt.Errorf("failed to create UDP connection to DNS server: %w", err)
	}
	defer udpConn.Close()

	// For example, querying "example.com" with a type A (IPv4) record
	query, err := createDNSQuery("example.com", 1)
	if err != nil {
		return fmt.Errorf("failed to create DNS query: %w", err)
	}
	if _, err := udpConn.Write(query); err != nil {
		return fmt.Errorf("failed to send DNS query: %w", err)
	}

	return nil
}

func createDNSQuery(domain string, qtype uint16) ([]byte, error) {
	var idBytes [2]byte
	if _, err := rand.Read(idBytes[:]); err != nil {
		return nil, fmt.Errorf("failed to generate random ID: %w", err)
	}

	id := binary.BigEndian.Uint16(idBytes[:])

	header := make([]byte, 12)
	binary.BigEndian.PutUint16(header[0:], id)
	header[2] = 1 << 0                        // Recursion desired
	binary.BigEndian.PutUint16(header[4:], 1) // One question

	question := make([]byte, 0, 32)
	labels := strings.Split(domain, ".")
	for _, label := range labels {
		question = append(question, byte(len(label)))
		question = append(question, []byte(label)...)
	}
	question = append(question, 0) // Zero-length label (root)

	binary.BigEndian.PutUint16(question, uint16(len(question)-2))
	binary.BigEndian.PutUint16(question, qtype)
	binary.BigEndian.PutUint16(question, 1) // Class IN

	return append(header, question...), nil
}

// DisconnectFromNetwork disconnects a container from a network.
func DisconnectFromNetwork(containerID, networkName string) error {
	if networkName == "" {
		return fmt.Errorf("invalid network name")
	}

	iface, err := net.InterfaceByName(networkName)
	if err != nil {
		return fmt.Errorf("network not found: %w", err)
	}

	link, err := netlink.LinkByIndex(iface.Index)
	if err != nil {
		return fmt.Errorf("failed to get network link: %w", err)
	}

	if err := netlink.LinkSetDown(link); err != nil {
		return fmt.Errorf("failed to bring down network link: %w", err)
	}

	log.Printf("Container %s disconnected from network %s", containerID, networkName)

	return nil
}
