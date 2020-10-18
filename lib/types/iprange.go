package types

import (
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"net"
	"strconv"
	"strings"
)

// selectMode tells how IP is selected in a IPBlock splited from --ip option

type selectMode uint

const (
	roundRobin selectMode = iota
	random
)

// IPBlock represents a continuous segment of IP addresses
// ip     - the first IP in this address segment. CIDR can skip lower IPs
// weight - how many IPs will be chosen from this segment. default is all
// mode   - method to choose IP, sequential(0) or random(1). default is 0
// hostStart - uint64 conversion of fistIP[8:16]
// netStart  - uint64 conversion of fistIP[0:8]
// hostN  - number of IPv4 addrs (IPv6 hosts) in this address segment
// netN   - number of IPv6 nets in this address segment, always 1 if IPv4
type IPBlock struct {
	ip        net.IP
	hostStart uint64
	netStart  uint64
	hostN     uint64
	netN      uint64
	weight    uint64
	split     uint64
	mode      selectMode
}

// IPPool represent a slice of IPBlocks
type IPPool []IPBlock

// GetIPBlock return a struct of a sequential IP range
// support single IP, range format by '-' or CIDR by '/'
// support up to 2^32 IPv4 addresses and 2^64 * 2^64 IPv6 addesses
// for IPv6, high 64 bits is independent to low 64 bits if input > 64 bits
// this is useful if you want to test some IPv6 nets without traveling 2^64 hosts
func GetIPBlock(s string) *IPBlock {
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '-':
			return ipBlockFromRange(s)
		case '/':
			return ipBlockFromCIDR(s)
		}
	}
	return ipBlockFromRange(s + "-" + s)
}

func hashIDToUint64(id uint64) uint64 {
	a, b := fnv.New64(), make([]byte, 8)
	binary.LittleEndian.PutUint64(b, id)
	if _, err := a.Write(b); err != nil {
		return 0
	}
	return a.Sum64()
}

func ipUint64(ip net.IP) (uint64, uint64) {
	ip128 := ip.To16()
	net64 := binary.BigEndian.Uint64(ip128[:8])
	host64 := binary.BigEndian.Uint64(ip128[8:])
	return net64, host64
}

func ipBlockFromRange(s string) *IPBlock {
	ss := strings.SplitN(s, "-", 2)
	ip0Str, ip1Str := strings.TrimSpace(ss[0]), strings.TrimSpace(ss[1])
	ip0, ip1 := net.ParseIP(ip0Str), net.ParseIP(ip1Str) // len(ParseIP())==16
	if ip0 == nil || ip1 == nil {
		fmt.Println("Wrong IP range format: ", s)
		return nil
	}
	n0, h0 := ipUint64(ip0)
	n1, h1 := ipUint64(ip1)
	if (n0 > n1) || (h0 > h1) {
		fmt.Println("Negative IP range: ", s)
		return nil
	}
	hostNum, netNum := h1-h0+1, n1-n0+1
	return &IPBlock{
		ip:        ip0,
		hostStart: h0,
		netStart:  n0,
		hostN:     hostNum,
		netN:      netNum,
		weight:    hostNum,
		split:     hostNum,
		mode:      roundRobin,
	}
}

func ipBlockFromCIDR(s string) *IPBlock {
	ipk, pnet, err := net.ParseCIDR(s) // range start ip, cidr ipnet
	if err != nil {
		fmt.Println("ParseCIDR() failed: ", s)
		return nil
	}
	ip0 := pnet.IP.To16() // cidr base ip
	nk, hk := ipUint64(ipk)
	n0, h0 := ipUint64(ip0)
	if hk < h0 || nk < n0 {
		fmt.Println("Wrong PraseCIDR result: ", s)
		return nil
	}
	ones, bits := pnet.Mask.Size()
	nz, hz := 0, bits-ones
	if hz > 64 {
		nz, hz = hz-64, 64
	}
	// must: 0 <= zbits <= 64
	offsetCIDR := func(zbits int, offset uint64) uint64 {
		switch zbits {
		case 0:
			return uint64(1)
		case 64:
			n := ^uint64(0) - offset
			if n+1 > n {
				n++
			}
			return n
		default:
			return (uint64(1) << zbits) - offset
		}
	}
	hostNum, netNum := offsetCIDR(hz, hk-h0), offsetCIDR(nz, nk-n0)
	// remove head IP like x.x.x.0 from CIDR
	if hk == h0 && hz > 0 {
		hk++
		hostNum--
	}
	// remove tail IP like x.x.x.255 from CIDR
	if hz > 1 {
		hostNum--
	}
	return &IPBlock{
		ip:        ipk,
		hostStart: hk,
		netStart:  nk,
		hostN:     hostNum,
		netN:      netNum,
		weight:    hostNum,
		split:     hostNum,
		mode:      roundRobin,
	}
}

