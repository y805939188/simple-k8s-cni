package watcher

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testcni/consts"
	"testcni/ipam"
	bpfmap "testcni/plugins/vxlan/map"
	"testcni/utils"

	"go.etcd.io/etcd/api/v3/mvccpb"
)

type tmpKV struct {
	key   bpfmap.PodNodeMapKey
	value bpfmap.PodNodeMapValue
}

const TEST_CNI_DEFAULT_DEAMON_HEALTH = "/childprocess/health"

func startHealthServer() {
	http.HandleFunc(consts.DEFAULT_TEST_CNI_API+TEST_CNI_DEFAULT_DEAMON_HEALTH, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK"))
	})
	err := http.ListenAndServe(":"+consts.DEFAULT_TMP_PORT, nil)
	if err != nil {
		utils.WriteLog("(RecordSyncProcessor) 启动监听子进程失败: ", err.Error())
	}
}

func getHostnameFromKey(key string) string {
	tmp := utils.GetParentDirectory(key)
	tmpArr := strings.Split(tmp, "/")
	return tmpArr[len(tmpArr)-1]
}

func getIpsFromValue(value string) []string {
	return strings.Split(value, ";")
}

// 这里的 initData 是 map[ip]hostname 的形式
func getBatchMapKV(ipam *ipam.IpamService, initData map[string]string) []tmpKV {
	res := []tmpKV{}

	for k, v := range initData {
		hostIp, err := ipam.Get().NodeIp(v)
		if err != nil {
			utils.WriteLog("(RecordSyncProcessor) 获取 host ip 失败")
			return nil
		}
		res = append(res, tmpKV{
			key: bpfmap.PodNodeMapKey{
				IP: utils.InetIpToUInt32(k),
			},
			value: bpfmap.PodNodeMapValue{
				IP: utils.InetIpToUInt32(hostIp),
			},
		})
	}
	return res
}

func transformTmpKV2PodNodeMapKV(tmp []tmpKV) ([]bpfmap.PodNodeMapKey, []bpfmap.PodNodeMapValue) {
	keys := []bpfmap.PodNodeMapKey{}
	values := []bpfmap.PodNodeMapValue{}
	for _, item := range tmp {
		keys = append(keys, item.key)
		values = append(values, item.value)
	}
	return keys, values
}

func InitRecordSyncProcessor(ipam *ipam.IpamService, initData map[string]string) func(_type mvccpb.Event_EventType, key, value []byte) {
	mm, err := bpfmap.GetMapsManager()
	if err != nil {
		utils.WriteLog("(RecordSyncProcessor) 获取 bpf maps manager 失败: ", err.Error())
		return nil
	}
	_, err = mm.CreatePodMap()
	if err != nil {
		utils.WriteLog("(RecordSyncProcessor) 创建 pod map 失败: ", err.Error())
		return nil
	}
	// 获取当前 etcd 中已经存在的 node 和 pod ip 的对应关系
	prevData := getBatchMapKV(ipam, initData)
	// 然后转成 keys 和 values 的数据
	prevKeys, prevValues := transformTmpKV2PodNodeMapKV(prevData)
	// 批量更新到本地 ebpf map
	res, err := mm.BatchSetPodMap(prevKeys, prevValues)
	if err != nil {
		utils.WriteLog("(RecordSyncProcessor) 批量初始化 node-pod maps 失败: ", err.Error())
		return nil
	}
	utils.WriteLog("(RecordSyncProcessor) 初始化 node-pod maps 成功, 数量: ", strconv.Itoa(res))
	return func(_type mvccpb.Event_EventType, key, value []byte) {
		utils.WriteLog(fmt.Sprintf("进到了 Processor: %s, %q, %q\n", _type, key, value))
		/**
		 * 进到这里, 一定是监听到了其他节点上的网段已经对应的 pod ip 的关系变化
		 * 比如其他节点添加了或者删除某个 pod, 这里能感知到其变化
		 * 将其存入到 POD_MAP_DEFAULT_PATH 中
		 */
		// 先从 key 中拿到 hostname
		hostname := getHostnameFromKey(string(key))
		if hostname == "" {
			utils.WriteLog("(RecordSyncProcessor) 获取 hostname 失败")
			return
		}
		// 从 value 获取到这次更新的 node 对应的所有 pod ip 地址
		ips := getIpsFromValue(string(value))
		// 存储格式 map[ip]hostname
		_maps := map[string]string{}
		for _, ip := range ips {
			_maps[ip] = hostname
		}
		// 把当前的 ips 和 node ip 的对应关系搞出来
		prevData = getBatchMapKV(ipam, _maps)
		// 然后转成 keys 和 values 的数据
		prevKeys, prevValues = transformTmpKV2PodNodeMapKV(prevData)

		mm, err := bpfmap.GetMapsManager()
		if err != nil {
			utils.WriteLog("(RecordSyncProcessor) 获取 bpf maps manager 失败: ", err.Error())
			return
		}

		// 先删除当前 pod node map 中的所有
		_, err = mm.DeleteAllPodMap()
		if err != nil {
			utils.WriteLog("(RecordSyncProcessor) 批量删除上次 key 失败: ", err.Error())
			// return
		}

		// 然后再把本次的都批量更新到 map
		_, err = mm.BatchSetPodMap(prevKeys, prevValues)
		if err != nil {
			utils.WriteLog("(RecordSyncProcessor) 批量更新 node-pod maps 失败: ", err.Error())
			return
		}
		utils.WriteLog("(RecordSyncProcessor) 更新 node-pod maps 成功, 数量: ", strconv.Itoa(res))
	}
}
