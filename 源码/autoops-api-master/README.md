# backend
cd  scripts/
./scripts/issue-license.sh AOPS-DE1A-E114-78FB-E070-3A1D-9188-E908-6F1D   30d

## Agent 离线部署

在可联网的编译机上生成两个 Linux 离线 Agent 包（不会连接目标主机）：

```bash
bash scripts/build-offline-agent.sh
```

默认输出到 `./upload/agent/`：

```text
upload/agent/dodevops-agent-linux-amd64
upload/agent/dodevops-agent-linux-arm64
```

如果需要指定后端心跳地址、Token 或版本，可以在命令前设置环境变量：

```bash
AGENT_VERSION=2.1.0 \
AGENT_HEARTBEAT_URL=https://autoops.example.com/api/v1/cmdb/agent/heartbeat \
AGENT_HEARTBEAT_TOKEN=your-token \
bash scripts/build-offline-agent.sh
```

如果交互式终端和 Bash 脚本使用了不同的 Go（例如版本管理器或 shell alias），可以显式指定 Go 可执行文件：

```bash
GO_BIN=/path/to/go1.25.6/bin/go bash scripts/build-offline-agent.sh
```

在线部署流程保持不变：前端继续调用 `POST /api/v1/cmdb/agent/deploy`，请求只传 `hostIds` 和可选的 `version`，服务端负责编译、传输和启动。

离线部署时，先通过上传接口把预编译的 Agent 二进制保存到服务端配置的 `imageSettings.uploadDir/agent/`（默认 `./upload/agent/`）目录。前端可调用 `GET /api/v1/cmdb/agent/offline-assets` 获取可选文件，然后复用同一个部署接口：

```bash
curl -X POST /api/v1/cmdb/agent/offline-assets/upload \
  -F "file=@dodevops-agent-linux-amd64"
```

可通过 `GET /api/v1/cmdb/agent/offline-assets/check` 查看 x86_64 和 arm64 包是否都已上传，服务端优先读取 Linux ELF 文件头识别架构。正式安装前，前端调用 `POST /api/v1/cmdb/agent/offline-assets/precheck`，服务端会通过 SSH 执行 `uname -m`，确认每台目标主机都有匹配的二进制；检查不通过时不会创建安装任务。

```json
{
  "hostIds": [101, 102],
  "binaryX86Path": "agent/dodevops-agent-linux-amd64",
  "binaryArm64Path": "agent/dodevops-agent-linux-arm64"
}
```

```json
{
  "hostIds": [101, 102],
  "version": "2.1.0",
  "deployMode": "offline",
  "binaryX86Path": "agent/dodevops-agent-linux-amd64",
  "binaryArm64Path": "agent/dodevops-agent-linux-arm64"
}
```

服务端通过 SSH 在目标主机执行 `uname -m`，自动选择 amd64 或 arm64 二进制，上传到 `/opt/agent/dodevops-agent`。配置文件由后端根据每台主机的 CMDB 分组、主机标识和全局 Agent/Pushgateway 配置自动生成，上传为 `/opt/agent/config.json`，写入 systemd 并以 `--config /opt/agent/config.json` 启动。常规离线部署必须同时提供 x86_64 和 arm64 二进制（`binaryPath` 可用于明确指定单个二进制），所有路径都必须位于 `uploadDir` 内。

部署后的进度、启动状态和心跳状态继续使用现有 Agent 列表及 `GET /api/v1/cmdb/agent/status/:id` 接口查询。

# 离线安装 agent
cd /Users/apple/Desktop/git/autoops-api
bash scripts/build-offline-agent.sh

# 指定参数离线安装
AGENT_VERSION=2.1.0 \
AGENT_HEARTBEAT_URL=https://autoops.example.com/api/v1/cmdb/agent/heartbeat \
AGENT_HEARTBEAT_TOKEN=your-token \
bash scripts/build-offline-agent.sh
