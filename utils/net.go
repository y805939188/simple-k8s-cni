package utils

import (
	"fmt"
	"math/big"
	"net"
	"os/exec"
	"strconv"
	"strings"
)

func CheckIP(ip string) bool {
	address := net.ParseIP(ip)
	return address != nil
}

func InetUint32ToIp(intIP uint32) string {
	var bytes [4]byte
	bytes[0] = byte(intIP & 0xFF)
	bytes[1] = byte((intIP >> 8) & 0xFF)
	bytes[2] = byte((intIP >> 16) & 0xFF)
	bytes[3] = byte((intIP >> 24) & 0xFF)
	return net.IPv4(bytes[3], bytes[2], bytes[1], bytes[0]).String()
}

func InetIpToUInt32(ip string) uint32 {
	bits := strings.Split(ip, ".")
	b0, _ := strconv.Atoi(bits[0])
	b1, _ := strconv.Atoi(bits[1])
	b2, _ := strconv.Atoi(bits[2])
	b3, _ := strconv.Atoi(bits[3])
	var sum uint32
	sum += uint32(b0) << 24
	sum += uint32(b1) << 16
	sum += uint32(b2) << 8
	sum += uint32(b3)
	return sum
}

func InetInt2Ip(ip int64) string {
	return fmt.Sprintf("%d.%d.%d.%d",
		byte(ip>>24), byte(ip>>16), byte(ip>>8), byte(ip))
}

func InetIP2Int(ip string) int64 {
	ret := big.NewInt(0)
	ret.SetBytes(net.ParseIP(ip).To4())
	return ret.Int64()
}

func GenIpRange(start, end string) []string {
	startInt, endInt := InetIpToUInt32(start), InetIpToUInt32(end)
	if startInt >= endInt {
		return nil
	}
	res := make([]string, endInt-startInt+1)
	for index := range res {
		_tmp := startInt + uint32(index)
		res[index] = InetUint32ToIp(_tmp)
	}
	return res
}

func GetMaxIP(ips []string) string {
	var _ips []int64
	for _, ip := range ips {
		_ips = append(_ips, InetIP2Int(ip))
	}
	maxNum := _ips[0]
	var maxArrayIndex int
	for i := 1; i < len(_ips); i++ {
		if _ips[i] > maxNum {
			maxNum = _ips[i]
			maxArrayIndex = i
		}
	}
	return ips[maxArrayIndex]
}

func GetPidByPort(port string) (int, string, error) {
	processInfo := exec.Command(
		"/bin/sh", "-c",
		fmt.Sprintf(`lsof -i:%s | awk '{print $2}' | awk  'NR==2{print}'`, port),
	)
	if pid, err := processInfo.Output(); err != nil {
		return -1, "", err
	} else {
		s := string(pid)
		if s == "" {
			return -1, "", err
		}
		str := strings.ReplaceAll(s, "\n", "")
		i, err := strconv.Atoi(strings.ReplaceAll(s, "\n", ""))
		if err != nil {
			return -1, "", err
		}
		return i, str, nil
	}
}

// func main() {
// 	ip := "192.168.0.1"
// 	ipInt := InetIP2Int(ip)

// 	fmt.Printf("convert string ip [%s] to int: %d\n", ip, ipInt)
// 	fmt.Printf("convert int ip [%d] to string: %s\n", ipInt, InetInt2Ip(ipInt))

// 	ip = "192.168.0.2"
// 	ipInt = InetIP2Int(ip)

// 	fmt.Printf("convert string ip [%s] to int: %d\n", ip, ipInt)
// 	fmt.Printf("convert int ip [%d] to string: %s\n", ipInt, InetInt2Ip(ipInt))

// 	ip = "192.168.0.255"
// 	ipInt = InetIP2Int(ip)

// 	fmt.Printf("convert string ip [%s] to int: %d\n", ip, ipInt)
// 	fmt.Printf("convert int ip [%d] to string: %s\n", ipInt, InetInt2Ip(ipInt))

// 	ip = "192.168.1.0"
// 	ipInt = InetIP2Int(ip)

// 	fmt.Printf("convert string ip [%s] to int: %d\n", ip, ipInt)
// 	fmt.Printf("convert int ip [%d] to string: %s\n", ipInt, InetInt2Ip(ipInt))
// }
