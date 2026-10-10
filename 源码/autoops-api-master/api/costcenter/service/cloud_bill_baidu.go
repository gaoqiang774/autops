package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"dodevops-api/api/costcenter/model"

	"github.com/baidubce/bce-sdk-go/bce"
	"github.com/baidubce/bce-sdk-go/http"
	baidubilling "github.com/baidubce/bce-sdk-go/services/billing"
)

const (
	baiduBillingEndpoint = "https://billing.baidubce.com"
	baiduBillPageSize    = 100
	baiduBalanceURI      = "/v1/finance/cash/balance"
)

type baiduBillClient interface {
	QueryBalance(context.Context) (*baiduBalanceResponse, error)
	ListBillDetails(context.Context, string, int, int) (*baidubilling.ResourceChargeItemBillResponse, error)
}

type baiduAPIClient struct {
	client *baidubilling.Client
}

type rateLimitedBaiduBillClient struct {
	inner   baiduBillClient
	limiter *cloudAPILimiter
}

type baiduBalanceResponse struct {
	AvailableBalance json.RawMessage `json:"availableBalance"`
	CashBalance      json.RawMessage `json:"cashBalance"`
	CreditBalance    json.RawMessage `json:"creditBalance"`
}

func newBaiduBillClient(accessKey, secretKey string) (baiduBillClient, error) {
	client, err := baidubilling.NewClient(accessKey, secretKey, baiduBillingEndpoint)
	if err != nil {
		return nil, err
	}
	return &rateLimitedBaiduBillClient{
		inner:   &baiduAPIClient{client: client},
		limiter: cloudAPILimiterFor("baidu", accessKey),
	}, nil
}

func (c *rateLimitedBaiduBillClient) QueryBalance(ctx context.Context) (*baiduBalanceResponse, error) {
	return callCloudAPI(ctx, c.limiter, func() (*baiduBalanceResponse, error) {
		return c.inner.QueryBalance(ctx)
	})
}

func (c *rateLimitedBaiduBillClient) ListBillDetails(ctx context.Context, month string, page, size int) (*baidubilling.ResourceChargeItemBillResponse, error) {
	return callCloudAPI(ctx, c.limiter, func() (*baidubilling.ResourceChargeItemBillResponse, error) {
		return c.inner.ListBillDetails(ctx, month, page, size)
	})
}

func (c *baiduAPIClient) QueryBalance(ctx context.Context) (*baiduBalanceResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	response := &baiduBalanceResponse{}
	err := bce.NewRequestBuilder(c.client).
		WithMethod(http.POST).
		WithURL(baiduBalanceURI).
		WithBody(struct{}{}).
		WithResult(response).
		Do()
	return response, err
}

func (c *baiduAPIClient) ListBillDetails(ctx context.Context, month string, page, size int) (*baidubilling.ResourceChargeItemBillResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return c.client.ResourceChargeItemBill(baidubilling.ResourceChargeItemBillRequest{
		BillMonth: month,
		PageNo:    page,
		PageSize:  size,
	})
}

func collectBaiduBalance(ctx context.Context, accessKey, secretKey string) (*cloudBalanceResult, error) {
	if err := validateCloudCredential("百度云", accessKey, secretKey); err != nil {
		return nil, err
	}
	client, err := newBaiduBillClient(accessKey, secretKey)
	if err != nil {
		return nil, fmt.Errorf("创建百度云账单客户端失败: %w", err)
	}
	response, err := client.QueryBalance(ctx)
	if err != nil {
		return nil, fmt.Errorf("查询百度云余额失败: %w", err)
	}
	balance, err := baiduAvailableBalance(response)
	if err != nil {
		return nil, fmt.Errorf("解析百度云余额失败: %w", err)
	}
	return &cloudBalanceResult{Balance: balance, Currency: "CNY"}, nil
}

func collectBaiduBills(ctx context.Context, accessKey, secretKey string, months []string) (*cloudCollectionResult, error) {
	if err := validateCloudCredential("百度云", accessKey, secretKey); err != nil {
		return nil, err
	}
	client, err := newBaiduBillClient(accessKey, secretKey)
	if err != nil {
		return nil, fmt.Errorf("创建百度云账单客户端失败: %w", err)
	}
	return collectBaiduBillsWithClient(ctx, client, months), nil
}

