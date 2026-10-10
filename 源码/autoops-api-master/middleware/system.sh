#!/bin/bash

# 获取私有 IP（默认路由实际使用的源地址）
privateIp=$(ip -4 route get 1.1.1.1 2>/dev/null | awk '{
    for (i = 1; i <= NF; i++) {
        if ($i == "src") {
            print $(i + 1)
            exit
        }
    }
}')

# 默认路由不可用时，跳过常见容器和虚拟网卡。
if [ -z "$privateIp" ]; then
    privateIp=$(ip -o -4 addr show scope global 2>/dev/null | awk '
        $2 !~ /^(docker|br-|virbr|veth|cni|flannel|podman|tun|tap)/ {
            split($4, address, "/")
            print address[1]
            exit
        }
    ')
fi

if [ -z "$privateIp" ]; then
    privateIp=$(hostname -I 2>/dev/null | awk '{print $1}')
fi
if [ -z "$privateIp" ]; then
    privateIp="unknown"
fi

# 获取公网 IP（如果存在）
publicIp=""
if command -v curl >/dev/null 2>&1; then
    publicIp=$(curl -4 -fsS --connect-timeout 3 --max-time 5 https://ipinfo.io/ip 2>/dev/null || true)
fi
# 获取操作系统版本
os=$(cat /etc/os-release 2>/dev/null | grep PRETTY_NAME | cut -d= -f2 | tr -d '"' | sed 's/ LTS//;s/ //g' || echo "unknown")
# 获取 CPU 核心数
cpu=$(nproc 2>/dev/null || echo "unknown")
# 获取内存大小（单位为 MB，转换为 G）
memory=$(free -m 2>/dev/null | awk '/^Mem:/{printf "%.0fG\n",$2/1024}' || echo "unknown")
# 获取磁盘总容量（单位为 G）
disk=$(df -h 2>/dev/null | awk '/\/$/ {print $2}' || echo "unknown")
# 输出 JSON 格式（确保只输出一次）
json_output=$(cat <<EOF
{"privateIp":"$privateIp","publicIp":"$publicIp","os":"$os","cpu":"$cpu","memory":"$memory","disk":"$disk"}
EOF
)

# 验证并输出JSON
if jq -e . >/dev/null 2>&1 <<<"$json_output"; then
    echo "SSH配置获取成功"
    echo "$json_output"
else
    echo "SSH配置获取失败"
    echo '{"error":"invalid json generated"}'
fi
