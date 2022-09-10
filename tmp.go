package main

import (
	"fmt"
	"testcni/ipam"
)

// 用来临时清理一些东西
func Tmp_test_clear_etcd() {
	ipam.Init("192.168.0.0", "16")
	ipam.Init("10.244.0.0", "16")
	is, err := ipam.GetIpamService()
	if err != nil {
		fmt.Println("ipam 初始化失败: ", err.Error())
		return
	}
	err = is.EtcdClient.Del("/testcni/ipam/10.244.0.0/16/ding-net-master")
	if err != nil {
		fmt.Println("删除 /testcni/ipam/10.244.0.0/16/ding-net-master 失败: ", err.Error())
		return
	}

	err = is.EtcdClient.Del("/testcni/ipam/10.244.0.0/16/pool")
	if err != nil {
		fmt.Println("删除 /testcni/ipam/10.244.0.0/16/pool 失败: ", err.Error())
		return
	}

	err = is.EtcdClient.Del("/testcni/ipam/testcni/ipam/10.244.0.0/16/ding-net-master/10.244.0.0")
	if err != nil {
		fmt.Println("删除 /testcni/ipam/testcni/ipam/10.244.0.0/16/ding-net-master/10.244.0.0 失败: ", err.Error())
		return
	}

	err = is.EtcdClient.Del("/testcni/ipam/192.168.0.0/16/ding-net-master")
	if err != nil {
		fmt.Println("删除 /testcni/ipam/192.168.0.0/16/ding-net-master 失败: ", err.Error())
		return
	}
	err = is.EtcdClient.Del("/testcni/ipam/192.168.0.0/16/pool")
	if err != nil {
		fmt.Println("/testcni/ipam/192.168.0.0/16/pool 失败: ", err.Error())
		return
	}
	err = is.EtcdClient.Del("/testcni/ipam/192.168.0.0/16/ding-net-node-1")
	if err != nil {
		fmt.Println("删除 /testcni/ipam/192.168.0.0/16/ding-net-node-1 失败: ", err.Error())
		return
	}
	err = is.EtcdClient.Del("/testcni/ipam/testcni/ipam/192.168.0.0/16/ding-net-master/192.168.0.0")
	if err != nil {
		fmt.Println("/testcni/ipam/testcni/ipam/192.168.0.0/16/ding-net-master/192.168.0.0 失败: ", err.Error())
		return
	}
	err = is.EtcdClient.Del("/testcni/ipam/testcni/ipam/192.168.0.0/16/ding-net-node-1/192.168.1.0")
	if err != nil {
		fmt.Println("/testcni/ipam/testcni/ipam/192.168.0.0/16/ding-net-node-1/192.168.1.0 失败: ", err.Error())
		return
	}
}
