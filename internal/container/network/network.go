package network

import (
	"fmt"
	"net"
	"time"
	"math/big"
	"crypto/rand"
	"syscall"

	"github.com/vishvananda/netlink"
	"github.com/containernetworking/cni/libcni"
	"github.com/insomniacslk/dhcp/dhcpv6/server6"
	"go.uber.org/zap"
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

func dhcpHandler(conn net.PacketConn, peer net.Addr, m dhcpv6.DHCPv6) {
	// Add your DHCP handling logic here
}

// CreateNetwork creates a new container network.
func CreateNetwork(config *Config, handler NetworkHandler) (*Network, error) {
	if config == nil || config.IPNet == nil {
		return nil, fmt.Errorf("invalid network configuration")
	}

	if _, err := handler.InterfaceByName(config.Name); err == nil {
		return nil, fmt.Errorf("network %s already exists: %w", config.Name, err)
	}

	if config.DHCP {
		laddr := &net.UDPAddr{
			IP:   net.ParseIP("::1"),
			Port: dhcpv6.DefaultServerPort,
		}
		server, err := server6.NewServer("", laddr, dhcpHandler)
		if err != nil {
			zap.L().Error("Failed to create DHCP server", zap.Error(err))
			return nil, fmt.Errorf("failed to create DHCP server: %w", err)
		}

		if err := server.Serve(); err != nil {
			zap.L().Error("Failed to start DHCP server", zap.Error(err))
			return nil, fmt.Errorf("failed to start DHCP server: %w", err)
		}
	} else {
		ip, err := GetAvailableIP(config.IPNet, handler)
		if err != nil {
			zap.L().Error("Failed to assign IP address to container", zap.Error(err))
			return nil, fmt.Errorf("failed to assign IP address to container: %w", err)
		}
		config.IPNet.IP = ip
	}

	gateway := config.Gateway
	if gateway == nil {
		defaultGateway, err := GetDefaultGateway(config.IPNet, handler)
		if err != nil {
			zap.L().Error("Failed to get default gateway", zap.Error(err))
			return nil, fmt.Errorf("failed to get default gateway: %w", err)
		}
		gateway = defaultGateway
	}

	dns := config.DNS
	if dns == nil {
		defaultDNS, err := GetDefaultDNS()
		if err != nil {
			zap.L().Error("Failed to get default DNS", zap.Error(err))
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

	if config.CNIPlugins != nil {
		cniNetwork, err := SetupCNINetwork(config)
		if err != nil {
			return nil, fmt.Errorf("failed to set up CNI network: %w", err)
		}
		network.CNINetwork = cniNetwork
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
			zap.L().Error("Failed to generate random IP address", zap.Error(err))
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
func DeleteNetwork(network *Network) error {
	if network.CNINetwork != nil {
		if err := CleanupCNINetwork(network.CNINetwork); err != nil {
			zap.L().Error("Failed to clean up CNI network", zap.Error(err))
		}
	}

	iface, err := net.InterfaceByName(network.Name)
	if err != nil {
		zap.L().Error("Failed to get network interface", zap.String("network.Name", network.Name), zap.Error(err))
		return fmt.Errorf("failed to get network interface %s: %w", network.Name, err)
	}

	link, err := netlink.LinkByIndex(iface.Index)
	if err != nil {
		zap.L().Error("Failed to get network link", zap.String("network.Name", network.Name), zap.Error(err))
		return fmt.Errorf("failed to get network link for %s: %w", network.Name, err)
	}

	err = netlink.LinkDel(link)
	if err != nil {
		zap.L().Error("Failed to delete network", zap.String("network.Name", network.Name), zap.Error(err))
		return fmt.Errorf("failed to delete network %s: %w", network.Name, err)
	}

	return nil
}

// ConnectToNetwork connects the container to an existing network.
func ConnectToNetwork(containerID string, network *Network) error {
	if network == nil {
		return fmt.Errorf("invalid network configuration")
	}

	iface, err := net.InterfaceByName(network.Name)
	if err != nil {
		zap.L().Error("Network not found", zap.String("network.Name", network.Name), zap.Error(err))
		return fmt.Errorf("network %s not found: %w", network.Name, err)
	}

	link, err := netlink.LinkByIndex(iface.Index)
	if err != nil {
		zap.L().Error("Failed to get network link", zap.String("network.Name", network.Name), zap.Error(err))
		return fmt.Errorf("failed to get network link for %s: %w", network.Name, err)
	}

	ipAddr := &netlink.Addr{
		IPNet: network.IPNet,
	}
	if err := netlink.AddrAdd(link, ipAddr); err != nil {
		zap.L().Error("Failed to assign IP address to container", zap.String("containerID", containerID), zap.Error(err))
		return fmt.Errorf("failed to assign IP address to container %s: %w", containerID, err)
	}

	if network.Gateway != nil {
		defaultRoute := &netlink.Route{
			Dst: nil,
			Gw:  network.Gateway,
		}
		if err := netlink.RouteAdd(defaultRoute); err != nil {
			zap.L().Error("Failed to add default route", zap.String("containerID", containerID), zap.Error(err))
			return fmt.Errorf("failed to add default route for container %s: %w", containerID, err)
		}
	}

	if network.DNS != nil && len(network.DNS) > 0 {
		dns := network.DNS[0].String()
		if err := configureDNS(containerID, dns); err != nil {
			zap.L().Error("Failed to configure DNS", zap.String("containerID", containerID), zap.Error(err))
			return fmt.Errorf("failed to configure DNS for container %s: %w", containerID, err)
		}
	}

	zap.L().Info("Container connected to network", zap.String("containerID", containerID), zap.String("network.Name", network.Name))

	return nil
}

// DisconnectFromNetwork disconnects a container from a network.
func DisconnectFromNetwork(containerID, networkName string) error {
	if network.Name == "" {
		return fmt.Errorf("invalid network name")
	}

	iface, err := net.InterfaceByName(network.Name)
	if err != nil {
		zap.L().Error("Network not found", zap.String("network.Name", network.Name), zap.Error(err))
		return fmt.Errorf("network %s not found: %w", network.Name, err)
	}

	link, err := netlink.LinkByIndex(iface.Index)
	if err != nil {
		zap.L().Error("Failed to get network link", zap.String("network.Name", network.Name), zap.Error(err))
		return fmt.Errorf("failed to get network link for %s: %w", network.Name, err)
	}

	if err := netlink.LinkSetDown(link); err != nil {
		zap.L().Error("Failed to bring down network link", zap.String("network.Name", network.Name), zap.Error(err))
		return fmt.Errorf("failed to bring down network link for %s: %w", network.Name, err)
	}

	zap.L().Info("Container disconnected from network", zap.String("containerID", containerID), zap.String("network.Name", network.Name))

	return nil
}