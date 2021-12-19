package ipam

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"testcni/etcd"
)

const (
	prefix = "/testcni-ipam"
)

type Target struct{}

type Set struct{}

func (s *Set) IPs(ips ...string) (bool, error) {

}

type Get struct{}
type Release struct{}

type Operator struct {
	operator string
	lock     sync.Mutex
	*Target
}

func (o *Operator) Get() *Get {
	if o.operator != "" {
		fmt.Println("IpamService.Get Error")
		return nil
	}
	o.lock.Lock()
	o.operator = "GET"
	return is
}

func (o *Operator) Set() *Set {
	if o.operator != "" {
		fmt.Println("IpamService.Set Error")
		return nil
	}
	o.operator = "SET"
	return is
}

func (o *Operator) Relase() *Release {
	if o.operator != "" {
		fmt.Println("IpamService.Set Error")
		return nil
	}
	o.operator = "SET"
	return is
}

type IpamService struct {
	Subnet string
	Mask   string
	etcd   *etcd.EtcdClient
	isAll  bool
	*Operator
}

func (t *Target) OneIP(ip string) {

}

func (t *Target) AllIPs(ips ...string) {

}

// func (o *Operator) Get() *Target {
// 	// test := IpamService{}.Get().Used().AllIPs()
// 	// test := IpasmService{}.Get().Unuser().IP()
// 	// test := IpamService{}.Set().Used().AllIPs()
// 	// test := IpasmService{}.Get().Unuser().IP()
// 	// test := IpamService{}.Get().Used().AllIPs()
// 	// test := IpasmService{}.Get().Unuser().IP()
// 	if o.operator != "" {
// 		fmt.Println("IpamService.Get Error")
// 		return nil
// 	}
// 	o.lock.Lock()
// 	o.operator = "GET"
// 	return is
// }

// func (o *Operator) Set() *Target {
// 	if o.operator != "" {
// 		fmt.Println("IpamService.Set Error")
// 		return nil
// 	}
// 	o.operator = "SET"
// 	return is
// }

func getEtcdPathWithPrefix(path string) string {
	if path != "" && path[0:1] == "/" {
		return "/" + prefix + path
	}
	return "/" + prefix + "/" + path
}

func (is *IpamService) One() *IpamService {

}

func (is *IpamService) All() *IpamService {

}

func (is *IpamService) UnusedIP() {

}

func (is *IpamService) GetUnusedIP() (string, error) {

	return "", nil
}

func (is *IpamService) GetAllUsedIP() []string {

	return []string{}
}

func (is *IpamService) GetAllUnusedIP() []string {

	return []string{}
}

func (is *IpamService) SetIpBeUsed(ip ...string) (bool, error) {

	return false, nil
}

func (is *IpamService) ReleaseIP(ip ...string) (bool, error) {

	return false, nil
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

			_ipam = &IpamService{
				Subnet: _subnet,
				Mask:   _mask,
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
	etcd.Init()
	etcdClient, err := etcd.GetEtcdClient()
	if err != nil {
		return nil, err
	}
	ipamService.etcd = etcdClient
	return ipamService, nil
}

func Init(subnet string, mask ...string) {
	if __GetIpamService == nil {
		__GetIpamService = _GetIpamService(subnet, mask...)
	}
}
