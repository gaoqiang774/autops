package service

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"dodevops-api/api/costcenter/model"

	coreauth "github.com/huaweicloud/huaweicloud-sdk-go-v3/core/auth/global"
	coreconfig "github.com/huaweicloud/huaweicloud-sdk-go-v3/core/config"
	bss "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/bss/v2"
	bssmodel "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/bss/v2/model"
	bssregion "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/bss/v2/region"
)

const huaweiBillPageSize int32 = 1000

type huaweiBillClient interface {
	ShowCustomerAccountBalances(*bssmodel.ShowCustomerAccountBalancesRequest) (*bssmodel.ShowCustomerAccountBalancesResponse, error)
	ShowCustomerMonthlySum(*bssmodel.ShowCustomerMonthlySumRequest) (*bssmodel.ShowCustomerMonthlySumResponse, error)
	ListCustomerBillsMonthlyBreakDown(*bssmodel.ListCustomerBillsMonthlyBreakDownRequest) (*bssmodel.ListCustomerBillsMonthlyBreakDownResponse, error)
}

type rateLimitedHuaweiBillClient struct {
	ctx     context.Context
	inner   huaweiBillClient
	limiter *cloudAPILimiter
}

func newHuaweiBillClient(ctx context.Context, accessKey, secretKey string) huaweiBillClient {
	auth := coreauth.NewCredentialsBuilder().WithAk(accessKey).WithSk(secretKey).Build()
	// Rate-limit retries are handled outside the SDK so every attempt passes through the limiter.
	httpConfig := coreconfig.DefaultHttpConfig().WithTimeout(45 * time.Second).WithRetries(0)
	hcClient := bss.BssClientBuilder().
		WithRegion(bssregion.CN_NORTH_1).
		WithCredential(auth).
		WithHttpConfig(httpConfig).
		Build()
	return &rateLimitedHuaweiBillClient{
		ctx:     ctx,
		inner:   bss.NewBssClient(hcClient),
		limiter: cloudAPILimiterFor("huawei", accessKey),
	}
}

func (c *rateLimitedHuaweiBillClient) ShowCustomerAccountBalances(request *bssmodel.ShowCustomerAccountBalancesRequest) (*bssmodel.ShowCustomerAccountBalancesResponse, error) {
	return callCloudAPI(c.ctx, c.limiter, func() (*bssmodel.ShowCustomerAccountBalancesResponse, error) {
		return c.inner.ShowCustomerAccountBalances(request)
	})
}

func (c *rateLimitedHuaweiBillClient) ShowCustomerMonthlySum(request *bssmodel.ShowCustomerMonthlySumRequest) (*bssmodel.ShowCustomerMonthlySumResponse, error) {
	return callCloudAPI(c.ctx, c.limiter, func() (*bssmodel.ShowCustomerMonthlySumResponse, error) {
		return c.inner.ShowCustomerMonthlySum(request)
	})
}

func (c *rateLimitedHuaweiBillClient) ListCustomerBillsMonthlyBreakDown(request *bssmodel.ListCustomerBillsMonthlyBreakDownRequest) (*bssmodel.ListCustomerBillsMonthlyBreakDownResponse, error) {
	return callCloudAPI(c.ctx, c.limiter, func() (*bssmodel.ListCustomerBillsMonthlyBreakDownResponse, error) {
		return c.inner.ListCustomerBillsMonthlyBreakDown(request)
	})
}

func collectHuaweiBalance(ctx context.Context, accessKey, secretKey string) (*cloudBalanceResult, error) {
	if err := validateCloudCredential("华为云", accessKey, secretKey); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	amount, currency, err := queryHuaweiBalance(newHuaweiBillClient(ctx, accessKey, secretKey))
	if err != nil {
		return nil, fmt.Errorf("查询华为云余额失败: %w", err)
	}
	return &cloudBalanceResult{Balance: amount, Currency: currency}, nil
}

func collectHuaweiBills(ctx context.Context, accessKey, secretKey string, months []string) (*cloudCollectionResult, error) {
	if err := validateCloudCredential("华为云", accessKey, secretKey); err != nil {
		return nil, err
	}
	return collectHuaweiBillsWithClient(ctx, newHuaweiBillClient(ctx, accessKey, secretKey), months), nil
}

