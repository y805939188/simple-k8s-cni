package ipam

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testcni/etcd"
	"testcni/utils"
)

const (
	prefix = "testcni/ipam"
)

type Get struct{ etcdClient *etcd.EtcdClient }
type Release struct{ etcdClient *etcd.EtcdClient }
type Set struct{ etcdClient *etcd.EtcdClient }

type operators struct {
	Get     *Get
	Set     *Set
	Release *Release
}

type operator struct {
	lock sync.Mutex
	*operators
}

type IpamService struct {
	Subnet     string
	Mask       string
	MaskIP     string
	EtcdClient *etcd.EtcdClient
	*operator
}

func getEtcdClient() *etcd.EtcdClient {
	etcd.Init()
	etcdClient, err := etcd.GetEtcdClient()
	if err != nil {
		return nil
	}
	return etcdClient
}

func getIpamSubnet() string {
	ipam, _ := GetIpamService()
	return ipam.Subnet
}

func getIpamMask() string {
	ipam, _ := GetIpamService()
	return ipam.Mask
}

func getIpamMaskIP() string {
	ipam, _ := GetIpamService()
	return ipam.MaskIP
}

func getHostPath() string {
	hostname, err := os.Hostname()
	if err != nil {
		fmt.Println("获取主机名失败: ", err.Error())
		return "/test-error-path"
	}
	return getEtcdPathWithPrefix("/" + getIpamSubnet() + "/" + getIpamMask() + "/" + hostname)
}

func getRecordPath(hostNetwork string) string {
	return getEtcdPathWithPrefix(getHostPath() + "/" + hostNetwork)
}

func getIPsPoolPath(subnet, mask string) string {
	return getEtcdPathWithPrefix("/" + subnet + "/" + mask + "/" + "pool")
}

var getSet = func() func() *Set {
	var _set *Set
	return func() *Set {
		if _set != nil {
			return _set
		}
		_set = &Set{}
		_set.etcdClient = getEtcdClient()
		return _set
	}
}()

var getGet = func() func() *Get {
	var _get *Get
	return func() *Get {
		if _get != nil {
			return _get
		}
		_get = &Get{}
		_get.etcdClient = getEtcdClient()
		return _get
	}
}()

var getRelase = func() func() *Release {
	var _release *Release
	return func() *Release {
		if _release != nil {
			return _release
		}
		_release = &Release{}
		_release.etcdClient = getEtcdClient()
		return _release
	}
}()

func unlock() error {
	svc, err := GetIpamService()
	if err != nil {
		return err
	}
	svc.lock.Unlock()
	return nil
}

func isGatewayIP(ip string) bool {
	// 把每个网段的 x.x.x.1 当做网关
	if ip == "" {
		return false
	}
	_arr := strings.Split(ip, ".")
	return _arr[3] == "1"
}

func isRetainIP(ip string) bool {
	// 把每个网段的 x.x.x.0 当做保留
	if ip == "" {
		return false
	}
	_arr := strings.Split(ip, ".")
	return _arr[3] == "0"
}

func (s *Set) IPs(ips ...string) error {
	// 先拿到当前主机对应的网段
	currentNetwork, err := s.etcdClient.Get(getHostPath())
	if err != nil {
		return err
	}
	// 拿到当前主机的网段下所有已经使用的 ip
	allUsedIPs, err := s.etcdClient.Get(getRecordPath(currentNetwork))
	_allUsedIPsArr := strings.Split(allUsedIPs, ";")
	_tempIPs := allUsedIPs
	for _, ip := range ips {
		if _tempIPs == "" {
			_tempIPs = ip
		} else {
			flag := true
			for i := 0; i < len(_allUsedIPsArr); i++ {
				if _allUsedIPsArr[i] == ip {
					// 如果 etcd 上已经存了则不用再写入了
					flag = false
					break
				}
			}
			if flag {
				_tempIPs += ";" + ip
			}
		}
	}
	s.etcdClient.Set(getRecordPath(currentNetwork), _tempIPs)
	// return unlock()
	return nil
}

