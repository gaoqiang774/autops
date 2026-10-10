package util

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	tianyiEndpointURL         = "https://ctecs-global.ctapi.ctyun.cn"
	tianyiSuccessStatusCode   = "800"
	tianyiHeaderRequestID     = "ctyun-eop-request-id"
	tianyiHeaderAuthorization = "Eop-Authorization"
	tianyiHeaderDate          = "Eop-date"
)

// TianyiCloudService provides Tianyi Cloud ECS discovery via AK/SK.
type TianyiCloudService struct {
	AccessKey    string
	AccessSecret string
	Region       string
	client       *http.Client
}

// NewTianyiCloudService creates a Tianyi Cloud service client wrapper.
func NewTianyiCloudService(accessKey, accessSecret, region string) *TianyiCloudService {
	return &TianyiCloudService{
		AccessKey:    strings.TrimSpace(accessKey),
		AccessSecret: strings.TrimSpace(accessSecret),
		Region:       strings.TrimSpace(region),
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// TianyiInstanceInfo is the normalized Tianyi ECS instance payload.
type TianyiInstanceInfo struct {
	InstanceID   string   `json:"instance_id"`
	InstanceName string   `json:"instance_name"`
	PrivateIPs   []string `json:"private_ips"`
	PublicIPs    []string `json:"public_ips"`
	CPU          int      `json:"cpu"`
	MemoryGB     int      `json:"memory_gb"`
	DiskGB       string   `json:"disk_gb"`
	BillingType  string   `json:"billing_type"`
	OSName       string   `json:"os_name"`
	CreateTime   string   `json:"create_time"`
	ExpireTime   string   `json:"expire_time"`
	Region       string   `json:"region"`
	Status       string   `json:"status"`
}

type tianyiStandardResponse struct {
	StatusCode  interface{}     `json:"statusCode"`
	ErrorCode   interface{}     `json:"errorCode"`
	Message     string          `json:"message"`
	Description string          `json:"description"`
	ReturnObj   json.RawMessage `json:"returnObj"`
}

type tianyiRegion struct {
	RegionParent string   `json:"regionParent"`
	RegionID     string   `json:"regionID"`
	RegionType   string   `json:"regionType"`
	RegionName   string   `json:"regionName"`
	IsMultiZones bool     `json:"isMultiZones"`
	ZoneList     []string `json:"zoneList"`
}

type tianyiRegionListResponse struct {
	RegionList []tianyiRegion `json:"regionList"`
}

type tianyiAddress struct {
	Addr    string `json:"addr"`
	Type    string `json:"type"`
	Version int    `json:"version"`
}

type tianyiAddressGroup struct {
	VpcName     string          `json:"vpcName"`
	AddressList []tianyiAddress `json:"addressList"`
}

type tianyiNetworkCard struct {
	Gateway       string   `json:"gateway"`
	IPv4Address   string   `json:"IPv4Address"`
	IPv6Address   []string `json:"IPv6Address"`
	IsMaster      bool     `json:"isMaster"`
	NetworkCardID string   `json:"networkCardID"`
	SecurityGroup []string `json:"securityGroup"`
	SubnetCidr    string   `json:"subnetCidr"`
	SubnetID      string   `json:"subnetID"`
}

type tianyiFlavor struct {
	FlavorCPU  int    `json:"flavorCPU"`
	FlavorID   string `json:"flavorID"`
	FlavorName string `json:"flavorName"`
	FlavorRAM  int    `json:"flavorRAM"`
}

type tianyiImage struct {
	ImageID   string `json:"imageID"`
	ImageName string `json:"imageName"`
}

type tianyiListInstanceResult struct {
	Addresses       []tianyiAddressGroup `json:"addresses"`
	CreatedTime     string               `json:"createdTime"`
	ExpiredTime     string               `json:"expiredTime"`
	FixedIPList     []string             `json:"fixedIpList"`
	FloatingIP      string               `json:"floatingIp"`
	Flavor          tianyiFlavor         `json:"flavor"`
	Image           tianyiImage          `json:"image"`
	InstanceID      string               `json:"instanceID"`
	InstanceName    string               `json:"instanceName"`
	InstanceStatus  string               `json:"instanceStatus"`
	NetworkCardList []tianyiNetworkCard  `json:"networkCardList"`
	OnDemand        bool                 `json:"onDemand"`
	OsType          int                  `json:"osType"`
	PrivateIP       string               `json:"privateIP"`
	ProjectID       string               `json:"projectID"`
	RegionID        string               `json:"regionID"`
}

type tianyiListInstancesResponse struct {
	CurrentCount int                        `json:"currentCount"`
	Results      []tianyiListInstanceResult `json:"results"`
	TotalCount   int                        `json:"totalCount"`
	TotalPage    int                        `json:"totalPage"`
}

type tianyiListInstancesRequest struct {
	RegionID *string `json:"regionID"`
	PageNo   *int    `json:"pageNo,omitempty"`
	PageSize *int    `json:"pageSize,omitempty"`
}

// GetInstances returns Tianyi ECS instances across all regions, or a single region when provided.
func (s *TianyiCloudService) GetInstances() ([]TianyiInstanceInfo, error) {
	if s.AccessKey == "" || s.AccessSecret == "" {
		return nil, fmt.Errorf("天翼云AccessKey或AccessSecret为空")
	}

	ctx := context.Background()
	regions, err := s.listRegions(ctx)
	if err != nil {
		return nil, err
	}

	allInstances := make([]TianyiInstanceInfo, 0)
	for _, regionID := range regions {
		pageNo := 1
		pageSize := 100

		for {
			resp, reqErr := s.listInstances(ctx, regionID, pageNo, pageSize)
			if reqErr != nil {
				fmt.Printf("[WARN] 查询天翼云区域 %s 实例失败: %v\n", regionID, reqErr)
				break
			}

			if resp == nil || len(resp.Results) == 0 {
				break
			}

			for _, instance := range resp.Results {
				allInstances = append(allInstances, convertTianyiInstance(regionID, instance))
			}

			if pageNo >= resp.TotalPage || len(resp.Results) < pageSize {
				break
			}
			pageNo++
		}
	}

	return allInstances, nil
}

// GetInstancesJSON returns Tianyi instances as JSON.
func (s *TianyiCloudService) GetInstancesJSON() ([]byte, error) {
	instances, err := s.GetInstances()
	if err != nil {
		return nil, err
	}
	return json.Marshal(instances)
}

func (s *TianyiCloudService) listRegions(ctx context.Context) ([]string, error) {
	if s.Region != "" {
		return []string{s.Region}, nil
	}

	var payload tianyiRegionListResponse
	if err := s.doRequest(ctx, http.MethodGet, "/v4/region/list-regions", nil, nil, &payload); err != nil {
		return nil, fmt.Errorf("查询天翼云区域列表失败: %v", err)
	}

	regions := make([]string, 0, len(payload.RegionList))
	seen := make(map[string]struct{}, len(payload.RegionList))
	for _, region := range payload.RegionList {
		regionID := strings.TrimSpace(region.RegionID)
		if regionID == "" {
			continue
		}
		if _, ok := seen[regionID]; ok {
			continue
		}
		seen[regionID] = struct{}{}
		regions = append(regions, regionID)
	}

	if len(regions) == 0 {
		return nil, fmt.Errorf("未获取到可用的天翼云区域")
	}

	return regions, nil
}

func (s *TianyiCloudService) listInstances(ctx context.Context, regionID string, pageNo, pageSize int) (*tianyiListInstancesResponse, error) {
	var payload tianyiListInstancesResponse
	if err := s.doRequest(ctx, http.MethodPost, "/v4/ecs/list-instances", nil, &tianyiListInstancesRequest{
		RegionID: &regionID,
		PageNo:   &pageNo,
		PageSize: &pageSize,
	}, &payload); err != nil {
		return nil, err
	}
	return &payload, nil
}

func (s *TianyiCloudService) doRequest(ctx context.Context, method, path string, query url.Values, body interface{}, target interface{}) error {
	var bodyBytes []byte
	var err error
	if body != nil {
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return err
		}
	}

	if query == nil {
		query = url.Values{}
	}
	rawQuery := query.Encode()
	requestID := uuid.NewString()
	now := time.Now().UTC()

	reqURL := tianyiEndpointURL + path
	if rawQuery != "" {
		reqURL += "?" + rawQuery
	}

	req, err := http.NewRequestWithContext(ctx, method, reqURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Content-Length", strconv.Itoa(len(bodyBytes)))
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set(tianyiHeaderRequestID, requestID)
	req.Header.Set(tianyiHeaderDate, now.Format("20060102T150405Z"))
	req.Header.Set(tianyiHeaderAuthorization, s.buildAuthorization(rawQuery, bodyBytes, now, requestID))

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var standard tianyiStandardResponse
	if err := json.Unmarshal(respBody, &standard); err != nil {
		return err
	}

	if standard.statusCodeString() != tianyiSuccessStatusCode {
		code := standard.errorCodeString()
		if code == "" {
			code = standard.statusCodeString()
		}
		message := strings.TrimSpace(strings.Join([]string{standard.Description, standard.Message}, ","))
		message = strings.Trim(message, ", ")
		if message == "" {
			message = "unknown error"
		}
		return fmt.Errorf("%s: %s", code, message)
	}

	if target != nil && len(standard.ReturnObj) > 0 && string(standard.ReturnObj) != "null" {
		if err := json.Unmarshal(standard.ReturnObj, target); err != nil {
			return err
		}
	}

	return nil
}

func (s *TianyiCloudService) buildAuthorization(rawQuery string, body []byte, now time.Time, requestID string) string {
	bodyHash := sha256.Sum256(body)
	contentHash := hex.EncodeToString(bodyHash[:])
	date := now.Format("20060102T150405Z")
	signatureText := fmt.Sprintf("%s:%s\n%s:%s\n\n%s\n%s",
		tianyiHeaderRequestID, requestID,
		strings.ToLower(tianyiHeaderDate), date,
		rawQuery,
		contentHash,
	)

	day := now.Format("20060102")
	kAk := hmacSHA256(s.AccessKey, string(hmacSHA256(date, s.AccessSecret)))
	kDate := hmacSHA256(day, string(kAk))
	signature := base64.StdEncoding.EncodeToString(hmacSHA256(signatureText, string(kDate)))

	return s.AccessKey + " Headers=ctyun-eop-request-id;eop-date Signature=" + signature
}

func hmacSHA256(signature string, key string) []byte {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(signature))
	return mac.Sum(nil)
}

func (r tianyiStandardResponse) statusCodeString() string {
	return stringifyAny(r.StatusCode)
}

func (r tianyiStandardResponse) errorCodeString() string {
	return stringifyAny(r.ErrorCode)
}

func stringifyAny(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return typed
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(typed), 'f', -1, 64)
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	case json.Number:
		return typed.String()
	default:
		return ""
	}
}

