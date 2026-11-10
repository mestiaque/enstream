// Package network finds the LAN addresses other devices can use to reach ENStream.
package network

import (
	"fmt"
	"net"
	"net/netip"
	"sort"
	"strings"
)

// Address is one usable IPv4 address and the URL built from it.
type Address struct {
	Interface string `json:"interface"`
	IP        string `json:"ip"`
	URL       string `json:"url"`
	Private   bool   `json:"private"`
}

// Interface is the subset of net.Interface discovery needs (fakeable in tests).
type Interface struct {
	Name  string
	Flags net.Flags
	Addrs []netip.Prefix
}

// Virtual interfaces that other devices cannot reach (containers, bridges, VPN tunnels
// are kept: a VPN like Tailscale is a legitimate way in).
var virtualPrefixes = []string{"docker", "br-", "veth", "virbr", "vmnet", "vboxnet", "lxc", "lxd", "cni", "flannel", "podman"}

func isVirtual(name string) bool {
	n := strings.ToLower(name)
	for _, p := range virtualPrefixes {
		if strings.HasPrefix(n, p) {
			return true
		}
	}
	return false
}

// Discover lists usable IPv4 addresses on active, non-loopback, non-virtual interfaces.
// Private addresses come first. Works for Ethernet and Wi-Fi alike; no subnet is assumed.
func Discover(port int) ([]Address, error) {
	ifs, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	var list []Interface
	for _, i := range ifs {
		addrs, err := i.Addrs()
		if err != nil {
			continue
		}
		var ps []netip.Prefix
		for _, a := range addrs {
			if p, err := netip.ParsePrefix(a.String()); err == nil {
				ps = append(ps, p)
			}
		}
		list = append(list, Interface{Name: i.Name, Flags: i.Flags, Addrs: ps})
	}
	return FromInterfaces(list, port), nil
}

// FromInterfaces applies the selection rules to a list of interfaces.
func FromInterfaces(ifs []Interface, port int) []Address {
	var out []Address
	for _, i := range ifs {
		if i.Flags&net.FlagUp == 0 || i.Flags&net.FlagLoopback != 0 || isVirtual(i.Name) {
			continue
		}
		for _, p := range i.Addrs {
			ip := p.Addr()
			if !ip.Is4() || ip.IsLoopback() || ip.IsUnspecified() || ip.IsMulticast() || ip.IsLinkLocalUnicast() {
				continue
			}
			out = append(out, Address{Interface: i.Name, IP: ip.String(), URL: fmt.Sprintf("http://%s:%d", ip, port), Private: ip.IsPrivate()})
		}
	}
	sort.SliceStable(out, func(a, b int) bool { return out[a].Private && !out[b].Private })
	return out
}

// CheckPort reports whether host:port can be bound right now.
func CheckPort(host string, port int) error {
	l, err := net.Listen("tcp", net.JoinHostPort(host, fmt.Sprint(port)))
	if err != nil {
		return fmt.Errorf("port %d is not available: %w", port, err)
	}
	return l.Close()
}
