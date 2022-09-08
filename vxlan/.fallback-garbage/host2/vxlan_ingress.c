#include <linux/bpf.h>
#include <linux/pkt_cls.h>
#include <bpf/bpf_helpers.h>
#include <linux/if_ether.h>

#ifndef __section
# define __section(x)  __attribute__((section(x), used))
#endif

/**
 * https://man7.org/linux/man-pages/man8/tc-bpf.8.html
 * 
 * 编译 ebpf
 * clang -O2 -emit-llvm -c test.c -o - | llc -march=bpf -filetype=obj -o test.o
 * 
 * 给 test-veth1 创建一个 clsact 类型的大队列
 * tc qdisc add dev veth1 clsact
 * (
 *  clsact 与 ingress qdisc 类似
 *  能够以 direct-action 模式 attach eBPF 程序
 *  其特点是不会执行任何排队
 *  clsact 是 ingress 的超集
 *  因为它还支持在 egress 上以 direct-action 模式 attach eBPF 程序
 * )
 * 
 * 把 test.o 以 da 的方式挂到 veth1 的 ingress 口上
 * tc filter add dev veth1 ingress bpf direct-action obj test.o
 * 
 * 删除 clsact 这个大队列
 * tc qdisc del dev veth1 clsact
 * 
 * 查看 test-veth1 ingress 方向的 ebpf
 * tc filter show dev veth1 ingress
 * 
 * 查看缓冲区
 * cat /sys/kernel/debug/tracing/trace_pipe
 */

__section("classifier")
int cls_main(struct __sk_buff *skb) {
  
  bpf_printk("host2 vxlan skb_len: %d", skb->data_end - skb->data);

  __u8 new_dst_mac[ETH_ALEN];
  // veth2:
  // [162, 38, 121, 215, 254, 116]
  new_dst_mac[0]= 130;
  new_dst_mac[1]= 246;
  new_dst_mac[2]= 157;
  new_dst_mac[3]= 169;
  new_dst_mac[4]= 176;
  new_dst_mac[5]= 16;
  bpf_skb_store_bytes(
    skb,
    offsetof(struct ethhdr, h_dest),
    new_dst_mac,
    ETH_ALEN,
    0
  );
  __u8 new_src_mac[ETH_ALEN];
  new_src_mac[0]= 182;
  new_src_mac[1]= 133;
  new_src_mac[2]= 88;
  new_src_mac[3]= 163;
  new_src_mac[4]= 103;
  new_src_mac[5]= 123;
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
  // return bpf_redirect_peer(4, 0);
  return bpf_redirect(4, 0);
}

char _license[] SEC("license") = "GPL";
