#! /bin/bash

if [ ! -n "$1" ] ;then
  echo "需要一个网桥名儿作为参数"
else
  echo "网桥名是 $1"
  echo "即将清除相关网桥以及 veth"
  ip link del dev $1

  ip netns del test.net.1

  ip netns del test.net.2

  ip netns add test.net.1

  ip netns add test.net.2

  # 这里的 10.244.x.0 要替换成其他节点的 pod 网段
  # 然后 192.168.98.x 也要替换成其他节点的网卡 ip
  ip route del 10.244.1.0/24 via 192.168.98.144 dev ens33

  ip route del 10.244.2.0/24 via 192.168.98.145 dev ens33

  brctl delif testcni0 ens33
fi
