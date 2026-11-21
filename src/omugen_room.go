package main

// Online play with a room code, without port forwarding.
//
// The host asks its router to open the netplay ports (UPnP, which most home
// routers do by default) and gets the router's internet address; the room
// code is that address and port, in 12 letters and digits. The friend types
// (or pastes) it and connects there. Nothing goes through a server: the two
// games talk directly, as with an IP address.
//
// Only the host needs an open port: the guest's own router lets the replies
// in, as for any connection it starts. When the host's router doesn't answer
// (UPnP off) or shares an internet address with others (mobile networks,
// some providers), the friend can host instead.
//
// Rollback (UDP): the host's packets pass through a small relay (udpShim)
// that answers the guest from wherever its packets come from, as its router
// may give them another port than the game's.

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/huin/goupnp/dcps/internetgateway2"
	lua "github.com/yuin/gopher-lua"
)

const (
	roomUDPOffset = 100 // the rollback port is the lobby port + 100 (7500, 7600)
	roomGuestUDP  = 7550
	roomLease     = 4 * 60 * 60 // seconds the router keeps the ports open
)

var omugenRoom struct {
	mu     sync.Mutex
	state  string // "", "opening", "ready", "failed"
	text   string // the code once ready, or why it failed
	gw     upnpGateway
	ext    uint16 // the lobby's port on the router (rollback: ext + 100)
	cancel context.CancelFunc

	joinPort int // the host's lobby and rollback ports, from the code (0: the defaults)
	joinUDP  int
}

// upnpGateway is a router's WAN connection service (IP or PPP).
type upnpGateway interface {
	AddPortMappingCtx(ctx context.Context, NewRemoteHost string, NewExternalPort uint16, NewProtocol string,
		NewInternalPort uint16, NewInternalClient string, NewEnabled bool, NewPortMappingDescription string,
		NewLeaseDuration uint32) error
	DeletePortMappingCtx(ctx context.Context, NewRemoteHost string, NewExternalPort uint16, NewProtocol string) error
	GetExternalIPAddressCtx(ctx context.Context) (string, error)
	LocalAddr() net.IP
}

// ---------------------------------------------------------------- codes

// Crockford's base 32: no I, L, O or U, so no 1/I or 0/O mix-ups.
const roomAlphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// roomEncode makes the code of an IPv4 address and port: 6 bytes and a
// check byte, as 12 characters (XXXX-XXXX-XXXX).
func roomEncode(ip net.IP, port uint16) string {
	ip4 := ip.To4()
	if ip4 == nil {
		return ""
	}
	b := []byte{ip4[0], ip4[1], ip4[2], ip4[3], byte(port >> 8), byte(port)}
	b = append(b, roomCheck(b))
	var bits uint64
	for _, c := range b {
		bits = bits<<8 | uint64(c)
	}
	bits <<= 4 // 56 bits, padded to 60
	var out []byte
	for i := 11; i >= 0; i-- {
		out = append(out, roomAlphabet[(bits>>(uint(i)*5))&31])
		if i == 8 || i == 4 {
			out = append(out, '-')
		}
	}
	return string(out)
}

// roomDecode reads a code (any case, dashes and spaces optional).
func roomDecode(code string) (net.IP, uint16, error) {
	var digits []byte
	for _, r := range strings.ToUpper(code) {
		switch {
		case r == '-' || r == ' ' || r == '\t':
			continue
		case r == 'O':
			r = '0'
		case r == 'I' || r == 'L':
			r = '1'
		}
		i := strings.IndexRune(roomAlphabet, r)
		if i < 0 {
			return nil, 0, errors.New("that isn't a room code")
		}
		digits = append(digits, byte(i))
	}
	if len(digits) != 12 {
		return nil, 0, errors.New("a room code has 12 letters and digits")
	}
	var bits uint64
	for _, d := range digits {
		bits = bits<<5 | uint64(d)
	}
	bits >>= 4
	b := make([]byte, 7)
	for i := 6; i >= 0; i-- {
		b[i] = byte(bits)
		bits >>= 8
	}
	if roomCheck(b[:6]) != b[6] {
		return nil, 0, errors.New("that room code has a typo")
	}
	port := uint16(b[4])<<8 | uint16(b[5])
	if port == 0 {
		return nil, 0, errors.New("that room code has a typo")
	}
	return net.IPv4(b[0], b[1], b[2], b[3]), port, nil
}

