package ipam

/**
 * 可通过命令查看 etcd 集群状态
 * ETCDCTL_API=3 etcdctl --endpoints https://192.168.98.143:2379 --cacert /etc/kubernetes/pki/etcd/ca.crt --cert /etc/kubernetes/pki/etcd/healthcheck-client.crt --key /etc/kubernetes/pki/etcd/healthcheck-client.key get / --prefix --keys-only
 */

import (
	"encoding/json"
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
	// 有些不会发生改变的东西可以做缓存
	nodeIpCache map[string]string
	cidrCache   map[string]string
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

type IPAMOptions struct {
	MaskSegment      string
	PodIpMaskSegment string
	RangeStart       string
	RangeEnd         string
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

func getHostPath() string {
	hostname, err := os.Hostname()
	if err != nil {
		return "/test-error-path"
	}
	return getEtcdPathWithPrefix("/" + getIpamSubnet() + "/" + getIpamMaskSegment() + "/" + hostname)
}

func getRecordPath(hostNetwork string) string {
	return getHostPath() + "/" + hostNetwork
}

func getIpRangesPath(network string) string {
	return getHostPath() + "/" + network + "/range"
}

func getIPsPoolPath(subnet, mask string) string {
	return getEtcdPathWithPrefix("/" + subnet + "/" + mask + "/" + "pool")
}

func (g *Get) MaskSegment() (string, error) {
	ipam, err := GetIpamService()
	if err != nil {
		return "", err
	}
	return ipam.MaskSegment, nil
}

func (g *Get) HostSubnetMapPath() (string, error) {
	ipam, err := GetIpamService()
	if err != nil {
		return "", err
	}
	m := fmt.Sprintf("/%s/%s/maps", ipam.Subnet, ipam.MaskSegment)
	return getEtcdPathWithPrefix(m), nil
}

func (g *Get) HostSubnetMap() (map[string]string, error) {
	ipam, err := GetIpamService()
	if err != nil {
		return nil, err
	}
	return ipam.getHostSubnetMap()
}

func (g *Get) RecordPathByHost(hostname string) (string, error) {
	cidr, err := g.CIDR(hostname)
	if err != nil {
		return "", err
	}
	subnetAndMask := strings.Split(cidr, "/")
	if len(subnetAndMask) > 1 {
		path := fmt.Sprintf("/%s/%s/%s/%s", getIpamSubnet(), getIpamMaskSegment(), hostname, subnetAndMask[0])
		return getEtcdPathWithPrefix(path), nil
	}
	return "", errors.New("can not get subnet address")
}

func (g *Get) CurrentSubnet() (string, error) {
	ipam, err := GetIpamService()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/%s", ipam.Subnet, ipam.MaskSegment), nil
}

func (g *Get) RecordByHost(hostname string) ([]string, error) {
	path, err := g.RecordPathByHost(hostname)
	if err != nil {
		return nil, err
	}
	str, err := g.etcdClient.Get(path)
	if err != nil {
		return nil, err
	}
	return strings.Split(str, ";"), nil
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
		_get = &Get{
			cidrCache:   map[string]string{},
			nodeIpCache: map[string]string{},
		}
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

/**
 * 将参数的 ips 设置到 etcd 中
 */
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

	return s.etcdClient.Set(getRecordPath(currentNetwork), _tempIPs)
}

// 根据主机名获取一个当前主机可用的网段
func (is *IpamService) networkInit(hostPath, poolPath string, ranges ...string) (string, error) {
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

	// 如果传了 ip 地址的 range 的话就创建一个 range 目录
	start := ""
	end := ""
	switch len(ranges) {
	case 1:
		start = ranges[0]
	case 2:
		start = ranges[0]
		end = ranges[1]
	}

	if start != "" && end != "" {
		ranges := utils.GenIpRange(start, end)
		if ranges != nil {
			currentIpRanges := strings.Join(utils.GenIpRange(start, end), ";")
			err = is.EtcdClient.Set(fmt.Sprintf(
				"%s/%s/range",
				hostPath,
				currentHostNetwork,
			), currentIpRanges)
			if err != nil {
				return "", err
			}
		}
	}

	return currentHostNetwork, nil
}

// 获取主机名和网段的映射
func (is *IpamService) getHostSubnetMap() (map[string]string, error) {
	path, err := is.Get().HostSubnetMapPath()
	if err != nil {
		return nil, err
	}

	_maps, err := is.EtcdClient.Get(path)
	if err != nil {
		return nil, err
	}

	resMaps := map[string]string{}
	err = json.Unmarshal(([]byte)(_maps), &resMaps)
	if err != nil {
		return nil, err
	}
	return resMaps, nil
}

// 初始化
func (is *IpamService) subnetMapInit(subnet, mask, hostname, currentSubnet string) error {
	lock()
	defer unlock()
	m := fmt.Sprintf("/%s/%s/maps", subnet, mask)
	path := getEtcdPathWithPrefix(m)
	maps, err := is.EtcdClient.Get(path)
	if err != nil {
		return err
	}

	if len(maps) == 0 {
		_maps := map[string]string{}
		_maps[currentSubnet] = hostname
		mapsStr, err := json.Marshal(_maps)
		if err != nil {
			return err
		}
		return is.EtcdClient.Set(path, string(mapsStr))
	}

	_tmpMaps := map[string]string{}
	err = json.Unmarshal(([]byte)(maps), &_tmpMaps)
	if err != nil {
		return err
	}

	if _, ok := _tmpMaps[currentSubnet]; ok {
		return nil
	}
	_tmpMaps[currentSubnet] = hostname
	mapsStr, err := json.Marshal(_tmpMaps)
	if err != nil {
		return err
	}
	return is.EtcdClient.Set(path, string(mapsStr))
}

/**
 * 用来初始化 ip 网段池
 * 比如 subnet 是 10.244.0.0, mask 是 24 的话
 * 就会在 etcd 中初始化出一个
 * 	10.244.0.0;10.244.1.0;10.244.2.0;......;10.244.254.0;10.244.255.0
 */
func (is *IpamService) ipsPoolInit(poolPath string) error {
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
	/**
	 * FIXME: 对于子网网段的创建, 其实可以不完全是 8 的倍数
	 * 比如 10.244.0.0/26 这种其实也可以
	 */
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
 * 这里直接从 etcd 的 key 下边查
 * 不调 k8s 去捞, k8s 捞一次出来的东西太多了
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
 * 获取集群中除了本机以外的全部节点的网络信息
 */
func (g *Get) AllOtherHostNetwork() ([]*Network, error) {
	networks, err := g.AllHostNetwork()
	if err != nil {
		return nil, err
	}
	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}
	result := []*Network{}
	for _, network := range networks {
		if network.Hostname == hostname {
			continue
		}
		result = append(result, network)
	}
	return result, nil
}

/**
 * 获取集群中除了本机以外的全部节点的 ip
 */
func (g *Get) AllOtherHostIP() (map[string]string, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}
	nodes, err := g.k8sClient.Get().Nodes()
	if err != nil {
		return nil, err
	}

	result := make(map[string]string, len(nodes.Items)-1)
	for _, node := range nodes.Items {
		ip := ""
		_hostname := ""
	iternal:
		for _, addr := range node.Status.Addresses {
			if addr.Type == "Hostname" {
				if addr.Address == hostname {
					ip = ""
					_hostname = ""
					break iternal
				}
				_hostname = addr.Address
			}
			if addr.Type == "InternalIP" {
				ip = addr.Address
			}
		}
		if ip != "" {
			result[_hostname] = ip
		}
	}
	return result, nil
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
	if val, ok := g.cidrCache[hostName]; ok {
		return val, nil
	}
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
		return "", nil
	}

	// 先获取一下 ipam
	ipam, err := GetIpamService()
	if err != nil {
		return "", err
	}
	cidr += ("/" + ipam.PodMaskSegment)
	g.cidrCache[hostName] = cidr
	return cidr, nil
}

