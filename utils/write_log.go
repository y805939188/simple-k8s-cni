package utils

import (
	"bufio"
	"io/ioutil"
	"os"
)

const DEFAULT_LOG_PATH = "/root/ding/go/src/simple-k8s-cni/test-cni.log"

var CURRENT_LOG_PATH = os.Getenv("TEST_CNI_LOG_PATH")

const DEFAULT_LOG_ERROR_PATH = "/root/ding/go/src/simple-k8s-cni/test-cni.log"

var CURRENT_LOG_ERROR_PATH = os.Getenv("TEST_CNI_LOG_ERROR_PATH")

var logPath string
var logErrPath string

func init() {
	logPath = DEFAULT_LOG_PATH
	if CURRENT_LOG_PATH != "" {
		logPath = CURRENT_LOG_PATH
	}
	if CURRENT_LOG_ERROR_PATH != "" {
		logErrPath = CURRENT_LOG_ERROR_PATH
	}
}

func WriteFile(content ...string) {
	contentRes := ""
	for _, c := range content {
		contentRes += c
	}
	var d = []byte(contentRes)
	err := ioutil.WriteFile(logErrPath, d, 0666)
	if err != nil {
		// fmt.Println("覆盖写入文件失败: ", err.Error())
	}
	// fmt.Println("覆盖写入文件成功")
}

func WriteLog(log ...string) {
	file, err := os.OpenFile(logPath, os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		// WriteFile("打开文件失败, 即将创建文件: ", err.Error())
		os.Create(logPath)
	}
	//及时关闭file句柄
	defer file.Close()
	//写入文件时，使用带缓存的 *Writer
	write := bufio.NewWriter(file)
	logRes := ""
	for _, c := range log {
		logRes += c
		logRes += " "
	}
	// fmt.Println(logRes)
	_, err = write.WriteString(logRes + "\r\n")
	if err != nil {
		// fmt.Println("失败: ", err.Error())
	}
	//Flush将缓存的文件真正写入到文件中
	write.Flush()
}
