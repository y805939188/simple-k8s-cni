// #include "vmlinux.h"
#include <linux/bpf.h>
#include <linux/pkt_cls.h>
#include <bpf/bpf_helpers.h>

#ifndef __section
# define __section(x)  __attribute__((section(x), used))
#endif

#ifndef __section_maps_btf
# define __section_maps_btf		__section(".maps")
#endif

struct endpointInfo {
  __u32 ifIndex;
  __u16 lxcID;
  __u16 _;
  __u64 mac;
  __u64 nodeMac;
};

struct endpointKey {
  __u32 ip;
};

// struct bpf_map_def ding_lxc = {
//   .type = BPF_MAP_TYPE_HASH,
//   .key_size = sizeof(struct endpointKey),
//   .value_size = sizeof(struct endpointInfo),
//   .max_entries = 65535,
//   .map_flags = 0,
// };
struct {
	__uint(type, BPF_MAP_TYPE_HASH);
  __uint(max_entries, 65535);
	__type(key, struct endpointKey);
  __type(value, struct endpointInfo);
  // 如果别的地方已经往某条路径 pin 了, 需要加上这个属性
  // 并且 struct 的名字一定得和 bpftool map list 出来的一样
  __uint(pinning, LIBBPF_PIN_BY_NAME);
// 加了 SEC(".maps") 的话, clang 在编译时需要加 -g 参数用来生成调试信息
} ding_lxc __section_maps_btf;

__section("classifier")
int cls_main(struct __sk_buff *skb) {
  bpf_printk("key size: %d", sizeof(struct endpointKey));
  bpf_printk("value size: %d", sizeof(struct endpointInfo));
  struct endpointKey epKey = {};
  epKey.ip = 6;

  struct endpointInfo *ep = bpf_map_lookup_elem(&ding_lxc, &epKey);
  // bpf_printk("33333: %d", ep);
  if (!ep) {
    bpf_printk("get value failed");
  } else {
    bpf_printk("value ifIndex: %d; nodeMac: %d", ep->ifIndex, ep->nodeMac);
  }
  return TC_ACT_OK;
}

char _license[] SEC("license") = "GPL";
