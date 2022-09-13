#include <linux/bpf.h>
#include <linux/pkt_cls.h>
#include <bpf/bpf_helpers.h>
#include <linux/if_ether.h>
#include <linux/ip.h>
#include <netinet/in.h>

#include "common.h"

#ifndef __section
# define __section(x)  __attribute__((section(x), used))
#endif

#ifndef __section_maps_btf
# define __section_maps_btf		__section(".maps")
#endif

// struct endpointInfo {
//   __u32 ifIndex;
//   __u16 lxcIfIndex;
//   __u16 _;
//   __u64 mac;
//   __u64 nodeMac;
// };

// struct endpointInfo {
//   __u32 ifIndex;
//   __u32 lxcIfIndex;
//   __u8 mac[8];
//   __u8 nodeMac[8];
// };

// struct endpointKey {
//   __u32 ip;
// };

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


// struct bpf_map_def ding_lxc = {
//   .type = BPF_MAP_TYPE_HASH,
//   .key_size = sizeof(struct endpointKey),
//   .value_size = sizeof(struct endpointInfo),
//   .max_entries = 65535,
//   .map_flags = 0,
// };
// struct {
// 	__uint(type, BPF_MAP_TYPE_HASH);
//   __uint(max_entries, 65535);
// 	__type(key, struct endpointKey);
//   __type(value, struct endpointInfo);
//   // 如果别的地方已经往某条路径 pin 了, 需要加上这个属性
//   // 并且 struct 的名字一定得和 bpftool map list 出来的 pinned 中的名字一样
//   __uint(pinning, LIBBPF_PIN_BY_NAME);
// // 加了 SEC(".maps") 的话, clang 在编译时需要加 -g 参数用来生成调试信息
// } ding_lxc __section_maps_btf;
#define LOCAL_DEV_VXLAN 1;
#define LOCAL_DEV_VETH 2;
__section("classifier")
int cls_main(struct __sk_buff *skb) {
	void *data = (void *)(long)skb->data;
	void *data_end = (void *)(long)skb->data_end;
	if (data + sizeof(struct ethhdr) + sizeof(struct iphdr) > data_end) {
    return TC_ACT_UNSPEC;
  }

	struct ethhdr  *eth  = data;
	struct iphdr   *ip   = (data + sizeof(struct ethhdr));
  if (eth->h_proto != __constant_htons(ETH_P_IP)) {
		return TC_ACT_UNSPEC;
  }

  // 在 go 那头儿往 ebpf 的 map 里存的时候我这个 arm 是按照小端序存的
  // 这里给转成网络的大端序
  __u32 src_ip = htonl(ip->saddr);
	__u32 dst_ip = htonl(ip->daddr);
  // 拿到 mac 地址
  __u8 src_mac[ETH_ALEN];
	__u8 dst_mac[ETH_ALEN];
  struct endpointKey epKey = {};
  epKey.ip = dst_ip;
  // 在 lxc 中查找
  struct endpointInfo *ep = bpf_map_lookup_elem(&ding_lxc, &epKey);
  if (ep) {
    // 如果能找到说明是要发往本机其他 pod 中的
    bpf_printk("ifIndex: %d", ep->ifIndex);
    bpf_printk("lxcIndex: %d", ep->lxcIfIndex);
    bpf_printk("mac: %d", ep->mac);
    bpf_printk("nodeMac: %d", ep->nodeMac);
    // 把 mac 地址改成目标 pod 的两对儿 veth 的 mac 地址
    bpf_memcpy(src_mac, ep->nodeMac, ETH_ALEN);
	  bpf_memcpy(dst_mac, ep->mac, ETH_ALEN);
    bpf_skb_store_bytes(skb, offsetof(struct ethhdr, h_source), dst_mac, ETH_ALEN, 0);
	  bpf_skb_store_bytes(skb, offsetof(struct ethhdr, h_dest), src_mac, ETH_ALEN, 0);
    return bpf_redirect_peer(ep->lxcIfIndex, 0);
  }
  bpf_printk("get value failed from ding_lxc, try to find from ding_ip");
  struct podNodeKey podNodeKey = {};
  podNodeKey.ip = dst_ip;
  struct podNodeValue *podNode = bpf_map_lookup_elem(&ding_ip, &podNodeKey);
  if (podNode) {
    // 进到这里说明该目标 ip 是本集群内的 ip
    bpf_printk("get value succeed from ding_ip, pod node ip: %d", podNode->ip);
    struct localNodeMapKey localKey = {};
    localKey.type = LOCAL_DEV_VXLAN;
    struct localNodeMapValue *localValue = bpf_map_lookup_elem(&ding_local, &localKey);
    
    if (localValue) {
      bpf_printk("get value succeed from ding_local, vxlan index: %d", localValue->ifIndex);
      // 转发给 vxlan 设备
      return bpf_redirect(localValue->ifIndex, 0);
    } 
    return TC_ACT_UNSPEC;
  }
  return TC_ACT_UNSPEC;
}

char _license[] SEC("license") = "GPL";
