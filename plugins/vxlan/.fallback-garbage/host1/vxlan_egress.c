#include <linux/bpf.h>
#include <linux/pkt_cls.h>
#include <bpf/bpf_helpers.h>
#include <linux/if_ether.h>
#include <linux/ip.h>
#include <linux/icmp.h>
#include <linux/if_arp.h>
#include <linux/if_ether.h>

#include "if_arp.h"

#define IP_SRC_OFF (ETH_HLEN + offsetof(struct iphdr, saddr))
#define IP_DST_OFF (ETH_HLEN + offsetof(struct iphdr, daddr))
#define ICMP_CSUM_OFF (ETH_HLEN + sizeof(struct iphdr) + offsetof(struct icmphdr, checksum))
#define ICMP_TYPE_OFF (ETH_HLEN + sizeof(struct iphdr) + offsetof(struct icmphdr, type))
#define ICMP_CSUM_SIZE sizeof(__u16)
#define ICMP_PING 8

#define bpf_memcpy __builtin_memcpy

#define trace_printk(fmt, ...) do { \
	char _fmt[] = fmt; \
	bpf_trace_printk(_fmt, sizeof(_fmt), ##__VA_ARGS__); \
	} while (0)

// #define MAC_ARG(x) ((__u8*)(x))[0],((__u8*)(x))[1],((__u8*)(x))[2],((__u8*)(x))[3],((__u8*)(x))[4],((__u8*)(x))[5]

// #include "skb.h"

