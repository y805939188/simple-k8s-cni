package bpf_map

const (
	APP_PREFIX = "ding"
)

const (
	DEFAULT_MAP_ROOT   = "/sys/fs/bpf"
	DEFAULT_MAP_PREFIX = "tc/globals"
)

const (
	DEFAULT_TC_MAP_PREFIX = DEFAULT_MAP_ROOT + "/" + DEFAULT_MAP_PREFIX + "/" + APP_PREFIX
)

const (
	// 绑 veth 网卡的 ip 以及对应的 mac 地址还有 ifindex
	LXC_MAP_DEFAULT_PATH = DEFAULT_TC_MAP_PREFIX + "_lxc"
	// 绑每个 pod 网段 ip 对应的 node ip 地址
	POD_MAP_DEFAULT_PATH = DEFAULT_TC_MAP_PREFIX + "_ip"
	// 用来存本机的网卡设备们 ip 和 ifindex 等信息
	NODE_LOCAL_MAP_DEFAULT_PATH = DEFAULT_TC_MAP_PREFIX + "_local"
)
