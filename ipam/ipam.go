package ipam

/**
 * 可通过命令查看 etcd 集群状态
 * ETCDCTL_API=3 etcdctl --endpoints https://192.168.98.143:2379 --cacert /etc/kubernetes/pki/etcd/ca.crt --cert /etc/kubernetes/pki/etcd/healthcheck-client.crt --key /etc/kubernetes/pki/etcd/healthcheck-client.key get / --prefix --keys-only
 */

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testcni/client"
	"testcni/consts"
	"testcni/etcd"
	"testcni/helper"
	"testcni/utils"

	"github.com/vishvananda/netlink"
	oriEtcd "go.etcd.io/etcd/client/v3"
)

const (
	prefix = "testcni/ipam"
)

type Get struct {
	etcdClient *etcd.EtcdClient
	k8sClient  *client.LightK8sClient
}
type Release struct {
	etcdClient *etcd.EtcdClient
	k8sClient  *client.LightK8sClient
}
type Set struct {
	etcdClient *etcd.EtcdClient
	k8sClient  *client.LightK8sClient
}

type operators struct {
	Get     *Get
	Set     *Set
	Release *Release
}

type operator struct {
	*operators
}

type Network struct {
	Name          string
	IP            string
	Hostname      string
	CIDR          string
	IsCurrentHost bool
}

type IpamService struct {
	Subnet             string
	MaskSegment        string
	MaskIP             string
	PodMaskSegment     string
	PodMaskIP          string
	CurrentHostNetwork string
	EtcdClient         *etcd.EtcdClient
	K8sClient          *client.LightK8sClient
	*operator
}

var _lock sync.Mutex
var _isLocking bool

func unlock() {
	if _isLocking {
		_lock.Unlock()
		_isLocking = false
	}
}

func lock() {
	if !_isLocking {
		_lock.Lock()
		_isLocking = true
	}
}

func getEtcdClient() *etcd.EtcdClient {
	etcd.Init()
	etcdClient, err := etcd.GetEtcdClient()
	if err != nil {
		return nil
	}
	return etcdClient
}

func getLightK8sClient() *client.LightK8sClient {
	paths, err := helper.GetHostAuthenticationInfoPath()
	if err != nil {
		utils.WriteLog("GetHostAuthenticationInfoPath 执行失败")
		return nil
	}
	client.Init(paths.CaPath, paths.CertPath, paths.KeyPath)
	k8sClient, err := client.GetLightK8sClient()
	if err != nil {
		return nil
	}
	return k8sClient
}

func getIpamSubnet() string {
	ipam, _ := GetIpamService()
	return ipam.Subnet
}

func getIpamMaskSegment() string {
	ipam, _ := GetIpamService()
	return ipam.MaskSegment
}

func getIpamMaskIP() string {
	ipam, _ := GetIpamService()
	return ipam.MaskIP
}

func getHostPath() string {
	hostname, err := os.Hostname()
	if err != nil {
		// fmt.Println("获取主机名失败: ", err.Error())
		return "/test-error-path"
	}
	return getEtcdPathWithPrefix("/" + getIpamSubnet() + "/" + getIpamMaskSegment() + "/" + hostname)
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
		_set.k8sClient = getLightK8sClient()
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
		_get.k8sClient = getLightK8sClient()
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
		_release.k8sClient = getLightK8sClient()
		return _release
	}
}()

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
	defer unlock()
	// 先拿到当前主机对应的网段
	currentNetwork, err := s.etcdClient.Get(getHostPath())
	if err != nil {
		return err
	}
	// 拿到当前主机的网段下所有已经使用的 ip
	allUsedIPs, err := s.etcdClient.Get(getRecordPath(currentNetwork))
	if err != nil {
		return err
	}
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
	// s.
	s.etcdClient.Set(getRecordPath(currentNetwork), _tempIPs)
	// return unlock()
	return nil
}

// 根据主机名获取一个当前主机可用的网段
func (is *IpamService) networkInit(hostPath, poolPath string) (string, error) {
	lock()
	defer unlock()
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
	tmpRandom := utils.GetRandomNumber(len(_tempIPs))
	// TODO: 这块还是得想办法加锁
	currentHostNetwork := _tempIPs[tmpRandom]
	newTmpIps := append([]string{}, _tempIPs[0:tmpRandom]...)
	_tempIPs = append(newTmpIps, _tempIPs[tmpRandom+1:]...)
	// 先把 pool 更新一下
	err = is.EtcdClient.Set(poolPath, strings.Join(_tempIPs, ";"))
	if err != nil {
		return "", err
	}
	// 再把这个网段存到对应的这台主机的 key 下
	err = is.EtcdClient.Set(hostPath, currentHostNetwork)
	if err != nil {
		return "", err
	}

	return currentHostNetwork, nil
}