func collectHuaweiBillsWithClient(ctx context.Context, client huaweiBillClient, months []string) *cloudCollectionResult {
	result := &cloudCollectionResult{Currency: "CNY", Months: make([]collectedMonth, 0, len(months))}
	if amount, currency, err := queryHuaweiBalance(client); err != nil {
		result.BalanceErr = fmt.Errorf("查询华为云余额失败: %w", err)
	} else {
		result.Balance = &amount
		result.Currency = currency
	}

	method := "oneself"
	for _, month := range months {
		if err := ctx.Err(); err != nil {
			result.PartialErr = appendCollectionError(result.PartialErr, err)
			break
		}
		if _, err := time.Parse("2006-01", month); err != nil {
			result.PartialErr = appendCollectionError(result.PartialErr, fmt.Errorf("华为云账单月份 %q 格式错误", month))
			continue
		}
		offset, limit := int32(0), huaweiBillPageSize
		response, summaryErr := client.ShowCustomerMonthlySum(&bssmodel.ShowCustomerMonthlySumRequest{
			BillCycle: month,
			Method:    &method,
			Offset:    &offset,
			Limit:     &limit,
		})
		amount, amountAvailable, amountErr := huaweiMonthlyAmount(response)
		currency := "CNY"
		if response != nil {
			currency = huaweiCurrency(response.Currency)
		}
		details, detailCurrency, err := collectHuaweiBillDetails(ctx, client, month)
		if err != nil {
			result.PartialErr = appendCollectionError(result.PartialErr, fmt.Errorf("查询华为云 %s 账单明细失败: %w", month, err))
		} else {
			if detailCurrency != "" {
				currency = detailCurrency
			}
			for index := range details {
				details[index].Currency = currency
			}
			result.DetailMonths = append(result.DetailMonths, month)
			result.Details = append(result.Details, details...)
			if !amountAvailable {
				amount = sumHuaweiBillDetails(details)
				amountAvailable = true
			}
		}

		if amountErr != nil {
			result.PartialErr = appendCollectionError(result.PartialErr, fmt.Errorf("解析华为云 %s 账单金额失败: %w", month, amountErr))
		}
		if !amountAvailable {
			if summaryErr != nil {
				result.PartialErr = appendCollectionError(result.PartialErr, fmt.Errorf("查询华为云 %s 账单失败: %w", month, summaryErr))
			} else {
				result.PartialErr = appendCollectionError(result.PartialErr, fmt.Errorf("华为云 %s 账单响应缺少消费总金额", month))
			}
			continue
		}
		result.Months = append(result.Months, collectedMonth{Month: month, Amount: amount, Currency: currency})
	}
	return result
}

func huaweiMonthlyAmount(response *bssmodel.ShowCustomerMonthlySumResponse) (float64, bool, error) {
	if response == nil {
		return 0, false, nil
	}
	if response.ConsumeAmount != nil {
		amount, err := parseCloudAmount(response.ConsumeAmount.String())
		return amount, err == nil, err
	}
	if response.TotalCount != nil && *response.TotalCount == 0 {
		return 0, true, nil
	}
	if response.BillSums == nil {
		return 0, false, nil
	}
	rows := *response.BillSums
	if response.TotalCount != nil && int(*response.TotalCount) > len(rows) {
		return 0, false, nil
	}
	amount := 0.0
	found := false
	for _, row := range rows {
		if row.ConsumeAmount == nil {
			continue
		}
		value, err := parseCloudAmount(row.ConsumeAmount.String())
		if err != nil {
			return 0, false, err
		}
		amount += value
		found = true
	}
	return amount, found || len(rows) == 0, nil
}

func sumHuaweiBillDetails(details []model.BillDetail) float64 {
	amount := 0.0
	for _, detail := range details {
		amount += detail.PayableAmount
	}
	return amount
}

func queryHuaweiBalance(client huaweiBillClient) (float64, string, error) {
	response, err := client.ShowCustomerAccountBalances(&bssmodel.ShowCustomerAccountBalancesRequest{})
	if err != nil {
		return 0, "", err
	}
	if response == nil || response.AccountBalances == nil {
		return 0, "", errors.New("余额响应缺少账户列表")
	}
	total := new(big.Rat)
	currency := huaweiCurrency(response.Currency)
	for _, account := range *response.AccountBalances {
		if account.AccountType == 7 || account.Amount == nil {
			continue
		}
		amount, ok := new(big.Rat).SetString(account.Amount.String())
		if !ok {
			return 0, "", fmt.Errorf("无效余额 %q", account.Amount.String())
		}
		total.Add(total, amount)
		if len(strings.TrimSpace(account.Currency)) == 3 {
			currency = strings.ToUpper(strings.TrimSpace(account.Currency))
		}
	}
	value, _ := total.Float64()
	return value, currency, nil
}

