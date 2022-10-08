package ipip

import (
	"errors"
	"fmt"
	"net"
	"testcni/cni"
	"testcni/consts"
	"testcni/ipam"
	"testcni/nettools"
	"testcni/skel"
	"testcni/utils"

	types "github.com/containernetworking/cni/pkg/types/100"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/vishvananda/netlink"
)

const MODE = consts.MODE_IPIP
const DEFAULT_POST_GW = "169.254.1.1/32"

type IpipCNI struct{}

func initEveryClient(args *skel.CmdArgs, pluginConfig *cni.PluginConf) (*ipam.IpamService, error) {
	ipam.Init(pluginConfig.Subnet)
	ipam, err := ipam.GetIpamService()
	if err != nil {
		return nil, errors.New(fmt.Sprintf("初始化 ipam 客户端失败: %s", err.Error()))
	}

	return ipam, nil
}

func setFibTalbeIntoNs(gw string, veth *netlink.Veth) error {
	// 启动之后给这个 netns 设置默认路由 以便让其他网段的包也能从 veth 走到网桥
	// 这里的 gw 模仿 calico 使用 “169.254.1.1”
	gwIp, gwNet, err := net.ParseCIDR(gw)
	if err != nil {
		utils.WriteLog("创建交换路由失败, err:", err.Error())
		return err
	}

	defIp, defNet, err := net.ParseCIDR("0.0.0.0/0")
	if err != nil {
		utils.WriteLog("创建交换路由失败, err:", err.Error())
		return err
	}

	// 设置交换路由, 让流量能从路由表中查询到下一条
	// 注意这里在设置交换路由的时候, 第一条的 gw -> 0.0.0.0 需要是 scope_link
	err = nettools.AddRoute(gwNet, defIp, veth, netlink.SCOPE_LINK)
	if err != nil {
		utils.WriteLog("设置交换路由 gw -> default 失败: ", err.Error())
		return err
	}
	// 然后创建默认的 0.0.0.0 -> gw 时就可以走默认的 scope universe 了
	// 否则会创建失败
	err = nettools.AddRoute(defNet, gwIp, veth)
	if err != nil {
		utils.WriteLog("设置交换路由 default -> gw 失败: ", err.Error())
		return err
	}
	return nil
}

func setUpHostPair(veth *netlink.Veth) error {
	v, err := netlink.LinkByName(veth.Attrs().Name)
	if err != nil {
		return err
	}
	veth = v.(*netlink.Veth)
	return nettools.SetUpVeth(veth)
}

func setUpHostPairProxyArp(veth *netlink.Veth) error {
	v, err := netlink.LinkByName(veth.Attrs().Name)
	if err != nil {
		return err
	}
	veth = v.(*netlink.Veth)
	return nettools.SetUpDeviceProxyArpV4(veth)
}

func setUpHostPairForwarding(veth *netlink.Veth) error {
	v, err := netlink.LinkByName(veth.Attrs().Name)
	if err != nil {
		return err
	}
	veth = v.(*netlink.Veth)
	return nettools.SetUpDeviceForwarding(veth)
}

func setLocalFibTable(podIP string, veth *netlink.Veth) error {
	_, gwNet, err := net.ParseCIDR(podIP)
	if err != nil {
		utils.WriteLog("创建交换路由失败, err:", err.Error())
		return err
	}

	defIp, _, err := net.ParseCIDR("0.0.0.0/0")
	if err != nil {
		utils.WriteLog("创建交换路由失败, err:", err.Error())
		return err
	}

	_veth, err := netlink.LinkByName(veth.Attrs().Name)
	if err != nil {
		return err
	}
	veth, ok := _veth.(*netlink.Veth)
	if !ok {
		return errors.New("can not covert link to veth")
	}

	return nettools.AddRoute(gwNet, defIp, veth, netlink.SCOPE_LINK)
}

func setPodNetwork(netns ns.NetNS, ifname string, podIP string) (*netlink.Veth, *netlink.Veth, error) {
	var containerVeth, hostVeth *netlink.Veth
	var err error
	err = netns.Do(func(hostNs ns.NetNS) error {
		// 在 netns 中创建一对儿 veth pair
		containerVeth, hostVeth, err = nettools.CreateVethPair(ifname, 1500)
		if err != nil {
			utils.WriteLog("创建 veth 失败, err: ", err.Error())
			return err
		}

		// 把随机起名的 veth 那头放在主机上
		err = nettools.SetVethNsFd(hostVeth, hostNs)
		if err != nil {
			utils.WriteLog("把 veth 设置到 ns 下失败: ", err.Error())
			return err
		}

		// 然后把要被放到 pod 中的那头 veth 塞上 podIP
		err = nettools.SetIpForVeth(containerVeth, podIP)
		if err != nil {
			utils.WriteLog("给 veth 设置 ip 失败, err: ", err.Error())
			return err
		}

		// 然后启动它
		err = nettools.SetUpVeth(containerVeth)
		if err != nil {
			utils.WriteLog("启动 veth pair 失败, err: ", err.Error())
			return err
		}

		return setFibTalbeIntoNs(DEFAULT_POST_GW, containerVeth)
	})
	if err != nil {
		return nil, nil, err
	}
	return containerVeth, hostVeth, nil
}

