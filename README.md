# k8s-cni-test

## 相关文章链接
[深入理解 k8s 的 CNI 网络](https://zhuanlan.zhihu.com/p/450140876)
</br>
[从 0 实现一个 CNI 网络插件](https://zhuanlan.zhihu.com/p/450514389)</br>
[基于 ebpf 和 vxlan 实现一个 k8s 网络插件（一）](https://zhuanlan.zhihu.com/p/565254116)</br>
[基于 ebpf 和 vxlan 实现一个 k8s 网络插件（二）](https://zhuanlan.zhihu.com/p/565420113))</br>
[基于 BGP 协议实现 Calico 的 IPIP 网络](https://zhuanlan.zhihu.com/p/571966611)
[基于 IPVlan & MACVlan 实现 CNI 网络插件](https://zhuanlan.zhihu.com/p/573914523)

## IPIP 模式测试方法
0. 最好有个干净的，没有安装任何网络插件的 k8s 环境

1.
```js
// 在 /etc/cni/net.d/ 目录下新建个 .conf 结尾的文件, 输入以下配置项
{
  "cniVersion": "0.3.0",
  "name": "testcni",
  "type": "testcni",
  "mode": "ipip",
  "subnet": "10.244.0.0/16"
}
```

2. 
```bash
# 在项目根目录执行
make build_main
```

3. 此时会生成一个 main

4. 去 https://github.com/projectcalico/bird clone 项目

5. 执行
```
# 编译 calico 的 bird
# 编译完后会在 dist 目录中有个 bird 二进制
ARCH=<你的计算机架构> ./build.sh
```

6. 创建 /opt/testcni 目录并把上边那个 bird 二进制拷贝到这里

7. 第三步中生成的 main 二进制拷贝到 /opt/cni/bin/testcni
```bash
mv main /opt/cni/bin/testcni
```
</br>
</br>
</br>

---
</br>

## vxlan 模式测试方法
0. 最好有个干净的，没有安装任何网络插件的 k8s 环境

1.
```js
// 在 /etc/cni/net.d/ 目录下新建个 .conf 结尾的文件, 输入以下配置项
{
  "cniVersion": "0.3.0",
  "name": "testcni",
  "type": "testcni",
  "mode": "vxlan",
  "subnet": "10.244.0.0"
}
```

2.
```bash
# 在项目根目录执行
make build
```
3. 此时会生成一个名为 testcni 的二进制文件。同时会产生三个 ebpf 文件。这三个 ebpf 文件会被自动拷贝到 “/opt/testcni/” 目录下。如果不存在这个目录的话可以手动创建一下

4. 把上一步生成的 testcni 拷贝到 “/opt/cni/bin” 目录下
</br>
</br>
</br>

---
</br>

## IPIP 模式测试方法
0. 最好有个干净的，没有安装任何网络插件的 k8s 环境

1.
```js
// 在每个节点的 /etc/cni/net.d/ 目录下新建个 .conf 结尾的文件, 输入以下配置项
// 注意其中的 “subnet” 和 “ipam” 中的 range 需要自己手动改成自己环境的，另外 range 的范围每个节点应该配置成不同的范围
{
  "cniVersion": "0.3.0",
  "name": "testcni",
  "type": "testcni",
  "mode": "ipvlan",
  "subnet": "192.168.64.0/24",
  "ipam": {
    "rangeStart": "192.168.64.90",
    "rangeEnd": "192.168.64.100"
  }
}
```

2. 
```bash
# 在项目根目录执行
make build_main
```

3. 此时会生成一个 main 二进制，把该二进制拷贝到 /opt/cni/bin/testcni

```bash
mv main /opt/cni/bin/testcni
```
</br>
</br>
</br>

---
</br>

## host-gw 模式测试方法
1. 
```js
// 在 /etc/cni/net.d/ 目录下新建个 .conf 结尾的文件, 输入以下配置项
{
  "cniVersion": "0.3.0",
  "name": "testcni",
  "type": "testcni",
  "bridge": "testcni0",
  "subnet": "10.244.0.0/16"
}
```
2. 把 /etcd/client.go 下的用来初始化 etcd 客户端的 ip 地址改成自己集群的 etcd 地址
3. go build main.go
4. mv main /opt/cni/bin/testcni
5. 每台主机上都重复以上三步
6. kubectl apply -f test-busybox.yaml
7. 查看集群 pod 状态

</br></br>

## 不使用 k8s 集群测试
1. 可通过 /test 目录下的 main_test.go 进行测试
2. 测试之前先 ip netns add test.net.1 创建一个命令空间
3. 然后 go test ./test/main_test.go -v
4. 之后在另外的节点上也执行同样的步骤
5. ip netns exec test.net.1 ping 另外一台节点上的 ns 下的网卡 ip

</br></br>

## 使用 k8s cni 的 repo 提供的 cnitool 测试
1. 切换到 test/cni-test 分支
2. 进入到 ./cnitool 目录
3. go build cnitool.go
4. ip netns add test.net.1 创建一个 net ns
5. 在 /etc/cni/net.d/ 目录下创建和上面一样的配置
6. ./cnitool add testcni /run/netns/test.net.1

</br></br>

## 问题以及排查
### 遇到问题排查方法
1. journalctl -xeu kubelet -f 通过命令查看 kubelet 日志
2. 在 ./utils/write_log.go 文件中修改 log 输出地址, 关键的报错信息会自动打到这个地址中

### 可能会遇到的问题:
1. 如果明明编译完的 main.go 已经被拷贝到 /opt/cni/bin/testcni 了但是 kubelet 还报错什么类似 "找不到" 之类的, 尝试看看给环境变量添加 "export CNI_PATH=/opt/cni/bin"
2. 如果 kubelet 日志显示什么 "配置文件有非法字符" 之类的, 检查所有代码中是否出现过使用 fmt 直接往标准输出中输出了什么日志. cni 通过标准输出读取配置, 所以一旦有任何非配置相关的信息被输出, 则一定会 gg

</br></br>

## TODO
1. 还没实现 del, 目前需要手动删一些资源以及 etcd 释放
2. ipam 当前是直接裸读的 etcd, 更好的方法是创建 crd
3. 当前是直接手动把编译后的二进制干到 /opt/cni/bin 下, 更好的方法应该是通过 daemonset 把二进制和配置拷贝到对应路径
