package bpf_map

type EndpointKey struct {
	IP uint32
}

type EndpointInfo struct {
	IfIndex uint32
	LxcID   uint16
	Flag    uint8 // 用来表示这块儿网卡是 veth 在 host 的那侧还是 ns 里那侧
	_       uint8
	MAC     uint64
	NodeMAC uint64
}
