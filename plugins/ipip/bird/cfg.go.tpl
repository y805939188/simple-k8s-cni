router id {{.HostIP}};

protocol static {
  route {{.HostCIDR}} blackhole;
}

function calico_aggr() {
  if ( net = {{.HostCIDR}} ) then { accept; }
  if ( net ~ {{.HostCIDR}} ) then { reject; }
}

filter calico_export_to_bgp_peers {
  calico_aggr();
  if ( net ~ {{.Gateway}} ) then {
    accept;
  }
  reject;
}

filter calico_kernel_programming {
  if ( net ~ {{.Gateway}} ) then {
    krt_tunnel = "tunl0";
    accept;
  }
  accept;
}

protocol kernel {
  learn;
  persist;
  scan time 2;
  import all;
  export filter calico_kernel_programming;
  graceful restart;
}

protocol device {
  # debug all;
  scan time 2;
}

protocol direct {
  # debug all;
  # 不包括 veth* 和 kube-ipvs*
  interface -"{{.VethPrefix}}*", -"kube-ipvs*", "*";
}

template bgp bgp_template {
  # debug all;
  description "Connection to BGP peer";
  local as 64512;
  multihop;
  gateway recursive;
  import all; 
  export filter calico_export_to_bgp_peers;
  add paths on;
  graceful restart;
  connect delay time 2;
  connect retry time 5;
  error wait time 5,30;
}

{{range $index, $neigh := .Neighbors}}
protocol bgp {{$neigh.Name}} from bgp_template {
  neighbor {{$neigh.IP}} as 64512;
  source address {{$.HostIP}};
}

{{end}}
