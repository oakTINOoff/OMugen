package main

import (
	"context"
	"math"
	"net"
	"os"
	"strings"
	"testing"
	"time"
)

func TestRoomCode(t *testing.T) {
	for _, c := range []struct {
		ip   string
		port uint16
	}{{"203.0.113.7", 7500}, {"1.2.3.4", 7507}, {"255.255.255.255", 65535}, {"8.8.4.4", 1}} {
		code := roomEncode(net.ParseIP(c.ip), c.port)
		if len(code) != 14 || strings.Count(code, "-") != 2 {
			t.Errorf("%s:%d: code %q", c.ip, c.port, code)
		}
		ip, port, err := roomDecode(code)
		if err != nil || !ip.Equal(net.ParseIP(c.ip)) || port != c.port {
			t.Errorf("%q decodes to %v:%d (%v)", code, ip, port, err)
		}
		// Typed by hand: lower case, no dashes, O for 0 and I/L for 1.
		loose := strings.ToLower(strings.ReplaceAll(code, "-", ""))
		loose = strings.ReplaceAll(strings.ReplaceAll(loose, "0", "o"), "1", "l")
		if ip, port, err := roomDecode(loose); err != nil || !ip.Equal(net.ParseIP(c.ip)) || port != c.port {
			t.Errorf("%q (as typed) decodes to %v:%d (%v)", loose, ip, port, err)
		}
	}
	code := roomEncode(net.ParseIP("203.0.113.7"), 7500)
	// A typo is caught by the check byte.
	b := []byte(code)
	if b[0] == 'A' {
		b[0] = 'B'
	} else {
		b[0] = 'A'
	}
	if _, _, err := roomDecode(string(b)); err == nil {
		t.Errorf("a typo in %q went unnoticed", string(b))
	}
	for _, bad := range []string{"", "ABCD", "ABCD-EFGH-JKLU", "192.168.1.1"} {
		if _, _, err := roomDecode(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestRoomPrivate(t *testing.T) {
	for ip, want := range map[string]bool{
		"192.168.1.10": true, "10.0.0.1": true, "172.16.5.4": true, "100.64.1.2": true,
		"100.127.255.1": true, "0.0.0.0": true, "127.0.0.1": true, "169.254.1.1": true,
		"203.0.113.7": false, "100.128.0.1": false, "8.8.8.8": false,
	} {
		if got := roomPrivate(net.ParseIP(ip)); got != want {
			t.Errorf("roomPrivate(%s) = %v, want %v", ip, got, want)
		}
	}
}

// The relay answers the guest from the port its packets come from, and
// passes them to and from the rollback (GGPO) socket.
func TestUDPShim(t *testing.T) {
	shim, err := newUDPShim(0, "127.0.0.1", 1) // a wrong guess: learned from the first packet
	if err != nil {
		t.Fatal(err)
	}
	defer shim.Close()
	publicPort := shim.public.LocalAddr().(*net.UDPAddr).Port
	ggpo, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: shim.ggpoPort})
	if err != nil {
		t.Fatal(err)
	}
	defer ggpo.Close()
	guest, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer guest.Close()
	buf := make([]byte, 64)
	guest.WriteToUDP([]byte("hello host"), &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: publicPort})
	ggpo.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, from, err := ggpo.ReadFromUDP(buf)
	if err != nil || string(buf[:n]) != "hello host" || from.Port != shim.localPort() {
		t.Fatalf("GGPO got %q from %v (%v)", buf[:n], from, err)
	}
	ggpo.WriteToUDP([]byte("hello guest"), &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: shim.localPort()})
	guest.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, from, err = guest.ReadFromUDP(buf)
	if err != nil || string(buf[:n]) != "hello guest" || from.Port != publicPort {
		t.Fatalf("guest got %q from %v (%v)", buf[:n], from, err)
	}
}

func TestDetMath(t *testing.T) {
	ulps := func(a, b float64) uint64 {
		if a == b || math.IsNaN(a) && math.IsNaN(b) {
			return 0
		}
		x, y := math.Float64bits(a), math.Float64bits(b)
		if x > y {
			return x - y
		}
		return y - x
	}
	for _, x := range []float64{-700, -20.5, -1, -0.3, 1e-9, 0.5, 1, 2.718281828, 10, 88.7, 700} {
		if d := ulps(detExp(x), math.Exp(x)); d > 1 {
			t.Errorf("exp(%v): %v vs %v", x, detExp(x), math.Exp(x))
		}
	}
	for _, x := range []float64{1e-300, 0.001, 0.5, 1, 2, 3.14159, 1000, 1e300} {
		if d := ulps(detLog(x), math.Log(x)); d > 1 {
			t.Errorf("log(%v): %v vs %v", x, detLog(x), math.Log(x))
		}
	}
	for _, c := range [][2]float64{{2, 10}, {2, 0.5}, {10, -2}, {1.5, 2.25}, {0.9, 37.3}, {-2, 3}, {-2, 0.5}, {0, -1}} {
		if d := ulps(detPow(c[0], c[1]), math.Pow(c[0], c[1])); d > 2 {
			t.Errorf("pow(%v, %v): %v vs %v", c[0], c[1], detPow(c[0], c[1]), math.Pow(c[0], c[1]))
		}
	}
	if detHypot(3, 4) != 5 || detHypot(-5, 12) != 13 {
		t.Errorf("hypot")
	}
}

// OMUGEN_LIVE_UPNP=1: asks this network's router for its WAN service and
// address (read only: no port is opened). Prints whether hosting would work.
func TestRoomLiveDiscover(t *testing.T) {
	if os.Getenv("OMUGEN_LIVE_UPNP") != "1" {
		t.Skip("set OMUGEN_LIVE_UPNP=1")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	start := time.Now()
	gw, err := roomDiscover(ctx)
	if err != nil {
		t.Logf("no router answered after %v: %v", time.Since(start).Round(time.Millisecond), err)
		return
	}
	ipText, err := gw.GetExternalIPAddressCtx(ctx)
	ip := net.ParseIP(strings.TrimSpace(ipText))
	t.Logf("router found in %v (local address found: %v); its address is valid: %v, reachable from the internet: %v (%v)",
		time.Since(start).Round(time.Millisecond), gw.LocalAddr() != nil, ip != nil, ip != nil && !roomPrivate(ip), err)
}
