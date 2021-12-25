package nettools

import (
	"crypto/rand"
	"errors"
	"fmt"
	"net"
	"os"
	"testcni/ipam"
	"testcni/utils"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/coreos/go-iptables/iptables"
	"github.com/vishvananda/netlink"
)

func CreateBridge(brName, gw string, mtu int) (*netlink.Bridge, error) {
	l, err := netlink.LinkByName(brName)

	br, ok := l.(*netlink.Bridge)
	if ok && br != nil {
		return br, nil
	}

	br = &netlink.Bridge{
		LinkAttrs: netlink.LinkAttrs{
			Name:   brName,
			MTU:    mtu,
			TxQLen: -1,
		},
	}

	err = netlink.LinkAdd(br)
	if err != nil {
		utils.WriteLog("无法创建网桥: ", brName, "err: ", err.Error())
		return nil, err
	}

	// 这里需要通过 netlink 重新获取网桥
	// 否则光创建的话无法从上头拿到其他属性
	l, err = netlink.LinkByName(brName)

	br, ok = l.(*netlink.Bridge)
	if !ok {
		utils.WriteLog("找到了设备, 但是该设备不是网桥")
		return nil, fmt.Errorf("找到 %q 但该设备不是网桥", brName)
	}

	// 给网桥绑定 ip 地址, 让网桥作为网关
	ipaddr, ipnet, err := net.ParseCIDR(gw)
	if err != nil {
		utils.WriteLog("无法 parse gw 为 ipnet, err: ", err.Error())
		return nil, fmt.Errorf("gatewayIP 转换失败 %q: %v", gw, err)
	}
	ipnet.IP = ipaddr
	addr := &netlink.Addr{IPNet: ipnet}
	if err = netlink.AddrAdd(br, addr); err != nil {
		utils.WriteLog("将 gw 添加到 bridge 失败, err: ", err.Error())
		return nil, fmt.Errorf("无法将 %q 添加到网桥设备 %q, err: %v", addr, brName, err)
	}

	// 然后还要把这个网桥给 up 起来
	if err = netlink.LinkSetUp(br); err != nil {
		utils.WriteLog("启动网桥失败, err: ", err.Error())
		return nil, fmt.Errorf("启动网桥 %q 失败, err: %v", brName, err)
	}
	return br, nil
}

func SetUpVeth(veth ...*netlink.Veth) error {
	for _, v := range veth {
		// 启动 veth 设备
		err := netlink.LinkSetUp(v)
		if err != nil {
			utils.WriteLog("启动 veth1 失败, err: ", err.Error())
			return err
		}
	}
	return nil
}

func CreateVethPair(ifName string, mtu int) (*netlink.Veth, *netlink.Veth, error) {
	vethPairName := ""
	for {
		_vname, err := RandomVethName()
		vethPairName = _vname
		if err != nil {
			utils.WriteLog("生成随机 veth pair 名字失败, err: ", err.Error())
			return nil, nil, err
		}

		_, err = netlink.LinkByName(vethPairName)
		if err != nil && !os.IsExist(err) {
			// 上面生成随机名字可能会重名, 所以这里先尝试按照这个名字获取一下
			// 如果没有这个名字的设备, 那就可以 break 了
			break
		}
	}

	if vethPairName == "" {
		return nil, nil, errors.New("生成 veth pair name 失败")
	}

	veth := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{
			Name: ifName,
			// Flags:     net.FlagUp,
			MTU: mtu,
			// Namespace: netlink.NsFd(int(ns.Fd())), // 先不设置 ns, 要不一会儿下头 LinkByName 时候找不到
		},
		PeerName: vethPairName,
		// PeerNamespace: netlink.NsFd(int(ns.Fd())),
	}

	// 创建 veth pair
	err := netlink.LinkAdd(veth)

	if err != nil {
		utils.WriteLog("创建 veth 设备失败, err: ", err.Error())
		return nil, nil, err
	}

	// 尝试重新获取 veth 设备看是否能成功
	veth1, err := netlink.LinkByName(ifName) // veth1 一会儿要在 pod(net ns) 里
	if err != nil {
		fmt.Println("创建完 veth11111 但是获取失败, err: ", err.Error())
		// 如果获取失败就尝试删掉
		netlink.LinkDel(veth1)
		utils.WriteLog("创建完 veth 但是获取失败, err: ", err.Error())
		return nil, nil, err
	}

	// 尝试重新获取 veth 设备看是否能成功
	veth2, err := netlink.LinkByName(vethPairName) // veth2 在主机上
	if err != nil {
		fmt.Println("创建完 veth2222222 但是获取失败, err: ", err.Error())
		// 如果获取失败就尝试删掉
		netlink.LinkDel(veth2)
		utils.WriteLog("创建完 veth 但是获取失败, err: ", err.Error())
		return nil, nil, err
	}

	return veth1.(*netlink.Veth), veth2.(*netlink.Veth), nil
}

