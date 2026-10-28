// Forwarding input bounds and endpoint safety by ir_spoof; https://t.me/ir_spoof.
package core

import (
	"net"
	"strconv"
	"testing"
)

func TestMappingsSupportRangesAndIPv6WithoutChangingTargets(t *testing.T) {
	values, err := ParseMappings([]string{"18080-18082=8080", "[::1]:18443=[2001:db8::2]:443", "19000-19001:9000"})
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 6 {
		t.Fatalf("expected six listeners; got %d", len(values))
	}
	for i, expected := range []int{18080, 18081, 18082} {
		if values[i].Bind != net.JoinHostPort("0.0.0.0", strconv.Itoa(expected)) || values[i].Target != "127.0.0.1:8080" {
			t.Fatal("range changed fixed backend target")
		}
	}
	if values[3] != (Mapping{Bind: "[::1]:18443", Target: "[2001:db8::2]:443"}) {
		t.Fatal("IPv6 endpoints changed")
	}
	if values[4].Target != "127.0.0.1:9000" || values[5].Target != "127.0.0.1:9000" {
		t.Fatal("legacy fixed-target range changed")
	}
}

func TestMappingsRejectInvalidBoundsAndDuplicates(t *testing.T) {
	for _, input := range [][]string{
		{"0"}, {"65536"}, {"1-257"}, {"9-1"}, {"1-2-3"},
		{"1=0"}, {"1=65536"}, {"1=18446744073709551617"},
		{"127.0.0.1:8080=1", "127.0.0.1:8080=2"},
		{"8080-8082", "8081"}, {"[::1]:8080", "[::1]:8080=8081"},
		{"example.invalid:8080"}, {"8080=bad host:80"}, {"8080=host\r\nheader:80"},
		{"::1:8080"}, {"[::1]:8080=2001:db8::1:80"}, {"8080=80=90"},
	} {
		if _, err := ParseMappings(input); err == nil {
			t.Errorf("invalid mappings accepted: %q", input)
		}
	}
	maximum, err := ParseMappings([]string{"1000-1255"})
	if err != nil || len(maximum) != 256 {
		t.Fatal("valid 256-listener bound rejected")
	}
	if _, err := ParseMappings([]string{"1000-1255", "2000"}); err == nil {
		t.Fatal("aggregate listener limit exceeded")
	}
}