/**
 * 根据 host name 获取节点 ip
 */
func (g *Get) NodeIp(hostName string) (string, error) {
	defer unlock()
	if val, ok := g.nodeIpCache[hostName]; ok {
		return val, nil
	}
	node, err := g.k8sClient.Get().Node(hostName)
	if err != nil {
		return "", err
	}
	for _, addr := range node.Status.Addresses {
		if addr.Type == "InternalIP" {
			g.nodeIpCache[hostName] = addr.Address
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

	ipsMap := map[string]bool{}
	ips := strings.Split(allUsedIPs, ";")
	for _, ip := range ips {
		ipsMap[ip] = true
	}
	a := getIpRangesPath(currentNetwork)
	fmt.Println(a)
	if rangesPathExist, err := g.etcdClient.GetKey(getIpRangesPath(currentNetwork)); rangesPathExist != "" && err == nil {
		if rangesIPs, err := g.etcdClient.Get(getIpRangesPath(currentNetwork)); err == nil {
			rangeIpsArr := strings.Split(rangesIPs, ";")
			if len(rangeIpsArr) == 0 {
				return "", errors.New("all of the ips are used")
			}
			nextIp := ""
			for {
				nextIp = ""
				for i, ip := range rangeIpsArr {
					if utils.GetRandomNumber(i+1) == 0 {
						nextIp = ip
					}
				}
				if _, ok := ipsMap[nextIp]; !ok {
					break
				}
			}

			return nextIp, nil
		}
	}

	gw, err := g.Gateway()
	if err != nil {
		return "", err
	}
	nextIp := ""
	gwNum := utils.InetIP2Int(gw)
	for {
		n := utils.GetRandomNumber(254)
		if n == 0 || n == 1 {
			continue
		}
		nextIpNum := gwNum + int64(n)
		nextIp = utils.InetInt2Ip(nextIpNum)
		if _, ok := ipsMap[nextIp]; !ok {
			break
		}
	}

	return nextIp, nil
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
	return strings.Split(allUsedIPs, ";"), nil
}

func (g *Get) AllUsedIPsByHost(hostname string) ([]string, error) {
	defer unlock()
	currentNetwork, err := g.etcdClient.Get(getHostPath())
	if err != nil {
		return nil, err
	}
	allUsedIPs, err := g.etcdClient.Get(getRecordPath(currentNetwork))
	if err != nil {
		return nil, err
	}
	return strings.Split(allUsedIPs, ";"), nil
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

/**
 * 释放这堆 ip
 */
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
	return r.etcdClient.Set(getRecordPath(currentNetwork), newIPsString)
}

func (r *Release) Pool() error {
	defer unlock()
	currentNetwork, err := r.etcdClient.Get(getIPsPoolPath(getIpamSubnet(), getIpamMaskSegment()))
	if err != nil {
		return err
	}

	return r.etcdClient.Set(currentNetwork, "")
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

func _GetIpamService(subnet string, options *IPAMOptions) func() (*IpamService, error) {

	return func() (*IpamService, error) {
		var _ipam *IpamService

		if _ipam != nil {
			return _ipam, nil
		} else {
			_subnet := subnet
			var _maskSegment string = consts.DEFAULT_MASK_NUM
			var _podIpMaskSegment string = consts.DEFAULT_MASK_NUM
			var _rangeStart string = ""
			var _rangeEnd string = ""
			if options != nil {
				if options.MaskSegment != "" {
					_maskSegment = options.MaskSegment
				}
				if options.PodIpMaskSegment != "" {
					_podIpMaskSegment = options.PodIpMaskSegment
				}
				if options.RangeStart != "" {
					_rangeStart = options.RangeStart
				}
				if options.RangeEnd != "" {
					_rangeEnd = options.RangeEnd
				}
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
			err := _ipam.ipsPoolInit(poolPath)
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
			currentHostNetwork, err := _ipam.networkInit(
				hostPath,
				poolPath,
				_rangeStart,
				_rangeEnd,
			)
			if err != nil {
				return nil, err
			}

			// 初始化一个 map 的地址给 ebpf 用
			err = _ipam.subnetMapInit(
				_subnet,
				_maskSegment,
				hostname,
				currentHostNetwork,
			)
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
	__GetIpamService = nil
	return is.EtcdClient.Del("/"+prefix, oriEtcd.WithPrefix())
}

func Init(subnet string, options *IPAMOptions) func() error {
	if __GetIpamService == nil {
		__GetIpamService = _GetIpamService(subnet, options)
	}
	is, err := GetIpamService()
	if err != nil {
		return func() error {
			return err
		}
	}
	return is.clear
}