// 根据主机名获取一个当前主机可用的网段
func (is *IpamService) _NetworkInit(hostPath, poolPath string) (string, error) {
	network, err := is.EtcdClient.Get(hostPath)
	if err != nil {
		return "", err
	}

	// 已经存过该主机对应的网段了
	if network != "" {
		return network, nil
	}

	// 从可用的 ip 池中捞一个
	pool, err := is.EtcdClient.Get(poolPath)
	if err != nil {
		return "", err
	}

	_tempIPs := strings.Split(pool, ";")
	currentHostNetwork := _tempIPs[0]
	_tempIPs = _tempIPs[1:]
	// 捞完这个网段存到对应的这台主机的 key 下
	err = is.EtcdClient.Set(hostPath, currentHostNetwork)
	if err != nil {
		return "", err
	}
	// 然后把 pool 更新一下
	err = is.EtcdClient.Set(poolPath, strings.Join(_tempIPs, ";"))
	if err != nil {
		return "", err
	}
	return currentHostNetwork, nil
}

func (is *IpamService) _IPsPoolInit(poolPath string) error {
	val, err := is.EtcdClient.Get(poolPath)
	if err != nil {
		return err
	}
	if len(val) > 0 {
		return nil
	}
	subnet := is.Subnet
	_temp := strings.Split(subnet, ".")
	_tempIndex := 0
	for _i := 0; _i < len(_temp); _i++ {
		if _temp[_i] == "0" {
			// 找到 subnet 中第一个 0 的位置
			_tempIndex = _i
			break
		}
	}
	// 创建出 255 个备用的网段
	// 每个节点从这些网段中选择一个还没有使用过的
	_tempIpStr := ""
	for _j := 0; _j <= 255; _j++ {
		_temp[_tempIndex] = fmt.Sprintf("%d", _j)
		_newIP := strings.Join(_temp, ".")
		if _tempIpStr == "" {
			_tempIpStr = _newIP
		} else {
			_tempIpStr += ";" + _newIP
		}
	}
	return is.EtcdClient.Set(poolPath, _tempIpStr)
}

func (g *Get) nextUnusedIP() (string, error) {
	currentNetwork, err := g.etcdClient.Get(getHostPath())
	if err != nil {
		return "", err
	}
	allUsedIPs, err := g.etcdClient.Get(getRecordPath(currentNetwork))
	if err != nil {
		return "", err
	}
	if allUsedIPs == "" {
		// 进到这里说明当前主机还没有使用任何一个 ip
		// 因此直接使用 currentNetwork 来生成第一个 ip
		// +2 是因为 currentNetwork 一定是 x.y.z.0 这种最后一位是 0 的格式
		// 一般 x.y.z.1 默认作为网关, 所以 +2 开始是要分发的 ip 地址
		return utils.InetInt2Ip(utils.InetIP2Int(currentNetwork) + 2), nil
	}
	ips := strings.Split(allUsedIPs, ";")
	maxIP := utils.GetMaxIP(ips)
	// 找到当前最大的 ip 然后 +1 就是下一个未使用的
	nextIP := utils.InetInt2Ip(utils.InetIP2Int(maxIP) + 1)
	// return nextIP, unlock()
	return nextIP, nil
}

func (g *Get) AllUsedIPs() ([]string, error) {
	currentNetwork, err := g.etcdClient.Get(getHostPath())
	if err != nil {
		return nil, err
	}
	allUsedIPs, err := g.etcdClient.Get(getRecordPath(currentNetwork))
	if err != nil {
		return nil, err
	}
	ips := strings.Split(allUsedIPs, ";")
	// return ips, unlock()
	return ips, nil
}

func (g *Get) UnusedIP() (string, error) {
	for {
		ip, err := g.nextUnusedIP()
		if err != nil {
			return "", err
		}
		if isGatewayIP(ip) || isRetainIP(ip) {
			err = getSet().IPs(ip)
			if err != nil {
				return "", err
			}
			continue
		}
		return ip, nil
	}
}

func (r *Release) IPs(ips ...string) error {
	currentNetwork, err := r.etcdClient.Get(getHostPath())
	if err != nil {
		return err
	}
	allUsedIPs, err := r.etcdClient.Get(getRecordPath(currentNetwork))
	if err != nil {
		return err
	}
	_allUsedIP := strings.Split(allUsedIPs, ";")
	var _newIPs []string
	for _, usedIP := range _allUsedIP {
		flag := false
		for _, ip := range ips {
			if usedIP == ip {
				flag = true
				break
			}
		}
		if !flag {
			_newIPs = append(_newIPs, usedIP)
		}
	}
	newIPsString := strings.Join(_newIPs, ";")
	err = r.etcdClient.Set(getRecordPath(currentNetwork), newIPsString)
	if err != nil {
		return err
	}
	// return unlock()
	return nil
}

