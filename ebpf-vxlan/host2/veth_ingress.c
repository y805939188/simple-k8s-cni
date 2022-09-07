#include <linux/bpf.h>
#include <linux/pkt_cls.h>
#include <bpf/bpf_helpers.h>
#include <linux/if_ether.h>
#include <linux/ip.h>
#include <linux/icmp.h>

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
  bpf_printk("redirect to vxlan: %d", skb->data_end - skb->data);

	// key.remote_ipv4 = 0xc0a8400e; /* 192.168.64.16 */
	// key.tunnel_id = 2;
	// key.tunnel_tos = 0;
	// key.tunnel_ttl = 64;

	// ret = bpf_skb_set_tunnel_key(skb, &key, sizeof(key), BPF_F_ZERO_CSUM_TX);
	// if (ret < 0) {
	// 	bpf_printk("bpf_skb_set_tunnel_key failed");
	// 	return TC_ACT_SHOT;
	// }

  return bpf_redirect(7, 0);
}

char _license[] SEC("license") = "GPL";
