// package net

package main

import (
	"fmt"
	oriNet "net"

	// "testing"

	"testcni/net"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/vishvananda/netlink"
)

func TestNettools(brName, cidr, ifName, podIP string, mtu int, netns ns.NetNS) {
	// brName := "testbr0"
	// cidr := "192.168.1.1/16"
	// mtu := 1500

	// 先创建网桥
	br, err := net.CreateBridge(brName, cidr, mtu)
	// br, err := CreateBridge(brName, cidr, mtu)
	if err != nil {
		fmt.Println("创建网卡失败, err: ", err.Error())
		return
	}

	// // 然后获取 pod 的命名空间
	// netns, err := ns.GetNS("/run/netns/test.net.1")
	// if err != nil {
	// 	fmt.Println("获取 ns 失败: ", err.Error())
	// 	return
	// }
	err = netns.Do(func(hostNs ns.NetNS) error {
		// ifName := "eth0"
		// mtu := 1500
		// 创建一对儿 veth 设备
		containerVeth, hostVeth, err := net.CreateVethPair(ifName, mtu)
		if err != nil {
			fmt.Println("创建 veth 失败, err: ", err.Error())
			return err
		}

		// 放一个到主机上
		err = net.SetVethNsFd(hostVeth, hostNs)
		if err != nil {
			fmt.Println("把 veth 设置到 ns 下失败: ", err.Error())
			return err
		}

		// 然后把要被放到 pod 中的塞上 podIP
		// podIP := "192.168.1.6/16"
		err = net.SetIpForVeth(containerVeth, podIP)
		if err != nil {
			fmt.Println("给 veth 设置 ip 失败, err: ", err.Error())
			return err
		}

		// 然后启动它
		err = net.SetUpVeth(containerVeth)
		if err != nil {
			fmt.Println("启动 veth pair 失败, err: ", err.Error())
			return err
		}

		// 启动之后给这个 netns 设置默认路由 以便让其他网段的包也能从 veth 走到网桥
		// TODO: 实测后发现还必须得写在这里, 如果写在下面 hostNs.Do 里头会报错目标 network 不可达(why?)
		gwip := cidr
		gwNetIP, _, err := oriNet.ParseCIDR(gwip)
		if err != nil {
			fmt.Println("转换 gwip 失败, err:", err.Error())
			return err
		}
		err = net.SetDefaultRouteToVeth(gwNetIP, containerVeth)
		if err != nil {
			fmt.Println("SetDefaultRouteToVeth 时出错, err: ", err.Error())
			return err
		}

		hostNs.Do(func(_ ns.NetNS) error {
			// 重新获取一次 host 上的 veth, 因为 hostVeth 发生了改变
			_hostVeth, err := netlink.LinkByName(hostVeth.Attrs().Name)
			hostVeth = _hostVeth.(*netlink.Veth)
			if err != nil {
				fmt.Println("重新获取 hostVeth 失败, err: ", err.Error())
				return err
			}
			// 启动它
			err = net.SetUpVeth(hostVeth)
			if err != nil {
				fmt.Println("启动 veth pair 失败, err: ", err.Error())
				return err
			}

			// 把它塞到网桥上
			err = net.SetVethMaster(hostVeth, br)
			if err != nil {
				fmt.Println("挂载 veth 到网桥失败, err: ", err.Error())
				return err
			}

			// 都完事儿之后理论上同一台主机下的俩 netns(pod) 就能通信了
			// 如果无法通信, 有可能是 iptables 被设置了 forward drop
			// 需要用 iptables 允许网桥做转发
			err = net.SetIptablesForBridgeToForwordAccept(br)
			if err != nil {
				fmt.Println("set iptables 失败", err.Error())
			}

			return nil
		})

		return nil
	})
}

// 写到这里方便使用 dlv 进行断点调试
// 在 mac 本地通过 vscode 的 remote 功能时
// 没法直接打断点, 必须得先开个 dlv
func main() {
	brName := "testbr0"
	cidr := "10.244.1.1/16"
	ifName := "eth0"
	podIP := "10.244.1.2/24"
	mtu := 1500
	netns, err := ns.GetNS("/run/netns/test.net.1")
	if err != nil {
		fmt.Println("获取 ns 失败: ", err.Error())
		return
	}

	TestNettools(brName, cidr, ifName, podIP, mtu, netns)

	brName = "testbr0"
	podIP = "10.244.1.3/24"
	mtu = 1500
	netns, err = ns.GetNS("/run/netns/test.net.2")
	if err != nil {
		fmt.Println("获取 ns 失败: ", err.Error())
		return
	}
	TestNettools(brName, cidr, ifName, podIP, mtu, netns)

	// 目前同一台主机上的 pod 可以 ping 通了
	// 接下来要让不同节点上的 pod 互相通信了
	/**
	 * 手动操作
	 * 	1. 主机上添加路由规则: ip route add 10.244.2.0/24 via 192.168.98.144 dev ens33
	 *  2. 对方主机也添加
	 *  3. 将双方主机上的网卡添加进网桥: brctl addif testbr0 ens33
	 * 以上手动操作可成功
	 * TODO: 接下来要给它转化成代码
	 */
}
