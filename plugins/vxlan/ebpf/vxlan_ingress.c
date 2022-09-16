#include <linux/bpf.h>
#include <linux/pkt_cls.h>
#include <bpf/bpf_helpers.h>
#include <linux/if_ether.h>
#include <linux/ip.h>
#include <netinet/in.h>

#include "common.h"
#include "maps.h"
/**
 * 在 vxlan 的 ingress 方向上收到包
 * 1. 先获取源 ip
 * 2. 根据 LXC_MAP_DEFAULT_PATH 判断源 ip 是否是本机 pod ip
 *  a. 不是集群内的 pod ip, 可能是不知道哪个东西发过来的 tunnel 包, 直接返回 TC_ACT_OK 给放掉
 *  b. 是集群内的 pod ip, 此时在 LXC_MAP_DEFAULT_PATH 中
 *     根据目标 ip 查询 veth 的 mac 地址
 *    b-1. 替换掉 skb 中的源 mac 和目标 mac
 *    b-2. 通过 bpf_redirect 给发送到目标 pod 的 veth 上
 *         (
 *           这里有个问题, 按照官方的说明, 此时重定向时
 *           用的应该是 bpf_redirect_peer 直接给数据包
 *           干到了 ns 里的那半拉 veth 上
 *           但是在我自己测试环境中, 使用 peer 的话
 *           确实可以把数据给直接重定向到 ns 内部
 *           在 pod 内部抓包也能看到 request
 *           但是奇怪的是网卡并不返回 reply
 *           换成 bpf_redirect 就好了
 *           该开的内核参数都已经开了
 *           不知道为啥, 百思不得其姐姐
 *         )
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
  bpf_printk("the dst_ip is: %d", dst_ip);
  bpf_printk("the ip->daddr is: %d", ip->daddr);

  // 拿到目标 ip
  struct endpointKey epKey = {};
  epKey.ip = dst_ip;
  // 在本地的 lxc map 中查找
  struct endpointInfo *ep = bpf_map_lookup_elem(&ding_lxc, &epKey);
  if (!ep) {
    // 如果没找到的话直接放到
    return TC_ACT_OK;
  }
  // 找到的话说明是发往本机 pod 中的流量
  // 此时需要做 stc mac 和 dst mac 的更新

  // 拿到 mac 地址
  __u8 src_mac[ETH_ALEN];
	__u8 dst_mac[ETH_ALEN];
  // 将 mac 改成本机 pod 的那对儿 veth pair 的 mac
  bpf_memcpy(src_mac, ep->nodeMac, ETH_ALEN);
  bpf_memcpy(dst_mac, ep->mac, ETH_ALEN);
  // 将 mac 更新到 skb 中
  bpf_skb_store_bytes(
    skb,
    offsetof(struct ethhdr, h_dest),
    dst_mac,
    ETH_ALEN,
    0
  );
    bpf_skb_store_bytes(
    skb,
    offsetof(struct ethhdr, h_source),
    src_mac,
    ETH_ALEN,
    0
  );
 
  return bpf_redirect(ep->lxcIfIndex, 0);
}

char _license[] SEC("license") = "GPL";