func roomCheck(b []byte) byte {
	var c byte = 0x5A
	for _, x := range b {
		c = c<<1 | c>>7
		c ^= x
	}
	return c
}

// ---------------------------------------------------------------- hosting

// roomPrivate reports an address the internet can't reach: a home network's,
// or a provider's shared one (carrier-grade NAT, 100.64.0.0/10).
func roomPrivate(ip net.IP) bool {
	ip4 := ip.To4()
	if ip4 == nil || ip4.IsUnspecified() || ip4.IsLoopback() || ip4.IsPrivate() || ip4.IsLinkLocalUnicast() {
		return true
	}
	return ip4[0] == 100 && ip4[1]&0xC0 == 64
}

// roomDiscover finds the router's WAN connection service.
func roomDiscover(ctx context.Context) (upnpGateway, error) {
	type found struct {
		gw  upnpGateway
		err error
	}
	ch := make(chan found, 3)
	go func() {
		cs, _, err := internetgateway2.NewWANIPConnection2ClientsCtx(ctx)
		if len(cs) > 0 {
			ch <- found{gw: cs[0]}
			return
		}
		ch <- found{err: err}
	}()
	go func() {
		cs, _, err := internetgateway2.NewWANIPConnection1ClientsCtx(ctx)
		if len(cs) > 0 {
			ch <- found{gw: cs[0]}
			return
		}
		ch <- found{err: err}
	}()
	go func() {
		cs, _, err := internetgateway2.NewWANPPPConnection1ClientsCtx(ctx)
		if len(cs) > 0 {
			ch <- found{gw: cs[0]}
			return
		}
		ch <- found{err: err}
	}()
	var firstErr error
	for i := 0; i < 3; i++ {
		f := <-ch
		if f.gw != nil {
			return f.gw, nil
		}
		if firstErr == nil {
			firstErr = f.err
		}
	}
	if firstErr == nil {
		firstErr = errors.New("no router answered")
	}
	return nil, firstErr
}

// roomMap opens the lobby (TCP) and rollback (UDP) ports on the router, on
// the first free pair from the game's own ports, and returns the lobby's.
func roomMap(ctx context.Context, gw upnpGateway, tcpPort, udpPort int) (uint16, error) {
	client := gw.LocalAddr()
	if client == nil {
		return 0, errors.New("no local address")
	}
	var lastErr error
	for try := 0; try < 12; try++ {
		ext := uint16(tcpPort + try*7)
		mapped := true
		for _, m := range []struct {
			proto    string
			ext, int uint16
		}{{"TCP", ext, uint16(tcpPort)}, {"UDP", ext + roomUDPOffset, uint16(udpPort)}} {
			err := gw.AddPortMappingCtx(ctx, "", m.ext, m.proto, m.int, client.String(), true, "OMugen", roomLease)
			if err != nil && strings.Contains(err.Error(), "725") {
				// OnlyPermanentLeasesSupported: removed when hosting ends.
				err = gw.AddPortMappingCtx(ctx, "", m.ext, m.proto, m.int, client.String(), true, "OMugen", 0)
			}
			if err != nil {
				lastErr = err
				mapped = false
				if m.proto == "UDP" {
					gw.DeletePortMappingCtx(ctx, "", ext, "TCP")
				}
				break
			}
		}
		if mapped {
			return ext, nil
		}
		if ctx.Err() != nil {
			break
		}
	}
	return 0, lastErr
}

