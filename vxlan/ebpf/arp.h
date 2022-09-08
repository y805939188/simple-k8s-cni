#include <linux/bpf.h>
#include <linux/pkt_cls.h>
#include <bpf/bpf_helpers.h>
#include <linux/if_ether.h>
#include <linux/ip.h>
#include <linux/icmp.h>
#include <linux/if_arp.h>
#include <linux/if_ether.h>

#include "common.h"

#define IP_SRC_OFF (ETH_HLEN + offsetof(struct iphdr, saddr))
#define IP_DST_OFF (ETH_HLEN + offsetof(struct iphdr, daddr))
#define ICMP_CSUM_OFF (ETH_HLEN + sizeof(struct iphdr) + offsetof(struct icmphdr, checksum))
#define ICMP_TYPE_OFF (ETH_HLEN + sizeof(struct iphdr) + offsetof(struct icmphdr, type))
#define ICMP_CSUM_SIZE sizeof(__u16)
#define ICMP_PING 8



static __always_inline int eth_store_saddr_aligned(
  struct __sk_buff *skb,
	const __u8 *mac,
  int off
) {
	return bpf_skb_store_bytes(skb, off + ETH_ALEN, mac, ETH_ALEN, 0);
}

static __always_inline int eth_store_saddr(
  struct __sk_buff *skb,
	const __u8 *mac,
  int off
) {
  return eth_store_saddr_aligned(skb, mac, off);
}

static __always_inline int eth_store_daddr_aligned(
  struct __sk_buff *skb,
	const __u8 *mac,
  int off
) {
	return bpf_skb_store_bytes(skb, off, mac, ETH_ALEN, 0);
}

static __always_inline int eth_store_daddr(
  struct __sk_buff *skb,
	const __u8 *mac,
  int off
) {
  return eth_store_daddr_aligned(skb, mac, off);
}

union macaddr {
	struct {
		__u32 p1;
		__u16 p2;
	};
	__u8 addr[6];
};

# define __bpf_htons(x)		__builtin_bswap16(x)
#define bpf_htons(x)				\
	(__builtin_constant_p(x) ?		\
	 __constant_htons(x) : __bpf_htons(x))

static __always_inline int
arp_prepare_response(
  struct __sk_buff *skb,
  union macaddr *smac,
  __be32 sip,
	union macaddr *dmac,
  __be32 tip
) {
	__be16 arpop = bpf_htons(ARPOP_REPLY);

	if (
      eth_store_saddr(skb, smac->addr, 0) < 0 ||
	    eth_store_daddr(skb, dmac->addr, 0) < 0 ||
			// skb_store_bytes === bpf_skb_store_bytes
	    bpf_skb_store_bytes(skb, 20, &arpop, sizeof(arpop), 0) < 0 ||
	    /* sizeof(macadrr)=8 because of padding, use ETH_ALEN instead */
	    bpf_skb_store_bytes(skb, 22, smac, ETH_ALEN, 0) < 0 ||
	    bpf_skb_store_bytes(skb, 28, &sip, sizeof(sip), 0) < 0 ||
	    bpf_skb_store_bytes(skb, 32, dmac, ETH_ALEN, 0) < 0 ||
	    bpf_skb_store_bytes(skb, 38, &tip, sizeof(tip), 0) < 0)
		return -1;

	return 0;
}

struct arp_eth {
	unsigned char		ar_sha[ETH_ALEN];
	__be32                  ar_sip;
	unsigned char		ar_tha[ETH_ALEN];
	__be32                  ar_tip;
} __packed; // 设置了这个 packed 之后, 编译器不会默认自动内存对齐

// 该函数先判断 p1, p1 是 32 位, 就是先看是不是同一家生产商
// 如果是同一家生产商, 返回 mac 地址后两字节的差
static __always_inline int eth_addrcmp(
  const union macaddr *a,
	const union macaddr *b
) {
	int tmp;

	tmp = a->p1 - b->p1;
	if (!tmp)
		tmp = a->p2 - b->p2;

	return tmp;
}

