// Safe bounded forwarding maps by ir_spoof. https://t.me/ir_spoof
package core

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
)

type Mapping struct{ Bind, Target string }

func port(s string) (int, error) {
	n, e := strconv.Atoi(s)
	if e != nil || n < 1 || n > 65535 {
		return 0, errors.New("invalid forwarding port")
	}
	return n, nil
}
func rangePorts(s string) ([]int, error) {
	parts := strings.Split(s, "-")
	a, e := port(parts[0])
	if e != nil {
		return nil, e
	}
	b := a
	if len(parts) == 2 {
		b, e = port(parts[1])
	}
	if e != nil || len(parts) > 2 || b < a || b-a > 255 {
		return nil, errors.New("forwarding range exceeds 256 ports or is invalid")
	}
	out := []int{}
	for n := a; n <= b; n++ {
		out = append(out, n)
	}
	return out, nil
}
func ParseMappings(values []string) ([]Mapping, error) {
	var out []Mapping
	seen := map[string]bool{}
	for _, text := range values {
		if strings.ContainsAny(text, "\r\n\x00") {
			return nil, errors.New("invalid map")
		}
		p := strings.Split(text, "=")
		if len(p) > 2 {
			return nil, errors.New("invalid map separator")
		}
		left := p[0]
		bindIP := "0.0.0.0"
		ports := left
		if strings.HasPrefix(left, "[") || strings.Count(left, ":") == 1 {
			h, v, e := net.SplitHostPort(left)
			if e != nil {
				return nil, errors.New("invalid map bind")
			}
			bindIP = h
			ports = v
		}
		// Recovered legacy range:fixed-port shorthand, without an explicit bind IP.
		if strings.Contains(left, "-") && strings.Contains(left, ":") && !strings.HasPrefix(left, "[") {
			a := strings.Split(left, ":")
			if len(a) == 2 && net.ParseIP(a[0]) == nil {
				ports = a[0]
				bindIP = "0.0.0.0"
				if len(p) == 1 {
					p = append(p, a[1])
				}
			}
		}
		if net.ParseIP(bindIP) == nil {
			return nil, errors.New("mapping bind must be a literal assigned IP")
		}
		numbers, e := rangePorts(ports)
		if e != nil {
			return nil, e
		}
		for _, n := range numbers {
			target := net.JoinHostPort("127.0.0.1", strconv.Itoa(n))
			if len(p) == 2 {
				target = p[1]
				if _, e := port(target); e == nil {
					target = net.JoinHostPort("127.0.0.1", target)
				}
				h, v, e := net.SplitHostPort(target)
				if e != nil || h == "" {
					return nil, errors.New("target must be port or host:port")
				}
				if _, e = port(v); e != nil {
					return nil, e
				}
				if strings.ContainsAny(h, " \t\r\n\x00") {
					return nil, errors.New("invalid target")
				}
			}
			bind := net.JoinHostPort(bindIP, strconv.Itoa(n))
			if seen[bind] {
				return nil, errors.New("duplicate map listener")
			}
			seen[bind] = true
			out = append(out, Mapping{bind, target})
			if len(out) > 256 {
				return nil, fmt.Errorf("at most 256 forwarding mappings are permitted")
			}
		}
	}
	return out, nil
}
