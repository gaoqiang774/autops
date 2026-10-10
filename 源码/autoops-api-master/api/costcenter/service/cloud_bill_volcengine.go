package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"dodevops-api/api/costcenter/model"

	"github.com/volcengine/volc-sdk-golang/base"
)

const (
	volcengineBillingHost        = "billing.volcengineapi.com"
	volcengineBillingVersion     = "2022-01-01"
	volcengineBillPageSize       = 100
	volcengineQueryBalanceAction = "QueryBalanceAcct"
	volcengineOverviewAction     = "ListBillOverviewByProd"
	volcengineDetailAction       = "ListBillDetail"
)

type volcengineBillClient interface {
	QueryBalance(context.Context) (*volcengineBalance, error)
	ListBillOverview(context.Context, string, int, int) (*volcengineBillOverviewPage, error)
	ListBillDetails(context.Context, string, int, int) (*volcengineBillDetailPage, error)
}

type volcengineAPIClient struct{ client *base.Client }

type rateLimitedVolcengineBillClient struct {
	inner   volcengineBillClient
	limiter *cloudAPILimiter
}

type volcengineResponseMetadata struct {
	RequestID string `json:"RequestId"`
	Error     *struct {
		Code    string `json:"Code"`
		Message string `json:"Message"`
	} `json:"Error"`
}

type volcengineBalance struct {
	AvailableBalance string `json:"AvailableBalance"`
}

type volcengineBalanceResponse struct {
	ResponseMetadata volcengineResponseMetadata `json:"ResponseMetadata"`
	Result           *volcengineBalance         `json:"Result"`
}

type volcengineBillOverviewItem struct {
	PayableAmount string `json:"PayableAmount"`
	PretaxAmount  string `json:"PretaxAmount"`
	Currency      string `json:"Currency"`
}

type volcengineBillOverviewPage struct {
	List   []*volcengineBillOverviewItem `json:"List"`
	Total  int                           `json:"Total"`
	Limit  int                           `json:"Limit"`
	Offset int                           `json:"Offset"`
}

type volcengineBillOverviewResponse struct {
	ResponseMetadata volcengineResponseMetadata  `json:"ResponseMetadata"`
	Result           *volcengineBillOverviewPage `json:"Result"`
}

type volcengineBillDetailItem struct {
	BillPeriod             string `json:"BillPeriod"`
	ExpenseDate            string `json:"ExpenseDate"`
	Product                string `json:"Product"`
	ProductZh              string `json:"ProductZh"`
	BusinessMode           string `json:"BusinessMode"`
	BillingMode            string `json:"BillingMode"`
	ExpenseBeginTime       string `json:"ExpenseBeginTime"`
	ExpenseEndTime         string `json:"ExpenseEndTime"`
	UseDuration            string `json:"UseDuration"`
	UseDurationUnit        string `json:"UseDurationUnit"`
	TradeTime              string `json:"TradeTime"`
	BillID                 string `json:"BillID"`
	BillCategory           string `json:"BillCategory"`
	InstanceNo             string `json:"InstanceNo"`
	InstanceName           string `json:"InstanceName"`
	ConfigName             string `json:"ConfigName"`
	Element                string `json:"Element"`
	Region                 string `json:"Region"`
	Zone                   string `json:"Zone"`
	Factor                 string `json:"Factor"`
	Count                  string `json:"Count"`
	Unit                   string `json:"Unit"`
	OriginalBillAmount     string `json:"OriginalBillAmount"`
	PreferentialBillAmount string `json:"PreferentialBillAmount"`
	DiscountBillAmount     string `json:"DiscountBillAmount"`
	PayableAmount          string `json:"PayableAmount"`
	PaidAmount             string `json:"PaidAmount"`
	Currency               string `json:"Currency"`
	SettlementType         string `json:"SettlementType"`
	Project                string `json:"Project"`
	SolutionZh             string `json:"SolutionZh"`
	BillDetailID           string `json:"BillDetailId"`
	ElementCode            string `json:"ElementCode"`
	RegionCode             string `json:"RegionCode"`
	ZoneCode               string `json:"ZoneCode"`
	FactorCode             string `json:"FactorCode"`
}

type volcengineBillDetailPage struct {
	List   []*volcengineBillDetailItem `json:"List"`
	Total  int                         `json:"Total"`
	Limit  int                         `json:"Limit"`
	Offset int                         `json:"Offset"`
}

