package common

import (
	"fmt"
	"net"
)

// In order for the IpPortTuple and the TcpTuple to be used as
// hashtable keys, they need to have a fixed size. This means the
// net.IP is problematic because it's internally represented as a slice.
// We're introducing the HashableIpPortTuple and the HashableTcpTuple
// types which are internally simple byte arrays.

const MaxIpPortTupleRawSize = 16 + 16 + 2 + 2

type HashableIpPortTuple [MaxIpPortTupleRawSize]byte

type IpPortTuple struct {
	IpLength         int
	SrcIp, DstIp     net.IP
	SrcPort, DstPort uint16

	raw    HashableIpPortTuple // SrcIp:SrcPort:DstIp:DstPort
	revRaw HashableIpPortTuple // DstIp:DstPort:SrcIp:SrcPort
}

func NewIpPortTuple(ipLength int, srcIp net.IP, srcPort uint16,
	dstIp net.IP, dstPort uint16) IpPortTuple {

	tuple := IpPortTuple{
		IpLength: ipLength,
		SrcIp:    srcIp,
		DstIp:    dstIp,
		SrcPort:  srcPort,
		DstPort:  dstPort,
	}
	tuple.ComputeHashables()

	return tuple
}

func (t *IpPortTuple) ComputeHashables() {
	copy(t.raw[0:16], t.SrcIp)
	copy(t.raw[16:18], []byte{byte(t.SrcPort >> 8), byte(t.SrcPort)})
	copy(t.raw[18:34], t.DstIp)
	copy(t.raw[34:36], []byte{byte(t.DstPort >> 8), byte(t.DstPort)})

	copy(t.revRaw[0:16], t.DstIp)
	copy(t.revRaw[16:18], []byte{byte(t.DstPort >> 8), byte(t.DstPort)})
	copy(t.revRaw[18:34], t.SrcIp)
	copy(t.revRaw[34:36], []byte{byte(t.SrcPort >> 8), byte(t.SrcPort)})
}

func (t *IpPortTuple) String() string {
	return fmt.Sprintf("IpPortTuple src[%s:%d] dst[%s:%d]",
		t.SrcIp.String(),
		t.SrcPort,
		t.DstIp.String(),
		t.DstPort)
}

// Hashable returns a hashable value that uniquely identifies
// the IP-port tuple.
func (t *IpPortTuple) Hashable() HashableIpPortTuple {
	return t.raw
}

// Hashable returns a hashable value that uniquely identifies
// the IP-port tuple after swapping the source and destination.
func (t *IpPortTuple) RevHashable() HashableIpPortTuple {
	return t.revRaw
}

const MaxTcpTupleRawSize = 16 + 16 + 2 + 2 + 4

type HashableTcpTuple [MaxTcpTupleRawSize]byte

type TcpTuple struct {
	IpLength         int
	SrcIp, DstIp     net.IP
	SrcPort, DstPort uint16
	StreamId         uint32

	raw HashableTcpTuple // SrcIp:SrcPort:DstIp:DstPort:stream_id
}

func TcpTupleFromIpPort(t *IpPortTuple, tcp_id uint32) TcpTuple {
	tuple := TcpTuple{
		IpLength: t.IpLength,
		SrcIp:    t.SrcIp,
		DstIp:    t.DstIp,
		SrcPort:  t.SrcPort,
		DstPort:  t.DstPort,
		StreamId: tcp_id,
	}
	tuple.ComputeHashables()

	return tuple
}

func (t *TcpTuple) ComputeHashables() {
	copy(t.raw[0:16], t.SrcIp)
	copy(t.raw[16:18], []byte{byte(t.SrcPort >> 8), byte(t.SrcPort)})
	copy(t.raw[18:34], t.DstIp)
	copy(t.raw[34:36], []byte{byte(t.DstPort >> 8), byte(t.DstPort)})
	copy(t.raw[36:40], []byte{byte(t.StreamId >> 24), byte(t.StreamId >> 16),
		byte(t.StreamId >> 8), byte(t.StreamId)})
}

func (t TcpTuple) String() string {
	return fmt.Sprintf("TcpTuple src[%s:%d] dst[%s:%d] stream_id[%d]",
		t.SrcIp.String(),
		t.SrcPort,
		t.DstIp.String(),
		t.DstPort,
		t.StreamId)
}

// Returns a pointer to the equivalent IpPortTuple.
func (t TcpTuple) IpPort() *IpPortTuple {
	ipPort := NewIpPortTuple(t.IpLength, t.SrcIp, t.SrcPort,
		t.DstIp, t.DstPort)
	return &ipPort
}

// Hashable() returns a hashable value that uniquely identifies
// the TCP tuple.
func (t *TcpTuple) Hashable() HashableTcpTuple {
	return t.raw
}

// Source and destination process names, as found by the proc module.
type CmdlineTuple struct {
	Src, Dst []byte
}
