package config

import (
	"os"
	"strings"

	"gopkg.in/yaml.v2"
)

type config struct {
	Server          server                `yaml:"server"`
	Db              db                    `yaml:"db"`
	Redis           redis                 `yaml:"redis"`
	ImageSettings   imageSettings         `yaml:"imageSettings"`
	Log             logConfig             `yaml:"log"`
	Logtail         logtailConfig         `yaml:"logtail"`
	Monitor         monitor               `yaml:"monitor"`
	WeCom           wecom                 `yaml:"wecom"`
	MCP             mcp                   `yaml:"mcp"`
	Guacd           guacd                 `yaml:"guacd"`
	WindowsExporter WindowsExporterConfig `yaml:"windows_exporter"`
}

type wecom struct {
	PluginToken string `yaml:"plugin_token"`
}

type mcp struct {
	Token  string `yaml:"token"`
	UserID uint   `yaml:"user_id"`
}

type monitor struct {
	Prometheus      prometheus  `yaml:"prometheus"`
	Pushgateway     pushgateway `yaml:"pushgateway"`
	VictoriaMetrics prometheus  `yaml:"victoriametrics"`
	MetricsPush     pushgateway `yaml:"metrics_push"`
	Agent           agent       `yaml:"agent"`
	Webhook         webhook     `yaml:"webhook"`

	AlertScraperTime int `yaml:"alert_scraper_time"`
}

type webhook struct {
	Token string `yaml:"token"`
}

type pushgateway struct {
	URL string `yaml:"url"`
}

type agent struct {
	HeartbeatServerURL string `yaml:"heartbeat_server_url"`
	HeartbeatToken     string `yaml:"heartbeat_token"`
	HeartbeatTimeout   int    `yaml:"heartbeat_timeout_seconds"`
	Version            string `yaml:"version"`
	VersionTag         string `yaml:"version_tag"`
}

type prometheus struct {
	URL string `yaml:"url"`
}

type server struct {
	Address       string `yaml:"address"`
	Model         string `yaml:"model"`
	EnableSwagger bool   `yaml:"enableSwagger"`
	PublicURL     string `yaml:"publicUrl"`
}

type db struct {
	Dialects    string `yaml:"dialects"`
	Host        string `yaml:"host"`
	Port        int    `yaml:"port"`
	Db          string `yaml:"db"`
	Username    string `yaml:"username"`
	Password    string `yaml:"password"`
	Charset     string `yaml:"charset"`
	MaxIdle     int    `yaml:"maxIdle"`
	MaxOpen     int    `yaml:"maxOpen"`
	AutoMigrate bool   `yaml:"autoMigrate"`
}

type redis struct {
	Address  string `yaml:"address"`
	Password string `yaml:"password"`
}

type imageSettings struct {
	UploadDir string `yaml:"uploadDir"`
	ImageHost string `yaml:"imageHost"`
}

type logConfig struct {
	Path  string `yaml:"path"`
	Name  string `yaml:"name"`
	Model string `yaml:"model"`
}

type logtailConfig struct {
	DeliveryMode string             `yaml:"delivery_mode"`
	VictoriaLogs victoriaLogsConfig `yaml:"victorialogs"`
	Kafka        kafkaConfig        `yaml:"kafka"`
	Vector       vectorConfig       `yaml:"vector"`
	Aggregator   aggregatorConfig   `yaml:"aggregator"`
}

type victoriaLogsConfig struct {
	URL            string `yaml:"url"`
	TimeoutSeconds int    `yaml:"timeout_seconds"`
}

type vectorConfig struct {
	PackagePath            string `yaml:"packagePath"`
	Image                  string `yaml:"image"`
	DiskBufferMaxSizeBytes int64  `yaml:"disk_buffer_max_size_bytes"`
}

type aggregatorConfig struct {
	Image string `yaml:"image"`
}

type kafkaConfig struct {
	Enabled     bool                   `yaml:"enabled"`
	Brokers     []string               `yaml:"brokers"`
	Topic       string                 `yaml:"topic"`
	HostAliases []kafkaHostAliasConfig `yaml:"host_aliases"`
}

type kafkaHostAliasConfig struct {
	IP        string   `yaml:"ip"`
	Hostnames []string `yaml:"hostnames"`
}

type guacd struct {
	Host    string `yaml:"host"`
	Port    int    `yaml:"port"`
	Timeout int    `yaml:"timeout"`
}

type WindowsExporterConfig struct {
	InstallerBaseURL  string                  `yaml:"installer_base_url"`
	DefaultVersion    string                  `yaml:"default_version"`
	DefaultPort       int                     `yaml:"default_port"`
	EnabledCollectors string                  `yaml:"enabled_collectors"`
	PushMode          WindowsExporterPushMode `yaml:"push_mode"`
}