static __always_inline int eth_is_bcast(const union macaddr *a) {
	union macaddr bcast;

	bcast.p1 = 0xffffffff;
	bcast.p2 = 0xffff;

	if (!eth_addrcmp(a, &bcast))
		return 1;
	else
		return 0;
}

/* Check if packet is ARP request for IP */
static __always_inline int arp_check(
  struct ethhdr *eth,
	const struct arphdr *arp,
	union macaddr *mac
) {
	union macaddr *dmac = (union macaddr *) &eth->h_dest;
  // 检查是不是 arp 请求
	return arp->ar_op  == bpf_htons(ARPOP_REQUEST) &&
    arp->ar_hrd == bpf_htons(ARPHRD_ETHER) &&
    // 先检查是不是广播或者看是不是同一块网卡
    (eth_is_bcast(dmac) || !eth_addrcmp(dmac, mac));
}

static __always_inline int
arp_validate(
  const struct __sk_buff *skb,
  union macaddr *mac,
	union macaddr *smac,
  __u32 *sip,
  __u32 *tip
) {
	void *data_end = (void *) (long) skb->data_end;
	void *data = (void *) (long) skb->data;
	struct arphdr *arp = data + ETH_HLEN;
	struct ethhdr *eth = data;
	struct arp_eth *arp_eth;
  int size = data_end - data;

  /**
   * arp 帧:
   *    以太网帧头 (目的 mac 6 字节 + 源 mac 6 字节 + 帧类型 2 字节)
   *  + ARP 报头 (硬件类型 2 字节 + 上层协议类型 2 字节 + mac 地址长度 1 字节 + ip 地址长度 1 字节 + 操作类型 2 字节)
   *  + 源 mac 6 字节 + 源 ip 4 字节 + 目的 mac 6 字节 + 目的 ip 4 字节
   */
	if (data + ETH_HLEN + sizeof(*arp) + sizeof(*arp_eth) > data_end) {
    bpf_printk("check arp length failed");
    return -1;
  }

	if (arp_check(eth, arp, mac) < 0) {
    bpf_printk("check arp failed");
    return -1;
  }

	arp_eth = data + ETH_HLEN + sizeof(*arp);
	*smac = *(union macaddr *) &eth->h_source;
	*sip = arp_eth->ar_sip;
	*tip = arp_eth->ar_tip;

	return 0;
}

struct vtep_value {
	__u64 vtep_mac;
	__u32 tunnel_endpoint;
};

#ifndef NODE_MAC
#define NODE_MAC_1 (0xde) << 24 | (0xad) << 16 | (0xbe) << 8 | (0xef)
#define NODE_MAC_2 (0xc0) << 8 | (0xde)
#define NODE_MAC { { NODE_MAC_1, NODE_MAC_2 } }
#endif
int tail_handle_arp(struct __sk_buff *skb) {
	union macaddr mac = NODE_MAC;
	union macaddr smac;
	int ret;
	__u32 sip;
	__u32 tip;
  struct vtep_value *info;

  if (arp_validate(skb, &mac, &smac, &sip, &tip) < 0) {
    bpf_printk("arp_validate was failed");
    return -1;
  }

  // 这里还要有个检查对端 ip 的操作
  // if (!__lookup_ip4_endpoint(tip)) {
  //   bpf_printk("lookup ip4 endpoint was failed");
  //   return -1;
  // }

  // 用一个假的 mac 地址作为源 mac, 目标 ip 也就是另一个节点上的 pod ip 作为源 ip
  // 以及用源 mac 作为目标 mac, 源 ip 也就是发出 arp request 的 ip 作为目的 ip
  // 相当于问的时候是问 “macA > macB, ipA > ipB, who has ipB tell ipA”
  // 这里响应的时候是 “dummyA > macA, ipB > ipA, ipB 的 mac 是 dummyA(因为 request 的时候 macB 一定是 fff)”

	ret = arp_prepare_response(skb, &mac, tip, &smac, sip);
  if (ret < 0) {
    bpf_printk("arp_prepare_response failed");
  }
  return ret;
}
