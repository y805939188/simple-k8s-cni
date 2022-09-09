#include <linux/bpf.h>
#include <linux/pkt_cls.h>
#include <bpf/bpf_helpers.h>
#include <linux/if_ether.h>
#include <linux/ip.h>
#include <linux/icmp.h>

#include "common.h"

__section("classifier")
int cls_main(struct __sk_buff *skb) {
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

  bpf_printk("host skb_len: %d", skb->data_end - skb->data);
  // return bpf_redirect_peer(8, 0);

  // return bpf_redirect_neigh(16, NULL, 0, 0);
  return bpf_redirect(16, 0);
}

char _license[] SEC("license") = "GPL";
