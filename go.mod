module testcni

go 1.16

require (
	github.com/cilium/ebpf v0.9.1
	github.com/containernetworking/cni v1.0.1
	github.com/containernetworking/plugins v1.0.1
	github.com/coreos/go-iptables v0.6.0
	github.com/dlclark/regexp2 v1.4.0
	github.com/kardianos/osext v0.0.0-20190222173326-2bc1f35cddc0 // indirect
	github.com/sevlyar/go-daemon v0.1.6
	github.com/stretchr/testify v1.7.0
	github.com/vishvananda/netlink v1.1.1-0.20210330154013-f5de75959ad5
	go.etcd.io/etcd/api/v3 v3.5.4
	go.etcd.io/etcd/client/pkg/v3 v3.5.1
	// go.etcd.io/etcd v0.5.0-alpha.5.0.20200910180754-dd1b699fc489
	go.etcd.io/etcd/client/v3 v3.5.1
	k8s.io/api v0.20.6
// k8s.io/client-go v1.4.0 // indirect
)