type volcengineBillDetailResponse struct {
	ResponseMetadata volcengineResponseMetadata `json:"ResponseMetadata"`
	Result           *volcengineBillDetailPage  `json:"Result"`
}

func newVolcengineBillClient(accessKey, secretKey string) volcengineBillClient {
	serviceInfo := &base.ServiceInfo{
		Timeout: 45 * time.Second,
		Scheme:  "https",
		Host:    volcengineBillingHost,
		Header:  http.Header{"Accept": []string{"application/json"}},
		Credentials: base.Credentials{
			AccessKeyID:     accessKey,
			SecretAccessKey: secretKey,
			Service:         "billing",
			Region:          "cn-north-1",
		},
	}
	apiInfo := map[string]*base.ApiInfo{
		volcengineQueryBalanceAction: volcengineBillingAPIInfo(volcengineQueryBalanceAction),
		volcengineOverviewAction:     volcengineBillingAPIInfo(volcengineOverviewAction),
		volcengineDetailAction:       volcengineBillingAPIInfo(volcengineDetailAction),
	}
	return &rateLimitedVolcengineBillClient{
		inner:   &volcengineAPIClient{client: base.NewClient(serviceInfo, apiInfo)},
		limiter: cloudAPILimiterFor("volcengine", accessKey),
	}
}

func (c *rateLimitedVolcengineBillClient) QueryBalance(ctx context.Context) (*volcengineBalance, error) {
	return callCloudAPI(ctx, c.limiter, func() (*volcengineBalance, error) {
		return c.inner.QueryBalance(ctx)
	})
}

func (c *rateLimitedVolcengineBillClient) ListBillOverview(ctx context.Context, month string, offset, limit int) (*volcengineBillOverviewPage, error) {
	return callCloudAPI(ctx, c.limiter, func() (*volcengineBillOverviewPage, error) {
		return c.inner.ListBillOverview(ctx, month, offset, limit)
	})
}

func (c *rateLimitedVolcengineBillClient) ListBillDetails(ctx context.Context, month string, offset, limit int) (*volcengineBillDetailPage, error) {
	return callCloudAPI(ctx, c.limiter, func() (*volcengineBillDetailPage, error) {
		return c.inner.ListBillDetails(ctx, month, offset, limit)
	})
}

func volcengineBillingAPIInfo(action string) *base.ApiInfo {
	return &base.ApiInfo{
		Method: http.MethodPost,
		Path:   "/",
		Query: url.Values{
			"Action":  []string{action},
			"Version": []string{volcengineBillingVersion},
		},
	}
}

func (c *volcengineAPIClient) QueryBalance(ctx context.Context) (*volcengineBalance, error) {
	body, status, err := c.client.CtxQuery(ctx, volcengineQueryBalanceAction, nil)
	if err != nil {
		return nil, err
	}
	var response volcengineBalanceResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("HTTP %d 响应解析失败: %w", status, err)
	}
	if err := volcengineAPIError(status, response.ResponseMetadata); err != nil {
		return nil, err
	}
	if response.Result == nil {
		return nil, errors.New("响应缺少 Result")
	}
	return response.Result, nil
}

func (c *volcengineAPIClient) ListBillOverview(ctx context.Context, month string, offset, limit int) (*volcengineBillOverviewPage, error) {
	query := volcengineBillQuery(month, offset, limit)
	body, status, err := c.client.CtxQuery(ctx, volcengineOverviewAction, query)
	if err != nil {
		return nil, err
	}
	var response volcengineBillOverviewResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("HTTP %d 响应解析失败: %w", status, err)
	}
	if err := volcengineAPIError(status, response.ResponseMetadata); err != nil {
		return nil, err
	}
	if response.Result == nil {
		return nil, errors.New("响应缺少 Result")
	}
	return response.Result, nil
}

func (c *volcengineAPIClient) ListBillDetails(ctx context.Context, month string, offset, limit int) (*volcengineBillDetailPage, error) {
	query := volcengineBillQuery(month, offset, limit)
	query.Set("GroupTerm", "0")
	query.Set("GroupPeriod", "2")
	body, status, err := c.client.CtxQuery(ctx, volcengineDetailAction, query)
	if err != nil {
		return nil, err
	}
	var response volcengineBillDetailResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("HTTP %d 响应解析失败: %w", status, err)
	}
	if err := volcengineAPIError(status, response.ResponseMetadata); err != nil {
		return nil, err
	}
	if response.Result == nil {
		return nil, errors.New("响应缺少 Result")
	}
	return response.Result, nil
}