/**
 * https://docs.cilium.io/en/latest/bpf/#tc-traffic-control
 * 
 * tc:
 *  queueing discipline (qdisc)：排队规则，根据某种算法完成限速、整形等功能
 *  class：用户定义的流量类别
 *  classifier (也称为 filter)：分类器，分类规则
 *  action：要对包执行什么动作
 * 
 *  说白了就是:
 *    可以给某个网络设备创建一条 qdisc, 相当于一条总的 queue
 *    然后还可以给某个网络设备创建一堆 class(约等于 “管儿”)
 *    这些“管儿”可以由用户定义一些“规则”(比如这个管儿中的包速率应该是多少 等)
 *    之后给某个设备创建一堆 filter(classifier)
 *    这些 filter 可以按照一定的规则(比如目标 ip, 源 ip 等)
 *    根据这些规则把不同的包丢到不同的 “管儿” 中
 *    在丢过去的同时还可以选择一些 action(比如直接丢弃, 或者重定向 等)
 * 
 * x:y 格式：
 * * x 表示 qdisc, y 表示这个 qdisc 内的某个 class
 * * 1: 是 1:0 的简写
 * 任何没有被分配到某个 “管儿” 中的流量将被分到默认的 11 号 class
 * $ tc qdisc add dev eth0 root handle 1: htb default 11
 * $ tc class add dev eth0 parent 1: classid 1:1 htb rate 100kbps ceil 100kbps
 * $ tc class add dev eth0 parent 1:1 classid 1:10 htb rate 30kbps ceil 100kbps
 * $ tc class add dev eth0 parent 1:1 classid 1:11 htb rate 10kbps ceil 100kbps
 * 
 * $ tc filter add dev eth0 protocol ip parent 1:0 prio 1 u32 \
 *   match ip src 1.2.3.4 match ip dport 80 0xffff flowid 1:10
 * $ tc filter add dev eth0 protocol ip parent 1:0 prio 1 u32 \
 *   match ip src 1.2.3.4 action drop
 * 
 * 
 * 在 tc-ebpf 的场景下, 内核支持两种类型的 ebpf
 * 一种是 classifier, 也就是说 ebpf 是 filter(分类器) 类型的程序
 * 此时 ebpf 程序的返回值表示
 *  0: 不匹配任何分类器
 *  -1: 表示匹配上当前 filter 的默认 class
 *  其他: 表示匹配上了其他的 class
 * 
 * 另一种是 action 类型
 * 此时 ebpf 程序的返回值表示要出发该包的 action
 * 
 * 内核当前支持这些 action: 
 * 
 * // 使用 tc 的默认 action（与 classifier/filter 返回 -1 时类似）
 * // 一般用作某个 tc bpf 程序被卸载了然后又被加上去了
 * // 这个时候一般返回这个 UNSPEC
 * // 还有就是多个 tc bpf 的场景, 为了让下一个程序去运行这个数据包
 * // 还用做告诉内核, 这个 skb 不会对他做什么骚操作
 * // 他也会把 skb 传给上层的协议栈
 * // 和 OK 的区别在于 OK 由 bpf 程序主动使用 classid 给 skb->tc_index 设置
 * // 而 UNSPEC 是直接用 skb 自己的 tc_classid
 * #define TC_ACT_UNSPEC	(-1)
 * // 结束处理过程 放行
 * #define TC_ACT_OK		0
 * // 从头开始 重新执行分类过程 约等于 TC_ACT_OK
 * #define TC_ACT_RECLASSIFY	1
 * // 丢弃包
 * #define TC_ACT_SHOT		2
 * #define TC_ACT_PIPE		3 // 如果有下一个 action 执行之
 * // STOLEN 和 SHOT 很像
 * // 区别在于 SHOT 释放 skb 是通过 kfree_skb
 * // 而 STOLEN 是通过 consume_skb 释放
 * // (注: 上边那俩函数是内核中唯二用于释放 skb 的俩函数)
 * // SHOT 的语义是说 skb 直接被我明确地扔了
 * // STOLEN 的语义是说 skb 被使用了, 但是现在去哪儿了不知道
 * #define TC_ACT_STOLEN		4
 * // 4,5,6 基本约等于
 * #define TC_ACT_QUEUED		5
 * #define TC_ACT_REPEAT		6
 * // 与 bpf_redirect() 方法配合着用
 * // 可以把数据包转发到任何一个网络设备上
 * // 该设备没有任何要求, 也不要求该设备上有任何的 bpf 程序
 * #define TC_ACT_REDIRECT		7
 * #define TC_ACT_TRAP		8
 * 
 * 注:
 * classifer 能对包进行匹配 但返回的 classid 只能告诉系统
 * 接下来把这个包送到哪个 class（队列）
 * 无法让系统对这个包执行动作（drop、allow、mirror 等）
 * 
 * action 返回的是动作 告诉系统接下来要对这个包做什么（drop、allow、mirror 等）
 * 但它无法对包进行分类（规则匹配）
 * 
 * 所以 如果要实现”匹配+执行动作“的目的
 * 例如 如果源 IP 是 10.1.1.1 则 drop 这个包
 * 如此的话需要两个步骤
 *  一个 classifier 和一个 action
 *  即 classfifier+action 模式
 * 
 * 所以社区引入了一个新的 flag: direct-action(简写 da)
 * 告诉系统: filter（classifier）的返回值应当被解读为 action
 * 即前面提到的 TC_ACT_XXX, 本来的话, 应当被解读为 classid
 * 
 * 这意味着一个作为 tc classifier 加载的 eBPF 程序
 * 现在可以返回 TC_ACT_SHOT, TC_ACT_OK 等 tc action 的返回值
 * 换句话说 现在不需要另一个专门的 tc action 对象来 drop 或 mirror 相应的包了
 * TC 子系统也就无需再将流量给调度到其他的 action 模块
 * 
 * 
 * 另外程序仍然可以获取到 classId
 * 传递给 filter 程序的参数是 struct __skb_buff\
 * 其中有个 tc_classid 字段
 * 存储的就是返回的 classId
 * 
 * 
 */

#ifndef __section
# define __section(x)  __attribute__((section(x), used))
#endif

/**
 * http://arthurchiao.art/blog/differentiate-bpf-redirects/
 * 
 * 关于重定向:
 *  1. bpf_redirect_peer()
 *  2. bpf_redirect_neighbor()
 *  3. bpf_redirect()
 * 
 * 其中和 bpf_redirect 类似的还有个 bpf_clone_redirect
 * 区别在于, bpf_redirect 性能高, 不需要 clone skb
 * 另外 bpf_clone_redirect 的重定向是在该函数被调用的时候同时发生的
 * 而 bpf_redirect 的重定向发生在该函数被调用结束之后
 * eBPF 程序执行完之后 tc 会执行一个 tc_classify 方法
 * 如果 tc_classify 检测到 eBPF 函数返回了 TC_ACT_REDIRECT
 * 在这个方法中就会真正执行 skb_do_redirect
 * 这个函数里边直接:
 *    (
 *      // 如果发现是从 ingress 口子进来的话就走转发
 *      BPF_IS_REDIRECT_INGRESS(ri->flags) ?
 *        dev_forward_skb(dev, skb) :
 *        // 从 egress 进来的就走发送
 *        (
 *          (skb->dev = dev) && dev_queue_xmit(skb)
 *        )
 *    )
 * 但是 bpf_redirect 只能在 eBPF 程序之内
 * 
 * 
 * 
 * bpf_redirect_neighbor() 方法和 bpf_redirect 相比
 * 前者只支持 egress 方向上, 后者俩方向都支持
 * 他俩都不能直接穿过 netns
 * 另外前者会用 kernel 的 stack 填充 L2 的 mac 地址
 * 说白了就是 bpf_redirect 是直接发送了
 * 而 bpf_redirect_neighbor 是先发送到邻居子系统
 * 然后邻居子系统处理完才往下走发送
 * 前者的 flag 参数必须为 0, 且该方法只能在 eBPF 程序中使用
 * 
 * 
 * 
 * bpf_redirect_peer()
 * 该方法与 bpf_redirect 相比, 只会在 ingress 方向被触发
 * 并且它可以直接穿越 netns, bpf_redirect 不能
 * 它的 flag 参数必须是 0, 目前只能在 eBPF 中使用
 * 并且必须是得穿越一个 netns
 */