func collectBaiduBillsWithClient(ctx context.Context, client baiduBillClient, months []string) *cloudCollectionResult {
	result := &cloudCollectionResult{Currency: "CNY", Months: make([]collectedMonth, 0, len(months))}
	if response, err := client.QueryBalance(ctx); err != nil {
		result.BalanceErr = fmt.Errorf("查询百度云余额失败: %w", err)
	} else if balance, err := baiduAvailableBalance(response); err != nil {
		result.BalanceErr = fmt.Errorf("解析百度云余额失败: %w", err)
	} else {
		result.Balance = &balance
	}

	for _, month := range months {
		// 百度云资源计费项接口只开放已结算月份，当月查询会固定返回 InvalidHTTPRequest。
		if month == time.Now().Format("2006-01") {
			continue
		}
		details, err := collectBaiduBillDetails(ctx, client, month)
		if err != nil {
			result.PartialErr = appendCollectionError(result.PartialErr, fmt.Errorf("查询百度云 %s 账单明细失败: %w", month, err))
			continue
		}
		amount := 0.0
		for _, detail := range details {
			amount += detail.PayableAmount
		}
		result.Months = append(result.Months, collectedMonth{Month: month, Amount: amount, Currency: "CNY"})
		result.Details = append(result.Details, details...)
		result.DetailMonths = append(result.DetailMonths, month)
	}
	return result
}

func collectBaiduBillDetails(ctx context.Context, client baiduBillClient, month string) ([]model.BillDetail, error) {
	result := make([]model.BillDetail, 0)
	for page := 1; ; page++ {
		response, err := client.ListBillDetails(ctx, month, page, baiduBillPageSize)
		if err != nil {
			return nil, err
		}
		if response == nil {
			return nil, errors.New("百度云账单明细响应为空")
		}
		for index := range response.Bills {
			result = append(result, convertBaiduBillDetail(response.Bills[index], month, (page-1)*baiduBillPageSize+index))
		}
		count := len(response.Bills)
		if count == 0 || count < baiduBillPageSize || (response.TotalCount > 0 && page*baiduBillPageSize >= response.TotalCount) {
			break
		}
	}
	return result, nil
}

func convertBaiduBillDetail(item baidubilling.ResourceChargeItemBill, month string, index int) model.BillDetail {
	payable := item.FinancePrice
	resourceID := firstNonBlank(item.InstanceId, item.ShortId)
	recordID := stableRecordID("baidu", month, item.BillId, resourceID, item.ServiceType, item.ChargeItem, item.StartTime, strconv.Itoa(index))
	return model.BillDetail{
		Operator: "baidu", Month: month, RecordID: recordID,
		ResourceID: resourceID, ResourceName: firstNonBlank(item.ShortId, item.InstanceId),
		ProductCode: item.ServiceType, ProductName: firstNonBlank(item.ServiceTypeName, item.ServiceType),
		SubProductName:  item.ProductType,
		BillingItemCode: item.ChargeItem, BillingItemName: firstNonBlank(item.ChargeItemDesc, item.ChargeItem),
		UsageQuantity: cloudAmountOrZero(item.Amount), UsageUnit: item.AmountUnit,
		OriginalAmount: item.OriginPrice, DiscountAmount: positiveDifference(item.OriginPrice, payable), PayableAmount: payable,
		Currency: "CNY", ChargeType: item.ProductType, Region: item.Region,
		BillingStartAt: parseCloudTime(item.StartTime), BillingEndAt: parseCloudTime(item.EndTime),
	}
}

func baiduAvailableBalance(response *baiduBalanceResponse) (float64, error) {
	if response == nil {
		return 0, errors.New("余额响应为空")
	}
	if available, exists, err := parseBaiduAmount(response.AvailableBalance); err != nil {
		return 0, err
	} else if exists {
		return available, nil
	}
	cash, cashExists, err := parseBaiduAmount(response.CashBalance)
	if err != nil {
		return 0, err
	}
	credit, creditExists, err := parseBaiduAmount(response.CreditBalance)
	if err != nil {
		return 0, err
	}
	if !cashExists && !creditExists {
		return 0, errors.New("余额响应缺少 availableBalance、cashBalance 和 creditBalance")
	}
	return cash + credit, nil
}

func parseBaiduAmount(raw json.RawMessage) (float64, bool, error) {
	value := strings.TrimSpace(string(raw))
	if value == "" || value == "null" {
		return 0, false, nil
	}
	if strings.HasPrefix(value, "\"") {
		var text string
		if err := json.Unmarshal(raw, &text); err != nil {
			return 0, true, err
		}
		amount, err := parseCloudAmount(text)
		return amount, true, err
	}
	amount, err := strconv.ParseFloat(value, 64)
	return amount, true, err
}