func volcengineBillQuery(month string, offset, limit int) url.Values {
	return url.Values{
		"BillPeriod":    []string{month},
		"Offset":        []string{strconv.Itoa(offset)},
		"Limit":         []string{strconv.Itoa(limit)},
		"IgnoreZero":    []string{"0"},
		"NeedRecordNum": []string{"1"},
	}
}

func volcengineAPIError(status int, metadata volcengineResponseMetadata) error {
	if metadata.Error != nil {
		message := strings.TrimSpace(metadata.Error.Message)
		if message == "" {
			message = "接口调用失败"
		}
		return fmt.Errorf("%s: %s", strings.TrimSpace(metadata.Error.Code), message)
	}
	if status < http.StatusOK || status >= http.StatusMultipleChoices {
		return fmt.Errorf("HTTP %d", status)
	}
	return nil
}

func collectVolcengineBalance(ctx context.Context, accessKey, secretKey string) (*cloudBalanceResult, error) {
	if err := validateCloudCredential("火山引擎", accessKey, secretKey); err != nil {
		return nil, err
	}
	response, err := newVolcengineBillClient(accessKey, secretKey).QueryBalance(ctx)
	if err != nil {
		return nil, fmt.Errorf("查询火山引擎余额失败: %w", err)
	}
	if response == nil || strings.TrimSpace(response.AvailableBalance) == "" {
		return nil, errors.New("火山引擎余额响应缺少可用余额")
	}
	amount, err := parseCloudAmount(response.AvailableBalance)
	if err != nil {
		return nil, fmt.Errorf("解析火山引擎余额失败: %w", err)
	}
	return &cloudBalanceResult{Balance: amount, Currency: "CNY"}, nil
}

func collectVolcengineBills(ctx context.Context, accessKey, secretKey string, months []string) (*cloudCollectionResult, error) {
	if err := validateCloudCredential("火山引擎", accessKey, secretKey); err != nil {
		return nil, err
	}
	return collectVolcengineBillsWithClient(ctx, newVolcengineBillClient(accessKey, secretKey), months), nil
}

func collectVolcengineBillsWithClient(ctx context.Context, client volcengineBillClient, months []string) *cloudCollectionResult {
	result := &cloudCollectionResult{Currency: "CNY", Months: make([]collectedMonth, 0, len(months))}
	if response, err := client.QueryBalance(ctx); err != nil {
		result.BalanceErr = fmt.Errorf("查询火山引擎余额失败: %w", err)
	} else if response == nil || strings.TrimSpace(response.AvailableBalance) == "" {
		result.BalanceErr = errors.New("火山引擎余额响应缺少可用余额")
	} else if amount, err := parseCloudAmount(response.AvailableBalance); err != nil {
		result.BalanceErr = fmt.Errorf("解析火山引擎余额失败: %w", err)
	} else {
		result.Balance = &amount
	}

	for _, month := range months {
		if err := ctx.Err(); err != nil {
			result.PartialErr = appendCollectionError(result.PartialErr, err)
			break
		}
		amount, currency, err := queryVolcengineMonthlyBill(ctx, client, month)
		if err != nil {
			result.PartialErr = appendCollectionError(result.PartialErr, fmt.Errorf("查询火山引擎 %s 账单失败: %w", month, err))
			continue
		}
		result.Months = append(result.Months, collectedMonth{Month: month, Amount: amount, Currency: currency})
		details, err := collectVolcengineBillDetails(ctx, client, month)
		if err != nil {
			result.PartialErr = appendCollectionError(result.PartialErr, fmt.Errorf("查询火山引擎 %s 账单明细失败: %w", month, err))
			continue
		}
		detailAmount := sumBillDetailPayableAmount(details)
		if detailAmount+0.01 < amount {
			if month == time.Now().Format("2006-01") {
				result.PartialErr = appendCollectionError(result.PartialErr, fmt.Errorf(
					"火山引擎 %s 当前月明细尚未完整生成: 汇总 %.2f CNY，已返回明细 %.2f CNY",
					month, amount, detailAmount,
				))
			} else {
				result.PartialErr = appendCollectionError(result.PartialErr, fmt.Errorf(
					"火山引擎 %s 历史月明细不完整，保留已有数据: 汇总 %.2f CNY，已返回明细 %.2f CNY",
					month, amount, detailAmount,
				))
				continue
			}
		}
		result.DetailMonths = append(result.DetailMonths, month)
		result.Details = append(result.Details, details...)
	}
	return result
}

