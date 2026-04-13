package collector

import (
	"context"
	"fmt"
	"testing"

	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/model"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/state"
)

func TestQoSCollectorPopulatesState(t *testing.T) {
	st := state.New()
	stub := func(_ context.Context, args ...string) ([]byte, error) {
		// args = ["-s", "qdisc", "show", "dev", iface]
		var iface string
		for i, a := range args {
			if a == "dev" && i+1 < len(args) {
				iface = args[i+1]
			}
		}
		switch iface {
		case "eth1":
			return []byte("qdisc cake 8003: root refcnt 2 bandwidth 100Mbit\n Sent 1000 bytes 10 pkt (dropped 1, overlimits 2 requeues 0) \n backlog 0b 0p requeues 0\n"), nil
		case "ifb4eth1":
			return []byte("qdisc htb 1: root refcnt 2\n Sent 2000 bytes 20 pkt (dropped 3, overlimits 0 requeues 0) \nqdisc fq_codel 8004: parent 1:1\n Sent 2000 bytes 20 pkt (dropped 3, overlimits 0 requeues 0) \n  maxpacket 1500 drop_overlimit 0 new_flow_count 5 ecn_mark 1\n"), nil
		}
		return nil, nil
	}

	c := NewQoS(QoSOpts{State: st, Run: stub, EgressInterface: "eth1", IngressInterface: "ifb4eth1"})
	if err := c.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	got, _ := st.SnapshotQoS()
	if got.Egress == nil || got.Egress.SentBytes != 1000 {
		t.Errorf("Egress = %+v", got.Egress)
	}
	if got.Ingress == nil || got.Ingress.SentBytes != 2000 || got.Ingress.ECNMark != 1 {
		t.Errorf("Ingress = %+v", got.Ingress)
	}
	if c.Tier() != Medium {
		t.Errorf("Tier = %v, want Medium", c.Tier())
	}
}

func TestQoSCollectorPreservesPreviousOnTotalFailure(t *testing.T) {
	st := state.New()
	// Seed a known-good QoS snapshot first.
	good := model.QoS{Egress: &model.QdiscStats{Kind: "cake", SentBytes: 500}}
	st.SetQoS(good)

	failingRun := func(_ context.Context, _ ...string) ([]byte, error) {
		return nil, fmt.Errorf("tc unavailable")
	}
	c := NewQoS(QoSOpts{State: st, Run: failingRun, EgressInterface: "eth1", IngressInterface: "ifb4eth1"})

	if err := c.Run(context.Background()); err == nil {
		t.Error("Run should error when both sides fail")
	}
	got, _ := st.SnapshotQoS()
	if got.Egress == nil || got.Egress.SentBytes != 500 {
		t.Errorf("previous good snapshot should be preserved, got Egress=%+v", got.Egress)
	}
}
