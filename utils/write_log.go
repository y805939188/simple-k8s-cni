package utils

import (
	"bufio"
	// "fmt"
	"io/ioutil"
	"os"
)

const logPath string = "/home/ding/go/k8s-cni-test/test-cni.log"
const logErrPath string = "/home/ding/go/k8s-cni-test/log.error.txt"

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
