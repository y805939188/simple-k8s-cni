package watcher

import (
	"fmt"

	"go.etcd.io/etcd/api/v3/mvccpb"
)

func RecordSyncProcessor(_type mvccpb.Event_EventType, key, value []byte) {
	fmt.Printf("进到了 Processor: %s, %q, %q\n", _type, key, value)
	/**
	 * 进到这里, 一定是监听到了其他节点上的网段已经对应的 pod ip 的关系变化
	 * 比如其他节点添加了或者删除某个 pod, 这里能感知到其变化
	 * 将其存入到 POD_MAP_DEFAULT_PATH 中
	 */

}
