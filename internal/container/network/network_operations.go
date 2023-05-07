package network

import (
	"crypto/rand"
	"fmt"
	"log"
	"math/big"
	"net"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv6"
	"github.com/insomniacslk/dhcp/dhcpv6/server6"
	"github.com/vishvananda/netlink"
)

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

func (dnh DefaultNetworkHandler) Addrs(iface *net.Interface) ([]net.Addr, error) {
	return iface.Addrs()
}

// CreateNetwork creates a new container network.
func CreateNetwork(config *Config, handler NetworkHandler) (*Network, error) {
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