func SetIpForVeth(veth *netlink.Veth, podIP string) error {
	// 给 veth1 也就是 pod(net ns) 里的设备添加上 podIP
	ipaddr, ipnet, err := net.ParseCIDR(podIP)
	if err != nil {
		utils.WriteLog("转换 podIP 为 net 类型失败: ", podIP, " err: ", err.Error())
		return err
	}
	ipnet.IP = ipaddr
	err = netlink.AddrAdd(veth, &netlink.Addr{IPNet: ipnet})
	if err != nil {
		utils.WriteLog("给 veth 添加 podIP 失败, podIP 是: ", podIP, " err: ", err.Error())
		return err
	}

	return nil
}

func SetVethToBridge(veth *netlink.Veth, br *netlink.Bridge) error {
	// 把 veth2 干到 br 上, veth1 不用, 因为在创建的时候已经被干到 ns 里头了
	err := netlink.LinkSetMaster(veth, br)
	if err != nil {
		utils.WriteLog("把 veth 查到网桥上失败, err: ", err.Error())
		return fmt.Errorf("把 veth %q 插到网桥 %v 失败, err: %v", veth.Attrs().Name, br.Attrs().Name, err)
	}
	return nil
}

func SetVethNsFd(veth *netlink.Veth, ns ns.NetNS) error {
	err := netlink.LinkSetNsFd(veth, int(ns.Fd()))
	if err != nil {
		return fmt.Errorf("把 veth %q 干到 netns 上失败: %v", veth.Attrs().Name, err)
	}
	return nil
}

func SetVethMaster(veth *netlink.Veth, br *netlink.Bridge) error {
	err := netlink.LinkSetMaster(veth, br)
	if err != nil {
		return fmt.Errorf("把 veth %q 干到网桥上失败: %v", veth.Attrs().Name, err)
	}
	return nil
}

func SetDeviceMaster(device *netlink.Device, br *netlink.Bridge) error {
	err := netlink.LinkSetMaster(device, br)
	if err != nil {
		return fmt.Errorf("把 veth %q 干到网桥上失败: %v", device.Attrs().Name, err)
	}
	return nil
}

func SetDefaultRouteToVeth(gwIP net.IP, veth netlink.Link) error {
	return AddDefaultRoute(gwIP, veth)
}

func SetOtherHostRouteToCurrentHost(networks []*ipam.Network, currentNetwork *ipam.Network) error {

	link, err := netlink.LinkByName(currentNetwork.Name)

	if err != nil {
		return err
	}

	for _, network := range networks {
		if !network.IsCurrentHost {
			// 对于其他主机, 需要获取到其他主机的对外网卡 ip, 以及它的 pods 们所占用的网段的 cidr
			// 然后用这个 cidr 和这个 ip 做一个路由表的映射
			if link == nil {
				return err
			}

			_, cidr, err := net.ParseCIDR(network.CIDR)
			if err != nil {
				return err
			}

			ip := net.ParseIP(network.IP)

			err = AddHostRoute(cidr, ip, link)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// forked from plugins/pkg/ip/route_linux.go
func AddRoute(ipn *net.IPNet, gw net.IP, dev netlink.Link) error {
	return netlink.RouteAdd(&netlink.Route{
		LinkIndex: dev.Attrs().Index,
		Scope:     netlink.SCOPE_UNIVERSE,
		Dst:       ipn,
		Gw:        gw,
	})
}

// forked from plugins/pkg/ip/route_linux.go
func AddHostRoute(ipn *net.IPNet, gw net.IP, dev netlink.Link) error {
	return netlink.RouteAdd(&netlink.Route{
		LinkIndex: dev.Attrs().Index,
		// Scope:     netlink.SCOPE_HOST,
		Dst: ipn,
		Gw:  gw,
	})
}

func AddHostRouteWithVia(ipn *net.IPNet, via *netlink.Via, dev netlink.Link) error {
	return netlink.RouteAdd(&netlink.Route{
		LinkIndex: dev.Attrs().Index,
		Scope:     netlink.SCOPE_HOST,
		Dst:       ipn,
		Via:       via,
	})
}

// forked from plugins/pkg/ip/route_linux.go
func AddDefaultRoute(gw net.IP, dev netlink.Link) error {
	_, defNet, _ := net.ParseCIDR("0.0.0.0/0")
	return AddRoute(defNet, gw, dev)
}

// forked from /plugins/pkg/ip/link_linux.go
// RandomVethName returns string "veth" with random prefix (hashed from entropy)
func RandomVethName() (string, error) {
	entropy := make([]byte, 4)
	_, err := rand.Read(entropy)
	if err != nil {
		return "", fmt.Errorf("failed to generate random veth name: %v", err)
	}

	// NetworkManager (recent versions) will ignore veth devices that start with "veth"
	return fmt.Sprintf("veth%x", entropy), nil
}

func SetIptablesForBridgeToForwordAccept(br *netlink.Bridge) error {
	ipt, err := iptables.NewWithProtocol(iptables.ProtocolIPv4)
	if err != nil {
		utils.WriteLog("这里 NewWithProtocol 失败, err: ", err.Error())
		return err
	}
	err = ipt.Append("filter", "FORWARD", "-i", br.Attrs().Name, "-j", "ACCEPT")
	if err != nil {
		utils.WriteLog("这里 ipt.Append 失败, err: ", err.Error())
		return err
	}
	return nil
}
