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
fi
