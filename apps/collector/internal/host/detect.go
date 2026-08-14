package host

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type InterfaceAddress struct {
	Interface string
	Address   netip.Addr
	Prefix    netip.Prefix
}

type RouteInfo struct {
	DefaultInterface string
	DefaultGateway   netip.Addr
	LocalAddresses   []InterfaceAddress
}

func Detect(procRoot string) (RouteInfo, error) {
	route, err := defaultRouteFromProc(procRoot)
	if err != nil {
		return RouteInfo{}, err
	}
	addresses, err := interfaceAddresses()
	if err != nil {
		return RouteInfo{}, err
	}
	route.LocalAddresses = addresses
	return route, nil
}

func HostIPs(info RouteInfo) []netip.Addr {
	values := make([]netip.Addr, 0, len(info.LocalAddresses))
	for _, addr := range info.LocalAddresses {
		values = append(values, addr.Address)
	}
	return values
}

func LocalCIDRs(info RouteInfo) []netip.Prefix {
	values := make([]netip.Prefix, 0, len(info.LocalAddresses))
	for _, addr := range info.LocalAddresses {
		values = append(values, addr.Prefix)
	}
	return values
}

func defaultRouteFromProc(procRoot string) (RouteInfo, error) {
	file, err := os.Open(filepath.Join(procRoot, "net", "route"))
	if err != nil {
		return RouteInfo{}, fmt.Errorf("open route table: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 3 || fields[0] == "Iface" {
			continue
		}
		if fields[1] != "00000000" {
			continue
		}
		gateway, err := parseLittleEndianIPv4Hex(fields[2])
		if err != nil {
			return RouteInfo{}, err
		}
		return RouteInfo{DefaultInterface: fields[0], DefaultGateway: gateway}, nil
	}
	if err := scanner.Err(); err != nil {
		return RouteInfo{}, err
	}
	return RouteInfo{}, fmt.Errorf("default route not found")
}

func parseLittleEndianIPv4Hex(value string) (netip.Addr, error) {
	raw, err := strconv.ParseUint(value, 16, 32)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("parse gateway %q: %w", value, err)
	}
	return netip.AddrFrom4([4]byte{byte(raw), byte(raw >> 8), byte(raw >> 16), byte(raw >> 24)}), nil
}

func interfaceAddresses() ([]InterfaceAddress, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	var result []InterfaceAddress
	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ip, prefix, ok := parseNetAddr(addr)
			if !ok {
				continue
			}
			result = append(result, InterfaceAddress{
				Interface: iface.Name,
				Address:   ip,
				Prefix:    prefix,
			})
		}
	}
	return result, nil
}

func parseNetAddr(addr net.Addr) (netip.Addr, netip.Prefix, bool) {
	ipnet, ok := addr.(*net.IPNet)
	if !ok {
		return netip.Addr{}, netip.Prefix{}, false
	}
	ones, _ := ipnet.Mask.Size()
	parsed, ok := netip.AddrFromSlice(ipnet.IP)
	if !ok {
		return netip.Addr{}, netip.Prefix{}, false
	}
	ip := parsed.Unmap()
	return ip, netip.PrefixFrom(ip, ones).Masked(), true
}

func (r RouteInfo) MarshalJSON() ([]byte, error) {
	type address struct {
		Interface string `json:"interface"`
		Address   string `json:"address"`
		Prefix    string `json:"prefix"`
	}
	values := make([]address, 0, len(r.LocalAddresses))
	for _, item := range r.LocalAddresses {
		values = append(values, address{
			Interface: item.Interface,
			Address:   item.Address.String(),
			Prefix:    item.Prefix.String(),
		})
	}
	return json.Marshal(struct {
		DefaultInterface string    `json:"default_interface"`
		DefaultGateway   string    `json:"default_gateway"`
		LocalAddresses   []address `json:"local_addresses"`
	}{
		DefaultInterface: r.DefaultInterface,
		DefaultGateway:   r.DefaultGateway.String(),
		LocalAddresses:   values,
	})
}