func (r *Release) Pool() error {
	currentNetwork, err := r.etcdClient.Get(getIPsPoolPath(getIpamSubnet(), getIpamMask()))
	if err != nil {
		return err
	}

	err = r.etcdClient.Set(currentNetwork, "")
	if err != nil {
		return err
	}
	// return unlock()
	return nil
}

func (o *operator) Get() *Get {
	// o.lock.Lock()
	return getGet()
}

func (o *operator) Set() *Set {
	// o.lock.Lock()
	return getSet()
}

func (o *operator) Release() *Release {
	// o.lock.Lock()
	return getRelase()
}

func getEtcdPathWithPrefix(path string) string {
	if path != "" && path[0:1] == "/" {
		return "/" + prefix + path
	}
	return "/" + prefix + "/" + path
}

var __GetIpamService func() (*IpamService, error)

func _GetIpamService(subnet string, mask ...string) func() (*IpamService, error) {

	return func() (*IpamService, error) {
		var _ipam *IpamService

		if _ipam != nil {
			return _ipam, nil
		} else {
			_subnet := subnet
			var _mask string

			if len(mask) > 0 {
				_mask = mask[0]
			}

			if withMask := strings.Contains(subnet, "/"); withMask {
				subnetAndMask := strings.Split(subnet, "/")
				_subnet = subnetAndMask[0]
				_mask = subnetAndMask[1]
			}

			var _maskIP string
			switch _mask {
			case "8":
				_maskIP = "255.0.0.0"
				break
			case "16":
				_maskIP = "255.255.0.0"
				break
			case "24":
				_maskIP = "255.255.255.0"
				break
			default:
				_maskIP = "255.255.0.0"
			}

			// 如果不是合法的子网地址的话就强转成合法
			_subnet = utils.InetInt2Ip(utils.InetIP2Int(_subnet) & utils.InetIP2Int(_maskIP))
			_ipam = &IpamService{
				Subnet: _subnet,
				Mask:   _mask,
				MaskIP: _maskIP,
			}
			_ipam.EtcdClient = getEtcdClient()
			// // // // ipamClient.Release().Pool()
			// // _ipam.EtcdClient.Del("//testcni/ipam/192.168.0.0/16/pool", "")
			// _ipam.EtcdClient.Del("/testcni/ipam/192.168.0.0/16/ding-net-master")
			// _ipam.EtcdClient.Del("/testcni/ipam/192.168.0.0/16/pool")
			// // // // _ipam.EtcdClient/
			// return nil, nil
			// 初始化一个 ip 网段的 pool
			// 如果已经初始化过就不再初始化
			poolPath := getEtcdPathWithPrefix("/" + _ipam.Subnet + "/" + _ipam.Mask + "/" + "pool")
			err := _ipam._IPsPoolInit(poolPath)
			if err != nil {
				return nil, err
			}
			// // 初始化一个 host 用来存放 network 的 key
			// err = _ipam.etcdClient.Set(getHostPath(), "")
			// if err != nil {
			// 	return nil, err
			// }
			// 然后尝试去拿一个当前主机可用的网段
			// 如果拿不到, 里面会尝试创建一个
			hostname, err := os.Hostname()
			if err != nil {
				return nil, err
			}
			hostPath := getEtcdPathWithPrefix("/" + _ipam.Subnet + "/" + _ipam.Mask + "/" + hostname)
			_, err = _ipam._NetworkInit(hostPath, poolPath)
			if err != nil {
				return nil, err
			}
			return _ipam, nil
		}
	}
}

func GetIpamService() (*IpamService, error) {
	if __GetIpamService == nil {
		return nil, errors.New("ipam service 需要初始化")
	}

	ipamService, err := __GetIpamService()
	if err != nil {
		return nil, err
	}
	return ipamService, nil
}

func Init(subnet string, mask ...string) {
	if __GetIpamService == nil {
		__GetIpamService = _GetIpamService(subnet, mask...)
	}
}
