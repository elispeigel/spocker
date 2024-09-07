package network

import (
	"net"
	"time"

	"github.com/containernetworking/cni/libcni"
	"github.com/vishvananda/netlink"
)

type Config struct {
	Name           string
	IPNet          *net.IPNet
	Gateway        net.IP
	DNS            []net.IP
	DHCP           bool
	DHCPArgs       []string
	CNIPlugins     []*libcni.NetworkConfig
	CNIPluginPaths []string
	CNIConfigJSON  string
	RuntimeConfig  *libcni.RuntimeConf
}

type Network struct {
	Name       string
	IPNet      *net.IPNet
	Gateway    net.IP
	DNS        []net.IP
	DHCP       bool
	CNINetwork *libcni.NetworkConfig
}

type NetworkHandler interface {
	InterfaceByName(name string) (*net.Interface, error)
	RouteList(link netlink.Link, family int) ([]netlink.Route, error)
	DialTimeout(network, address string, timeout time.Duration) (net.Conn, error)
	ResolveUDPAddr(network, address string) (*net.UDPAddr, error)
	Addrs(*net.Interface) ([]net.Addr, error)
}

type DefaultNetworkHandler struct{}

type Answer struct {
	Name string
	Type uint16
	TTL  uint32
	Data string
}

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