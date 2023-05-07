package network

import (
	"net"
	"time"

	"github.com/vishvananda/netlink"
)

// Represents the configuration for a container network, including properties like its name, IP network, gateway, DNS, and DHCP-related details.
type NetworkConfig struct {
	Name     string
	IPNet    *net.IPNet
	Gateway  net.IP
	DNS      []net.IP
	DHCP     bool
	DHCPArgs []string
}

// An abstraction over a container network, containing properties such as its name, IP network, gateway, DNS, and whether it uses DHCP.
type Network struct {
	Name    string
	IPNet   *net.IPNet
	Gateway net.IP
	DNS     []net.IP
	DHCP    bool
}

// Defines the methods required for a network handler to interact with and manage container networks.
type NetworkHandler interface {
	InterfaceByName(name string) (*net.Interface, error)
	RouteList(link netlink.Link, family int) ([]netlink.Route, error)
	DialTimeout(network, address string, timeout time.Duration) (net.Conn, error)
	ResolveUDPAddr(network, address string) (*net.UDPAddr, error)
	Addrs(*net.Interface) ([]net.Addr, error)
}

// An empty placeholder for the default implementation of the NetworkHandler interface
type DefaultNetworkHandler struct{}

// Represents a DNS answer, containing the name, type, time-to-live (TTL), and data of the DNS response.
type Answer struct {
	Name string
	Type uint16
	TTL  uint32
	Data string
}

// Represents the header of a DNS message, containing various fields such as id, flags, and count fields for question, answer, authority, and additional records.
type dnsHeader struct {
	id      uint16
	qr      byte
	opcode  byte
	aa      byte
	tc      byte
	rd      byte
	ra      byte
	z       byte
	rcode   byte
	qdcount uint16
	ancount uint16
	nscount uint16
	arcount uint16
}
