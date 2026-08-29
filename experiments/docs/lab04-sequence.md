# 实验 04：veth + bridge + NAT

```text
宿主
  ├─ 创建 bridge gantry-br0 = 10.200.1.1/24
  ├─ 创建 veth pair gantry-veth0 ↔ gantry-veth1
  ├─ 将 gantry-veth0 接入 bridge
  ├─ 创建命名空间 gantry-lab-net
  └─ 从宿主把 gantry-veth1 移入该命名空间
       └─ 配置 10.200.1.2/24 与默认路由
            ├─ 必须通过：ping bridge 10.200.1.1
            └─ 可选通过：经 MASQUERADE ping 8.8.8.8
```

脚本先保存 `ip_forward`，退出时恢复原值，并删除精确的 NAT 规则、命名空间、veth 和 bridge。外网 ICMP 可能被环境阻断，所以只作为可选观察，不参与核心成功判定。
