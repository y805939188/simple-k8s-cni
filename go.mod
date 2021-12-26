module testcni

go 1.16

require (
	github.com/containernetworking/cni v1.0.1
	github.com/containernetworking/plugins v1.0.1
	github.com/coreos/go-iptables v0.6.0
	github.com/dlclark/regexp2 v1.4.0
	github.com/stretchr/testify v1.7.0
	github.com/vishvananda/netlink v1.1.1-0.20210330154013-f5de75959ad5
	go.etcd.io/etcd/client/pkg/v3 v3.5.1
	// go.etcd.io/etcd v0.5.0-alpha.5.0.20200910180754-dd1b699fc489
	go.etcd.io/etcd/client/v3 v3.5.1
	k8s.io/client-go v0.20.6
// k8s.io/client-go v1.4.0 // indirect
)
