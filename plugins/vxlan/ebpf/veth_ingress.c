#include <linux/bpf.h>
#include <linux/pkt_cls.h>
#include <bpf/bpf_helpers.h>
#include <linux/if_ether.h>
#include <linux/ip.h>
#include <linux/icmp.h>
#include <netinet/in.h>

#include "common.h"
#include "maps.h"

/**
 * 这里首先从 skb 里看是啥协议
 * 支持的协议: ICMP/UDP/TCP
 * 需要分别处理
 * 
 * 然后从 ip 头中把目标 ip 给捞出来
 * 该 ip 有:
 *  1. 访问本机 pod
 *  2. 访问其他 node 的 pod
 *  3. 访问本地某个进程
 *  4. 外网
 *  5. cilium 还有个访问 lb 的情况
 * 
 * 当前暂只处理 1 和 2 的情况
 * 1.
 *  a. 获取 dst ip
 *  b. 从 POD_MAP_DEFAULT_PATH 中查找 dst ip
 *    b1. 没找到的话说明 dst ip 不是当前集群的 pod ip
 *        直接丢弃
 *    b2. 找到了说明是本集群内的 pod 的 ip
 *      b2-1. 用该 ip 对应的 node ip 在 NODE_LOCAL_MAP_DEFAULT_PATH 中查找
 *        b2-1-1. 如果找到对应的 value 说明目标 ip 就是本机的 pod
 *          b2-1-1-2. 本机 pod 时, 在 LXC_MAP_DEFAULT_PATH 中查找
 *                    根据 ip 找到对应的 mac 和 node mac 后
 *                    把 nodeMac 作为 srouce mac, mac 作为 target mac
 *                    然后重定向到目标 pod 留在 host 上的那半拉 veth
 *                    使用 bpf_redirect_peer 能直接给发到 ns 下
 * 2. 
 *  a. 如果没找到对应的 value 说明目标 ip 是集群内其他 node 的 pod
 *  b. 访问跨节点的 pod ip 需要将流量包给重定向到本机 vxlan 设备上
 *     vxlan 的设备 ifindex 从 NODE_LOCAL_MAP_DEFAULT_PATH 中获取
 *     使用 bpf_redirect 重定向过去
 *     (
 *      这里有个问题,
 *      按照 cilium 官方博客的说法
 *      是使用 bpf_redirect_neigh 给定过去
 *      中间还走了个二层
 *      走二层的话大概率 vxlan 还得先发个 arp 去找 remote pod 的 ip 的 mac 地址
 *      但是在 cilium 集群中抓包貌似并没有这步
 *      不确定他们内部到底是不是用的 bpf_redirect_neigh
 *      如果真用的 bpf_redirect_neigh,
 *      不确定他们是怎么处理的 arp response
 *     )
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
    // 把 mac 地址改成目标 pod 的两对儿 veth 的 mac 地址
    bpf_memcpy(src_mac, ep->nodeMac, ETH_ALEN);
	  bpf_memcpy(dst_mac, ep->mac, ETH_ALEN);
    bpf_skb_store_bytes(skb, offsetof(struct ethhdr, h_source), dst_mac, ETH_ALEN, 0);
	  bpf_skb_store_bytes(skb, offsetof(struct ethhdr, h_dest), src_mac, ETH_ALEN, 0);
    return bpf_redirect_peer(ep->lxcIfIndex, 0);
  }
  struct podNodeKey podNodeKey = {};
  podNodeKey.ip = dst_ip;
  struct podNodeValue *podNode = bpf_map_lookup_elem(&ding_ip, &podNodeKey);
  if (podNode) {
    // 进到这里说明该目标 ip 是本集群内的 ip
    struct localNodeMapKey localKey = {};
    localKey.type = LOCAL_DEV_VXLAN;
    struct localNodeMapValue *localValue = bpf_map_lookup_elem(&ding_local, &localKey);
    
    if (localValue) {
      // 转发给 vxlan 设备
      return bpf_redirect(localValue->ifIndex, 0);
    } 
    return TC_ACT_UNSPEC;
  }
  return TC_ACT_UNSPEC;
}

char _license[] SEC("license") = "GPL";