// GetRandomIP return a random IP by ID from an IP block
func (b IPBlock) GetRandomIP(id uint64) net.IP {
	r := hashIDToUint64(id % b.weight)
	if ip4 := b.ip.To4(); ip4 != nil {
		i := b.hostStart + r%b.hostN
		return net.IPv4(byte(i>>24), byte(i>>16), byte(i>>8), byte(i))
	}
	if ip6 := b.ip.To16(); ip6 != nil {
		netN := b.netStart + r%b.netN
		hostN := b.hostStart + r%b.hostN
		if hostN < b.hostStart {
			netN++
		}
		if ip := make(net.IP, net.IPv6len); ip != nil {
			binary.BigEndian.PutUint64(ip[:8], netN)
			binary.BigEndian.PutUint64(ip[8:], hostN)
			return ip
		}
	}
	return nil
}

// GetRoundRobinIP return a IP by indexes from an IP block
func (b IPBlock) GetRoundRobinIP(hostIndex, netIndex uint64) net.IP {
	if ip4 := b.ip.To4(); ip4 != nil {
		i := b.hostStart + hostIndex%b.weight
		return net.IPv4(byte(i>>24), byte(i>>16), byte(i>>8), byte(i))
	}
	if ip6 := b.ip.To16(); ip6 != nil {
		netN := b.netStart + netIndex%b.netN
		hostN := b.hostStart + hostIndex%b.weight
		if hostN < b.hostStart {
			netN++
		}
		if ip := make(net.IP, net.IPv6len); ip != nil {
			binary.BigEndian.PutUint64(ip[:8], netN)
			binary.BigEndian.PutUint64(ip[8:], hostN)
			return ip
		}
	}
	return nil
}

// GetPool return an IPBlock slice with corret weight and mode
// Possible format is range1[:mode[:weight]][,range2[:mode[:weight]]]
func GetPool(ranges string) IPPool {
	ss := strings.Split(strings.TrimSpace(ranges), ",")
	pool := make([]IPBlock, 0)
	for _, bs := range ss {
		// range | mode | weight
		rmw := strings.Split(strings.TrimSpace(bs), "|")
		sz := len(rmw)
		if sz < 1 {
			continue
		}
		rangeStr := strings.TrimSpace(rmw[0])
		r := GetIPBlock(rangeStr)
		if r == nil {
			continue
		}
		r.weight = r.hostN
		r.mode = roundRobin
		if sz > 1 {
			modeStr := strings.TrimSpace(rmw[1])
			mode, err := strconv.Atoi(modeStr)
			if err == nil {
				r.mode = selectMode(mode)
			}
		}
		if sz > 2 {
			weightStr := strings.TrimSpace(rmw[2])
			weight, err := strconv.ParseUint(weightStr, 10, 64)
			if err == nil {
				r.weight = weight
			}
		}
		r.split = r.weight
		if len(pool) > 0 {
			r.split += pool[len(pool)-1].split
		}
		pool = append(pool, *r)
	}
	return pool
}

// GetIP return an IP from a pool of IPBlock slice
func (pool IPPool) GetIP(id uint64) net.IP {
	if len(pool) < 1 {
		return nil
	}
	idx := id % pool[len(pool)-1].split
	for _, b := range pool {
		if idx < b.split {
			switch b.mode {
			case random:
				return b.GetRandomIP(id)
			case roundRobin:
				return b.GetRoundRobinIP(id, id)
			default:
				return nil
			}
		}
	}
	return nil
}
