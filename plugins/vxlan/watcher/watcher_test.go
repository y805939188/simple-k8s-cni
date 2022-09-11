package watcher

import (
	"fmt"
	"sync"
	"testcni/etcd"
	"testcni/ipam"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.etcd.io/etcd/api/v3/mvccpb"
)

func TestIpam(t *testing.T) {
	test := assert.New(t)
	// clear := ipam.Init("10.244.0.0", "16", "32")
	ipam.Init("10.244.0.0", "16", "32")
	etcd.Init()
	i, err := ipam.GetIpamService()
	test.Nil(err)
	test.NotNil(i)
	e, err := etcd.GetEtcdClient()
	test.Nil(err)
	test.NotNil(e)

	wg := sync.WaitGroup{}
	wg.Add(2)
	nums := 0
	var testHandler = func(_type mvccpb.Event_EventType, key, value []byte) {
		fmt.Printf("%s, %q, %q\n", _type, key, value)
		nums++
		wg.Done()
	}

	testHandlers := &Handlers{
		SubnetRecordHandler: testHandler,
	}
	w, err := GetWatcher(i, e, testHandlers)
	test.Nil(err)
	test.NotNil(w)
	w.StartWatch()
	// hostname, err := os.Hostname()
	test.Nil(err)
	// 增加一个 /testcni/ipam/10.244.0.0/16/cni-test-666: 1.1.1.1
	e.Set("/testcni/ipam/10.244.0.0/16/cni-test-666", "1.1.1.1")
	// /testcni/ipam/10.244.0.0/16/maps: {1.1.1.1: cni-test-666, 10.244.71.6: cni-test-1}
	e.Set("/testcni/ipam/10.244.0.0/16/maps", "{\"1.1.1.1\":\"cni-test-666\",\"10.244.71.6\":\"cni-test-1\"}")
	// 增加一个 /testcni/ipam/10.244.0.0/16/cni-test-666/1.1.1.1: 2.2.2.2
	e.Set("/testcni/ipam/10.244.0.0/16/cni-test-666/1.1.1.1", "2.2.2.2")
	e.Set("/testcni/ipam/10.244.0.0/16/cni-test-666/1.1.1.1", "3.4.5.6")
	wg.Wait()
	test.Equal(nums, 2)
	// clear()
}
