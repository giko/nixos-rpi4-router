package wireguard

import (
	"testing"
)

const dumpFixture = "PRIV1\tPUB_LOCAL\t51820\t0x20000\n" +
	"PEER_PUB\tpsk\t1.2.3.4:51820\t0.0.0.0/0\t1712845000\t12345\t67890\t25\n"

func TestParseDump(t *testing.T) {
	d, err := ParseDump(dumpFixture)
	if err != nil {
		t.Fatal(err)
	}

	if d.LocalPrivateKey != "PRIV1" {
		t.Errorf("LocalPrivateKey = %q, want %q", d.LocalPrivateKey, "PRIV1")
	}
	if d.LocalPublicKey != "PUB_LOCAL" {
		t.Errorf("LocalPublicKey = %q, want %q", d.LocalPublicKey, "PUB_LOCAL")
	}
	if d.ListenPort != 51820 {
		t.Errorf("ListenPort = %d, want %d", d.ListenPort, 51820)
	}
	if d.Fwmark != "0x20000" {
		t.Errorf("Fwmark = %q, want %q", d.Fwmark, "0x20000")
	}

	if len(d.Peers) != 1 {
		t.Fatalf("expected 1 peer, got %d", len(d.Peers))
	}

	p := d.Peers[0]
	if p.PublicKey != "PEER_PUB" {
		t.Errorf("PublicKey = %q, want %q", p.PublicKey, "PEER_PUB")
	}
	if p.PresharedKey != "psk" {
		t.Errorf("PresharedKey = %q, want %q", p.PresharedKey, "psk")
	}
	if p.Endpoint != "1.2.3.4:51820" {
		t.Errorf("Endpoint = %q, want %q", p.Endpoint, "1.2.3.4:51820")
	}
	if p.AllowedIPs != "0.0.0.0/0" {
		t.Errorf("AllowedIPs = %q, want %q", p.AllowedIPs, "0.0.0.0/0")
	}
	if p.LatestHandshakeUnix != 1712845000 {
		t.Errorf("LatestHandshakeUnix = %d, want %d", p.LatestHandshakeUnix, 1712845000)
	}
	if p.RXBytes != 12345 {
		t.Errorf("RXBytes = %d, want %d", p.RXBytes, 12345)
	}
	if p.TXBytes != 67890 {
		t.Errorf("TXBytes = %d, want %d", p.TXBytes, 67890)
	}
	if p.PersistentKeepalive != 25 {
		t.Errorf("PersistentKeepalive = %d, want %d", p.PersistentKeepalive, 25)
	}
}

func TestParseDumpMultiplePeers(t *testing.T) {
	fixture := "PRIV1\tPUB_LOCAL\t51820\t0x20000\n" +
		"PEER1\tpsk1\t1.2.3.4:51820\t0.0.0.0/0\t1712845000\t100\t200\t25\n" +
		"PEER2\tpsk2\t5.6.7.8:51821\t10.0.0.0/8\t1712845100\t300\t400\t0\n"

	d, err := ParseDump(fixture)
	if err != nil {
		t.Fatal(err)
	}

	if len(d.Peers) != 2 {
		t.Fatalf("expected 2 peers, got %d", len(d.Peers))
	}

	if d.Peers[0].PublicKey != "PEER1" {
		t.Errorf("Peers[0].PublicKey = %q, want %q", d.Peers[0].PublicKey, "PEER1")
	}
	if d.Peers[1].PublicKey != "PEER2" {
		t.Errorf("Peers[1].PublicKey = %q, want %q", d.Peers[1].PublicKey, "PEER2")
	}
	if d.Peers[1].Endpoint != "5.6.7.8:51821" {
		t.Errorf("Peers[1].Endpoint = %q, want %q", d.Peers[1].Endpoint, "5.6.7.8:51821")
	}
}

func TestParseDumpNoPeers(t *testing.T) {
	fixture := "PRIV1\tPUB_LOCAL\t51820\t0x20000\n"

	d, err := ParseDump(fixture)
	if err != nil {
		t.Fatal(err)
	}

	if d.LocalPublicKey != "PUB_LOCAL" {
		t.Errorf("LocalPublicKey = %q, want %q", d.LocalPublicKey, "PUB_LOCAL")
	}
	if len(d.Peers) != 0 {
		t.Fatalf("expected 0 peers, got %d", len(d.Peers))
	}
}

func TestParseDumpEmpty(t *testing.T) {
	_, err := ParseDump("")
	if err == nil {
		t.Fatal("expected error for empty input")
	}
}

func TestParseDumpBadPort(t *testing.T) {
	_, err := ParseDump("PRIV1\tPUB_LOCAL\tnotaport\t0x20000\n")
	if err == nil {
		t.Fatal("expected error for non-integer listen port")
	}
}

func TestParseDumpBadPeerLine(t *testing.T) {
	fixture := "PRIV1\tPUB_LOCAL\t51820\t0x20000\n" +
		"PEER_PUB\tpsk\t1.2.3.4:51820\n" // too few fields
	_, err := ParseDump(fixture)
	if err == nil {
		t.Fatal("expected error for peer line with too few fields")
	}
}