func setHostNetwork(netns ns.NetNS, hostVeth *netlink.Veth, podIP string) error {
	return netns.Do(func(_ ns.NetNS) error {
		// 启动 ns 留在 host 上那半拉 veth
		err := setUpHostPair(hostVeth)
		if err != nil {
			return err
		}

		// 把主机上那半拉的 veth 的 proxy arp 给打开
		err = setUpHostPairProxyArp(hostVeth)
		if err != nil {
			return err
		}

		err = setUpHostPairForwarding(hostVeth)
		if err != nil {
			return err
		}

		// 创建一条本地的路由表, 是本机的 podIP → 0.0.0.0 via host-veth
		err = setLocalFibTable(podIP, hostVeth)
		if err != nil {
			return err
		}

		// 允许这个设备做流量转发
		return nettools.SetIptablesForToForwardAccept(hostVeth)
	})
}

func (ipip *IpipCNI) Bootstrap(
	args *skel.CmdArgs,
	pluginConfig *cni.PluginConf,
) (*types.Result, error) {
	// 初始化 ipam
	ipamClient, err := initEveryClient(args, pluginConfig)
	if err != nil {
		return nil, err
	}

	// 从 ipam 中拿到一个未使用的 ip 地址
	podIP, err := ipamClient.Get().UnusedIP()
	if err != nil {
		utils.WriteLog("获取 podIP 出错, err: ", err.Error())
		return nil, err
	}

	// calico 内部的 pod 的 ip 都是 32 掩码的
	podIP = podIP + "/" + "32"

	// 获取 netns
	netns, err := ns.GetNS(args.Netns)
	if err != nil {
		utils.WriteLog("获取 ns 失败: ", err.Error())
		return nil, err
	}

	// 设置 pod 中的网络让其中的流量能走到 host 上
	_, hostVeth, err := setPodNetwork(netns, args.IfName, podIP)
	if err != nil {
		return nil, err
	}

	// 获取默认网络命名空间
	hostNs, err := ns.GetCurrentNS()
	if err != nil {
		return nil, err
	}

	// 设置 host 上的 pod 网络, 主要是开启 proxy arp 以及设置路由表
	err = setHostNetwork(hostNs, hostVeth, podIP)
	if err != nil {
		return nil, err
	}

	// 走到这儿基本上 pod 内部就配置完了

	// // 2. 根据 subnet 网段来得到网关, 表示所有的节点上的 pod 的 ip 都在这个网关范围内
	// gateway, err := ipamClient.Get().Gateway()
	// if err != nil {
	// 	utils.WriteLog("获取 gateway 出错, err: ", err.Error())
	// 	return nil, err
	// }

	// // 3. 获取网关＋网段号
	// gatewayWithMaskSegment, err := ipamClient.Get().GatewayWithMaskSegment()
	// if err != nil {
	// 	utils.WriteLog("获取 gatewayWithMaskSegment 出错, err: ", err.Error())
	// 	return nil, err
	// }

	return nil, errors.New("tmp error")
	// _gw := net.ParseIP(gateway)

	// _, _podIP, _ := net.ParseCIDR(podIP)

	// result := &types.Result{
	// 	CNIVersion: pluginConfig.CNIVersion,
	// 	IPs: []*types.IPConfig{
	// 		{
	// 			Address: *_podIP,
	// 			Gateway: _gw,
	// 		},
	// 	},
	// }
	// return result, nil
}

func (ipip *IpipCNI) Unmount(
	args *skel.CmdArgs,
	pluginConfig *cni.PluginConf,
) error {
	// TODO
	return nil
}

func (ipip *IpipCNI) Check(
	args *skel.CmdArgs,
	pluginConfig *cni.PluginConf,
) error {
	// TODO
	return nil
}

func (ipip *IpipCNI) GetMode() string {
	return MODE
}

func init() {
	ipipCNI := &IpipCNI{}
	manager := cni.GetCNIManager()
	err := manager.Register(ipipCNI)
	if err != nil {
		utils.WriteLog("注册 ipip cni 失败: ", err.Error())
		panic(err.Error())
	}
}
