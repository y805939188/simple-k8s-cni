package nettools

import (
	"fmt"
	oriNet "net"

	"testing"

	"testcni/ipam"
	"testcni/utils"

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
		containerVeth, hostVeth, err := CreateVethPair(ifName, mtu, "")
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
			err = SetIptablesForToForwardAccept(br)
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

	ipam.Init("10.244.0.0")
	is, err := ipam.GetIpamService()
	if err != nil {
		fmt.Println("ipam 初始化失败: ", err.Error())
		return
	}

	vxlan, err := CreateVxlanAndUp("ding_vxlan", 1500)
	test.Nil(err)
	fmt.Println(vxlan)
	err = DelVxlan("ding_vxlan")
	test.Nil(err)

	ipip, err := CreateIPIPDeviceAndUp("tunl0")
	test.Nil(err)
	fmt.Println(ipip)
	test.Equal(ipip.Type(), "ipip")
	err = DelIPIP("tunl0")
	test.Nil(err)

	DelVethPair("ding-test")
	_, hostVeth, err := CreateVethPair("ding-test", 1500)
	fmt.Println("这里的 hostVeth name 是: ", hostVeth.Name)
	test.Nil(err)
	err = SetUpVeth(hostVeth)
	test.Nil(err)
	err = SetUpDeviceProxyArpV4(hostVeth)
	test.Nil(err)
	str, err := utils.ReadContentFromFile(fmt.Sprintf("/proc/sys/net/ipv4/conf/%s/proxy_arp", hostVeth.Name))
	test.Nil(err)
	// 不知道为啥通过命令执行 echo 1 > xxxx 后边会有个 \n 的换行符
	// 不过啥也不影响
	test.Equal(str, "1\n")
	err = SetDownDeviceProxyArpV4(hostVeth)
	test.Nil(err)
	str, err = utils.ReadContentFromFile(fmt.Sprintf("/proc/sys/net/ipv4/conf/%s/proxy_arp", hostVeth.Name))
	test.Nil(err)
	test.Equal(str, "0\n")
	err = DelVethPair(hostVeth.Attrs().Name)
	test.Nil(err)

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

	// ipam.Init("10.244.0.0", "16")
	// is, err := ipam.GetIpamService()
	// if err != nil {
	// 	fmt.Println("ipam 初始化失败: ", err.Error())
	// 	return
	// }

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
