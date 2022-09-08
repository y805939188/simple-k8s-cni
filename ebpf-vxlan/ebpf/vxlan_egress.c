#include <linux/bpf.h>
#include <linux/pkt_cls.h>
#include <bpf/bpf_helpers.h>
#include <linux/if_ether.h>
#include <linux/ip.h>
#include <linux/icmp.h>
#include <linux/if_arp.h>
#include <linux/if_ether.h>

#include "common.h"

__section("classifier")
int cls_main(struct __sk_buff *skb) {

  bpf_printk("host vxlan_len: %d", skb->data_end - skb->data);

  /**
   * 如果 vxlan 设备收到了数据包
   * 说明是要发送到其他 node 中不同网段的 pod 上
   * 1. 在 POD_MAP_DEFAULT_PATH 中查询目标 pod 所在的 node ip
   * 2. 用 bpf_skb_set_tunnel_key 给原始数据包设置外层的 udp 的 target ip
   * 
   */

  struct bpf_tunnel_key key;
	int ret;

	__builtin_memset(&key, 0x0, sizeof(key));
	// key.remote_ipv4 = 0xac100164; /* 172.16.1.100 */
  key.remote_ipv4 = 0xc0a84010; /* 192.168.64.16 */
	key.tunnel_id = 2;
	key.tunnel_tos = 0;
	key.tunnel_ttl = 64;

	ret = bpf_skb_set_tunnel_key(skb, &key, sizeof(key), BPF_F_ZERO_CSUM_TX);
	if (ret < 0) {
		bpf_printk("bpf_skb_set_tunnel_key failed");
		return TC_ACT_SHOT;
	}
  bpf_printk("vxlan_length: %d", skb->data_end - skb->data);
  return TC_ACT_OK;
}

char _license[] SEC("license") = "GPL";
