package agent

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
)

// Agent版本信息在这里定义已经在其他文件中定义了，这里不需要重复

// findProjectRoot 查找包含go.mod的项目根目录
func findProjectRoot() string {
	// 获取当前执行文件的路径
	execPath, err := exec.LookPath(os.Args[0])
	if err != nil {
		return "/tmp" // 如果都失败了，返回临时目录
	}

	// 从执行文件所在目录向上查找go.mod
	dir := execPath
	for i := 0; i < 10; i++ { // 最多向上查找10级目录
		dir = dir[:strings.LastIndex(dir, "/")]
		if _, err := os.Stat(dir + "/go.mod"); err == nil {
			return dir
		}
		if dir == "/" || dir == "" {
			break
		}
	}

	// 如果还是找不到，返回当前目录
	if pwd, err := os.Getwd(); err == nil {
		return pwd
	}
	return "/tmp"
}

// CreateAgentMainFile 创建独立可执行的agent main文件（完全独立，不依赖任何外部包）
// 新增 groupID 和 groupName 参数，用于在所有指标中注入分组标签
func CreateAgentMainFile(heartbeatURL, heartbeatToken, pushgatewayURL string, groupID uint, groupName, agentVersion, agentVersionTag string) string {
	log.Printf("CreateAgentMainFile: 开始生成agent代码，heartbeatURL=%s, groupID=%d, groupName=%s", heartbeatURL, groupID, groupName)

	// 设置默认值
	if heartbeatURL == "" {
		heartbeatURL = "http://127.0.0.1:8000/api/v1/cmdb/agent/heartbeat"
	}
	if heartbeatToken == "" {
		heartbeatToken = "agent-heartbeat-token-2024"
	}
	if pushgatewayURL == "" {
		pushgatewayURL = "http://8.130.14.34:9091"
	}
	if groupName == "" {
		groupName = "default"
	}
	if agentVersion == "" {
		agentVersion = "2.0.0"
	}
	if agentVersionTag == "" {
		agentVersionTag = "v2"
	}

	// 直接构建完整的agent代码
	mainCode := fmt.Sprintf(`package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/client_golang/prometheus/push"
	"github.com/prometheus/common/expfmt"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
	psnet "github.com/shirou/gopsutil/v3/net"
	"github.com/shirou/gopsutil/v3/process"
)

// Agent版本信息
const (
	AgentVersion    = "%[6]s" // Agent版本号
	AgentVersionTag = "%[7]s" // Agent版本标签
	AgentBuildTime  = ""      // 构建时间(编译时可注入)
)

// 主机分组信息 - 编译时注入
	const (
	defaultHostGroupID   = %[4]d
	defaultHostGroupName = "%[5]s"
	)

	var (
		hostGroupID   = uint(defaultHostGroupID)
		hostGroupName = defaultHostGroupName
		agentVersion  = AgentVersion
		agentVersionTag = AgentVersionTag
	)

	// Config 结构体定义
	type Config struct {
	Version     string `+"`json:\"version\"`"+`
	VersionTag  string `+"`json:\"version_tag\"`"+`
	Processes []struct {
		Name string `+"`json:\"name\"`"+`
	}
	Endpoints []struct {
		URL    string `+"`json:\"url\"`"+`
		Method string `+"`json:\"method\"`"+`
	}
	Pushgateway struct {
		URL          string `+"`json:\"url\"`"+`
		PushInterval string `+"`json:\"push_interval\"`"+`
		Job          string `+"`json:\"job\"`"+`
		Instance     string `+"`json:\"instance\"`"+`
	}
	Heartbeat struct {
		ServerURL string `+"`json:\"server_url\"`"+`
		Interval  string `+"`json:\"interval\"`"+`
		Token     string `+"`json:\"token\"`"+`
	}
	Logtail struct {
		BaseURL      string `+"`json:\"base_url\"`"+`
		PollInterval string `+"`json:\"poll_interval\"`"+`
		Token        string `+"`json:\"token\"`"+`
	}
	CollectInterval string `+"`json:\"collect_interval\"`"+`
	Group struct {
		ID   uint   `+"`json:\"id\"`"+`
		Name string `+"`json:\"name\"`"+`
	}
	}

// 声明全局变量
var (
	// CPU使用率监控
	cpuUsage = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "system_cpu_usage_percent",
			Help: "CPU使用率百分比",
		},
		[]string{"instance", "cpu"},
	)

	// 内存使用率监控
	memUsage = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "system_memory_usage_percent",
			Help: "内存使用率百分比",
		},
		[]string{"instance"},
	)

	// 总内存容量监控(GB)
	memTotal = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "system_memory_total_gb",
			Help: "系统总内存容量(GB)",
		},
		[]string{"instance"},
	)

	// 已用内存监控(GB)
	memUsed = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "system_memory_used_gb",
			Help: "已使用内存容量(GB)",
		},
		[]string{"instance"},
	)

	// 可用内存监控(GB)
	memFree = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "system_memory_free_gb",
			Help: "可用内存容量(GB)",
		},
		[]string{"instance"},
	)

	// 磁盘使用率监控
	diskUsage = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "system_disk_usage_percent",
			Help: "磁盘使用率百分比",
		},
		[]string{"instance", "device", "mountpoint"},
	)

	// 磁盘inode使用率监控
	diskInodeUsage = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "system_disk_inode_usage_percent",
			Help: "磁盘inode使用率百分比",
		},
		[]string{"instance", "device", "mountpoint"},
	)

	// 磁盘inode总数
	diskInodeTotal = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "system_disk_inode_total",
			Help: "磁盘inode总数",
		},
		[]string{"instance", "device", "mountpoint"},
	)

	// 磁盘inode已用数
	diskInodeUsed = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "system_disk_inode_used",
			Help: "磁盘inode已用数",
		},
		[]string{"instance", "device", "mountpoint"},
	)

	// 磁盘inode可用数
	diskInodeFree = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "system_disk_inode_free",
			Help: "磁盘inode可用数",
		},
		[]string{"instance", "device", "mountpoint"},
	)

	// 系统负载监控 - 1分钟
	systemLoad1min = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "system_load_average_1min",
			Help: "系统负载平均值-1分钟",
		},
		[]string{"instance"},
	)

	// 系统负载监控 - 5分钟
	systemLoad5min = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "system_load_average_5min",
			Help: "系统负载平均值-5分钟",
		},
		[]string{"instance"},
	)

	// 系统负载监控 - 15分钟
	systemLoad15min = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "system_load_average_15min",
			Help: "系统负载平均值-15分钟",
		},
		[]string{"instance"},
	)

	// 系统负载比 - 1分钟 (负载/CPU核数)
	systemLoadRatio1min = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "system_load_ratio_1min",
			Help: "系统负载比-1分钟(负载/CPU核数),>1表示过载",
		},
		[]string{"instance"},
	)

	// 系统负载比 - 5分钟
	systemLoadRatio5min = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "system_load_ratio_5min",
			Help: "系统负载比-5分钟(负载/CPU核数),>1表示过载",
		},
		[]string{"instance"},
	)

	// 系统负载比 - 15分钟
	systemLoadRatio15min = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "system_load_ratio_15min",
			Help: "系统负载比-15分钟(负载/CPU核数),>1表示过载",
		},
		[]string{"instance"},
	)

	// CPU核数(用于负载比计算)
	cpuCoreCount = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "system_cpu_core_count",
			Help: "系统CPU核心数",
		},
		[]string{"instance"},
	)

	// 磁盘读取速度监控
	diskRead = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "system_disk_read_kb_per_second",
			Help: "磁盘读取速度(KB/秒)",
		},
		[]string{"instance", "device"},
	)

	// 磁盘写入速度监控
	diskWrite = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "system_disk_write_kb_per_second",
			Help: "磁盘写入速度(KB/秒)",
		},
		[]string{"instance", "device"},
	)

	// 进程总数监控
	totalProcesses = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "system_total_processes",
			Help: "系统进程总数",
		},
		[]string{"instance"},
	)

	// 网络接收流量监控
	netRecv = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "system_network_receive_kb_per_second",
			Help: "网络接收流量速度(KB/秒)",
		},
		[]string{"instance"},
	)

	// 网络发送流量监控
	netSent = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "system_network_send_kb_per_second",
			Help: "网络发送流量速度(KB/秒)",
		},
		[]string{"instance"},
	)

	// 进程CPU占用监控
	processCPU = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "process_cpu_percent",
			Help: "TOP进程CPU使用率百分比",
		},
		[]string{"host", "name"},
	)

	// 进程内存占用监控
	processMem = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "process_memory_percent",
			Help: "TOP进程内存使用率百分比",
		},
		[]string{"host", "name"},
	)

	// TCP端口服务监控
	tcpPortStatus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "tcp_port_listening",
			Help: "TCP端口监听状态(1=监听中,0=未监听)",
		},
		[]string{"instance", "port", "service"},
	)

	// 监听服务CPU使用率监控
	processCPUUsage = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "process_cpu_usage_percent",
			Help: "监听服务CPU使用率百分比",
		},
		[]string{"instance", "name"},
	)

	// 监听服务内存使用率监控
	processMemoryUsage = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "process_memory_usage_percent",
			Help: "监听服务内存使用率百分比",
		},
		[]string{"instance", "name"},
	)

	// 端点响应时间监控
	endpointResponseTime = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "endpoint_response_time_ms",
			Help: "HTTP端点响应时间(毫秒)",
		},
		[]string{"host", "url", "method"},
	)

	// 端点存活监控
	endpointStatus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "endpoint_status",
			Help: "HTTP端点存活状态(1=正常,0=异常)",
		},
		[]string{"host", "url", "method"},
	)

	// Ping响应时间监控
	pingResponseTime = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "ping_response_time_ms",
			Help: "Ping网络连通性响应时间(毫秒)",
		},
		[]string{"host", "target"},
	)

	// Ping存活监控
	pingStatus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "ping_status",
			Help: "Ping网络连通性存活状态(1=成功,0=失败)",
		},
		[]string{"host", "target"},
	)


	// TCP连接数
	tcpConnections = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "system_tcp_connections",
			Help: "TCP连接总数",
		},
		[]string{"instance"},
	)

	// V2新增指标 - UDP连接数
	udpConnections = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "system_udp_connections",
			Help: "UDP连接总数",
		},
		[]string{"instance"},
	)

	// V2新增指标 - 磁盘读IOPS
	diskReadIOPS = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "system_disk_read_iops",
			Help: "磁盘读IOPS(次/秒)",
		},
		[]string{"instance", "device"},
	)

	// V2新增指标 - 磁盘写IOPS
	diskWriteIOPS = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "system_disk_write_iops",
			Help: "磁盘写IOPS(次/秒)",
		},
		[]string{"instance", "device"},
	)

	// V2新增指标 - CPU用户态使用率
	cpuUserUsage = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "system_cpu_user_percent",
			Help: "CPU用户态使用率百分比",
		},
		[]string{"instance"},
	)

	// V2新增指标 - CPU内核态使用率
	cpuSystemUsage = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "system_cpu_system_percent",
			Help: "CPU内核态使用率百分比",
		},
		[]string{"instance"},
	)

	// 交换内存使用量(GB)
	swapUsed = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "system_swap_used_gb",
			Help: "交换内存使用量(GB)",
		},
		[]string{"instance"},
	)

	// V2新增指标 - 磁盘已用空间(GB)
	diskUsedBytes = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "system_disk_used_bytes",
			Help: "磁盘已用空间(GB) - 直接存储GB值",
		},
		[]string{"instance", "device", "mountpoint"},
	)

	// V2新增指标 - 磁盘总空间(GB)
	diskTotalBytes = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "system_disk_total_bytes",
			Help: "磁盘总空间(GB) - 直接存储GB值",
		},
		[]string{"instance", "device", "mountpoint"},
	)

	// V2新增指标 - 服务CPU使用率
	serviceCPUUsage = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "service_cpu_usage_percent",
			Help: "服务CPU使用率百分比",
		},
		[]string{"instance", "service_name"},
	)

	// V2新增指标 - 服务内存使用率
	serviceMemoryUsage = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "service_memory_usage_percent",
			Help: "服务内存使用率百分比",
		},
		[]string{"instance", "service_name"},
	)

	// V2新增指标 - Agent版本信息
	agentInfo = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "agent_info",
			Help: "Agent版本信息(值固定为1)",
		},
		[]string{"instance", "version", "version_tag"},
	)

	// V2.1新增指标 - 已用内存(MB)
	memUsedMB = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "system_memory_used_mb",
			Help: "已使用内存容量(MB)",
		},
		[]string{"instance"},
	)

	// V2.1新增指标 - 总内存(MB)
	memTotalMB = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "system_memory_total_mb",
			Help: "系统总内存容量(MB)",
		},
		[]string{"instance"},
	)

	// V2.1新增指标 - 可用内存(MB)
	memFreeMB = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "system_memory_free_mb",
			Help: "可用内存容量(MB)",
		},
		[]string{"instance"},
	)

	// V2.1新增指标 - 交换内存(MB)
	swapUsedMB = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "system_swap_used_mb",
			Help: "交换内存使用量(MB)",
		},
		[]string{"instance"},
	)

	// V2.1新增指标 - 系统运行天数
	systemUptimeDays = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "system_uptime_days",
			Help: "系统运行天数",
		},
		[]string{"instance"},
	)

	// 文件打开数量
	openFileDescriptors = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "system_open_file_descriptors",
			Help: "系统当前打开的文件描述符数量",
		},
		[]string{"instance"},
	)

	// 上一次读取的磁盘IO数据，用于计算速率
	lastDiskIO  disk.IOCountersStat
	lastNetIO   psnet.IOCountersStat
	lastCPUTime cpu.TimesStat // 保存上次CPU时间用于计算增量

	runtimeHostID uint
)

// HeartbeatData 心跳数据结构
type HeartbeatData struct {
	PID      int    `+"`json:\"pid\"`"+`
	IP       string `+"`json:\"ip\"`"+`
	Hostname string `+"`json:\"hostname\"`"+`
	Port     int    `+"`json:\"port\"`"+`
	Token    string `+"`json:\"token\"`"+`
	Version    string `+"`json:\"version\"`"+`    // Agent版本号
	VersionTag string `+"`json:\"versionTag\"`"+` // Agent版本标签
}

func getLocalIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		log.Printf("Error getting local IP: %%v", err)
		return ""
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String()
}

func sendHeartbeat(serverURL, token string) {
	pid := os.Getpid()
	localIP := getLocalIP()
	if localIP == "" {
		log.Println("Warning: Could not determine local IP address")
		return
	}

	// 获取hostname
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

		heartbeat := HeartbeatData{
		PID:        pid,
		IP:         localIP,
		Hostname:   hostname,
		Port:       12580,
		Token:      token,
		Version:    agentVersion,
		VersionTag: agentVersionTag,
	}

	jsonData, err := json.Marshal(heartbeat)
	if err != nil {
		log.Printf("Error marshaling heartbeat data: %%v", err)
		return
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(serverURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("Error sending heartbeat: %%v", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == 200 {
		log.Printf("Heartbeat sent successfully - PID: %%d, IP: %%s, Port: %%d, Token: %%s", pid, localIP, 12580, token)
		var wrapper struct {
			Data map[string]interface{}
		}
		if err := json.Unmarshal(body, &wrapper); err == nil {
			if value, ok := wrapper.Data["hostId"]; ok {
				switch v := value.(type) {
				case float64:
					runtimeHostID = uint(v)
				case string:
					if parsed, parseErr := strconv.ParseUint(v, 10, 32); parseErr == nil {
						runtimeHostID = uint(parsed)
					}
				}
			}
		}
	} else {
		log.Printf("Heartbeat failed with status: %%d", resp.StatusCode)
	}
}

type APIResult struct {
	Code    int
	Message string
	Data    json.RawMessage
}

type VectorPackageInfo struct {
	PackagePath string
	PackageURL  string
	Exists      bool
	FileName    string
	SizeBytes   int64
	SHA256      string
	Version     string
	ModTime     string
	Error       string
}

type LogtailConfigResponse struct {
	NeedUpdate    bool
	AgentID       uint
	HostID        uint
	ConfigVersion int
	ConfigHash    string
	ConfigPath    string
	Content       string
	Package       VectorPackageInfo
	HostAliases   []KafkaHostAlias
}

type KafkaHostAlias struct {
	IP        string
	Hostnames []string
}

type LogtailOperation struct {
	ID            uint
	AgentID       uint
	HostID        uint
	OpType        string
	OpStatus      string
	Progress      int
	ProgressText  string
	TargetVersion string
	ConfigVersion int
	RequestJSON   string
	ResultJSON    string
	ErrorMsg      string
	Operator      string
}

type LogtailOperationDispatch struct {
	Operation LogtailOperation
	Package   VectorPackageInfo
	Config    *LogtailConfigResponse
}

type LogtailOperationResponse struct {
	AgentID    uint
	HostID     uint
	Operations []LogtailOperationDispatch
}

type VectorState struct {
	DesiredConfigVersion int
	AppliedConfigVersion int
	DesiredConfigHash    string
	AppliedConfigHash    string
}

const (
	vectorInstallBase = "/opt/aiops/vector"
	vectorCurrentPath = "/opt/aiops/vector/current"
	vectorConfigPath  = "/opt/aiops/vector/current/vector.yaml"
	vectorDataDir     = "/var/lib/aiops-vector"
	vectorLogDir      = "/var/log/aiops-vector"
	vectorServicePath = "/etc/systemd/system/aiops-vector.service"
)

func startLogtailRuntime(config *Config) {
	baseURL := strings.TrimSpace(config.Logtail.BaseURL)
	if baseURL == "" {
		baseURL = deriveLogtailBaseURL(config.Heartbeat.ServerURL)
	}
	if baseURL == "" {
		log.Println("[Logtail] base url empty, skip Vector runtime")
		return
	}
	token := strings.TrimSpace(config.Logtail.Token)
	if token == "" {
		token = config.Heartbeat.Token
	}
	pollInterval := durationOrDefault(config.Logtail.PollInterval, 30*time.Second)
	go func() {
		log.Printf("[Logtail] Vector runtime started, baseURL=%%s interval=%%s", baseURL, pollInterval)
		ticker := time.NewTicker(pollInterval)
		defer ticker.Stop()
		for {
			if runtimeHostID > 0 {
				processLogtailOperations(baseURL, token)
				syncLogtailConfig(baseURL, token)
				reportVectorStatus(baseURL, token, "")
			} else {
				debugf("[Logtail] hostId not ready, wait heartbeat")
			}
			<-ticker.C
		}
	}()
}

func deriveLogtailBaseURL(heartbeatURL string) string {
	value := strings.TrimSpace(heartbeatURL)
	for _, suffix := range []string{"/monitor/agent/heartbeat", "/cmdb/agent/heartbeat"} {
		if strings.HasSuffix(value, suffix) {
			return strings.TrimSuffix(value, suffix)
		}
	}
	return ""
}

func logtailURL(baseURL, path string) string {
	return strings.TrimRight(baseURL, "/") + path
}

func getAgentJSON(baseURL, path, token string, out interface{}) error {
	req, err := http.NewRequest("GET", logtailURL(baseURL, path), nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-Agent-Token", token)
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	var wrapper APIResult
	if err := json.Unmarshal(body, &wrapper); err != nil {
		return err
	}
	if wrapper.Code != 200 {
		return fmt.Errorf("api failed: code=%%d message=%%s", wrapper.Code, wrapper.Message)
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(wrapper.Data, out)
}

func postAgentJSON(baseURL, path, token string, payload interface{}, out interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("POST", logtailURL(baseURL, path), bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Agent-Token", token)
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	var wrapper APIResult
	if err := json.Unmarshal(body, &wrapper); err != nil {
		return err
	}
	if wrapper.Code != 200 {
		return fmt.Errorf("api failed: code=%%d message=%%s", wrapper.Code, wrapper.Message)
	}
	if out == nil || len(wrapper.Data) == 0 {
		return nil
	}
	return json.Unmarshal(wrapper.Data, out)
}

func processLogtailOperations(baseURL, token string) {
	var resp LogtailOperationResponse
	path := fmt.Sprintf("/logtail/agent/operations?hostId=%%d&limit=10", runtimeHostID)
	if err := getAgentJSON(baseURL, path, token, &resp); err != nil {
		log.Printf("[Logtail] claim operations failed: %%v", err)
		return
	}
	for _, dispatch := range resp.Operations {
		reportOperationProgress(baseURL, token, dispatch.Operation.ID, 0, "开始执行", "")
		resultJSON, err := executeLogtailOperation(baseURL, token, dispatch)
		status := "success"
		errMsg := ""
		if err != nil {
			status = "failed"
			errMsg = err.Error()
			log.Printf("[Logtail] operation failed: id=%%d type=%%s err=%%v", dispatch.Operation.ID, dispatch.Operation.OpType, err)
		} else if runtimeErr := runtimeErrorForOperation(dispatch.Operation.OpType); runtimeErr != "" {
			status = "failed"
			errMsg = runtimeErr
			log.Printf("[Logtail] operation runtime check failed: id=%%d type=%%s err=%%s", dispatch.Operation.ID, dispatch.Operation.OpType, runtimeErr)
		}
		reportOperationResult(baseURL, token, dispatch.Operation.ID, status, resultJSON, errMsg)
		reportVectorStatus(baseURL, token, errMsg)
	}
}

func executeLogtailOperation(baseURL, token string, dispatch LogtailOperationDispatch) (string, error) {
	op := dispatch.Operation
	switch op.OpType {
	case "install_vector", "upgrade_vector":
		version, err := installVectorPackage(baseURL, dispatch.Package, func(progress int, text string) {
			reportOperationProgress(baseURL, token, op.ID, progress, text, "")
		})
		if err != nil {
			return "", err
		}
		return fmt.Sprintf(`+"`{\"message\":\"vector installed\",\"version\":\"%%s\"}`"+`, version), nil
	case "validate_config":
		if dispatch.Config == nil {
			cfg, err := fetchLogtailConfig(baseURL, token)
			if err != nil {
				return "", err
			}
			dispatch.Config = cfg
		}
		if err := writeAndValidateVectorConfig(dispatch.Config, false, func(progress int, text string) {
			reportOperationProgress(baseURL, token, op.ID, progress, text, "")
		}); err != nil {
			return "", err
		}
		return `+"`{\"message\":\"config validated\"}`"+`, nil
	case "reload_vector", "restart_vector":
		if dispatch.Config == nil {
			cfg, err := fetchLogtailConfig(baseURL, token)
			if err != nil {
				return "", err
			}
			dispatch.Config = cfg
		}
		if isEmptyLogtailConfig(dispatch.Config) {
			reportOperationProgress(baseURL, token, op.ID, 80, "清理 Vector 采集配置", "")
			clearVectorRuntimeConfig()
			return `+"`{\"message\":\"vector config cleared\"}`"+`, nil
		}
		if err := writeAndValidateVectorConfig(dispatch.Config, true, func(progress int, text string) {
			reportOperationProgress(baseURL, token, op.ID, progress, text, "")
		}); err != nil {
			return "", err
		}
		return `+"`{\"message\":\"vector restarted\"}`"+`, nil
	case "stop_vector":
		reportOperationProgress(baseURL, token, op.ID, 50, "停止 Vector 服务", "")
		if err := runCommand("systemctl", "stop", "aiops-vector"); err != nil {
			return "", err
		}
		return `+"`{\"message\":\"vector stopped\"}`"+`, nil
	case "uninstall_vector":
		if err := uninstallVectorPackage(func(progress int, text string) {
			reportOperationProgress(baseURL, token, op.ID, progress, text, "")
		}); err != nil {
			return "", err
		}
		return `+"`{\"message\":\"vector uninstalled\"}`"+`, nil
	default:
		return "", fmt.Errorf("unsupported operation: %%s", op.OpType)
	}
}

func syncLogtailConfig(baseURL, token string) {
	cfg, err := fetchLogtailConfig(baseURL, token)
	if err != nil {
		log.Printf("[Logtail] fetch config failed: %%v", err)
		return
	}
	if isEmptyLogtailConfig(cfg) {
		clearVectorRuntimeConfig()
		return
	}
	if err := applyKafkaHostAliases(cfg.HostAliases); err != nil {
		log.Printf("[Logtail] apply kafka host aliases failed: %%v", err)
		reportVectorStatus(baseURL, token, err.Error())
		return
	}
	if !cfg.NeedUpdate {
		return
	}
	if _, err := os.Stat(vectorBinary()); err != nil {
		debugf("[Logtail] vector not installed, skip config apply")
		return
	}
	if err := writeAndValidateVectorConfig(cfg, true, nil); err != nil {
		log.Printf("[Logtail] apply config failed: %%v", err)
		reportVectorStatus(baseURL, token, err.Error())
		return
	}
	reportVectorStatus(baseURL, token, "")
}

func fetchLogtailConfig(baseURL, token string) (*LogtailConfigResponse, error) {
	state := loadVectorState()
	path := fmt.Sprintf("/logtail/agent/config?hostId=%%d&currentVersion=%%d&currentHash=%%s",
		runtimeHostID,
		state.AppliedConfigVersion,
		url.QueryEscape(state.AppliedConfigHash),
	)
	var cfg LogtailConfigResponse
	if err := getAgentJSON(baseURL, path, token, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func isEmptyLogtailConfig(cfg *LogtailConfigResponse) bool {
	if cfg == nil {
		return true
	}
	return cfg.ConfigVersion == 0 && strings.TrimSpace(cfg.Content) == ""
}

func clearVectorRuntimeConfig() {
	if _, err := os.Stat(vectorBinary()); err != nil {
		return
	}
	_ = runCommandAllowFailure("systemctl", "stop", "aiops-vector")
	_ = os.Remove(vectorConfigPath)
	saveVectorState(VectorState{})
}

func installVectorPackage(baseURL string, pkg VectorPackageInfo, progress func(int, string)) (string, error) {
	if progress != nil {
		progress(10, "准备安装目录")
	}
	if strings.TrimSpace(pkg.PackageURL) == "" {
		return "", fmt.Errorf("package url empty")
	}
	if err := os.MkdirAll(vectorInstallBase, 0755); err != nil {
		return "", err
	}
	if err := os.MkdirAll(vectorDataDir, 0755); err != nil {
		return "", err
	}
	if err := os.MkdirAll(vectorLogDir, 0755); err != nil {
		return "", err
	}
	tmpDir := filepath.Join(os.TempDir(), "aiops-vector")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return "", err
	}
	tmpFile := filepath.Join(tmpDir, "vector")
	if progress != nil {
		progress(25, "下载 Vector 包")
	}
	if err := downloadFile(absoluteURL(baseURL, pkg.PackageURL), tmpFile); err != nil {
		return "", err
	}
	if pkg.SHA256 != "" {
		if progress != nil {
			progress(45, "校验安装包")
		}
		sha, err := fileSHA256(tmpFile)
		if err != nil {
			return "", err
		}
		if !strings.EqualFold(sha, pkg.SHA256) {
			return "", fmt.Errorf("sha256 mismatch: got %%s want %%s", sha, pkg.SHA256)
		}
	}
	if err := os.Chmod(tmpFile, 0755); err != nil {
		return "", err
	}
	version, err := detectLocalVectorVersionDetailed(tmpFile)
	if err != nil {
		return "", err
	}
	if version == "" {
		version = fmt.Sprintf("unknown-%%d", time.Now().Unix())
	}
	version = sanitizePathName(version)
	installDir := filepath.Join(vectorInstallBase, version)
	if err := os.MkdirAll(installDir, 0755); err != nil {
		return "", err
	}
	target := filepath.Join(installDir, "vector")
	if progress != nil {
		progress(65, "写入安装目录")
	}
	if err := copyFile(tmpFile, target); err != nil {
		return "", err
	}
	if err := os.Chmod(target, 0755); err != nil {
		return "", err
	}
	_ = os.Remove(vectorCurrentPath)
	if err := os.Symlink(installDir, vectorCurrentPath); err != nil {
		return "", err
	}
	_ = os.Remove(vectorConfigPath)
	saveVectorState(VectorState{})
	if err := writeVectorServiceFile(); err != nil {
		return "", err
	}
	if progress != nil {
		progress(85, "注册并启用服务")
	}
	_ = runCommand("systemctl", "daemon-reload")
	_ = runCommand("systemctl", "enable", "aiops-vector")
	if progress != nil {
		progress(100, "安装完成")
	}
	return version, nil
}

func uninstallVectorPackage(progress func(int, string)) error {
	if progress != nil {
		progress(10, "检查 Vector 安装状态")
	}

	if progress != nil {
		progress(25, "停止 Vector 服务")
	}
	_ = runCommandAllowFailure("systemctl", "stop", "aiops-vector")

	if progress != nil {
		progress(40, "禁用 Vector 服务")
	}
	_ = runCommandAllowFailure("systemctl", "disable", "aiops-vector")

	if progress != nil {
		progress(55, "删除 systemd 服务文件")
	}
	_ = os.Remove(vectorServicePath)
	_ = runCommandAllowFailure("systemctl", "daemon-reload")
	_ = runCommandAllowFailure("systemctl", "reset-failed", "aiops-vector")

	if progress != nil {
		progress(70, "删除安装目录")
	}
	if err := os.RemoveAll(vectorInstallBase); err != nil {
		return err
	}

	if progress != nil {
		progress(85, "清理运行数据")
	}
	if err := os.RemoveAll(vectorDataDir); err != nil {
		return err
	}
	if err := os.RemoveAll(vectorLogDir); err != nil {
		return err
	}

	saveVectorState(VectorState{})

	if progress != nil {
		progress(100, "卸载完成")
	}
	return nil
}

func writeAndValidateVectorConfig(cfg *LogtailConfigResponse, restart bool, progress func(int, string)) error {
	if progress != nil {
		progress(20, "写入配置文件")
	}
	if cfg == nil || strings.TrimSpace(cfg.Content) == "" {
		return fmt.Errorf("empty vector config")
	}
	configPath := strings.TrimSpace(cfg.ConfigPath)
	if configPath == "" {
		configPath = vectorConfigPath
	}
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(vectorDataDir, 0755); err != nil {
		return err
	}
	tmpPath := strings.TrimSuffix(configPath, filepath.Ext(configPath)) + ".tmp" + filepath.Ext(configPath)
	if err := os.WriteFile(tmpPath, []byte(cfg.Content), 0644); err != nil {
		return err
	}
	if progress != nil {
		progress(50, "校验配置")
	}
	if err := runCommand(vectorBinary(), "validate", tmpPath); err != nil {
		return err
	}
	if progress != nil {
		progress(65, "写入 Kafka 主机解析")
	}
	if err := applyKafkaHostAliases(cfg.HostAliases); err != nil {
		return fmt.Errorf("apply kafka host aliases: %%w", err)
	}
	if err := os.Rename(tmpPath, configPath); err != nil {
		return err
	}
	saveVectorState(VectorState{
		DesiredConfigVersion: cfg.ConfigVersion,
		AppliedConfigVersion: cfg.ConfigVersion,
		DesiredConfigHash:    cfg.ConfigHash,
		AppliedConfigHash:    cfg.ConfigHash,
	})
	if restart {
		if progress != nil {
			progress(80, "重启 Vector 服务")
		}
		if err := writeVectorServiceFile(); err != nil {
			return err
		}
		_ = runCommand("systemctl", "daemon-reload")
		if err := runCommand("systemctl", "restart", "aiops-vector"); err != nil {
			return err
		}
	}
	if progress != nil {
		progress(100, "执行完成")
	}
	return nil
}

func applyKafkaHostAliases(aliases []KafkaHostAlias) error {
	const beginMarker = "# BEGIN AIOPS VECTOR KAFKA HOST ALIASES"
	const endMarker = "# END AIOPS VECTOR KAFKA HOST ALIASES"
	raw, err := os.ReadFile("/etc/hosts")
	if err != nil {
		return err
	}
	content := strings.ReplaceAll(string(raw), "\r\n", "\n")
	cleaned := removeManagedHostsBlock(content, beginMarker, endMarker)
	if len(aliases) == 0 {
		if cleaned == content {
			return nil
		}
		return os.WriteFile("/etc/hosts", []byte(strings.TrimRight(cleaned, "\n")+"\n"), 0644)
	}
	var builder strings.Builder
	builder.WriteString(strings.TrimRight(cleaned, "\n"))
	builder.WriteString("\n\n")
	builder.WriteString(beginMarker)
	builder.WriteString("\n")
	for _, alias := range aliases {
		ip := strings.TrimSpace(alias.IP)
		hostnames := normalizeHostAliasNames(alias.Hostnames)
		if ip == "" || len(hostnames) == 0 {
			continue
		}
		builder.WriteString(ip)
		builder.WriteString(" ")
		builder.WriteString(strings.Join(hostnames, " "))
		builder.WriteString("\n")
	}
	builder.WriteString(endMarker)
	builder.WriteString("\n")
	return os.WriteFile("/etc/hosts", []byte(builder.String()), 0644)
}

func removeManagedHostsBlock(content, beginMarker, endMarker string) string {
	begin := strings.Index(content, beginMarker)
	if begin < 0 {
		return content
	}
	end := strings.Index(content[begin:], endMarker)
	if end < 0 {
		return strings.TrimRight(content[:begin], "\n")
	}
	end += begin + len(endMarker)
	for end < len(content) && (content[end] == '\n' || content[end] == '\r') {
		end++
	}
	return strings.TrimRight(content[:begin], "\n") + "\n" + strings.TrimLeft(content[end:], "\n")
}

func normalizeHostAliasNames(values []string) []string {
	result := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func reportOperationResult(baseURL, token string, operationID uint, status, resultJSON, errorMsg string) {
	if operationID == 0 || runtimeHostID == 0 {
		return
	}
	progress := 100
	progressText := "执行完成"
	if status != "success" {
		progress = -1
		progressText = "执行失败"
	}
	payload := map[string]interface{}{
		"token":        token,
		"hostId":       runtimeHostID,
		"opStatus":     status,
		"progress":     progress,
		"progressText": progressText,
		"resultJson":   resultJSON,
		"errorMsg":     errorMsg,
	}
	path := fmt.Sprintf("/logtail/agent/operations/%%d/report", operationID)
	if err := postAgentJSON(baseURL, path, token, payload, nil); err != nil {
		log.Printf("[Logtail] report operation failed: %%v", err)
	}
}

func reportOperationProgress(baseURL, token string, operationID uint, progress int, progressText, resultJSON string) {
	if operationID == 0 || runtimeHostID == 0 {
		return
	}
	payload := map[string]interface{}{
		"token":        token,
		"hostId":       runtimeHostID,
		"opStatus":     "running",
		"progress":     progress,
		"progressText": progressText,
		"resultJson":   resultJSON,
		"errorMsg":     "",
	}
	path := fmt.Sprintf("/logtail/agent/operations/%%d/report", operationID)
	if err := postAgentJSON(baseURL, path, token, payload, nil); err != nil {
		log.Printf("[Logtail] report operation progress failed: %%v", err)
	}
}

func reportVectorStatus(baseURL, token, lastError string) {
	if runtimeHostID == 0 {
		return
	}
	state := loadVectorState()
	vectorStatus := "not_installed"
	vectorVersion := ""
	vectorPID := 0
	installPath := ""
	configPath := ""
	hasAppliedConfig := vectorHasAppliedConfig(state)
	if _, err := os.Stat(vectorBinary()); err == nil {
		vectorStatus = "installed"
		vectorVersion = detectLocalVectorVersion(vectorBinary())
		installPath = vectorCurrentPath
		configPath = vectorConfigPath
		if runCommand("systemctl", "is-active", "--quiet", "aiops-vector") == nil {
			vectorStatus = "running"
		} else if hasAppliedConfig {
			vectorStatus = "error"
		}
		if output, err := exec.Command("pidof", "vector").Output(); err == nil {
			first := strings.Fields(string(output))
			if len(first) > 0 {
				if pid, parseErr := strconv.Atoi(first[0]); parseErr == nil {
					vectorPID = pid
				}
			}
		}
	}
	configStatus := "pending"
	if state.AppliedConfigVersion > 0 && state.AppliedConfigVersion == state.DesiredConfigVersion && state.AppliedConfigHash == state.DesiredConfigHash {
		configStatus = "applied"
	}
	if vectorStatus == "error" && strings.TrimSpace(lastError) == "" {
		lastError = collectVectorErrorDetails()
	}
	if lastError != "" {
		configStatus = "failed"
		if vectorStatus != "not_installed" {
			vectorStatus = "error"
		}
	}
	payload := map[string]interface{}{
		"token":                token,
		"hostId":               runtimeHostID,
		"runtimeType":          "host",
		"vectorVersion":        vectorVersion,
		"vectorStatus":         vectorStatus,
		"vectorPid":            vectorPID,
		"installPath":          installPath,
		"configPath":           configPath,
		"desiredConfigVersion": state.DesiredConfigVersion,
		"appliedConfigVersion": state.AppliedConfigVersion,
		"desiredConfigHash":    state.DesiredConfigHash,
		"appliedConfigHash":    state.AppliedConfigHash,
		"configStatus":         configStatus,
		"lastError":            lastError,
		"metricsJson":          "{}",
	}
	if err := postAgentJSON(baseURL, "/logtail/agent/status", token, payload, nil); err != nil {
		log.Printf("[Logtail] report status failed: %%v", err)
	}
}

func vectorHasAppliedConfig(state VectorState) bool {
	if state.AppliedConfigVersion > 0 && strings.TrimSpace(state.AppliedConfigHash) != "" {
		return true
	}
	if stat, err := os.Stat(vectorConfigPath); err == nil && stat.Size() > 0 {
		return true
	}
	return false
}

func vectorBinary() string {
	return filepath.Join(vectorCurrentPath, "vector")
}

func writeVectorServiceFile() error {
	content := "[Unit]\n" +
		"Description=AIOps Vector Log Collector\n" +
		"After=network-online.target\n" +
		"Wants=network-online.target\n\n" +
		"[Service]\n" +
		"Type=simple\n" +
		"ExecStart=" + vectorBinary() + " --config " + vectorConfigPath + "\n" +
		"Restart=always\n" +
		"RestartSec=5\n" +
		"LimitNOFILE=65536\n\n" +
		"[Install]\n" +
		"WantedBy=multi-user.target\n"
	return os.WriteFile(vectorServicePath, []byte(content), 0644)
}

func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("command failed: %%s %%s: %%w output=%%s", name, strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return nil
}

func runCommandAllowFailure(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		debugf("[Logtail] ignore command error: %%s %%s err=%%v output=%%s", name, strings.Join(args, " "), err, strings.TrimSpace(string(output)))
		return err
	}
	return nil
}

func runCommandOutput(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(output)), err
}

func downloadFile(rawURL, dest string) error {
	resp, err := http.Get(rawURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("download failed: status=%%d", resp.StatusCode)
	}
	tmp := dest + ".download"
	file, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(file, resp.Body); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, dest)
}

func absoluteURL(baseURL, raw string) string {
	parsed, err := url.Parse(raw)
	if err == nil && parsed.IsAbs() {
		return raw
	}
	base, err := url.Parse(baseURL)
	if err != nil || base.Scheme == "" || base.Host == "" {
		return raw
	}
	if strings.HasPrefix(raw, "/") {
		return base.Scheme + "://" + base.Host + raw
	}
	return strings.TrimRight(baseURL, "/") + "/" + strings.TrimLeft(raw, "/")
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return fmt.Sprintf("%%X", hash.Sum(nil)), nil
}

func detectLocalVectorVersion(path string) string {
	version, err := detectLocalVectorVersionDetailed(path)
	if err != nil {
		return ""
	}
	return version
}

func detectLocalVectorVersionDetailed(path string) (string, error) {
	output, err := runCommandOutput(path, "--version")
	if err != nil {
		if output == "" {
			return "", fmt.Errorf("vector binary check failed: %%w", err)
		}
		return "", fmt.Errorf("vector binary check failed: %%w output=%%s", err, output)
	}
	fields := strings.Fields(string(output))
	if len(fields) >= 2 {
		return fields[1], nil
	}
	return strings.TrimSpace(string(output)), nil
}

func runtimeErrorForOperation(opType string) string {
	switch opType {
	case "reload_vector", "restart_vector":
		if runCommand("systemctl", "is-active", "--quiet", "aiops-vector") != nil {
			return collectVectorErrorDetails()
		}
	}
	return ""
}

func collectVectorErrorDetails() string {
	sections := make([]string, 0, 3)
	if _, err := os.Stat(vectorBinary()); err == nil {
		if _, versionErr := detectLocalVectorVersionDetailed(vectorBinary()); versionErr != nil {
			sections = append(sections, versionErr.Error())
		}
	}
	if output, _ := runCommandOutput("systemctl", "status", "aiops-vector", "--no-pager"); output != "" {
		sections = append(sections, "systemctl status:\n"+truncateText(output, 1200))
	}
	if output, _ := runCommandOutput("journalctl", "-u", "aiops-vector", "-n", "50", "--no-pager"); output != "" && !strings.Contains(output, "-- No entries --") {
		sections = append(sections, "journalctl:\n"+truncateText(output, 2000))
	}
	if len(sections) == 0 {
		return "Vector 服务异常，但暂未采集到更多错误详情"
	}
	return truncateText(strings.Join(sections, "\n\n"), 4000)
}

func truncateText(value string, max int) string {
	value = strings.TrimSpace(value)
	if max <= 0 || len(value) <= max {
		return value
	}
	return strings.TrimSpace(value[:max]) + "\n...(truncated)"
}

func sanitizePathName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}
	replacer := strings.NewReplacer("/", "_", "\\", "_", " ", "_", ":", "_")
	return replacer.Replace(value)
}

func vectorStatePath() string {
	return filepath.Join(vectorDataDir, "state.json")
}

func loadVectorState() VectorState {
	var state VectorState
	data, err := os.ReadFile(vectorStatePath())
	if err != nil {
		return state
	}
	_ = json.Unmarshal(data, &state)
	return state
}

func saveVectorState(state VectorState) {
	_ = os.MkdirAll(vectorDataDir, 0755)
	data, _ := json.Marshal(state)
	_ = os.WriteFile(vectorStatePath(), data, 0644)
}

func upsertMetricLabel(metric *dto.Metric, name, value string) {
	if metric == nil || name == "" || value == "" {
		return
	}
	for _, label := range metric.Label {
		if label.GetName() == name {
			label.Value = &value
			return
		}
	}
	metric.Label = append(metric.Label, &dto.LabelPair{Name: &name, Value: &value})
}

func pushMetricsToImportEndpoint(importURL string, registry *prometheus.Registry, config *Config) error {
	families, err := registry.Gather()
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	encoder := expfmt.NewEncoder(&buf, expfmt.NewFormat(expfmt.TypeTextPlain))
	for _, family := range families {
		for _, metric := range family.Metric {
			upsertMetricLabel(metric, "job", config.Pushgateway.Job)
			upsertMetricLabel(metric, "instance", config.Pushgateway.Job)
			upsertMetricLabel(metric, "hostname", config.Pushgateway.Instance)
			upsertMetricLabel(metric, "group_id", fmt.Sprintf("%%d", hostGroupID))
			upsertMetricLabel(metric, "group_name", hostGroupName)
		}
		if err := encoder.Encode(family); err != nil {
			return err
		}
	}

	req, err := http.NewRequest(http.MethodPost, importURL, bytes.NewReader(buf.Bytes()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", string(expfmt.NewFormat(expfmt.TypeTextPlain)))

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("metrics import returned status %%d", resp.StatusCode)
	}
	return nil
}

func collectSystemMetrics() {
	// 使用本地IP作为instance标识（用于Prometheus查询区分主机）
	instanceID := getLocalIP()
	if instanceID == "" {
		// 如果获取IP失败，使用hostname作为备选
		instanceID, err := os.Hostname()
		if err != nil || instanceID == "" {
			instanceID = "unknown"
		}
	}

	// 采集CPU指标（总体使用率）
	if cpuPercents, err := cpu.Percent(0, false); err == nil {
		if len(cpuPercents) > 0 {
			cpuUsage.WithLabelValues(instanceID, "total").Set(cpuPercents[0])
		}
	}

	// 采集内存指标
	if memStats, err := mem.VirtualMemory(); err == nil {
		memUsage.WithLabelValues(instanceID).Set(memStats.UsedPercent)
		// GB单位的内存指标（使用Total-Available计算真实已用内存，与free命令一致）
		realUsed := float64(memStats.Total - memStats.Available)
		memTotal.WithLabelValues(instanceID).Set(float64(memStats.Total) / 1024 / 1024 / 1024)
		memUsed.WithLabelValues(instanceID).Set(realUsed / 1024 / 1024 / 1024)
		memFree.WithLabelValues(instanceID).Set(float64(memStats.Available) / 1024 / 1024 / 1024)
	}

	// 采集磁盘指标
	if diskStats, err := disk.Partitions(false); err == nil {
		for _, partition := range diskStats {
			// 只采集根分区
			if partition.Mountpoint == "/" {
				if usage, err := disk.Usage(partition.Mountpoint); err == nil {
					// 磁盘空间使用率
					diskUsage.WithLabelValues(instanceID, partition.Device, partition.Mountpoint).Set(usage.UsedPercent)
					diskUsedBytes.WithLabelValues(instanceID, partition.Device, partition.Mountpoint).Set(float64(usage.Used) / 1024 / 1024 / 1024)
					diskTotalBytes.WithLabelValues(instanceID, partition.Device, partition.Mountpoint).Set(float64(usage.Total) / 1024 / 1024 / 1024)

					// inode使用率
					if usage.InodesTotal > 0 {
						inodeUsedPercent := float64(usage.InodesUsed) / float64(usage.InodesTotal) * 100
						diskInodeUsage.WithLabelValues(instanceID, partition.Device, partition.Mountpoint).Set(inodeUsedPercent)
						diskInodeTotal.WithLabelValues(instanceID, partition.Device, partition.Mountpoint).Set(float64(usage.InodesTotal))
						diskInodeUsed.WithLabelValues(instanceID, partition.Device, partition.Mountpoint).Set(float64(usage.InodesUsed))
						diskInodeFree.WithLabelValues(instanceID, partition.Device, partition.Mountpoint).Set(float64(usage.InodesFree))
					}
				}
			}
		}
	}

	// 采集负载平均值和负载比
	if loadStats, err := load.Avg(); err == nil {
		// 原始负载值
		systemLoad1min.WithLabelValues(instanceID).Set(loadStats.Load1)
		systemLoad5min.WithLabelValues(instanceID).Set(loadStats.Load5)
		systemLoad15min.WithLabelValues(instanceID).Set(loadStats.Load15)

		// 获取CPU核数
		cpuCount, err := cpu.Counts(true)
		if err != nil {
			log.Printf("获取CPU核数失败: %%v", err)
		} else if cpuCount <= 0 {
			log.Printf("CPU核数异常: %%d", cpuCount)
		} else {
			// CPU核数指标
			cpuCoreCount.WithLabelValues(instanceID).Set(float64(cpuCount))

			// 计算负载比 (负载/核数)
			ratio1min := loadStats.Load1 / float64(cpuCount)
			ratio5min := loadStats.Load5 / float64(cpuCount)
			ratio15min := loadStats.Load15 / float64(cpuCount)

			systemLoadRatio1min.WithLabelValues(instanceID).Set(ratio1min)
			systemLoadRatio5min.WithLabelValues(instanceID).Set(ratio5min)
			systemLoadRatio15min.WithLabelValues(instanceID).Set(ratio15min)

			log.Printf("负载比计算: Load1=%%.2f, Load5=%%.2f, Load15=%%.2f, CPU核数=%%d, Ratio1min=%%.4f, Ratio5min=%%.4f, Ratio15min=%%.4f",
				loadStats.Load1, loadStats.Load5, loadStats.Load15, cpuCount, ratio1min, ratio5min, ratio15min)
		}
	} else {
		log.Printf("获取负载失败: %%v", err)
	}

	// 采集磁盘IO指标
	const intervalSeconds = 10.0
	if diskIOStats, err := disk.IOCounters(); err == nil {
		var totalReadBytes, totalWriteBytes uint64
		var totalReadCount, totalWriteCount uint64
		for _, diskIO := range diskIOStats {
			totalReadBytes += diskIO.ReadBytes
			totalWriteBytes += diskIO.WriteBytes
			totalReadCount += diskIO.ReadCount
			totalWriteCount += diskIO.WriteCount
		}
		diskIO := disk.IOCountersStat{
			ReadBytes:  totalReadBytes,
			WriteBytes: totalWriteBytes,
			ReadCount:  totalReadCount,
			WriteCount: totalWriteCount,
		}

		if lastDiskIO.ReadBytes != 0 {
			// 计算吞吐量(KB/s)
			readRate := float64(diskIO.ReadBytes-lastDiskIO.ReadBytes) / 1024 / intervalSeconds
			writeRate := float64(diskIO.WriteBytes-lastDiskIO.WriteBytes) / 1024 / intervalSeconds
			diskRead.WithLabelValues(instanceID, "vda").Set(readRate)
			diskWrite.WithLabelValues(instanceID, "vda").Set(writeRate)

			// 计算IOPS(次/秒)
			readIOPS := float64(diskIO.ReadCount-lastDiskIO.ReadCount) / intervalSeconds
			writeIOPS := float64(diskIO.WriteCount-lastDiskIO.WriteCount) / intervalSeconds
			diskReadIOPS.WithLabelValues(instanceID, "vda").Set(readIOPS)
			diskWriteIOPS.WithLabelValues(instanceID, "vda").Set(writeIOPS)
		}
		lastDiskIO = diskIO
	}

	// 采集进程总数
	if processes, err := process.Processes(); err == nil {
		totalProcesses.WithLabelValues(instanceID).Set(float64(len(processes)))
	}

	// 采集网络指标
	if netStats, err := psnet.IOCounters(true); err == nil {
		if len(netStats) > 0 {
			var mainInterface *psnet.IOCountersStat
			for i := range netStats {
				name := netStats[i].Name
				if strings.HasPrefix(name, "docker") ||
					strings.HasPrefix(name, "br-") ||
					strings.HasPrefix(name, "veth") ||
					strings.HasPrefix(name, "lo") {
					continue
				}
				if netStats[i].BytesRecv > 0 || netStats[i].BytesSent > 0 {
					mainInterface = &netStats[i]
					break
				}
			}
			if mainInterface == nil {
				for i := range netStats {
					name := netStats[i].Name
					if !strings.HasPrefix(name, "docker") &&
						!strings.HasPrefix(name, "br-") &&
						!strings.HasPrefix(name, "veth") &&
						!strings.HasPrefix(name, "lo") {
						mainInterface = &netStats[i]
						break
					}
				}
			}
			if mainInterface == nil && len(netStats) > 0 {
				mainInterface = &netStats[0]
			}

			if lastNetIO.Name != "" {
				const intervalSeconds = 10.0
				recvRate := float64(mainInterface.BytesRecv-lastNetIO.BytesRecv) / 1024 / intervalSeconds
				sentRate := float64(mainInterface.BytesSent-lastNetIO.BytesSent) / 1024 / intervalSeconds
				netRecv.WithLabelValues(instanceID).Set(recvRate)
				netSent.WithLabelValues(instanceID).Set(sentRate)
			}
			lastNetIO = *mainInterface
		}
	}

	// 采集CPU用户态/内核态（使用增量计算）
	if times, err := cpu.Times(false); err == nil && len(times) > 0 {
		t := times[0]
		// 如果有上次的数据，计算增量
		if lastCPUTime.User > 0 {
			deltaUser := t.User - lastCPUTime.User
			deltaSystem := t.System - lastCPUTime.System
			deltaIdle := t.Idle - lastCPUTime.Idle
			deltaTotal := deltaUser + deltaSystem + deltaIdle +
				(t.Nice - lastCPUTime.Nice) +
				(t.Iowait - lastCPUTime.Iowait) +
				(t.Irq - lastCPUTime.Irq) +
				(t.Softirq - lastCPUTime.Softirq) +
				(t.Steal - lastCPUTime.Steal)

			if deltaTotal > 0 {
				userPercent := (deltaUser / deltaTotal) * 100
				systemPercent := (deltaSystem / deltaTotal) * 100
				cpuUserUsage.WithLabelValues(instanceID).Set(userPercent)
				cpuSystemUsage.WithLabelValues(instanceID).Set(systemPercent)
			}
		}
		// 保存当前值用于下次计算
		lastCPUTime = t
	}

	// 采集交换内存(GB)
	if swapInfo, err := mem.SwapMemory(); err == nil {
		swapUsed.WithLabelValues(instanceID).Set(float64(swapInfo.Used) / 1024 / 1024 / 1024)
	}

	// 采集系统运行时间（天）
	if content, err := exec.Command("cat", "/proc/uptime").Output(); err == nil {
		if fields := strings.Fields(string(content)); len(fields) > 0 {
			if uptime, err := strconv.ParseFloat(fields[0], 64); err == nil {
				// 只保留天数
				systemUptimeDays.WithLabelValues(instanceID).Set(uptime / 86400)
			}
		}
	}

	// V2新增 - 采集TCP/UDP连接数（使用ss命令统计，更准确）
	// 统计TCP连接数（排除表头）
	if tcpOut, err := exec.Command("bash", "-c", "ss -tan 2>/dev/null | tail -n +2 | wc -l").Output(); err == nil {
		if tcpCount, err := strconv.Atoi(strings.TrimSpace(string(tcpOut))); err == nil {
			tcpConnections.WithLabelValues(instanceID).Set(float64(tcpCount))
		}
	}

	// 统计UDP连接数（排除表头）
	if udpOut, err := exec.Command("bash", "-c", "ss -uan 2>/dev/null | tail -n +2 | wc -l").Output(); err == nil {
		if udpCount, err := strconv.Atoi(strings.TrimSpace(string(udpOut))); err == nil {
			udpConnections.WithLabelValues(instanceID).Set(float64(udpCount))
		}
	}

	// 采集文件打开数量
	if fileNrOut, err := exec.Command("bash", "-c", "cat /proc/sys/fs/file-nr | awk '{print $1}'").Output(); err == nil {
		if openFiles, err := strconv.Atoi(strings.TrimSpace(string(fileNrOut))); err == nil {
			openFileDescriptors.WithLabelValues(instanceID).Set(float64(openFiles))
		}
	}
}

func monitorProcesses(config *Config) {
	// 使用本地IP作为host标识
	hostIdentifier := getLocalIP()
	if hostIdentifier == "" {
		// 如果获取IP失败，使用hostname作为备选
		hostname, err := os.Hostname()
		if err != nil {
			hostIdentifier = "unknown"
		} else {
			hostIdentifier = hostname
		}
	}
	processes, _ := process.Processes()

	type procStats struct {
		name string
		cpu  float64
		mem  float32
	}
	statsByName := make(map[string]*procStats)

	for _, p := range processes {
		name, _ := p.Name()
		if name == "" {
			name = "unknown"
		}
		cpuPercent, _ := p.CPUPercent()
		memPercent, _ := p.MemoryPercent()

		stat, ok := statsByName[name]
		if !ok {
			stat = &procStats{name: name}
			statsByName[name] = stat
		}
		stat.cpu += cpuPercent
		stat.mem += memPercent
	}

	var allProcs []procStats
	for _, stat := range statsByName {
		allProcs = append(allProcs, *stat)
	}

	// 按CPU使用率排序
	sort.Slice(allProcs, func(i, j int) bool {
		return allProcs[i].cpu > allProcs[j].cpu
	})

	processCPU.Reset()

	// 记录CPU使用率最高的5个进程
	for i := 0; i < 5 && i < len(allProcs); i++ {
		p := allProcs[i]
		processCPU.WithLabelValues(hostIdentifier, p.name).Set(p.cpu)
	}

	// 按内存使用率排序
	sort.Slice(allProcs, func(i, j int) bool {
		return allProcs[i].mem > allProcs[j].mem
	})

	processMem.Reset()

	// 记录内存使用率最高的5个进程
	for i := 0; i < 5 && i < len(allProcs); i++ {
		p := allProcs[i]
		processMem.WithLabelValues(hostIdentifier, p.name).Set(float64(p.mem))
	}
}

func monitorEndpoints(config *Config) {
	// 使用本地IP作为host标识
	hostIdentifier := getLocalIP()
	if hostIdentifier == "" {
		// 如果获取IP失败，使用hostname作为备选
		hostname, err := os.Hostname()
		if err != nil {
			hostIdentifier = "unknown"
		} else {
			hostIdentifier = hostname
		}
	}
	for _, endpoint := range config.Endpoints {
		start := time.Now()
		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Get(endpoint.URL)
		duration := time.Since(start).Milliseconds()

		status := 0
		if err == nil {
			if resp.StatusCode >= 200 && resp.StatusCode < 400 {
				status = 1
			}
			resp.Body.Close()
		}

		endpointResponseTime.WithLabelValues(hostIdentifier, endpoint.URL, endpoint.Method).Set(float64(duration))
		endpointStatus.WithLabelValues(hostIdentifier, endpoint.URL, endpoint.Method).Set(float64(status))
	}
}

func monitorPing(target string) {
	// 使用本地IP作为host标识
	hostIdentifier := getLocalIP()
	if hostIdentifier == "" {
		// 如果获取IP失败，使用hostname作为备选
		hostname, err := os.Hostname()
		if err != nil {
			hostIdentifier = "unknown"
		} else {
			hostIdentifier = hostname
		}
	}
	start := time.Now()

	cmd := exec.Command("ping", "-c", "1", target)
	err := cmd.Run()
	duration := time.Since(start).Milliseconds()

	status := 0
	if err == nil {
		status = 1
	}

	pingResponseTime.WithLabelValues(hostIdentifier, target).Set(float64(duration))
	pingStatus.WithLabelValues(hostIdentifier, target).Set(float64(status))
}

func monitorTCPPorts() {
	// 使用本地IP作为instance标识
	hostIdentifier := getLocalIP()
	if hostIdentifier == "" {
		// 如果获取IP失败，使用hostname作为备选
		hostname, err := os.Hostname()
		if err != nil {
			hostIdentifier = "unknown"
		} else {
			hostIdentifier = hostname
		}
	}

	// 重置端口监控指标，清除已停止监听的端口
	tcpPortStatus.Reset()
	// 重置进程资源监控指标，清除已停止的进程
	processCPUUsage.Reset()
	processMemoryUsage.Reset()

	// 优先尝试 ss 命令
	cmd := exec.Command("ss", "-ntlp")
	output, err := cmd.Output()
	usingSS := true
	if err != nil {
		// ss 失败，尝试 netstat
		cmd = exec.Command("netstat", "-ntlp")
		output, err = cmd.Output()
		usingSS = false
		if err != nil {
			// 两个命令都失败，直接返回
			return
		}
	}

	lines := strings.Split(string(output), "\n")
	startLine := 1
	if !usingSS {
		startLine = 2 // netstat 需要跳过两行标题
	}

	for i := startLine; i < len(lines); i++ {
		line := lines[i]
		if strings.TrimSpace(line) == "" {
			continue
		}

		fields := strings.Fields(line)

		var localAddr, processInfo string
		var isListening bool

		if usingSS && len(fields) >= 6 && fields[0] == "LISTEN" {
			// ss 格式
			isListening = true
			localAddr = fields[3]
			processInfo = fields[5]
		} else if !usingSS && len(fields) >= 7 && fields[5] == "LISTEN" {
			// netstat 格式
			isListening = true
			localAddr = fields[3]
			processInfo = fields[6]
		}

		if !isListening {
			continue
		}

		if strings.Contains(localAddr, ":") {
			parts := strings.Split(localAddr, ":")
			if len(parts) >= 2 {
				port := parts[len(parts)-1]
				pid := ""
				service := ""

				// 解析进程信息
				if processInfo != "" && processInfo != "-" {
					if strings.Contains(processInfo, "pid=") {
						// ss 格式: users:(("nginx",pid=2642106,fd=6))
						// 提取服务名: 查找第一个引号对
						startQuote := strings.Index(processInfo, "\"")
						if startQuote != -1 {
							endQuote := strings.Index(processInfo[startQuote+1:], "\"")
							if endQuote != -1 {
								service = processInfo[startQuote+1 : startQuote+1+endQuote]
							}
						}
						// 提取 PID
						pidIdx := strings.Index(processInfo, "pid=")
						if pidIdx != -1 {
							pidStr := processInfo[pidIdx+4:]
							commaIdx := strings.Index(pidStr, ",")
							if commaIdx != -1 {
								pid = pidStr[:commaIdx]
							}
						}
					} else if strings.Contains(processInfo, "/") {
						// netstat 格式: 123/nginx
						parts := strings.Split(processInfo, "/")
						if len(parts) >= 2 {
							pid = parts[0]
							service = parts[1]
						}
					}
				}

				if service == "" {
					service = "port-" + port
				}

				tcpPortStatus.WithLabelValues(hostIdentifier, port, service).Set(1)

				// 收集该进程的CPU和内存使用率
				if pid != "" {
					if pidInt, err := strconv.Atoi(pid); err == nil {
						if p, err := process.NewProcess(int32(pidInt)); err == nil {
							if cpuPercent, err := p.CPUPercent(); err == nil {
								processCPUUsage.WithLabelValues(hostIdentifier, service).Set(cpuPercent)
							}
							if memPercent, err := p.MemoryPercent(); err == nil {
								processMemoryUsage.WithLabelValues(hostIdentifier, service).Set(float64(memPercent))
							}
						}
					}
				}
			}
		}
	}
}

func monitorServices() {
	// 获取hostname作为instance标识
	hostname := getLocalIP()
	if hostname == "" {
		// 如果获取IP失败，使用hostname作为备选
		if h, err := os.Hostname(); err == nil {
			hostname = h
		} else {
			hostname = "unknown"
		}
	}

	// 排除的系统服务模式
	excludePattern := "systemd|systemd-resolve|systemd-network|sshd|chronyd|pmproxy|pmcd|pmlogger|pmie|docker-proxy"

	// 存储找到的服务
	servicesMap := make(map[string]bool)

	// 1. 从监听端口获取服务
	cmd := exec.Command("bash", "-c", "ss -ntpl 2>/dev/null || netstat -ntlp 2>/dev/null")
	output, err := cmd.Output()
	if err == nil {
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if strings.TrimSpace(line) == "" || strings.Contains(line, "State") {
				continue
			}

			fields := strings.Fields(line)
			if len(fields) < 6 {
				continue
			}

			// 提取进程信息(ss和netstat格式略有不同)
			var processInfo string
			for _, field := range fields {
				if strings.Contains(field, "users:") || strings.Contains(field, "/") {
					processInfo = field
					break
				}
			}

			if processInfo == "" {
				continue
			}

			// 解析服务名
			var serviceName string

			// ss格式: users:(("nginx",pid=123,fd=6))
			if strings.Contains(processInfo, "users:") {
				start := strings.Index(processInfo, "\"")
				if start != -1 {
					end := strings.Index(processInfo[start+1:], "\"")
					if end != -1 {
						serviceName = processInfo[start+1 : start+1+end]
					}
				}
			} else if strings.Contains(processInfo, "/") {
				// netstat格式: 123/nginx
				parts := strings.Split(processInfo, "/")
				if len(parts) >= 2 {
					serviceName = parts[1]
				}
			}

			// 跳过系统服务
			if serviceName != "" && !strings.Contains(excludePattern, serviceName) {
				// 特殊处理java/python/node
				if serviceName == "java" {
					servicesMap["java"] = true
				} else if strings.HasPrefix(serviceName, "python") {
					servicesMap["python"] = true
				} else if serviceName == "node" || serviceName == "nodejs" {
					servicesMap["node"] = true
				} else {
					servicesMap[serviceName] = true
				}
			}
		}
	}

	// 2. 检查docker/containerd进程
	dockerCmd := exec.Command("bash", "-c", "ps -ef | grep -E '[d]ockerd|[c]ontainerd' | grep -v 'containerd-shim' | awk '{print $8}'")
	dockerOutput, err := dockerCmd.Output()
	if err == nil {
		lines := strings.Split(string(dockerOutput), "\n")
		for _, line := range lines {
			if strings.TrimSpace(line) != "" {
				serviceName := strings.TrimPrefix(line, "/usr/bin/")
				serviceName = strings.TrimPrefix(serviceName, "/usr/local/bin/")
				if serviceName != "" {
					servicesMap[serviceName] = true
				}
			}
		}
	}

	// 清空旧的服务指标
	serviceCPUUsage.Reset()
	serviceMemoryUsage.Reset()

	// 3. 获取每个服务的资源使用情况
	for serviceName := range servicesMap {
		var cpuUsage, memUsage float64

		switch serviceName {
		case "java":
			// 查找所有java进程及其jar/war文件
			javaCmd := exec.Command("bash", "-c", "ps -ef | grep '[j]ava' | awk '{for(i=8;i<=NF;i++) printf \"%%s \", $i; print \"\"}'")
			javaOut, err := javaCmd.Output()
			if err == nil {
				// 查找jar/war文件
				jarCmd := exec.Command("bash", "-c", fmt.Sprintf("echo '%%s' | grep -oP '(\\S+\\.jar|\\S+\\.war)' | sort -u", string(javaOut)))
				jarOut, err := jarCmd.Output()
				if err == nil {
					jars := strings.Split(strings.TrimSpace(string(jarOut)), "\n")
					for _, jar := range jars {
						if jar != "" {
							jarBase := jar[strings.LastIndex(jar, "/")+1:]
							cpu, mem := getServiceResourceUsage("java: " + jarBase)
							serviceCPUUsage.WithLabelValues(hostname, "java: "+jarBase).Set(cpu)
							serviceMemoryUsage.WithLabelValues(hostname, "java: "+jarBase).Set(mem)
						}
					}
				}
			}

		case "python":
			// 查找所有python进程及其.py文件
			pythonCmd := exec.Command("bash", "-c", "ps -ef | grep '[p]ython' | awk '{for(i=8;i<=NF;i++) printf \"%%s \", $i; print \"\"}'")
			pythonOut, err := pythonCmd.Output()
			if err == nil {
				pyCmd := exec.Command("bash", "-c", fmt.Sprintf("echo '%%s' | grep -oP '\\s(\\S+\\.py)\\b' | sed 's/^\\s*//' | sort -u", string(pythonOut)))
				pyOut, err := pyCmd.Output()
				if err == nil {
					scripts := strings.Split(strings.TrimSpace(string(pyOut)), "\n")
					for _, script := range scripts {
						if script != "" {
							scriptBase := script[strings.LastIndex(script, "/")+1:]
							cpu, mem := getServiceResourceUsage("python: " + scriptBase)
							serviceCPUUsage.WithLabelValues(hostname, "python: "+scriptBase).Set(cpu)
							serviceMemoryUsage.WithLabelValues(hostname, "python: "+scriptBase).Set(mem)
						}
					}
				}
			}

		case "node":
			// 查找所有node进程及其.js文件
			nodeCmd := exec.Command("bash", "-c", "ps -ef | grep '[n]ode' | awk '{for(i=8;i<=NF;i++) printf \"%%s \", $i; print \"\"}'")
			nodeOut, err := nodeCmd.Output()
			if err == nil {
				jsCmd := exec.Command("bash", "-c", fmt.Sprintf("echo '%%s' | grep -oP '\\s(\\S+\\.js)\\b' | sed 's/^\\s*//' | sort -u", string(nodeOut)))
				jsOut, err := jsCmd.Output()
				if err == nil {
					scripts := strings.Split(strings.TrimSpace(string(jsOut)), "\n")
					for _, script := range scripts {
						if script != "" {
							scriptBase := script[strings.LastIndex(script, "/")+1:]
							cpu, mem := getServiceResourceUsage("node: " + scriptBase)
							serviceCPUUsage.WithLabelValues(hostname, "node: "+scriptBase).Set(cpu)
							serviceMemoryUsage.WithLabelValues(hostname, "node: "+scriptBase).Set(mem)
						}
					}
				}
			}

		default:
			// 其他服务直接查询
			cpuUsage, memUsage = getServiceResourceUsage(serviceName)
			serviceCPUUsage.WithLabelValues(hostname, serviceName).Set(cpuUsage)
			serviceMemoryUsage.WithLabelValues(hostname, serviceName).Set(memUsage)
		}
	}
}

// getServiceResourceUsage 获取服务的CPU和内存使用率
func getServiceResourceUsage(serviceName string) (cpu float64, mem float64) {
	var cmd *exec.Cmd

	if strings.HasPrefix(serviceName, "java: ") {
		jarName := strings.TrimPrefix(serviceName, "java: ")
		cmdStr := fmt.Sprintf("ps aux | grep 'java' | grep '%%s' | grep -v grep | awk '{cpu+=$3; mem+=$4} END {printf \"%%%%f %%%%f\", cpu, mem}'", jarName)
		debugf("[DEBUG] Java服务[%%s] 执行命令: %%s", serviceName, cmdStr)
		cmd = exec.Command("bash", "-c", cmdStr)
	} else if strings.HasPrefix(serviceName, "python: ") {
		scriptName := strings.TrimPrefix(serviceName, "python: ")
		cmdStr := fmt.Sprintf("ps aux | grep 'python' | grep '%%s' | grep -v grep | awk '{cpu+=$3; mem+=$4} END {printf \"%%%%f %%%%f\", cpu, mem}'", scriptName)
		debugf("[DEBUG] Python服务[%%s] 执行命令: %%s", serviceName, cmdStr)
		cmd = exec.Command("bash", "-c", cmdStr)
	} else if strings.HasPrefix(serviceName, "node: ") {
		scriptName := strings.TrimPrefix(serviceName, "node: ")
		cmdStr := fmt.Sprintf("ps aux | grep 'node' | grep '%%s' | grep -v grep | awk '{cpu+=$3; mem+=$4} END {printf \"%%%%f %%%%f\", cpu, mem}'", scriptName)
		debugf("[DEBUG] Node服务[%%s] 执行命令: %%s", serviceName, cmdStr)
		cmd = exec.Command("bash", "-c", cmdStr)
	} else {
		// 参考check_services.sh的实现：匹配进程名在第11列（可能是完整路径或进程名）
		// $11 ~ svc 匹配包含服务名的进程
		// $11 ~ "/"svc"$" 匹配路径末尾是服务名的进程（如 /usr/bin/containerd）
		cmdStr := fmt.Sprintf("ps aux | awk -v svc='%%s' '$11 ~ svc || $11 ~ \"/\"svc\"$\" {cpu+=$3; mem+=$4} END {if(cpu>0 || mem>0) printf \"%%%%f %%%%f\", cpu, mem; else print \"0 0\"}'", serviceName)
		debugf("[DEBUG] 普通服务[%%s] 执行命令: %%s", serviceName, cmdStr)
		cmd = exec.Command("bash", "-c", cmdStr)
	}

	output, err := cmd.Output()
	if err != nil {
		debugf("[DEBUG] 服务[%%s] 命令执行失败: %%v", serviceName, err)
		return 0, 0
	}

	result := strings.TrimSpace(string(output))
	debugf("[DEBUG] 服务[%%s] 命令输出: [%%s]", serviceName, result)

	if result == "" {
		debugf("[DEBUG] 服务[%%s] 输出为空", serviceName)
		return 0, 0
	}

	n, err := fmt.Sscanf(result, "%%f %%f", &cpu, &mem)
	if err != nil || n != 2 {
		debugf("[DEBUG] 服务[%%s] 解析失败: scanned=%%d, err=%%v, result=[%%s]", serviceName, n, err, result)
		return 0, 0
	}

	debugf("[DEBUG] 服务[%%s] 最终结果: CPU=%%.2f%%%%, MEM=%%.2f%%%%", serviceName, cpu, mem)
	return cpu, mem
}
func registerMetrics(registry *prometheus.Registry) {
	registry.MustRegister(cpuUsage)
	registry.MustRegister(memUsage)
	registry.MustRegister(memTotal)
	registry.MustRegister(memUsed)
	registry.MustRegister(memFree)
	registry.MustRegister(diskUsage)
	registry.MustRegister(diskInodeUsage)
	registry.MustRegister(diskInodeTotal)
	registry.MustRegister(diskInodeUsed)
	registry.MustRegister(diskInodeFree)
	registry.MustRegister(systemLoad1min)
	registry.MustRegister(systemLoad5min)
	registry.MustRegister(systemLoad15min)
	registry.MustRegister(systemLoadRatio1min)
	registry.MustRegister(systemLoadRatio5min)
	registry.MustRegister(systemLoadRatio15min)
	registry.MustRegister(cpuCoreCount)
	registry.MustRegister(diskRead)
	registry.MustRegister(diskWrite)
	registry.MustRegister(totalProcesses)
	registry.MustRegister(netRecv)
	registry.MustRegister(netSent)
	registry.MustRegister(processCPU)
	registry.MustRegister(processMem)
	registry.MustRegister(tcpPortStatus)
	registry.MustRegister(processCPUUsage)
	registry.MustRegister(processMemoryUsage)
	registry.MustRegister(endpointResponseTime)
	registry.MustRegister(endpointStatus)
	registry.MustRegister(pingResponseTime)
	registry.MustRegister(pingStatus)
	registry.MustRegister(tcpConnections)
	registry.MustRegister(udpConnections)
	registry.MustRegister(diskReadIOPS)
	registry.MustRegister(diskWriteIOPS)
	registry.MustRegister(cpuUserUsage)
	registry.MustRegister(cpuSystemUsage)
	registry.MustRegister(swapUsed)
	registry.MustRegister(diskUsedBytes)
	registry.MustRegister(diskTotalBytes)
	registry.MustRegister(serviceCPUUsage)
	registry.MustRegister(serviceMemoryUsage)
	registry.MustRegister(agentInfo)
	registry.MustRegister(systemUptimeDays)
	registry.MustRegister(openFileDescriptors)
}
func getEnvVar(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func durationOrDefault(value string, fallback time.Duration) time.Duration {
	if d, err := time.ParseDuration(value); err == nil && d > 0 {
		return d
	}
	return fallback
}

func agentDebugEnabled() bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv("AGENT_DEBUG")))
	return value == "1" || value == "true" || value == "yes"
}

func debugf(format string, args ...interface{}) {
	if agentDebugEnabled() {
		log.Printf(format, args...)
	}
}

func loadAgentConfig(path string, config *Config) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read config file: %%w", err)
	}
	if err := json.Unmarshal(data, config); err != nil {
		return fmt.Errorf("parse config file: %%w", err)
	}
	if config.Group.ID > 0 {
		hostGroupID = config.Group.ID
	}
	if strings.TrimSpace(config.Group.Name) != "" {
		hostGroupName = config.Group.Name
	}
	if strings.TrimSpace(config.Version) != "" {
		agentVersion = strings.TrimSpace(config.Version)
	}
	if strings.TrimSpace(config.VersionTag) != "" {
		agentVersionTag = strings.TrimSpace(config.VersionTag)
	}
	return nil
}

func StartAgentWithConfig(config *Config) error {
	// 创建自定义注册表，不包含Go运行时指标
	registry := prometheus.NewRegistry()
	registerMetrics(registry)
	http.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))

	go func() {
		log.Println("Agent starting on :12580")
		if err := http.ListenAndServe(":12580", nil); err != nil {
			log.Printf("HTTP server failed: %%v", err)
		}
	}()

	collectOnce := func() {
		collectSystemMetrics()
		monitorProcesses(config)
		monitorEndpoints(config)
		monitorPing("8.8.8.8")
		monitorTCPPorts()
		monitorServices()
	}
	collectOnce()
	if config.Heartbeat.ServerURL != "" {
		sendHeartbeat(config.Heartbeat.ServerURL, config.Heartbeat.Token)
	}

	collectInterval := durationOrDefault(config.CollectInterval, 30*time.Second)
	pushInterval := durationOrDefault(config.Pushgateway.PushInterval, 60*time.Second)
	heartbeatInterval := durationOrDefault(config.Heartbeat.Interval, 30*time.Second)
	startLogtailRuntime(config)

	// 启动抖动，避免大量Agent在同一秒集中采集和推送。
	startupJitter := time.Duration(time.Now().UnixNano()%%int64(30*time.Second))
	if startupJitter > 0 {
		debugf("Agent startup jitter: %%s", startupJitter)
		time.Sleep(startupJitter)
	}

	// 定期采集系统指标
	ticker := time.NewTicker(collectInterval)
	defer ticker.Stop()

	// 定期推送指标
	pushTicker := time.NewTicker(pushInterval)
	defer pushTicker.Stop()

	// 心跳
	heartbeatTicker := time.NewTicker(heartbeatInterval)
	defer heartbeatTicker.Stop()

	for {
		select {
		case <-ticker.C:
			collectOnce()
		case <-pushTicker.C:
			if config.Pushgateway.URL != "" {
			debugf("[分组标签] 推送指标 - group_id: %%d, group_name: %%s", hostGroupID, hostGroupName)
				if strings.Contains(config.Pushgateway.URL, "/api/v1/import/prometheus") {
					if err := pushMetricsToImportEndpoint(config.Pushgateway.URL, registry, config); err != nil {
						log.Printf("Could not push metrics to import endpoint: %%v", err)
					} else {
					debugf("Successfully pushed metrics to import endpoint: %%s (group: %%s)", config.Pushgateway.URL, hostGroupName)
					}
				} else {
					pusher := push.New(config.Pushgateway.URL, config.Pushgateway.Job)
					pusher = pusher.Format(expfmt.NewFormat(expfmt.TypeTextPlain))
					// 使用hostname作为instance分组标签
					pusher = pusher.Grouping("hostname", config.Pushgateway.Instance)
					// 添加分组标签
					pusher = pusher.Grouping("group_id", fmt.Sprintf("%%d", hostGroupID))
					pusher = pusher.Grouping("group_name", hostGroupName)

					// 使用registry推送所有指标（不包含Go运行时指标）
					if err := pusher.Gatherer(registry).Push(); err != nil {
						log.Printf("Could not push to gateway: %%v", err)
					} else {
					debugf("Successfully pushed metrics to gateway: %%s (group: %%s)", config.Pushgateway.URL, hostGroupName)
					}
				}
			}
		case <-heartbeatTicker.C:
			if config.Heartbeat.ServerURL != "" {
				sendHeartbeat(config.Heartbeat.ServerURL, config.Heartbeat.Token)
			}
		}
	}
}

func main() {
	log.Println("Starting dodevops-agent...")
	log.Printf("[分组信息] HostGroupID: %%d, HostGroupName: %%s", hostGroupID, hostGroupName)

	// 获取本地IP作为Job名称
	localIP := getLocalIP()
	if localIP == "" {
		localIP = "unknown-ip"
		log.Printf("Warning: could not get local IP")
	}
	log.Printf("Agent local IP: %%s", localIP)

	// 获取hostname作为Instance标识
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown-host"
		log.Printf("Warning: could not get hostname: %%v", err)
	}
	log.Printf("Agent hostname: %%s", hostname)

	config := &Config{
		Processes: []struct {
			Name string `+"`json:\"name\"`"+`
		}{
			{Name: "nginx"},
			{Name: "mysql"},
			{Name: "redis"},
		},
		Endpoints: []struct {
			URL    string `+"`json:\"url\"`"+`
			Method string `+"`json:\"method\"`"+`
		}{
			{URL: "http://localhost:12580/metrics", Method: "GET"},
		},
		Pushgateway: struct {
			URL          string `+"`json:\"url\"`"+`
			PushInterval string `+"`json:\"push_interval\"`"+`
			Job          string `+"`json:\"job\"`"+`
			Instance     string `+"`json:\"instance\"`"+`
		}{
			URL:          getEnvVar("PUSHGATEWAY_URL", "%[1]s"),
			PushInterval: getEnvVar("PUSH_INTERVAL", "60s"),
			Job:          localIP,      // Job使用本地IP
			Instance:     hostname,     // Instance使用hostname
		},
		Heartbeat: struct {
			ServerURL string `+"`json:\"server_url\"`"+`
			Interval  string `+"`json:\"interval\"`"+`
			Token     string `+"`json:\"token\"`"+`
		}{
			ServerURL: getEnvVar("HEARTBEAT_SERVER_URL", "%[2]s"),
			Interval:  getEnvVar("HEARTBEAT_INTERVAL", "30s"),
			Token:     getEnvVar("HEARTBEAT_TOKEN", "%[3]s"),
		},
		Logtail: struct {
			BaseURL      string `+"`json:\"base_url\"`"+`
			PollInterval string `+"`json:\"poll_interval\"`"+`
			Token        string `+"`json:\"token\"`"+`
		}{
			BaseURL:      getEnvVar("LOGTAIL_BASE_URL", ""),
			PollInterval: getEnvVar("LOGTAIL_POLL_INTERVAL", "30s"),
			Token:        getEnvVar("LOGTAIL_TOKEN", getEnvVar("HEARTBEAT_TOKEN", "%[3]s")),
		},
		CollectInterval: getEnvVar("COLLECT_INTERVAL", "30s"),
	}

	configPath := flag.String("config", getEnvVar("AGENT_CONFIG", ""), "path to agent JSON config")
	flag.Parse()
	if err := loadAgentConfig(*configPath, config); err != nil {
		log.Fatalf("Failed to load agent config: %%v", err)
	}
	if config.Pushgateway.Job == "" {
		config.Pushgateway.Job = localIP
	}
	if config.Pushgateway.Instance == "" {
		config.Pushgateway.Instance = hostname
	}

	if err := StartAgentWithConfig(config); err != nil {
		log.Fatalf("Failed to start agent: %%v", err)
	}
	select {}
}
`, pushgatewayURL, heartbeatURL, heartbeatToken, groupID, groupName, agentVersion, agentVersionTag)

	log.Printf("CreateAgentMainFile: 生成的代码长度: %d", len(mainCode))
	log.Printf("CreateAgentMainFile: 代码生成完成，heartbeat URL=%s, groupID=%d, groupName=%s", heartbeatURL, groupID, groupName)

	return mainCode
}

// ValidateAgentCode 验证生成的agent代码是否有效
func ValidateAgentCode(code string) error {
	// 检查基本结构
	if !strings.Contains(code, "package main") {
		return fmt.Errorf("缺少package main声明")
	}

	if !strings.Contains(code, "func main()") {
		return fmt.Errorf("缺少main函数")
	}

	// 简化验证：只检查明显的语法错误，不做复杂的解析
	lines := strings.Split(code, "\n")

	// 检查是否有明显的语法问题
	openBraces := 0
	openParens := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") {
			continue
		}

		openBraces += strings.Count(trimmed, "{")
		openBraces -= strings.Count(trimmed, "}")
		openParens += strings.Count(trimmed, "(")
		openParens -= strings.Count(trimmed, ")")
	}

	// 检查括号匹配
	if openBraces != 0 {
		return fmt.Errorf("大括号不匹配")
	}
	if openParens != 0 {
		return fmt.Errorf("圆括号不匹配")
	}

	return nil
}
