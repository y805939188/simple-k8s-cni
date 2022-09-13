package hostgw

import (
	"net"
	"testcni/cni"
	"testcni/consts"
	"testcni/ipam"
	"testcni/nettools"
	"testcni/skel"
	"testcni/utils"

	types "github.com/containernetworking/cni/pkg/types/100"
	// "github.com/containernetworking/cni/pkg/types"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/vishvananda/netlink"
)

const MODE = consts.MODE_HOST_GW

type HostGatewayCNI struct{}

func (hostGW *HostGatewayCNI) Bootstrap(
	args *skel.CmdArgs,
	pluginConfig *cni.PluginConf,
) (*types.Result, error) {
	// 使用 kubelet(containerd) 传过来的 subnet 地址初始化 ipam
	ipam.Init(pluginConfig.Subnet)
	ipamClient, err := ipam.GetIpamService()
	if err != nil {
		utils.WriteLog("创建 ipam 客户端出错, err: ", err.Error())
		return nil, err
	}

	// 根据 subnet 网段来得到网关, 表示所有的节点上的 pod 的 ip 都在这个网关范围内
	gateway, err := ipamClient.Get().Gateway()
	if err != nil {
		utils.WriteLog("获取 gateway 出错, err: ", err.Error())
		return nil, err
	}

	// 获取网关＋网段号
	gatewayWithMaskSegment, err := ipamClient.Get().GatewayWithMaskSegment()
	if err != nil {
		utils.WriteLog("获取 gatewayWithMaskSegment 出错, err: ", err.Error())
		return nil, err
	}

	// 获取网桥名字
	bridgeName := pluginConfig.Bridge
	if bridgeName != "" {
		bridgeName = "testcni0"
	}

	// 这里如果不同节点间通信的方式使用 vxlan 的话, 这里需要变成 1450
	// 因为 vxlan 设备会给报头中加一个 50 字节的 vxlan 头部
	mtu := 1500
	// 获取 containerd 传过来的网卡名, 这个网卡名要被插到 net ns 中
	ifName := args.IfName
	// 根据 containerd 传过来的 netns 的地址获取 ns
	netns, err := ns.GetNS(args.Netns)
	if err != nil {
		utils.WriteLog("获取 ns 失败: ", err.Error())
		return nil, err
	}

	// 从 ipam 中拿到一个未使用的 ip 地址
	podIP, err := ipamClient.Get().UnusedIP()
	if err != nil {
		utils.WriteLog("获取 podIP 出错, err: ", err.Error())
		return nil, err
	}

	// 走到这儿的话说明这个 podIP 已经在 etcd 中占上坑位了
	// 占坑的操作是直接在 Get().UnusedIP() 的时候就做了
	// 后续如果有什么 error 的话可以再 release

	// 这里拼接 pod 的 cidr
	// podIP = podIP + "/" + ipamClient.MaskSegment
	podIP = podIP + "/" + "24"

	/**
	 * 准备操作做完之后就可以调用网络工具来创建网络了
	 * nettools 主要做的事情:
	 *		1. 根据网桥名创建一个网桥
	 *		2. 根据网卡名儿创建一对儿 veth
	 *		3. 把叫做 IfName 的怼到 pod(netns) 上
	 *		4. 把另外一个干到主机的网桥上
	 *		5. set up 网桥以及这对儿 veth
	 *		6. 在 pod(netns) 里创建一个默认路由, 把匹配到 0.0.0.0 的 ip 都让其从 IfName 那块儿 veth 往外走
	 *		7. 设置主机的 iptables, 让所有来自 bridgeName 的流量都能做 forward(因为 docker 可能会自己设置 iptables 不让转发的规则)
	 */

	err = nettools.CreateBridgeAndCreateVethAndSetNetworkDeviceStatusAndSetVethMaster(bridgeName, gatewayWithMaskSegment, ifName, podIP, mtu, netns)
	if err != nil {
		utils.WriteLog("执行创建网桥, 创建 veth 设备, 添加默认路由等操作失败, err: ", err.Error())
		err = ipamClient.Release().IPs(podIP)
		if err != nil {
			utils.WriteLog("释放 podIP", podIP, " 失败: ", err.Error())
		}
	}

	/**
	 * 到这儿为止, 同一台主机上的 pod 可以 ping 通了
	 * 并且也可以访问其他网段的 ip 了
	 * 不过此时只能 ping 通主机上的网卡的网段(如果数据包没往外走的话需要确定主机是否开启了 ip_forward)
	 * 暂时没法 ping 通外网
	 * 因为此时的流量包只能往外出而不能往里进
	 * 原因是流量包往外出的时候还需要做一次 snat
	 * 没做 nat 转换的话, 外网在往回送消息的时候不知道应该往哪儿发
	 * 不过 testcni 这里暂时没有做 snat 的操作, 因为暂时没这个需求~
	 *
	 *
	 * 接下来要让不同节点上的 pod 互相通信了
	 * 可以尝试先手动操作
	 *  1. 主机上添加路由规则: ip route add 10.244.x.0/24 via 192.168.98.x dev ens33, 也就是把非本机的节点的网段和其他 node 的 ip 做个映射
	 *  2. 其他每台集群中的主机也添加
	 *  3. 把每台主机上的对外网卡都用 iptables 设置为可 ip forward: iptables -A FORWARD -i testcni0 -j ACCEPT
	 * 以上手动操作可成功
	 */

	// 首先通过 ipam 获取到 etcd 中存放的集群中所有节点的相关网络信息
	networks, err := ipamClient.Get().AllHostNetwork()
	if err != nil {
		utils.WriteLog("这里的获取所有节点的网络信息失败, err: ", err.Error())
		return nil, err
	}

	// 然后获取一下本机的网卡信息
	currentNetwork, err := ipamClient.Get().HostNetwork()
	if err != nil {
		utils.WriteLog("获取本机网卡信息失败, err: ", err.Error())
		return nil, err
	}

	// 这里面要做的就是把其他节点上的 pods 的 cidr 和其主机的网卡 ip 作为一条路由规则创建到当前主机上
	err = nettools.SetOtherHostRouteToCurrentHost(networks, currentNetwork)
	if err != nil {
		utils.WriteLog("给主机添加其他节点网络信息失败, err: ", err.Error())
		return nil, err
	}

	link, err := netlink.LinkByName(currentNetwork.Name)
	if err != nil {
		utils.WriteLog("获取本机网卡失败, err: ", err.Error())
		return nil, err
	}
	err = nettools.SetIptablesForDeviceToFarwordAccept(link.(*netlink.Device))
	if err != nil {
		utils.WriteLog("设置本机网卡转发规则失败")
		return nil, err
	}

	_gw := net.ParseIP(gateway)

	_, _podIP, _ := net.ParseCIDR(podIP)

	result := &types.Result{
		CNIVersion: pluginConfig.CNIVersion,
		IPs: []*types.IPConfig{
			{
				Address: *_podIP,
				Gateway: _gw,
			},
		},
	}
	return result, nil
}

func (hostGW *HostGatewayCNI) Unmount(
	args *skel.CmdArgs,
	pluginConfig *cni.PluginConf,
) error {
	// TODO
	return nil
}

func (hostGW *HostGatewayCNI) Check(
	args *skel.CmdArgs,
	pluginConfig *cni.PluginConf,
) error {
	// TODO
	return nil
}

func (hostGW *HostGatewayCNI) GetMode() string {
	return MODE
}

func init() {
	hostGatewayCNI := &HostGatewayCNI{}
	manager := cni.GetCNIManager()
	err := manager.Register(hostGatewayCNI)
	if err != nil {
		utils.WriteLog("注册 host gw cni 失败: ", err.Error())
		panic(err.Error())
	}
}