func sumBillDetailPayableAmount(details []model.BillDetail) float64 {
	total := 0.0
	for _, detail := range details {
		total += detail.PayableAmount
	}
	return total
}

func queryVolcengineMonthlyBill(ctx context.Context, client volcengineBillClient, month string) (float64, string, error) {
	total := 0.0
	currency := "CNY"
	for offset := 0; ; {
		response, err := client.ListBillOverview(ctx, month, offset, volcengineBillPageSize)
		if err != nil {
			return 0, "", err
		}
		if response == nil {
			return 0, "", errors.New("账单响应为空")
		}
		for _, item := range response.List {
			if item == nil {
				continue
			}
			value := firstNonBlank(item.PayableAmount, item.PretaxAmount)
			amount, err := parseCloudAmount(value)
			if err != nil {
				return 0, "", fmt.Errorf("无效账单金额 %q: %w", value, err)
			}
			total += amount
			if len(strings.TrimSpace(item.Currency)) == 3 {
				currency = strings.ToUpper(strings.TrimSpace(item.Currency))
			}
		}
		count := len(response.List)
		offset += count
		if count == 0 || (response.Total > 0 && offset >= response.Total) || (response.Total <= 0 && count < volcengineBillPageSize) {
			break
		}
	}
	return total, currency, nil
}

func collectVolcengineBillDetails(ctx context.Context, client volcengineBillClient, month string) ([]model.BillDetail, error) {
	result := make([]model.BillDetail, 0)
	for offset := 0; ; {
		response, err := client.ListBillDetails(ctx, month, offset, volcengineBillPageSize)
		if err != nil {
			return nil, err
		}
		if response == nil {
			return nil, errors.New("账单明细响应为空")
		}
		for index, item := range response.List {
			if item != nil {
				result = append(result, convertVolcengineBillDetail(item, month, offset+index))
			}
		}
		count := len(response.List)
		offset += count
		if count == 0 || (response.Total > 0 && offset >= response.Total) || (response.Total <= 0 && count < volcengineBillPageSize) {
			break
		}
	}
	return result, nil
}

func convertVolcengineBillDetail(item *volcengineBillDetailItem, month string, index int) model.BillDetail {
	resourceID := firstNonBlank(item.InstanceNo, "-")
	resourceName := firstNonBlank(item.InstanceName, resourceID, "-")
	productName := firstNonBlank(item.ProductZh, item.Product, "未分类")
	itemCode := firstNonBlank(item.ElementCode, item.FactorCode, "-")
	itemName := firstNonBlank(item.Element, item.Factor, "-")
	original := cloudAmountOrZero(item.OriginalBillAmount)
	payable := cloudAmountOrZero(firstNonBlank(item.PayableAmount, item.PaidAmount))
	usageText := firstNonBlank(item.Count, item.UseDuration)
	usageUnit := firstNonBlank(item.Unit, item.UseDurationUnit, "-")
	recordID := firstNonBlank(item.BillDetailID, stableRecordID("volcengine", month, item.BillID, resourceID, itemCode, strconv.Itoa(index)))
	return model.BillDetail{
		Operator: "volcengine", Month: month, RecordID: recordID,
		ResourceID: resourceID, ResourceName: resourceName,
		ProductCode: item.Product, ProductName: productName, SubProductName: firstNonBlank(item.SolutionZh, item.ConfigName, "-"),
		BillingItemCode: itemCode, BillingItemName: itemName,
		UsageQuantity: cloudAmountOrZero(usageText), UsageUnit: usageUnit,
		OriginalAmount: original, DiscountAmount: positiveDifference(original, payable), PayableAmount: payable,
		Currency: normalizedCurrency(firstNonBlank(item.Currency, "CNY")), ChargeType: firstNonBlank(item.BillingMode, item.SettlementType, "-"),
		TransactionType: firstNonBlank(item.BillCategory, item.BusinessMode, "-"),
		Region:          firstNonBlank(item.Region, item.RegionCode, "-"), Zone: firstNonBlank(item.Zone, item.ZoneCode, "-"), ProjectName: firstNonBlank(item.Project, "-"),
		OrderID: item.BillID, TransactionID: item.BillDetailID,
		DeductedAt:     parseCloudTime(firstNonBlank(item.TradeTime, item.ExpenseDate)),
		BillingStartAt: parseCloudTime(item.ExpenseBeginTime), BillingEndAt: parseCloudTime(item.ExpenseEndTime),
	}
}
