package etcd

import (
	"context"
	"errors"
	"os"
	"strings"
	"testcni/utils"
	"time"

	"go.etcd.io/etcd/api/v3/mvccpb"
	"go.etcd.io/etcd/client/pkg/v3/transport"
	etcd "go.etcd.io/etcd/client/v3"
)

type WatchCallback func(_type mvccpb.Event_EventType, key, value []byte)

type EtcdConfig struct {
	EtcdScheme       string `json:"etcdScheme" envconfig:"APIV1_ETCD_SCHEME" default:""`
	EtcdAuthority    string `json:"etcdAuthority" envconfig:"APIV1_ETCD_AUTHORITY" default:""`
	EtcdEndpoints    string `json:"etcdEndpoints" envconfig:"APIV1_ETCD_ENDPOINTS"`
	EtcdDiscoverySrv string `json:"etcdDiscoverySrv" envconfig:"APIV1_ETCD_DISCOVERY_SRV"`
	EtcdUsername     string `json:"etcdUsername" envconfig:"APIV1_ETCD_USERNAME"`
	EtcdPassword     string `json:"etcdPassword" envconfig:"APIV1_ETCD_PASSWORD"`
	EtcdKeyFile      string `json:"etcdKeyFile" envconfig:"APIV1_ETCD_KEY_FILE"`
	EtcdCertFile     string `json:"etcdCertFile" envconfig:"APIV1_ETCD_CERT_FILE"`
	EtcdCACertFile   string `json:"etcdCACertFile" envconfig:"APIV1_ETCD_CA_CERT_FILE"`
}

type Watcher struct {
	client        *EtcdClient
	watcher       etcd.Watcher
	cancelWatcher context.CancelFunc
	ctx           context.Context
	// kMap map[string]
}

type EtcdClient struct {
	client  *etcd.Client
	watcher *Watcher
	Version string
}

const (
	clientTimeout = 30 * time.Second
	etcdTimeout   = 2 * time.Second
)

func newEtcdClient(config *EtcdConfig) (*etcd.Client, error) {
	var etcdLocation []string
	if config.EtcdAuthority != "" {
		etcdLocation = []string{config.EtcdScheme + "://" + config.EtcdAuthority}
	}
	if config.EtcdEndpoints != "" {
		etcdLocation = strings.Split(config.EtcdEndpoints, ",")
	}

	if len(etcdLocation) == 0 {
		return nil, errors.New("找不到 etcd")
	}

	tlsInfo := transport.TLSInfo{
		CertFile:      config.EtcdCertFile,
		KeyFile:       config.EtcdKeyFile,
		TrustedCAFile: config.EtcdCACertFile,
	}

	tlsConfig, err := tlsInfo.ClientConfig()

	client, err := etcd.New(etcd.Config{
		Endpoints:   etcdLocation,
		TLS:         tlsConfig,
		DialTimeout: clientTimeout,
	})

	if err != nil {
		return nil, err
	}

	return client, nil
}

var __GetEtcdClient func() (*EtcdClient, error)

func GetEtcdClient() (*EtcdClient, error) {

	if __GetEtcdClient == nil {
		return nil, nil
	}
	return __GetEtcdClient()

}

func _GetEtcdClient() func() (*EtcdClient, error) {
	var _client *EtcdClient

	return func() (*EtcdClient, error) {
		if _client != nil {
			return _client, nil
		} else {
			// ETCDCTL_API=3 etcdctl --endpoints https://192.168.64.19:2379 --cacert /etc/kubernetes/pki/etcd/ca.crt --cert /etc/kubernetes/pki/etcd/healthcheck-client.crt --key /etc/kubernetes/pki/etcd/healthcheck-client.key get / --prefix --keys-only

			etcdEp := os.Getenv("ETCD_ENDPOINT")
			if etcdEp == "" {
				panic("get etcd endpoint failed from env")
			}
			client, err := newEtcdClient(&EtcdConfig{
				EtcdEndpoints:  etcdEp,
				EtcdCertFile:   "/etc/kubernetes/pki/etcd/healthcheck-client.crt",
				EtcdKeyFile:    "/etc/kubernetes/pki/etcd/healthcheck-client.key",
				EtcdCACertFile: "/etc/kubernetes/pki/etcd/ca.crt",
			})

			if err != nil {
				return nil, err
			}

			status, err := client.Status(context.TODO(), etcdEp)

			if err != nil {
				utils.WriteLog("无法获取到 etcd 版本")
				return nil, err
			}

			if client != nil {
				_client = &EtcdClient{
					client: client,
				}

				if status != nil && status.Version != "" {
					_client.Version = status.Version
				}
				// fmt.Println("etcd 客户端初始化成功")
				return _client, nil
			}
		}
		return nil, errors.New("初始化 etcd client 失败")
	}
}

