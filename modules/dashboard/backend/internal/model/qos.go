package model

// QoS is the snapshot served by /api/qos. Egress is CAKE on the WAN
// physical interface; Ingress is HTB+fq_codel on the WAN ingress IFB.
type QoS struct {
	Egress  *QdiscStats `json:"wan_egress,omitempty"`
	Ingress *QdiscStats `json:"wan_ingress,omitempty"`
}

// QdiscStats is the unified shape both qdiscs serialize to. Fields
// not relevant for a given qdisc kind stay zero.
type QdiscStats struct {
	Kind          string    `json:"kind"`
	BandwidthBps  int64     `json:"bandwidth_bps"`
	SentBytes     int64     `json:"sent_bytes"`
	SentPackets   int64     `json:"sent_packets"`
	Dropped       int64     `json:"dropped"`
	Overlimits    int64     `json:"overlimits"`
	Requeues      int64     `json:"requeues"`
	BacklogBytes  int64     `json:"backlog_bytes"`
	BacklogPkts   int64     `json:"backlog_pkts"`
	Tins          []CAKETin `json:"tins,omitempty"`
	NewFlowCount  int64     `json:"new_flow_count,omitempty"`
	OldFlowsLen   int64     `json:"old_flows_len,omitempty"`
	NewFlowsLen   int64     `json:"new_flows_len,omitempty"`
	ECNMark       int64     `json:"ecn_mark,omitempty"`
	DropOverlimit int64     `json:"drop_overlimit,omitempty"`
}

// CAKETin is one CAKE traffic class.
type CAKETin struct {
	Name         string `json:"name"`
	ThreshKbit   int64  `json:"thresh_kbit"`
	TargetUs     int64  `json:"target_us"`
	IntervalUs   int64  `json:"interval_us"`
	PeakDelayUs  int64  `json:"peak_delay_us"`
	AvgDelayUs   int64  `json:"avg_delay_us"`
	BacklogBytes int64  `json:"backlog_bytes"`
	Packets      int64  `json:"packets"`
	Bytes        int64  `json:"bytes"`
	Drops        int64  `json:"drops"`
	Marks        int64  `json:"marks"`
}
