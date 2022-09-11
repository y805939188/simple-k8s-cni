package watcher

import (
	"fmt"

	"go.etcd.io/etcd/api/v3/mvccpb"
)

func RecordSyncProcessor(_type mvccpb.Event_EventType, key, value []byte) {
	fmt.Printf("进到了 Processor: %s, %q, %q", _type, key, value)
	fmt.Print("\n")

}