enum {
	CB_SRC_LABEL, // 0
	CB_IFINDEX, // 1
};

#ifndef __packed
# define __packed		__attribute__((packed))
#endif

#ifndef __maybe_unused
# define __maybe_unused		__attribute__((__unused__))
#endif

static __always_inline __maybe_unused __u32
skb_get_ifindex(const struct __sk_buff *skb) {
	return skb->ifindex;
}

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

static __always_inline int eth_is_bcast(const union macaddr *a)
{
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

  // info->vtep_mac = "";

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

  // bpf_printk("vxlan-egress!!!: src_mac: ");
  // bpf_printk("1: %d",smac.addr[0]);
  // bpf_printk("2: %d",smac.addr[1]);
  // bpf_printk("3: %d",smac.addr[2]);
  // bpf_printk("4: %d",smac.addr[3]);
  // bpf_printk("5: %d",smac.addr[4]);
  // bpf_printk("6: %d",smac.addr[5]);

  // bpf_printk("vxlan-egress!!!: mac: ");
  // bpf_printk("1: %d",mac.addr[0]);
  // bpf_printk("2: %d",mac.addr[1]);
  // bpf_printk("3: %d",mac.addr[2]);
  // bpf_printk("4: %d",mac.addr[3]);
  // bpf_printk("5: %d",mac.addr[4]);
  // bpf_printk("6: %d",mac.addr[5]);

  // bpf_printk("tip: %d", tip);
  // bpf_printk("sip: %d", sip);
	ret = arp_prepare_response(skb, &mac, tip, &smac, sip);
  if (ret < 0) {
    bpf_printk("arp_prepare_response failed");
  }
  return ret;
}

