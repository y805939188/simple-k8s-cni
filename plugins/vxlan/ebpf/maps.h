#include <linux/bpf.h>
#include <linux/pkt_cls.h>
#include <bpf/bpf_helpers.h>

#include "common.h"

#define LOCAL_DEV_VXLAN 1;
#define LOCAL_DEV_VETH 2;

#define DEFAULT_TUNNEL_ID 13190

struct endpointKey {
  __u32 ip;
};

struct endpointInfo {
  __u32 ifIndex;
  __u32 lxcIfIndex;
  __u8 mac[8];
  __u8 nodeMac[8];
};

struct {
	__uint(type, BPF_MAP_TYPE_HASH);
  __uint(max_entries, 255);
	__type(key, struct endpointKey);
  __type(value, struct endpointInfo);
  // 如果别的地方已经往某条路径 pin 了, 需要加上这个属性
  // 并且 struct 的名字一定得和 bpftool map list 出来的一样
  __uint(pinning, LIBBPF_PIN_BY_NAME);
// 加了 SEC(".maps") 的话, clang 在编译时需要加 -g 参数用来生成调试信息
// 这里 ding_lxc 是必须要和 bpftool map list 出来的那个 pinned 中路径的名字一样
} ding_lxc __section_maps_btf;


struct podNodeKey {
  __u32 ip;
};

struct podNodeValue {
  __u32 ip;
};

struct {
	__uint(type, BPF_MAP_TYPE_HASH);
  __uint(max_entries, 255);
	__type(key, struct podNodeKey);
  __type(value, struct podNodeValue);
  __uint(pinning, LIBBPF_PIN_BY_NAME);
} ding_ip __section_maps_btf;

struct localNodeMapKey {
	__u32 type;
};

struct localNodeMapValue {
  __u32 ifIndex;
};

struct {
	__uint(type, BPF_MAP_TYPE_HASH);
  __uint(max_entries, 255);
	__type(key, struct localNodeMapKey);
  __type(value, struct localNodeMapValue);
  __uint(pinning, LIBBPF_PIN_BY_NAME);
} ding_local __section_maps_btf;

