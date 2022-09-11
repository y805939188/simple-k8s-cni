package watcher

import (
	"encoding/json"
	"os"
	"testcni/etcd"
	"testcni/ipam"
	"testcni/utils"
	"time"

	"go.etcd.io/etcd/api/v3/mvccpb"
)

type WatcherProcess struct {
	ipam                *ipam.IpamService
	etcd                *etcd.EtcdClient
	watcher             *etcd.Watcher
	subnetRecordHandler etcd.WatchCallback
	isWatching          bool
	watchingMap         map[string]bool
	mapsPath            string
}

type Handlers struct {
	HostnameAndSubnetMapsHandler etcd.WatchCallback
	SubnetRecordHandler          etcd.WatchCallback
}

func (wp *WatcherProcess) doWatch(promise []string) {
	for _, path := range promise {
		wp.watcher.Watch(path, wp.subnetRecordHandler)
		wp.watchingMap[path] = true
		time.Sleep(1 * time.Second)
	}
}

// watching 是监听中的 hostname 和网段的映射, promise 是要希望要被监听的地址
func (wp *WatcherProcess) getShouldWatchPath(watching map[string]bool, promise map[string]string) ([]string, error) {
	res := []string{}
	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}

	for _, v := range promise {
		// 不用监听自己这台主机
		if hostname == v {
			continue
		}
		path, err := wp.ipam.Get().RecordPathByHost(v)
		if err != nil {
			return nil, err
		}
		// 看该 ip 当前是否已经被监听
		if watched, ok := watching[path]; ok && watched {
			continue
		}
		res = append(res, path)
	}
	return res, nil
}

func (wp *WatcherProcess) StartWatch() (func(), error) {
	if wp.isWatching {
		return wp.CancelWatch, nil
	}
	if len(wp.mapsPath) == 0 {
		return utils.Noop, nil
	}

	// 获取所有被分配出去的网段以及对应的 hostname
	maps, err := wp.ipam.Get().HostSubnetMap()
	if err != nil {
		return utils.Noop, err
	}

	paths, err := wp.getShouldWatchPath(wp.watchingMap, maps)
	if err != nil {
		return utils.Noop, err
	}
	// 开始监听这些路径
	wp.doWatch(paths)

	// 然后再开始监听 hostname 和网段关系映射的地址
	wp.watcher.Watch(wp.mapsPath, func(_type mvccpb.Event_EventType, key, value []byte) {
		// 每次监听到 maps 路径的变化时应该就多监听一个新加进来的 key
		newMaps := map[string]string{}
		err := json.Unmarshal(value, &newMaps)
		if err != nil {
			return
		}
		paths, err := wp.getShouldWatchPath(wp.watchingMap, newMaps)
		if err != nil {
			return
		}
		wp.doWatch(paths)
	})
	return wp.CancelWatch, nil
}

func (wp *WatcherProcess) CancelWatch() {
	wp.isWatching = false
	cancel := wp.watcher.Cancel
	cancel()
}

var GetWatcher = func() func(ipam *ipam.IpamService, etcd *etcd.EtcdClient, handlers *Handlers) (*WatcherProcess, error) {
	var wp *WatcherProcess
	return func(ipam *ipam.IpamService, etcd *etcd.EtcdClient, handlers *Handlers) (*WatcherProcess, error) {
		if wp != nil {
			return wp, nil
		}
		wp = &WatcherProcess{
			ipam:                ipam,
			etcd:                etcd,
			watchingMap:         map[string]bool{},
			subnetRecordHandler: handlers.SubnetRecordHandler,
		}

		mapsPath, err := ipam.Get().HostSubnetMapPath()
		if err != nil {
			return nil, err
		}
		wp.mapsPath = mapsPath

		watcher, err := etcd.GetWatcher()
		if err != nil {
			return nil, err
		}
		wp.watcher = watcher
		return wp, nil
	}
}()