// omugenRoomHost opens the ports and makes the code, in the background.
func omugenRoomHost() {
	omugenRoomClose()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	omugenRoom.mu.Lock()
	omugenRoom.state, omugenRoom.text, omugenRoom.cancel = "opening", "", cancel
	omugenRoom.mu.Unlock()
	tcpPort := sys.cfg.Netplay.ListenPort
	go func() {
		defer cancel()
		fail := func(why string, err error) {
			if err != nil {
				LogMessage("OMugen: room code: %s (%v)", why, err)
			}
			omugenRoom.mu.Lock()
			if omugenRoom.state == "opening" {
				omugenRoom.state, omugenRoom.text = "failed", why
			}
			omugenRoom.mu.Unlock()
		}
		// Tests: OMUGEN_ROOM_FAKE=<address> makes a code for it, router aside.
		if fake := net.ParseIP(os.Getenv("OMUGEN_ROOM_FAKE")); fake != nil {
			omugenRoom.mu.Lock()
			omugenRoom.state, omugenRoom.text = "ready", roomEncode(fake, uint16(tcpPort))
			omugenRoom.mu.Unlock()
			return
		}
		dctx, dcancel := context.WithTimeout(ctx, 6*time.Second)
		gw, err := roomDiscover(dctx)
		dcancel()
		if err != nil {
			fail("Your router didn't answer, so it can't open the way for your friend. "+
				"Turn on UPnP in your router's settings, or let your friend host.", err)
			return
		}
		ipText, err := gw.GetExternalIPAddressCtx(ctx)
		ip := net.ParseIP(strings.TrimSpace(ipText))
		if err != nil || ip == nil {
			fail("Your router didn't give its internet address. Let your friend host.", err)
			return
		}
		if roomPrivate(ip) {
			fail("Your internet connection is shared with others (your provider's network, or "+
				"two routers in a row), so your friend can't reach you. Let your friend host.",
				fmt.Errorf("router address %s", ip))
			return
		}
		ext, err := roomMap(ctx, gw, tcpPort, 7600)
		if err != nil {
			fail("Your router refused to open the way for your friend. Let your friend host.", err)
			return
		}
		code := roomEncode(ip, ext)
		LogMessage("OMugen: room code ready (router ports %d/tcp, %d/udp)", ext, ext+roomUDPOffset)
		omugenRoom.mu.Lock()
		if omugenRoom.state == "opening" {
			omugenRoom.state, omugenRoom.text = "ready", code
			omugenRoom.gw, omugenRoom.ext = gw, ext
		} else {
			// Cancelled meanwhile: close them again.
			go roomUnmap(gw, ext)
		}
		omugenRoom.mu.Unlock()
	}()
}

func roomUnmap(gw upnpGateway, ext uint16) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	gw.DeletePortMappingCtx(ctx, "", ext, "TCP")
	gw.DeletePortMappingCtx(ctx, "", ext+roomUDPOffset, "UDP")
}

// omugenRoomClose closes the ports the room opened, and forgets the code
// (when online play ends, or OMugen quits).
func omugenRoomClose() {
	omugenRoom.mu.Lock()
	gw, ext, cancel := omugenRoom.gw, omugenRoom.ext, omugenRoom.cancel
	omugenRoom.state, omugenRoom.text, omugenRoom.gw, omugenRoom.ext, omugenRoom.cancel = "", "", nil, 0, nil
	omugenRoom.joinPort, omugenRoom.joinUDP = 0, 0
	omugenRoom.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if gw != nil {
		roomUnmap(gw, ext)
	}
}

// ---------------------------------------------------------------- rollback relay

// udpShim stands between the host's rollback (GGPO, on the loopback) and the
// internet: it sends to wherever the guest's packets come from.
type udpShim struct {
	public, local *net.UDPConn
	ggpo          *net.UDPAddr
	remoteIP      net.IP
	remote        atomic.Value // *net.UDPAddr
	ggpoPort      int
}

var omugenShim *udpShim

// newUDPShim listens on publicPort for the guest at remoteIP (its packets
// are sent to guessPort until one arrives).
func newUDPShim(publicPort int, remoteIP string, guessPort int) (*udpShim, error) {
	ip := net.ParseIP(remoteIP)
	if ip == nil {
		return nil, errors.New("no guest address")
	}
	pub, err := net.ListenUDP("udp", &net.UDPAddr{Port: publicPort})
	if err != nil {
		return nil, err
	}
	loc, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		pub.Close()
		return nil, err
	}
	// A free port for GGPO's own socket.
	probe, err := net.ListenUDP("udp", &net.UDPAddr{Port: 0})
	if err != nil {
		pub.Close()
		loc.Close()
		return nil, err
	}
	gp := probe.LocalAddr().(*net.UDPAddr).Port
	probe.Close()
	s := &udpShim{
		public: pub, local: loc, remoteIP: ip, ggpoPort: gp,
		ggpo: &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: gp},
	}
	s.remote.Store(&net.UDPAddr{IP: ip, Port: guessPort})
	go s.fromGuest()
	go s.fromGGPO()
	return s, nil
}

