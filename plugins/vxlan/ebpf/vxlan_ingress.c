#include <linux/bpf.h>
#include <linux/pkt_cls.h>
#include <bpf/bpf_helpers.h>
#include <linux/if_ether.h>

#include "common.h"

__section("classifier")
int cls_main(struct __sk_buff *skb) {
  
  bpf_printk("host1 vxlan skb_len: %d", skb->data_end - skb->data);

  /**
   * 在 vxlan 的 ingress 方向上收到包
   * 1. 先获取源 ip
   * 2. 根据 POD_MAP_DEFAULT_PATH 判断源 ip 是否是集群内的 ip
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

  __u8 new_dst_mac[ETH_ALEN];

  new_dst_mac[0]= 162;
  new_dst_mac[1]= 38;
  new_dst_mac[2]= 121;
  new_dst_mac[3]= 215;
  new_dst_mac[4]= 254;
  new_dst_mac[5]= 116;
  bpf_skb_store_bytes(
    skb,
    offsetof(struct ethhdr, h_dest),
    new_dst_mac,
    ETH_ALEN,
    0
  );
  __u8 new_src_mac[ETH_ALEN];
  new_src_mac[0]= 138;
  new_src_mac[1]= 125;
  new_src_mac[2]= 180;
  new_src_mac[3]= 99;
  new_src_mac[4]= 106;
  new_src_mac[5]= 173;
  bpf_skb_store_bytes(
    skb,
    offsetof(struct ethhdr, h_source),
    new_src_mac,
    ETH_ALEN,
    0
  );
  // return TC_ACT_OK;
  // 这里如果用 bpf_redirect_peer(4, 0) 的话
  // 在 ns 中抓包网卡就只能抓到 request 而无法抓到 reply
  // 不知道是为啥
  return bpf_redirect(4, 0);
}

char _license[] SEC("license") = "GPL";