func Init() {
	if __GetEtcdClient == nil {
		__GetEtcdClient = _GetEtcdClient()
	}
}

func (c *EtcdClient) Set(key, value string) error {
	_, err := c.client.Put(context.TODO(), key, value)

	if err != nil {
		return err
	}
	return nil
}

func (c *EtcdClient) Del(key string, opts ...etcd.OpOption) error {
	_, err := c.client.Delete(context.TODO(), key, opts...)
	if err != nil {
		return err
	}
	return err
}

func (c *EtcdClient) GetVersion(key string, opts ...etcd.OpOption) (int64, error) {
	resp, err := c.client.Get(context.TODO(), key, opts...)
	if err != nil {
		return 0, err
	}
	if len(resp.Kvs) > 0 {
		return resp.Kvs[len(resp.Kvs)-1:][0].Version, nil
	}
	return 0, nil
}

func (c *EtcdClient) Get(key string, opts ...etcd.OpOption) (string, error) {
	resp, err := c.client.Get(context.TODO(), key, opts...)
	if err != nil {
		return "", err
	}

	// for _, ev := range resp.Kvs {
	// 	fmt.Println("这里的 ev 是: ", ev)
	// 	fmt.Printf("%s : %s\n", ev.Key, ev.Value)
	// }

	if len(resp.Kvs) > 0 {
		return string(resp.Kvs[len(resp.Kvs)-1:][0].Value), nil
	}
	return "", nil
}

func (c *EtcdClient) GetKey(key string, opts ...etcd.OpOption) (string, error) {
	resp, err := c.client.Get(context.TODO(), key, opts...)
	if err != nil {
		return "", err
	}

	// for _, ev := range resp.Kvs {
	// 	fmt.Println("这里的 ev 是: ", ev)
	// 	fmt.Printf("%s : %s\n", ev.Key, ev.Value)
	// }

	if len(resp.Kvs) > 0 {
		return string(resp.Kvs[len(resp.Kvs)-1:][0].Key), nil
	}
	return "", nil
}

func (c *EtcdClient) GetAll(key string, opts ...etcd.OpOption) ([]string, error) {
	resp, err := c.client.Get(context.TODO(), key, opts...)
	if err != nil {
		return nil, err
	}

	var res []string

	for _, ev := range resp.Kvs {
		// fmt.Println("这里的 ev 是: ", ev)
		// fmt.Printf("%s : %s\n", ev.Key, ev.Value)
		res = append(res, string(ev.Value))
	}

	return res, nil
}

func (c *EtcdClient) GetAllKey(key string, opts ...etcd.OpOption) ([]string, error) {
	resp, err := c.client.Get(context.TODO(), key, opts...)
	if err != nil {
		return nil, err
	}

	var res []string

	for _, ev := range resp.Kvs {
		res = append(res, string(ev.Key))
	}

	return res, nil
}

func (c *EtcdClient) Watch(key string, cb WatchCallback) {
	go func() {
		for {
			change := c.client.Watch(context.Background(), key)
			for wresp := range change {
				for _, ev := range wresp.Events {
					cb(ev.Type, ev.Kv.Key, ev.Kv.Value)
				}
			}
		}
	}()
}

func (w *Watcher) Done() <-chan struct{} {
	return w.ctx.Done()
}

func (w *Watcher) Deadline() (deadline time.Time, ok bool) {
	return w.ctx.Deadline()
}

func (w *Watcher) Error() error {
	return w.ctx.Err()
}

func (w *Watcher) Value(_any interface{}) interface{} {
	return w.ctx.Value(_any)
}

func (w *Watcher) Cancel() {
	w.cancelWatcher()
}

func (w *Watcher) Watch(key string, cb WatchCallback) {
	go func() {
		defer func() {
			w.Cancel()
			time.Sleep(2 * time.Second)
		}()
		for {
			change := w.watcher.Watch(context.Background(), key)
			for wresp := range change {
				for _, ev := range wresp.Events {
					cb(ev.Type, ev.Kv.Key, ev.Kv.Value)
				}
			}
		}
	}()
	// TODO: sleep change to sync
	time.Sleep(1 * time.Second)
}

func (c *EtcdClient) GetWatcher() (*Watcher, error) {
	if c.watcher != nil {
		return c.watcher, nil
	}
	watcher := &Watcher{client: c}
	_watcher := etcd.NewWatcher(c.client)
	watcher.watcher = _watcher
	ctx, cancelFunc := context.WithCancel(context.TODO())
	watcher.cancelWatcher = cancelFunc
	watcher.ctx = ctx

	return watcher, nil
}
