package xvlan_bash

import (
	"errors"
	"fmt"
	"testcni/cni"
	"testcni/ipam"
	"testcni/nettools"
	"testcni/skel"
	"testcni/utils"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/vishvananda/netlink"
)

type xvlan_mode int

const (
	MODE_IPVLAN xvlan_mode = iota
	MODE_MACVlan
)

func initEveryClient(args *skel.CmdArgs, pluginConfig *cni.PluginConf) (*ipam.IpamService, error) {
	if pluginConfig.IPAM == nil {
		return nil, errors.New("a range of ip addresses must be specified in the ipvlan mode")
	}
	if pluginConfig.IPAM.RangeStart == "" || pluginConfig.IPAM.RangeEnd == "" {
		return nil, errors.New("a range of ip addresses must be specified in the ipvlan mode")
	}

	if !utils.CheckIP(pluginConfig.IPAM.RangeStart) || !utils.CheckIP(pluginConfig.IPAM.RangeEnd) {
		return nil, errors.New("ipam's ip address is invalid")
	}

	ipam.Init(pluginConfig.Subnet, &ipam.IPAMOptions{
		RangeStart: pluginConfig.IPAM.RangeStart,
		RangeEnd:   pluginConfig.IPAM.RangeEnd,
	})
	ipam, err := ipam.GetIpamService()
	if err != nil {
		return nil, fmt.Errorf("failed to init ipam client: %s", err.Error())
	}

	return ipam, nil
}

func SetXVlanDevice(
	mode xvlan_mode,
	args *skel.CmdArgs,
	pluginConfig *cni.PluginConf,
) (string, string, error) {
	// 初始化 ipam
	ipamClient, err := initEveryClient(args, pluginConfig)
	if err != nil {
		return "", "", err
	}

	// 获取本机网卡信息
	currentNetwork, err := ipamClient.Get().HostNetwork()
	if err != nil {
		return "", "", err
	}

	// 创建一个 ipvlan 设备
	ifname := ""
	if mode == MODE_IPVLAN {
		ifname = "ipvlan"
	} else {
		ifname = "macvlan"
	}

	var device netlink.Link
	if mode == MODE_IPVLAN {
		device, err = nettools.CreateIPVlan(ifname, currentNetwork.Name)
		if err != nil {
			return "", "", err
		}
	} else {
		device, err = nettools.CreateMacVlan(ifname, currentNetwork.Name)
		if err != nil {
			return "", "", err
		}
	}

	// 获取到 netns
	netns, err := ns.GetNS(args.Netns)
	if err != nil {
		return "", "", err
	}

	// 把这个 ipvlan 设备塞到 netns 中
	err = nettools.SetDeviceToNS(device, netns)
	if err != nil {
		return "", "", err
	}

	// 获取一个未使用的 ip 地址
	ip, err := ipamClient.Get().UnusedIP()
	if err != nil {
		return "", "", err
	}

	subnet, err := ipamClient.Get().Subnet()
	if err != nil {
		return "", "", err
	}
	err = netns.Do(func(hostNs ns.NetNS) error {
		_device, err := netlink.LinkByName(device.Attrs().Name)
		if err != nil {
			return err
		}

		mask, err := ipamClient.Get().MaskSegment()
		if err != nil {
			return err
		}
		ip = fmt.Sprintf("%s/%s", ip, mask)
		// 设置 ip 给这个 ipvlan 设备
		err = nettools.SetIpForIPVlan(_device.Attrs().Name, ip)
		if err != nil {
			return err
		}
		// 启动这个 ipvlan 设备
		return nettools.SetUpIPVlan(_device.Attrs().Name)
	})

	return ip, subnet, err
}
