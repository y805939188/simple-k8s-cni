package etcd

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.etcd.io/etcd/api/v3/mvccpb"
)

func TestIpam(t *testing.T) {
	test := assert.New(t)
	Init()
	client, err := GetEtcdClient()
	test.Nil(err)
	test.NotNil(client.client)
	test.Nil(client.watcher)

	/********** test put **********/
	err = client.Set("/ding-test-1", "111")
	test.Nil(err)
	err = client.Set("/ding-test-2", "222")
	test.Nil(err)

	/********** test get **********/
	res, err := client.Get("/ding-test-1")
	test.Nil(err)
	test.Equal(res, "111")
	res, err = client.Get("/ding-test-2")
	test.Nil(err)
	test.Equal(res, "222")

	/********** test del **********/
	err = client.Del("/ding-test-1")
	test.Nil(err)
	res, err = client.Get("/ding-test-1")
	test.Nil(err)
	test.Equal(res, "")
	err = client.Del("/ding-test-2")
	test.Nil(err)
	res, err = client.Get("/ding-test-2")
	test.Nil(err)
	test.Equal(res, "")

	/********** test watcher **********/
	watcher, err := client.GetWatcher()
	test.Nil(err)
	test.NotNil(watcher)

	err = client.Set("/ding-test-1", "111")
	test.Nil(err)
	err = client.Set("/ding-test-2", "222")
	test.Nil(err)
	err = client.Set("/ding-test-3", "333")
	test.Nil(err)
	wg := sync.WaitGroup{}
	wg.Add(3)
	nums := 0
	// NOTE: 这里建立连接需要一点点时间, 如果刚建立完连接下头马上就 put 的话有可能收不到响应
	watcher.Watch("/ding-test-1", func(_type mvccpb.Event_EventType, key, value []byte) {
		fmt.Printf("%s, %q, %q", _type, key, value)
		fmt.Print("\n")
		nums++
		wg.Done()
	})
	watcher.Watch("/ding-test-2", func(_type mvccpb.Event_EventType, key, value []byte) {
		fmt.Printf("%s, %q, %q", _type, key, value)
		fmt.Print("\n")
		nums++
		wg.Done()
	})
	watcher.Watch("/ding-test-3", func(_type mvccpb.Event_EventType, key, value []byte) {
		fmt.Printf("%s, %q, %q", _type, key, value)
		fmt.Print("\n")
		nums++
		wg.Done()
	})
	time.Sleep(1 * time.Second)
	client.Set("/ding-test-1", "666")
	time.Sleep(1 * time.Second)
	client.Set("/ding-test-2", "777")
	time.Sleep(1 * time.Second)
	client.Set("/ding-test-3", "888")
	wg.Wait()
	test.Equal(3, nums)
	res, err = client.Get("/ding-test-1")
	test.Nil(err)
	test.Equal(res, "666")
	res, err = client.Get("/ding-test-2")
	test.Nil(err)
	test.Equal(res, "777")
	res, err = client.Get("/ding-test-3")
	test.Nil(err)
	test.Equal(res, "888")
}
