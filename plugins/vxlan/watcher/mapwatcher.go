package watcher

import (
	"fmt"
	"testcni/etcd"
	"testcni/ipam"
	"testcni/utils"
)

func getAllInitPath(ipam *ipam.IpamService) (map[string]string, error) {
	networks, err := ipam.Get().AllHostNetwork()
	if err != nil {
		return nil, err
	}
	maps := map[string]string{}
	for _, network := range networks {
		if network.IsCurrentHost {
			continue
		}
		ips, err := ipam.Get().RecordByHost(network.Hostname)
		if err != nil {
			return nil, err
		}
		for _, ip := range ips {
			maps[ip] = network.Hostname
		}
	}
	return maps, nil
}

func StartMapWatcher(ipam *ipam.IpamService, etcd *etcd.EtcdClient) error {
	/**
	 * 这里要负责监听各个节点的变换
	 * 并把得到的结果给塞到 ebpf 的 map 中
	 */

	// 先去获取其他节点所有的 ip 地址
	initMaps, err := getAllInitPath(ipam)
	if err != nil {
		return err
	}
	handlers := &Handlers{
		SubnetRecordHandler: InitRecordSyncProcessor(ipam, initMaps),
	}
	watcher, err := GetWatcher(ipam, etcd, handlers)
	if err != nil {
		return err
	}

	child := utils.StartDeamon(func() {
		watcher.StartWatch()
	})
	utils.WriteLog(fmt.Sprintf("启动的守护进程是: %d", child.Pid))
	return nil
}