func collectHuaweiBillDetails(ctx context.Context, client huaweiBillClient, month string) ([]model.BillDetail, string, error) {
	method := "oneself"
	currency := "CNY"
	result := make([]model.BillDetail, 0)
	for offset := int32(0); ; {
		if err := ctx.Err(); err != nil {
			return nil, "", err
		}
		response, err := client.ListCustomerBillsMonthlyBreakDown(&bssmodel.ListCustomerBillsMonthlyBreakDownRequest{
			SharedMonth: month,
			Method:      &method,
			Offset:      &offset,
			Limit:       int32Pointer(huaweiBillPageSize),
		})
		if err != nil {
			return nil, "", err
		}
		if response == nil {
			return nil, "", errors.New("账单明细响应为空")
		}
		currency = huaweiCurrency(response.Currency)
		if response.Details == nil {
			if response.TotalCount != nil && *response.TotalCount == 0 {
				return result, currency, nil
			}
			return nil, "", errors.New("账单明细响应缺少记录列表")
		}
		rows := *response.Details
		for index := range rows {
			result = append(result, convertHuaweiBillDetail(rows[index], month, currency, int(offset)+index))
		}
		count := int32(len(rows))
		offset += count
		if count == 0 || (response.TotalCount != nil && offset >= *response.TotalCount) || (response.TotalCount == nil && count < huaweiBillPageSize) {
			break
		}
	}
	return result, currency, nil
}

func convertHuaweiBillDetail(item bssmodel.NvlCostAnalysedBillDetail, month, currency string, index int) model.BillDetail {
	resourceID := firstNonBlank(pointerString(item.SubResourceId), pointerString(item.ResourceId), "-")
	resourceName := firstNonBlank(pointerString(item.SubResourceName), pointerString(item.ResourceName), resourceID, "-")
	productCode := firstNonBlank(pointerString(item.SubServiceTypeCode), pointerString(item.ServiceTypeCode))
	productName := firstNonBlank(pointerString(item.SubServiceTypeName), pointerString(item.ServiceTypeName), productCode, "未分类")
	resourceTypeCode := firstNonBlank(pointerString(item.SubResourceTypeCode), pointerString(item.ResourceTypeCode), "-")
	resourceTypeName := firstNonBlank(pointerString(item.SubResourceTypeName), pointerString(item.ResourceTypeName), "-")
	payable := 0.0
	if item.ConsumeAmount != nil {
		payable = cloudAmountOrZero(item.ConsumeAmount.String())
	}
	usage := 0.0
	if item.Usage != nil {
		usage = cloudAmountOrZero(item.Usage.String())
	}
	recordID := stableRecordID("huawei", month, pointerString(item.OrderId), resourceID, resourceTypeCode, pointerString(item.EffectiveTime), strconv.Itoa(index))
	return model.BillDetail{
		Operator: "huawei", Month: month, RecordID: recordID,
		ResourceID: resourceID, ResourceName: resourceName,
		ProductCode: productCode, ProductName: productName, SubProductName: resourceTypeName,
		BillingItemCode: resourceTypeCode, BillingItemName: firstNonBlank(pointerString(item.UsageType), resourceTypeName),
		UsageQuantity: usage, UsageUnit: huaweiUsageUnit(item.UsageMeasureId),
		OriginalAmount: payable, PayableAmount: payable, DiscountAmount: 0,
		Currency: normalizedCurrency(currency), ChargeType: huaweiChargingMode(item.ChargingMode), TransactionType: huaweiBillType(item.BillType),
		Region: firstNonBlank(pointerString(item.RegionName), pointerString(item.RegionCode), "-"), Zone: "-",
		ProjectName: firstNonBlank(pointerString(item.EnterpriseProjectName), pointerString(item.EnterpriseProjectId), "-"),
		OrderID:     pointerString(item.OrderId), TransactionID: "",
		BillingStartAt: parseCloudTime(pointerString(item.EffectiveTime)), BillingEndAt: parseCloudTime(pointerString(item.ExpireTime)),
	}
}

func validateCloudCredential(provider, keyID, keySecret string) error {
	if strings.TrimSpace(keyID) == "" || strings.TrimSpace(keySecret) == "" {
		return fmt.Errorf("%s Access Key ID 或 Secret Access Key 为空", provider)
	}
	return nil
}

func huaweiCurrency(value *string) string {
	if value != nil {
		currency := strings.ToUpper(strings.TrimSpace(*value))
		if len(currency) == 3 {
			return currency
		}
	}
	return "CNY"
}

func pointerString(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func int32Pointer(value int32) *int32 { return &value }

func huaweiChargingMode(value *int32) string {
	if value == nil {
		return "-"
	}
	switch *value {
	case 1:
		return "包年包月"
	case 3:
		return "按需计费"
	case 10:
		return "预留实例"
	case 11:
		return "节省计划"
	default:
		return fmt.Sprintf("计费模式 %d", *value)
	}
}

func huaweiBillType(value *int32) string {
	if value == nil {
		return "-"
	}
	labels := map[int32]string{1: "新购", 2: "续订", 3: "变更", 4: "退订", 5: "使用", 8: "自动续订", 9: "调账补偿", 16: "调账扣费", 18: "按月付费", 20: "退款变更", 103: "按年付费"}
	if label := labels[*value]; label != "" {
		return label
	}
	return fmt.Sprintf("账单类型 %d", *value)
}

func huaweiUsageUnit(value *int32) string {
	if value == nil {
		return "-"
	}
	return strconv.FormatInt(int64(*value), 10)
}