func convertTianyiInstance(regionID string, instance tianyiListInstanceResult) TianyiInstanceInfo {
	privateIPs := make([]string, 0)
	publicIPs := make([]string, 0)

	privateIPs = append(privateIPs, splitAndCollectIPs(instance.PrivateIP)...)
	privateIPs = append(privateIPs, instance.FixedIPList...)

	if instance.FloatingIP != "" {
		publicIPs = append(publicIPs, instance.FloatingIP)
	}

	for _, nic := range instance.NetworkCardList {
		privateIPs = append(privateIPs, strings.TrimSpace(nic.IPv4Address))
	}

	for _, address := range instance.Addresses {
		for _, item := range address.AddressList {
			ip := strings.TrimSpace(item.Addr)
			if ip == "" {
				continue
			}
			switch strings.ToLower(strings.TrimSpace(item.Type)) {
			case "floating", "public":
				publicIPs = append(publicIPs, ip)
			case "fixed", "private":
				privateIPs = append(privateIPs, ip)
			default:
				if item.Version == 4 {
					privateIPs = append(privateIPs, ip)
				}
			}
		}
	}

	memoryGB := instance.Flavor.FlavorRAM / 1024
	if memoryGB == 0 && instance.Flavor.FlavorRAM > 0 {
		memoryGB = 1
	}

	osName := strings.TrimSpace(instance.Image.ImageName)
	if osName == "" {
		osName = tianyiOsTypeName(instance.OsType)
	}

	billingType := "包年包月"
	if instance.OnDemand {
		billingType = "按需付费"
	}

	return TianyiInstanceInfo{
		InstanceID:   instance.InstanceID,
		InstanceName: instance.InstanceName,
		PrivateIPs:   dedupeIPs(privateIPs),
		PublicIPs:    dedupeIPs(publicIPs),
		CPU:          instance.Flavor.FlavorCPU,
		MemoryGB:     memoryGB,
		DiskGB:       "0",
		BillingType:  billingType,
		OSName:       osName,
		CreateTime:   instance.CreatedTime,
		ExpireTime:   instance.ExpiredTime,
		Region:       regionID,
		Status:       instance.InstanceStatus,
	}
}

func dedupeIPs(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
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

func splitAndCollectIPs(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}

	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ';' || r == ' ' || r == '\n' || r == '\t'
	})

	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

func tianyiOsTypeName(osType int) string {
	switch osType {
	case 1:
		return "Linux"
	case 2:
		return "Windows"
	default:
		return ""
	}
}
