package consts

const (
	MODE_HOST_GW = "host-gw"
	MODE_VXLAN   = "vxlan"
)

const (
	DEFAULT_MASK_NUM = "24"
	DEFAULT_MASK_IP  = "255.255.0.0"
)

const (
	KUBE_API                            = "/api/v1"
	KUBE_DEFAULT_PATH                   = "/etc/kubernetes"
	KUBE_LOCAL_DEFAULT_PATH             = "~/.kube/conf"
	KUBE_DEFAULT_CA_PATH                = KUBE_DEFAULT_PATH + "/pki/ca.crt"
	KUBELET_CONFIG_DEFAULT_PATH         = KUBE_DEFAULT_PATH + "/kubelet.conf"
	KUBE_CONF_ADMIN_DEFAULT_PATH        = KUBE_DEFAULT_PATH + "/admin.conf"
	KUBE_TEST_CNI_DEFAULT_PATH          = "/opt/testcni"
	KUBE_TEST_CNI_TMP_CA_DEFAULT_PATH   = KUBE_TEST_CNI_DEFAULT_PATH + "/ca.crt"
	KUBE_TEST_CNI_TMP_CERT_DEFAULT_PATH = KUBE_TEST_CNI_DEFAULT_PATH + "/cert.crt"
	KUBE_TEST_CNI_TMP_KEY_DEFAULT_PATH  = KUBE_TEST_CNI_DEFAULT_PATH + "/key.key"
)