type WindowsExporterPushMode struct {
	Enabled        bool   `yaml:"enabled"`
	Interval       int    `yaml:"interval"`
	PushgatewayURL string `yaml:"pushgateway_url"`
	MetricsPushURL string `yaml:"metrics_push_url"`
}

var Config *config

func init() {}

func LoadConfig(configPath string) error {
	if configPath == "" {
		configPath = "./config.yaml"
	}

	yamlFile, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	if err := yaml.Unmarshal(yamlFile, &Config); err != nil {
		return err
	}
	Config.Logtail.applyDefaults()
	Config.Monitor.applyCompatibilityDefaults()

	return nil
}

func (c *logtailConfig) applyDefaults() {
	c.DeliveryMode = normalizeLogtailDeliveryMode(c.DeliveryMode)
	if c.VictoriaLogs.TimeoutSeconds <= 0 {
		c.VictoriaLogs.TimeoutSeconds = 10
	}
	c.Kafka.applyDefaults()
	if c.Vector.DiskBufferMaxSizeBytes <= 0 {
		c.Vector.DiskBufferMaxSizeBytes = 2 * 1024 * 1024 * 1024
	}
	if strings.TrimSpace(c.Aggregator.Image) == "" {
		c.Aggregator.Image = "docker.1ms.run/timberio/vector:latest-debian"
	}
}

func normalizeLogtailDeliveryMode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "kafka":
		return "kafka"
	case "direct":
		return "direct"
	default:
		return "auto"
	}
}

func (k *kafkaConfig) applyDefaults() {
	k.Brokers = normalizeKafkaBrokers(k.Brokers)
	k.HostAliases = normalizeKafkaHostAliases(k.HostAliases)
	if strings.TrimSpace(k.Topic) == "" {
		k.Topic = "aiops-logs"
	}
}

func normalizeKafkaBrokers(brokers []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(brokers))
	for _, broker := range brokers {
		broker = strings.TrimSpace(broker)
		if broker == "" {
			continue
		}
		if _, ok := seen[broker]; ok {
			continue
		}
		seen[broker] = struct{}{}
		result = append(result, broker)
	}
	return result
}

func normalizeKafkaHostAliases(aliases []kafkaHostAliasConfig) []kafkaHostAliasConfig {
	byIP := map[string]int{}
	result := make([]kafkaHostAliasConfig, 0, len(aliases))
	for _, alias := range aliases {
		ip := strings.TrimSpace(alias.IP)
		if ip == "" {
			continue
		}
		hostnames := normalizeKafkaHostnames(alias.Hostnames)
		if len(hostnames) == 0 {
			continue
		}
		if index, ok := byIP[ip]; ok {
			result[index].Hostnames = mergeKafkaHostnames(result[index].Hostnames, hostnames)
			continue
		}
		byIP[ip] = len(result)
		result = append(result, kafkaHostAliasConfig{IP: ip, Hostnames: hostnames})
	}
	return result
}

func normalizeKafkaHostnames(hostnames []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(hostnames))
	for _, hostname := range hostnames {
		hostname = strings.TrimSpace(hostname)
		if hostname == "" {
			continue
		}
		if _, ok := seen[hostname]; ok {
			continue
		}
		seen[hostname] = struct{}{}
		result = append(result, hostname)
	}
	return result
}

func mergeKafkaHostnames(base, extra []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(base)+len(extra))
	for _, hostname := range base {
		if _, ok := seen[hostname]; ok {
			continue
		}
		seen[hostname] = struct{}{}
		result = append(result, hostname)
	}
	for _, hostname := range extra {
		if _, ok := seen[hostname]; ok {
			continue
		}
		seen[hostname] = struct{}{}
		result = append(result, hostname)
	}
	return result
}

func (m *monitor) applyCompatibilityDefaults() {
	if m.VictoriaMetrics.URL != "" {
		m.Prometheus.URL = m.VictoriaMetrics.URL
	} else {
		m.VictoriaMetrics.URL = m.Prometheus.URL
	}
	if m.MetricsPush.URL != "" {
		m.Pushgateway.URL = m.MetricsPush.URL
	} else {
		m.MetricsPush.URL = m.Pushgateway.URL
	}
}

func GetConfig() *db {
	if Config == nil {
		panic("Config is not initialized")
	}
	return &Config.Db
}

func GetRedisConfig() *redis {
	if Config == nil {
		panic("Config is not initialized")
	}
	return &Config.Redis
}

func Setup() {
	if Config == nil {
		panic("Config initialization failed")
	}
}

// BuildRouteURL joins a configured base URL with an application route while
// keeping exactly one slash between the two parts.
func BuildRouteURL(baseURL, route string) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	route = "/" + strings.TrimLeft(strings.TrimSpace(route), "/")
	if baseURL == "" {
		return route
	}
	return baseURL + route
}