/**
 * 用来初始化 ip 网段池
 * 比如 subnet 是 10.244.0.0, mask 是 24 的话
 * 就会在 etcd 中初始化出一个
 * 	10.244.0.0;10.244.1.0;10.244.2.0;......;10.244.254.0;10.244.255.0
 */
func (is *IpamService) _IPsPoolInit(poolPath string) error {
	lock()
	defer unlock()
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

/**
 * 用来获取集群中全部的 host name
 */
func (g *Get) NodeNames() ([]string, error) {
	defer unlock()
	const _minionsNodePrefix = "/registry/minions/"

	nodes, err := g.etcdClient.GetAllKey(_minionsNodePrefix, oriEtcd.WithKeysOnly(), oriEtcd.WithPrefix())

	if err != nil {
		utils.WriteLog("这里从 etcd 获取全部 nodes key 失败, err: ", err.Error())
		return nil, err
	}

	var res []string
	for _, node := range nodes {
		node = strings.Replace(node, _minionsNodePrefix, "", 1)
		res = append(res, node)
	}
	return res, nil
}

/**
 * 获取集群中全部节点的网络信息
 */
func (g *Get) AllHostNetwork() ([]*Network, error) {
	names, err := g.NodeNames()
	if err != nil {
		return nil, err
	}

	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}

	res := []*Network{}
	for _, name := range names {
		ip, err := g.NodeIp(name)
		if err != nil {
			return nil, err
		}

		cidr, err := g.CIDR(name)
		if err != nil {
			return nil, err
		}

		if name == hostname {
			res = append(res, &Network{
				Hostname:      name,
				IP:            ip,
				IsCurrentHost: true,
				CIDR:          cidr,
			})
		} else {
			res = append(res, &Network{
				Hostname:      name,
				IP:            ip,
				IsCurrentHost: false,
				CIDR:          cidr,
			})
		}
	}
	return res, nil
}

/**
 * 获取本机网卡的信息
 */
func (g *Get) HostNetwork() (*Network, error) {
	// 先拿到本机上所有的网络相关设备
	linkList, err := netlink.LinkList()
	if err != nil {
		return nil, err
	}

	// 先获取一下 ipam
	ipam, err := GetIpamService()
	if err != nil {
		return nil, err
	}
	// 然后拿本机的 hostname
	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}

	// 用这个 hostname 获取本机的 ip
	hostIP, err := ipam.Get().NodeIp(hostname)
	if err != nil {
		return nil, err
	}
	for _, link := range linkList {
		// 就看类型是 device 的
		if link.Type() == "device" {
			// 找每块儿设备的地址信息
			addr, err := netlink.AddrList(link, netlink.FAMILY_V4)
			if err != nil {
				continue
			}
			if len(addr) >= 1 {
				// TODO: 这里其实应该处理一下一块儿网卡绑定了多个 ip 的情况
				// 数组中的每项都是这样的格式 "192.168.98.143/24 ens33"
				_addr := strings.Split(addr[0].String(), " ")
				ip := _addr[0]
				name := _addr[1]
				ip = strings.Split(ip, "/")[0]
				if ip == hostIP {
					// 走到这儿说明主机走的就是这块儿网卡
					return &Network{
						Name:          name,
						IP:            hostIP,
						Hostname:      hostname,
						IsCurrentHost: true,
					}, nil
				}
			}
		}
	}
	return nil, errors.New("no valid network device found")
}

// 获取当前节点被分配到的网段 + mask
func (g *Get) CIDR(hostName string) (string, error) {
	defer unlock()

	_cidrPath := getEtcdPathWithPrefix("/" + getIpamSubnet() + "/" + getIpamMaskSegment() + "/" + hostName)

	etcd := getEtcdClient()
	if etcd == nil {
		return "", errors.New("etcd client not found")
	}

	cidr, err := etcd.Get(_cidrPath)
	if err != nil {
		return "", err
	}

	if cidr == "" {
		return "", fmt.Errorf("the host %s cidr not found", hostName)
	}

	// 先获取一下 ipam
	ipam, err := GetIpamService()
	if err != nil {
		return "", err
	}
	cidr += ("/" + ipam.PodMaskSegment)

	return cidr, nil
}

/**
 * 根据 host name 获取节点 ip
 */
func (g *Get) NodeIp(hostName string) (string, error) {
	defer unlock()
	node, err := g.k8sClient.Get().Node(hostName)
	if err != nil {
		return "", err
	}
	for _, addr := range node.Status.Addresses {
		if addr.Type == "InternalIP" {
			return addr.Address, nil
		}
	}
	return "", errors.New("没有找到 ip")
}

