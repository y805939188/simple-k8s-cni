#include <linux/bpf.h>
#include <linux/pkt_cls.h>
#include <bpf/bpf_helpers.h>
#include <linux/if_ether.h>
#include <linux/ip.h>
#include <linux/if_arp.h>
#include <linux/if_ether.h>
#include <netinet/in.h>

#include "common.h"
#include "maps.h"

/**
 * 如果 vxlan 设备收到了数据包
 * 说明是要发送到其他 node 中不同网段的 pod 上
 * 1. 在 POD_MAP_DEFAULT_PATH 中查询目标 pod 所在的 node ip
 * 2. 用 bpf_skb_set_tunnel_key 给原始数据包设置外层的 udp 的 target ip
 * 
 */
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

  __u32 src_ip = htonl(ip->saddr);
	__u32 dst_ip = htonl(ip->daddr);
  // 获取目标 ip 所在的 node ip
  struct podNodeKey podNodeKey = {};
  podNodeKey.ip = dst_ip;
  struct podNodeValue *podNode = bpf_map_lookup_elem(&ding_ip, &podNodeKey);
  if (podNode) {
    __u32 dst_node_ip = podNode->ip;
    // 准备一个 tunnel
    struct bpf_tunnel_key key;
    int ret;
    __builtin_memset(&key, 0x0, sizeof(key));
    key.remote_ipv4 = podNode->ip;
    key.tunnel_id = DEFAULT_TUNNEL_ID;
    key.tunnel_tos = 0;
    key.tunnel_ttl = 64;
    // 添加外头的隧道 udp
    ret = bpf_skb_set_tunnel_key(skb, &key, sizeof(key), BPF_F_ZERO_CSUM_TX);
    if (ret < 0) {
      bpf_printk("bpf_skb_set_tunnel_key failed");
      return TC_ACT_SHOT;
    }
    return TC_ACT_OK;
  }
  return TC_ACT_OK;
}

char _license[] SEC("license") = "GPL";
