package helper

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"testcni/consts"
	"testcni/utils"

	"github.com/dlclark/regexp2"
)

// 先尝试去捞 admin.conf, 没有的话就用 kubelet.conf, 再没有的话就用 ~/.kube/conf
// 都没有的话那就凉了
func GetClientConfigPath() string {
	clusterConfPath := consts.KUBE_LOCAL_DEFAULT_PATH
	if utils.PathExists(consts.KUBE_CONF_ADMIN_DEFAULT_PATH) {
		clusterConfPath = consts.KUBE_CONF_ADMIN_DEFAULT_PATH
	} else if utils.PathExists(consts.KUBELET_CONFIG_DEFAULT_PATH) {
		clusterConfPath = consts.KUBELET_CONFIG_DEFAULT_PATH
	}
	return clusterConfPath
}

func GetMasterEndpoint() (string, error) {
	// 先尝试去捞 admin.conf, 没有的话就用 kubelet.conf, 再没有的话就用 ~/.kube/conf
	clusterConfPath := GetClientConfigPath()
	confByte, err := ioutil.ReadFile(clusterConfPath)
	if err != nil {
		utils.WriteLog("读取 path: ", clusterConfPath, " 失败: ", err.Error())
		return "", err
	}
	masterEndpoint, err := GetLineFromYaml(string(confByte), "server")
	if err != nil {
		utils.WriteLog("读取 path: ", clusterConfPath, " 失败: ", err.Error())
		return "", err
	}
	return masterEndpoint, nil
}

func GetLineFromYaml(yaml string, key string) (string, error) {
	r, err := regexp2.Compile(fmt.Sprintf(`(?<=%s: )(.*)`, key), 0)
	if err != nil {
		utils.WriteLog("初始化正则表达式失败, err: ", err.Error())
		return "", err
	}

	res, err := r.FindStringMatch(yaml)
	if err != nil {
		utils.WriteLog("正则匹配 ip 失败, err: ", err.Error())
		return "", err
	}
	return res.String(), nil
}

type AuthenticationInfoPath struct {
	CaPath   string // api server 的证书
	CertPath string // 本机的证书
	KeyPath  string // 本机的私钥
}

func GetHostAuthenticationInfoPath() (*AuthenticationInfoPath, error) {
	paths := &AuthenticationInfoPath{}
	if !utils.PathExists(consts.KUBE_TEST_CNI_DEFAULT_PATH) {
		err := utils.CreateDir(consts.KUBE_TEST_CNI_DEFAULT_PATH)
		if err != nil {
			return nil, err
		}
	}
	// 如果几个关键的文件已经存在就直接返回路径
	if utils.PathExists(consts.KUBE_TEST_CNI_TMP_CA_DEFAULT_PATH) {
		paths.CaPath = consts.KUBE_TEST_CNI_TMP_CA_DEFAULT_PATH
	}
	if utils.PathExists(consts.KUBE_TEST_CNI_TMP_CERT_DEFAULT_PATH) {
		paths.CertPath = consts.KUBE_TEST_CNI_TMP_CERT_DEFAULT_PATH
	}
	if utils.PathExists(consts.KUBE_TEST_CNI_TMP_KEY_DEFAULT_PATH) {
		paths.KeyPath = consts.KUBE_TEST_CNI_TMP_KEY_DEFAULT_PATH
	}
	if paths.CaPath != "" && paths.CertPath != "" && paths.KeyPath != "" {
		return paths, nil
	}

	var caPath string
	if utils.PathExists(consts.KUBE_DEFAULT_CA_PATH) {
		caPath = consts.KUBE_DEFAULT_CA_PATH
		err := utils.FileCopy(caPath, consts.KUBE_TEST_CNI_TMP_CA_DEFAULT_PATH)
		if err != nil {
			return nil, err
		}
		paths.CaPath = consts.KUBE_TEST_CNI_TMP_CA_DEFAULT_PATH
	}
	clusterConfPath := GetClientConfigPath()

	confByte, err := ioutil.ReadFile(clusterConfPath)
	if err != nil {
		utils.WriteLog("读取 path: ", clusterConfPath, " 失败: ", err.Error())
		return nil, err
	}

	// 由于可能读到 admin.conf/kubelet.conf/conf 中的任何一个
	// 里边用来记录客户端证书和 key 的方式也不一定
	// 先看里头是否有 client-certificate-data/client-key-data
	// 有的话读出来存到 tmp 目录中
	// 如果是 client-certificate/client-key 的话
	// 那读出来时就是俩目录, 直接拷贝到 tmp 就行
	cert, err := GetLineFromYaml(string(confByte), "client-certificate-data")
	if err != nil {
		return nil, err
	}
	key, err := GetLineFromYaml(string(confByte), "client-key-data")
	if err != nil {
		return nil, err
	}
	if cert != "" && key != "" {
		decodedCert, err := base64.StdEncoding.DecodeString(cert)
		if err != nil {
			return nil, err
		}
		err = utils.CreateFile(consts.KUBE_TEST_CNI_TMP_CERT_DEFAULT_PATH, ([]byte)(decodedCert), 0766)
		if err != nil {
			return nil, err
		}
		decodedKey, err := base64.StdEncoding.DecodeString(key)
		if err != nil {
			return nil, err
		}
		err = utils.CreateFile(consts.KUBE_TEST_CNI_TMP_KEY_DEFAULT_PATH, ([]byte)(decodedKey), 0766)
		if err != nil {
			return nil, err
		}
		paths.CertPath = consts.KUBE_TEST_CNI_TMP_CERT_DEFAULT_PATH
		paths.KeyPath = consts.KUBE_TEST_CNI_TMP_KEY_DEFAULT_PATH
		return paths, nil
	}

	cert, err = GetLineFromYaml(string(confByte), "client-certificate")
	if err != nil {
		return nil, err
	}
	key, err = GetLineFromYaml(string(confByte), "client-key")
	if err != nil {
		return nil, err
	}

	err = utils.FileCopy(cert, consts.KUBE_TEST_CNI_TMP_CERT_DEFAULT_PATH)
	if err != nil {
		return nil, err
	}
	err = utils.FileCopy(key, consts.KUBE_TEST_CNI_TMP_KEY_DEFAULT_PATH)
	if err != nil {
		return nil, err
	}

	paths.CertPath = cert
	paths.KeyPath = key
	return paths, nil
}
