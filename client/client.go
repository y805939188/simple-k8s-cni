package client

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"testcni/consts"
	"testcni/helper"

	v1 "k8s.io/api/core/v1"
)

type Get struct {
	httpsClient *http.Client
	client      *LightK8sClient
}

type operators struct {
	Get *Get
}

type operator struct {
	*operators
}

type LightK8sClient struct {
	caCertPath, certFile, keyFile string
	pool                          *x509.CertPool
	client                        *http.Client
	masterEndpoint                string
	kubeApi                       string
	*operator
}

var getGet = func() func() *Get {
	var _get *Get
	return func() *Get {
		if _get != nil {
			return _get
		}
		_get = &Get{}
		client, _ := GetLightK8sClient()
		if client != nil {
			_get.httpsClient = client.client
		}
		_get.client = client
		return _get
	}
}()

func (get *Get) getRoute(api string) string {
	return get.client.masterEndpoint + get.client.kubeApi + api
}

func (o *operator) Get() *Get {
	return getGet()
}

func (get *Get) getBody(resp *http.Response) ([]byte, error) {
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

func (get *Get) Nodes() (*v1.NodeList, error) {
	url := get.getRoute("/nodes?limit=500")
	resp, err := get.httpsClient.Get(url)
	if err != nil {
		return nil, err
	}
	body, err := get.getBody(resp)
	if err != nil {
		return nil, err
	}
	var nodes *v1.NodeList
	err = json.Unmarshal(body, &nodes)
	if err != nil {
		return nil, err
	}
	return nodes, nil
}

func (get *Get) Node(name string) (*v1.Node, error) {
	url := get.getRoute(fmt.Sprintf("/nodes/%s", name))
	resp, err := get.httpsClient.Get(url)
	if err != nil {
		return nil, err
	}
	body, err := get.getBody(resp)
	if err != nil {
		return nil, err
	}
	var node *v1.Node
	err = json.Unmarshal(body, &node)
	if err != nil {
		return nil, err
	}
	return node, nil
}

var __GetLightK8sClient func() (*LightK8sClient, error)

func _GetLightK8sClient(caCertPath, certFile, keyFile string) func() (*LightK8sClient, error) {
	return func() (*LightK8sClient, error) {
		var client *LightK8sClient
		if client != nil {
			return client, nil
		} else {
			// 读取 k8s 的证书
			caCrt, err := ioutil.ReadFile(caCertPath)
			if err != nil {
				return nil, err
			}
			// new 个 pool
			pool := x509.NewCertPool()
			// 解析一系列PEM编码的证书, 从 base64 中解析证书到池子中
			pool.AppendCertsFromPEM(caCrt)
			// 加载客户端的证书和私钥
			cliCrt, err := tls.LoadX509KeyPair(certFile, keyFile)
			if err != nil {
				return nil, err
			}
			// 创建一个 https 客户端
			_client := &http.Client{
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{
						RootCAs:      pool,
						Certificates: []tls.Certificate{cliCrt},
					},
				},
			}

			masterEndpoint, err := helper.GetMasterEndpoint()
			if err != nil {
				return nil, err
			}
			client = &LightK8sClient{
				caCertPath:     caCertPath,
				certFile:       certFile,
				keyFile:        keyFile,
				pool:           pool,
				client:         _client,
				kubeApi:        consts.KUBE_API,
				masterEndpoint: masterEndpoint,
			}

			return client, nil
		}
	}
}

func GetLightK8sClient() (*LightK8sClient, error) {
	if __GetLightK8sClient == nil {
		return nil, errors.New("k8s clinet 需要初始化")
	}

	lightK8sClient, err := __GetLightK8sClient()
	if err != nil {
		return nil, err
	}
	return lightK8sClient, nil
}

func Init(caCertPath, certFile, keyFile string) {
	if __GetLightK8sClient == nil {
		__GetLightK8sClient = _GetLightK8sClient(caCertPath, certFile, keyFile)
	}
}
