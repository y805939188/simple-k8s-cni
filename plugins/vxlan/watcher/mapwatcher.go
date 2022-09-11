package watcher

import (
	"fmt"
	"testcni/etcd"
	"testcni/ipam"
	"testcni/utils"
)

func StartMapWatcher(ipam *ipam.IpamService, etcd *etcd.EtcdClient) error {
	/**
	 * 这里要负责监听各个节点的变换
	 * 并把得到的结果给塞到 ebpf 的 map 中
	 */

	handlers := &Handlers{
		SubnetRecordHandler: RecordSyncProcessor,
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
