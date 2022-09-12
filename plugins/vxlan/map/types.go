package bpf_map

/********* 存本机网络设备的 ip - ifindex *********/
/********* pin path: NODE_LOCAL_MAP_DEFAULT_PATH *********/
type LocalNodeMapKey struct {
	IP uint32
}

type LocalNodeMapValue struct {
	IfIndex uint32
}

/********* 存本机每个 veth pair 的信息 *********/
/********* pin path: LXC_MAP_DEFAULT_PATH *********/
type EndpointMapKey struct {
	IP uint32
}

type EndpointMapInfo struct {
	IfIndex    uint32
	LxcIfIndex uint32 // 标记另一半的 ifindex
	// MAC        uint64
	// NodeMAC    uint64
	MAC     [8]byte
	NodeMAC [8]byte
}

/********* 存整个集群的 pod ip 以及对应的 node ip *********/
/********* pin path: POD_MAP_DEFAULT_PATH *********/
/********* 起一条常驻进程监听 etcd 以更新该 map *********/
type PodNodeMapKey struct {
	IP uint32
}

type PodNodeMapValue struct {
	IP uint32
}
