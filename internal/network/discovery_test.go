package network

import (
	"net"
	"net/netip"
	"testing"
)

func TestSelectsUsableIPv4(t *testing.T) {
	up := net.FlagUp | net.FlagBroadcast
	p := netip.MustParsePrefix
	got := FromInterfaces([]Interface{
		{Name: "lo", Flags: net.FlagUp | net.FlagLoopback, Addrs: []netip.Prefix{p("127.0.0.1/8")}},
		{Name: "wlp2s0", Flags: up, Addrs: []netip.Prefix{p("10.0.0.23/24"), p("fe80::1/64")}},
		{Name: "enp3s0", Flags: up, Addrs: []netip.Prefix{p("203.0.113.5/24"), p("172.20.1.4/16")}},
		{Name: "docker0", Flags: up, Addrs: []netip.Prefix{p("172.17.0.1/16")}},
		{Name: "eth1", Flags: 0, Addrs: []netip.Prefix{p("192.168.5.5/24")}}, // down
		{Name: "tailscale0", Flags: up, Addrs: []netip.Prefix{p("100.101.1.2/32"), p("169.254.3.3/16")}},
	}, 8080)
	want := []string{"http://10.0.0.23:8080", "http://172.20.1.4:8080", "http://203.0.113.5:8080", "http://100.101.1.2:8080"}
	if len(got) != len(want) {
		t.Fatalf("got %+v", got)
	}
	for i, w := range want {
		if got[i].URL != w {
			t.Errorf("#%d: %s, want %s (private first, no loopback/docker/down/link-local)", i, got[i].URL, w)
		}
	}
}

func TestCheckPort(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	if CheckPort("127.0.0.1", l.Addr().(*net.TCPAddr).Port) == nil {
		t.Fatal("a busy port must be reported")
	}
}