__section("classifier")
int cls_main(struct __sk_buff *skb) {

  bpf_printk("host vxlan_len: %d", skb->data_end - skb->data);
  // if (tail_handle_arp(skb) < -1) {
  //   bpf_printk("tail_handle_arp error");
  //   return TC_ACT_SHOT;
  // }
  // bpf_redirect(skb->ifindex, 0);
  // bpf_clone_redirect(skb, skb->ifindex, 0);
  // return TC_ACT_OK;

  // bpf_printk("vxlan_length: %d", skb->data_end - skb->data);

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

  // bpf_printk(xxx) 要通过 “cat  /sys/kernel/debug/tracing/trace_pipe” 查看
  // bpf_printk("ingress!!!%d", CB_IFINDEX);
  // skb->cb[CB_IFINDEX];
  // int ifindex = skb_load_meta(skb, CB_IFINDEX);
  // bpf_printk("the ifindex is: %d", ifindex);
  // return TC_ACT_OK;

  /**
   * 
   * ns1 veth1:
   *  mac 8a:7d:b4:63:6a:ad
   *  intMac [138, 125, 180, 99, 106, 173]
   *  ifindex: 4
   * 
   * ns1 veth2:
   *  ip 10.0.0.2
   *  mac a2:26:79:d7:fe:74
   *  intMac [162, 38, 121, 215, 254, 116]
   *  ifindex: 3
   * 
   * ns2 veth3:
   *  mac fa:c7:d4:a2:53:c6
   *  intMac [250, 199, 212, 162, 83, 198]
   *  ifindex: 8
   * 
   * ns2 veth4:
   *  ip 10.0.0.4
   *  mac e2:fe:71:ed:db:61
   *  intMac [226, 254, 113, 237, 219, 97]
   *  ifindex: 7
   */

  // void *data = (void *)(long)skb->data;
	// void *data_end = (void *)(long)skb->data_end;

  // if (
  //   data + sizeof(struct ethhdr) +
  //   sizeof(struct iphdr) +
  //   sizeof(struct icmphdr) > data_end
  // ) {
  //   return TC_ACT_UNSPEC;
  // }

  // struct ethhdr  *eth  = data;
	// struct iphdr   *ip   = (data + sizeof(struct ethhdr));
	// struct icmphdr *icmp = (data + sizeof(struct ethhdr) + sizeof(struct iphdr));
  
  // __u8 src_mac[ETH_ALEN];
	// __u8 dst_mac[ETH_ALEN];
	// bpf_memcpy(src_mac, eth->h_source, ETH_ALEN);
	// bpf_memcpy(dst_mac, eth->h_dest, ETH_ALEN);
  // // __u32 src_ip = ip->saddr;
	// // __u32 dst_ip = ip->daddr;
  // // __u32 src_ip = 239118528;
	// // __u32 dst_ip = 34209802;

  // __u32 src_ip = 239118528;
	// __u32 dst_ip = 34209802;

  // __u16 arpop = bpf_htons(ARPOP_REPLY);

  // bpf_printk("vxlan src_ip: %d, %d", src_ip, &src_ip);
  // bpf_printk("vxlan dst_ip: %d, %d", dst_ip, &dst_ip);
  
  // if (
  //   bpf_skb_store_bytes(skb, 20, &arpop, sizeof(arpop), 0) < 0 ||
  //   bpf_skb_store_bytes(skb, offsetof(struct ethhdr, h_source), dst_mac, ETH_ALEN, 0) < 0 ||
  //   bpf_skb_store_bytes(skb, offsetof(struct ethhdr, h_dest), src_mac, ETH_ALEN, 0) < 0 ||
  //   bpf_skb_store_bytes(skb, IP_SRC_OFF, &dst_ip, sizeof(dst_ip), 0) < 0 ||
  //   bpf_skb_store_bytes(skb, IP_DST_OFF, &src_ip, sizeof(src_ip), 0)
  // ) {
  //   bpf_printk("ingress!!! error !!!");
  // }
  
  // // /* Swap the MAC addresses */
	// // bpf_skb_store_bytes(skb, offsetof(struct ethhdr, h_source), dst_mac, ETH_ALEN, 0);
	// // bpf_skb_store_bytes(skb, offsetof(struct ethhdr, h_dest), src_mac, ETH_ALEN, 0);

	// // /* Swap the IP addresses.
	// //  * IP contains a checksum, but just swapping bytes does not change it.
	// //  * so no need to recalculate */
	// // bpf_skb_store_bytes(skb, IP_SRC_OFF, &dst_ip, sizeof(dst_ip), 0);
	// // bpf_skb_store_bytes(skb, IP_DST_OFF, &src_ip, sizeof(src_ip), 0);

  
  // // if (
  // //   bpf_skb_store_bytes(skb, 20, &arpop, sizeof(arpop), 0) < 0 ||
  // //   bpf_skb_store_bytes(skb, 22, dst_mac, ETH_ALEN, 0) < 0 ||
  // //   bpf_skb_store_bytes(skb, 28, &dst_ip, sizeof(dst_ip), 0) < 0 ||
  // //   bpf_skb_store_bytes(skb, 32, src_mac, ETH_ALEN, 0) < 0 ||
  // //   bpf_skb_store_bytes(skb, 38, &src_ip, sizeof(src_ip), 0) < 0
  // // ) {
  // //   bpf_printk("ingress!!! error !!!");
  // // }

  // return TC_ACT_OK;

  // // /* Swap the MAC addresses */
	// // bpf_skb_store_bytes(skb, offsetof(struct ethhdr, h_source), dst_mac, ETH_ALEN, 0);
	// // bpf_skb_store_bytes(skb, offsetof(struct ethhdr, h_dest), src_mac, ETH_ALEN, 0);

	// // /* Swap the IP addresses.
	// //  * IP contains a checksum, but just swapping bytes does not change it.
	// //  * so no need to recalculate */
	// // bpf_skb_store_bytes(skb, IP_SRC_OFF, &dst_ip, sizeof(dst_ip), 0);
	// // bpf_skb_store_bytes(skb, IP_DST_OFF, &src_ip, sizeof(src_ip), 0);

	// // /* Change the type of the ICMP packet to 0 (ICMP Echo Reply).
	// //  * This changes the data, so we need to re-calculate the checksum
	// //  */
	// // __u8 new_type = 0;
	// // /* We need to pass the full size of the checksum here (2 bytes) */
	// // bpf_l4_csum_replace(skb, ICMP_CSUM_OFF, ICMP_PING, new_type, ICMP_CSUM_SIZE);
	// // bpf_skb_store_bytes(skb, ICMP_TYPE_OFF, &new_type, sizeof(new_type), 0);

	// // /* Now redirecting the modified skb on the same interface to be transmitted again */
	// // bpf_clone_redirect(skb, skb->ifindex, 0);
  // // bpf_printk("vxlan-egress!!!: src_mac: ");

	// /* We modified the packet and redirected it, it can be dropped here */
	// // return TC_ACT_SHOT;
  // // return TC_ACT_OK;
  // // // bpf_printk(
  // // //   "1: %d, 2: %d, 3: %d, 4: %d, 5: %d",
  // // //   src_mac[0],src_mac[1],src_mac[2],src_mac[3],src_mac[4]
  // // // );
  // // bpf_printk("vxlan-egress!!!: src_mac: ");
  // // bpf_printk("1: %d",src_mac[0]);
  // // bpf_printk("2: %d",src_mac[1]);
  // // bpf_printk("3: %d",src_mac[2]);
  // // bpf_printk("4: %d",src_mac[3]);
  // // bpf_printk("5: %d",src_mac[4]);
  // // bpf_printk("6: %d",src_mac[5]);

  // // bpf_printk("vxlan-egress!!!: dst_mac: ");
  // // bpf_printk("1: %d",dst_mac[0]);
  // // bpf_printk("2: %d",dst_mac[1]);
  // // bpf_printk("3: %d",dst_mac[2]);
  // // bpf_printk("4: %d",dst_mac[3]);
  // // bpf_printk("5: %d",dst_mac[4]);
  // // bpf_printk("6: %d",dst_mac[5]);

  // // bpf_printk("vxlan-egress!!!: src_ip: %d", src_ip);
  // // bpf_printk("vxlan-egress!!!: dst_ip: %d", dst_ip);

	// // __u8 new_dst_mac[ETH_ALEN];
  // // new_dst_mac[0]= 162;
  // // new_dst_mac[1]= 38;
  // // new_dst_mac[2]= 121;
  // // new_dst_mac[3]= 215;
  // // new_dst_mac[4]= 254;
  // // new_dst_mac[5]= 116;
  // // bpf_skb_store_bytes(
  // //   skb,
  // //   offsetof(struct ethhdr, h_dest),
  // //   new_dst_mac,
  // //   ETH_ALEN,
  // //   0
  // // );


  // // // // __u8 new_src_mac[ETH_ALEN];
	// // // __u8 new_dst_mac[ETH_ALEN];
  // // // // veth2:
  // // // // [162, 38, 121, 215, 254, 116]
  // // // new_dst_mac[0]= 162;
  // // // new_dst_mac[1]= 38;
  // // // new_dst_mac[2]= 121;
  // // // new_dst_mac[3]= 215;
  // // // new_dst_mac[4]= 254;
  // // // new_dst_mac[5]= 116;
  // // // bpf_skb_store_bytes(
  // // //   skb,
  // // //   offsetof(struct ethhdr, h_dest),
  // // //   new_dst_mac,
  // // //   ETH_ALEN,
  // // //   0
  // // // );

  // // // bpf_printk("ingress!!!vxlan");
  // // // veth1 出来的要给到 veth3
  // // // return bpf_redirect_peer(8, 0);

  // // // return bpf_redirect_neigh(14, NULL, 0, 0);
  // // return TC_ACT_OK;

  // // trace_printk(
  // //   "1:%d, 2:%d, 3:%d, 4:%d, 5:%d, 6:%d",
  // //   src_mac[0], src_mac[1], src_mac[2],
  // //   src_mac[3], src_mac[4], src_mac[5]
  // // );
  // // trace_printk("DEST: %s", MAC_ARG(dst_mac));
  // // trace_printk("[action] IP Packet, proto= %d, src= %lu, dst= %lu\n", ip->protocol, src_ip, dst_ip);
  // // __u16 a = eth->h_proto;
  // // bpf_printk("MAC:%s >> %s\n",src_mac,dst_mac);
  // // // 第一个参数的 10 表示留在 host 上的那半拉 ifindex
  // // // 然后 peer 会自动帮忙给送进 ns
  // // bpf_redirect_peer(4, 0);
  // // // bpf_redirect(10, 0);
  // // return TC_ACT_REDIRECT;
  // // return TC_ACT_OK;
}

char _license[] SEC("license") = "GPL";