func (s *udpShim) localPort() int { return s.local.LocalAddr().(*net.UDPAddr).Port }

func (s *udpShim) fromGuest() {
	buf := make([]byte, 8192)
	for {
		n, from, err := s.public.ReadFromUDP(buf)
		if err != nil {
			return
		}
		if !from.IP.Equal(s.remoteIP) {
			continue // only the player in the lobby
		}
		if r := s.remote.Load().(*net.UDPAddr); r.Port != from.Port {
			s.remote.Store(&net.UDPAddr{IP: from.IP, Port: from.Port})
		}
		s.local.WriteToUDP(buf[:n], s.ggpo)
	}
}

func (s *udpShim) fromGGPO() {
	buf := make([]byte, 8192)
	for {
		n, from, err := s.local.ReadFromUDP(buf)
		if err != nil {
			return
		}
		if from.Port != s.ggpo.Port {
			continue
		}
		s.public.WriteToUDP(buf[:n], s.remote.Load().(*net.UDPAddr))
	}
}

func (s *udpShim) Close() {
	s.public.Close()
	s.local.Close()
}

func omugenShimClose() {
	if omugenShim != nil {
		omugenShim.Close()
		omugenShim = nil
	}
}

// ---------------------------------------------------------------- Lua

func omugenRoomScriptInit(l *lua.LState) {
	luaRegister(l, "omugenRoomHost", func(l *lua.LState) int {
		/*Opens the router's ports for online play and makes a room code (in the background).
		@function omugenRoomHost*/
		omugenRoomHost()
		return 0
	})
	luaRegister(l, "omugenRoomStatus", func(l *lua.LState) int {
		/*Where the room code is: "opening", "ready" (with the code) or "failed" (with why).
		@function omugenRoomStatus
		@treturn string state
		@treturn string text The code, or why it failed.*/
		omugenRoom.mu.Lock()
		l.Push(lua.LString(omugenRoom.state))
		l.Push(lua.LString(omugenRoom.text))
		omugenRoom.mu.Unlock()
		return 2
	})
	luaRegister(l, "omugenRoomClose", func(l *lua.LState) int {
		/*Closes the router's ports and forgets the room code.
		@function omugenRoomClose*/
		omugenRoomClose()
		return 0
	})
	luaRegister(l, "omugenRoomValid", func(l *lua.LState) int {
		/*Whether a text is a room code.
		@function omugenRoomValid
		@tparam string text
		@treturn boolean valid*/
		_, _, err := roomDecode(strArg(l, 1))
		l.Push(lua.LBool(err == nil))
		return 1
	})
	luaRegister(l, "omugenRoomJoin", func(l *lua.LState) int {
		/*Reads a room code: the host's address (the next enterNetPlay connects to
		its ports), or nil and why the code is wrong.
		@function omugenRoomJoin
		@tparam string code
		@treturn string|nil address
		@treturn string error*/
		ip, port, err := roomDecode(strArg(l, 1))
		if err != nil {
			l.Push(lua.LNil)
			l.Push(lua.LString(err.Error()))
			return 2
		}
		omugenRoom.mu.Lock()
		omugenRoom.joinPort, omugenRoom.joinUDP = int(port), int(port)+roomUDPOffset
		omugenRoom.mu.Unlock()
		l.Push(lua.LString(ip.String()))
		l.Push(lua.LString(""))
		return 2
	})
	luaRegister(l, "omugenClipboardSet", func(l *lua.LState) int {
		/*Copies a text to the clipboard.
		@function omugenClipboardSet
		@tparam string text*/
		omugenSetClipboard(strArg(l, 1))
		return 0
	})
	luaRegister(l, "omugenTextInput", func(l *lua.LState) int {
		/*Starts or stops text input (on phones, shows or hides the keyboard).
		@function omugenTextInput
		@tparam boolean on*/
		omugenSetTextInput(boolArg(l, 1))
		return 0
	})
}