func (g *Get) nextUnusedIP() (string, error) {
	defer unlock()
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

func (g *Get) Gateway() (string, error) {
	defer unlock()
	currentNetwork, err := g.etcdClient.Get(getHostPath())
	if err != nil {
		return "", err
	}

	return utils.InetInt2Ip((utils.InetIP2Int(currentNetwork) + 1)), nil
}

func (g *Get) GatewayWithMaskSegment() (string, error) {
	defer unlock()
	currentNetwork, err := g.etcdClient.Get(getHostPath())
	if err != nil {
		return "", err
	}

	return utils.InetInt2Ip((utils.InetIP2Int(currentNetwork) + 1)) + "/" + getIpamMaskSegment(), nil
}

func (g *Get) AllUsedIPs() ([]string, error) {
	defer unlock()
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
	defer unlock()
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
		// 先把这个 ip 占上坑位
		// 坑位先占上不影响大局
		// 但是如果坑位占晚了被别人抢先的话可能会导致有俩 pod 的 ip 冲突
		err = getSet().IPs(ip)
		if err != nil {
			return "", err
		}
		return ip, nil
	}
}

func (r *Release) IPs(ips ...string) error {
	defer unlock()
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
	defer unlock()
	currentNetwork, err := r.etcdClient.Get(getIPsPoolPath(getIpamSubnet(), getIpamMaskSegment()))
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
	lock()
	return getGet()
}

func (o *operator) Set() *Set {
	lock()
	return getSet()
}

func (o *operator) Release() *Release {
	lock()
	return getRelase()
}

func getEtcdPathWithPrefix(path string) string {
	if path != "" && path[0:1] == "/" {
		return "/" + prefix + path
	}
	return "/" + prefix + "/" + path
}

func getMaskIpFromNum(numStr string) string {
	switch numStr {
	case "8":
		return "255.0.0.0"
	case "16":
		return "255.255.0.0"
	case "24":
		return "255.255.255.0"
	case "32":
		return "255.255.255.255"
	default:
		return "255.255.0.0"
	}
}

var __GetIpamService func() (*IpamService, error)

func _GetIpamService(subnet string, maskSegment ...string) func() (*IpamService, error) {

	return func() (*IpamService, error) {
		var _ipam *IpamService

		if _ipam != nil {
			return _ipam, nil
		} else {
			_subnet := subnet
			var _maskSegment string = consts.DEFAULT_MASK_NUM
			var _podIpMaskSegment string = consts.DEFAULT_MASK_NUM

			if len(maskSegment) > 0 {
				_maskSegment = maskSegment[0]
			}
			if len(maskSegment) > 1 {
				_podIpMaskSegment = maskSegment[1]
			}

			// 配置文件中传参数的时候可能直接传了个子网掩码
			// 传了的话就直接使用这个掩码
			if withMask := strings.Contains(subnet, "/"); withMask {
				subnetAndMask := strings.Split(subnet, "/")
				_subnet = subnetAndMask[0]
				_maskSegment = subnetAndMask[1]
			}

			var _maskIP string = getMaskIpFromNum(_maskSegment)
			var _podMaskIP string = getMaskIpFromNum(_podIpMaskSegment)

			// 如果不是合法的子网地址的话就强转成合法
			// 比如 _subnet 传了个数字过来, 要给它先干成 a.b.c.d 的样子
			// 然后 & maskIP, 给做成类似 a.b.0.0 的样子
			_subnet = utils.InetInt2Ip(utils.InetIP2Int(_subnet) & utils.InetIP2Int(_maskIP))
			_ipam = &IpamService{
				Subnet:         _subnet,           // 子网网段
				MaskSegment:    _maskSegment,      // 掩码 10 进制
				MaskIP:         _maskIP,           // 掩码 ip
				PodMaskSegment: _podIpMaskSegment, // pod 的 mask 10 进制
				PodMaskIP:      _podMaskIP,        // pod 的 mask ip
			}
			_ipam.EtcdClient = getEtcdClient()
			_ipam.K8sClient = getLightK8sClient()
			// 初始化一个 ip 网段的 pool
			// 如果已经初始化过就不再初始化
			poolPath := getEtcdPathWithPrefix("/" + _ipam.Subnet + "/" + _ipam.MaskSegment + "/" + "pool")
			err := _ipam._IPsPoolInit(poolPath)
			if err != nil {
				return nil, err
			}

			// 然后尝试去拿一个当前主机可用的网段
			// 如果拿不到, 里面会尝试创建一个
			hostname, err := os.Hostname()
			if err != nil {
				return nil, err
			}
			hostPath := getEtcdPathWithPrefix("/" + _ipam.Subnet + "/" + _ipam.MaskSegment + "/" + hostname)
			currentHostNetwork, err := _ipam.networkInit(hostPath, poolPath)
			if err != nil {
				return nil, err
			}
			_ipam.CurrentHostNetwork = currentHostNetwork
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

func (is *IpamService) clear() error {
	return is.EtcdClient.Del("/"+prefix, oriEtcd.WithPrefix())
}

func Init(subnet string, maskSegment ...string) func() error {
	if __GetIpamService == nil {
		__GetIpamService = _GetIpamService(subnet, maskSegment...)
	}
	is, err := GetIpamService()
	if err != nil {
		return func() error {
			return err
		}
	}
	return is.clear
}
