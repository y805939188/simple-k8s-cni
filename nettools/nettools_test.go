package nettools

import (
	"fmt"
	oriNet "net"

	"testing"

	"testcni/ipam"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/stretchr/testify/assert"
	"github.com/vishvananda/netlink"
)

func _nettools(brName, gw, ifName, podIP string, mtu int, netns ns.NetNS) {
	// 先创建网桥
	br, err := CreateBridge(brName, gw, mtu)
	if err != nil {
		fmt.Println("创建网卡失败, err: ", err.Error())
		return
	}

	err = netns.Do(func(hostNs ns.NetNS) error {
		// 创建一对儿 veth 设备
		containerVeth, hostVeth, err := CreateVethPair(ifName, mtu)
		if err != nil {
			fmt.Println("创建 veth 失败, err: ", err.Error())
			return err
		}

		// 放一个到主机上
		err = SetVethNsFd(hostVeth, hostNs)
		if err != nil {
			fmt.Println("把 veth 设置到 ns 下失败: ", err.Error())
			return err
		}

		// 然后把要被放到 pod 中的塞上 podIP
		err = SetIpForVeth(containerVeth, podIP)
		if err != nil {
			fmt.Println("给 veth 设置 ip 失败, err: ", err.Error())
			return err
		}

		// 然后启动它
		err = SetUpVeth(containerVeth)
		if err != nil {
			fmt.Println("启动 veth pair 失败, err: ", err.Error())
			return err
		}

		// 启动之后给这个 netns 设置默认路由 以便让其他网段的包也能从 veth 走到网桥
		// TODO: 实测后发现还必须得写在这里, 如果写在下面 hostNs.Do 里头会报错目标 network 不可达(why?)
		gwNetIP, _, err := oriNet.ParseCIDR(gw)
		if err != nil {
			fmt.Println("转换 gwip 失败, err:", err.Error())
			return err
		}
		err = SetDefaultRouteToVeth(gwNetIP, containerVeth)
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
			err = SetUpVeth(hostVeth)
			if err != nil {
				fmt.Println("启动 veth pair 失败, err: ", err.Error())
				return err
			}

			// 把它塞到网桥上
			err = SetVethMaster(hostVeth, br)
			if err != nil {
				fmt.Println("挂载 veth 到网桥失败, err: ", err.Error())
				return err
			}

			// 都完事儿之后理论上同一台主机下的俩 netns(pod) 就能通信了
			// 如果无法通信, 有可能是 iptables 被设置了 forward drop
			// 需要用 iptables 允许网桥做转发
			err = SetIptablesForBridgeToForwardAccept(br)
			if err != nil {
				fmt.Println("set iptables 失败", err.Error())
			}

			return nil
		})

		return nil
	})

	if err != nil {
		fmt.Println("这里的 err 是: ", err.Error())
	}
}

func TestNettools(t *testing.T) {
	test := assert.New(t)
	vxlan, err := CreateVxlanAndUp("ding_vxlan", 1500)
	test.Nil(err)
	fmt.Println(vxlan)
	// err = DelVxlan("ding_test1")
	// test.Nil(err)
	return

	// brName := "testbr0"
	// cidr := "10.244.1.1/16"
	// ifName := "eth0"
	// podIP := "10.244.1.2/24"
	// mtu := 1500
	// netns, err := ns.GetNS("/run/netns/test.net.1")
	// if err != nil {
	// 	fmt.Println("获取 ns 失败: ", err.Error())
	// 	return
	// }

	// _nettools(brName, cidr, ifName, podIP, mtu, netns)

	// brName = "testbr0"
	// podIP = "10.244.1.3/24"
	// mtu = 1500
	// netns, err = ns.GetNS("/run/netns/test.net.2")
	// if err != nil {
	// 	fmt.Println("获取 ns 失败: ", err.Error())
	// 	return
	// }
	// _nettools(brName, cidr, ifName, podIP, mtu, netns)

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

	ipam.Init("10.244.0.0", "16")
	is, err := ipam.GetIpamService()
	if err != nil {
		fmt.Println("ipam 初始化失败: ", err.Error())
		return
	}

	fmt.Println("成功: ", is.MaskIP)
	//  test.Equal(is.MaskIP, "255.255.0.0")

	names, err := is.Get().NodeNames()
	if err != nil {
		fmt.Println("这里的 err 是: ", err.Error())
		return
	}

	//  test.Equal(len(names), 3)

	for _, name := range names {
		fmt.Println("这里的 name 是: ", name)
		ip, err := is.Get().NodeIp(name)
		if err != nil {
			fmt.Println("这里的 err 是: ", err.Error())
			return
		}
		fmt.Println("这里的 ip 是: ", ip)
	}

}
