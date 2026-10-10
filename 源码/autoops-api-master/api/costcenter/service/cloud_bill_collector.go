package service

import (
	"context"
	"crypto/sha1"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"dodevops-api/api/costcenter/model"

	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/requests"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/bssopenapi"
	mysqlDriver "github.com/go-sql-driver/mysql"
	billing "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/billing/v20180709"
	tencentcommon "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

const (
	cloudBillDetailBatchSize    = 100
	cloudBillSaveMaxAttempts    = 3
	cloudBillSaveRetryBaseDelay = 200 * time.Millisecond
)

type collectedMonth struct {
	Month    string
	Amount   float64
	Currency string
}

type cloudCollectionResult struct {
	Balance      *float64
	BalanceErr   error
	Currency     string
	Months       []collectedMonth
	Details      []model.BillDetail
	DetailMonths []string
	PartialErr   error
}

type cloudBalanceResult struct {
	Balance  float64
	Currency string
}

type tencentBillClient interface {
	DescribeAccountBalanceWithContext(context.Context, *billing.DescribeAccountBalanceRequest) (*billing.DescribeAccountBalanceResponse, error)
	DescribeBillSummaryWithContext(context.Context, *billing.DescribeBillSummaryRequest) (*billing.DescribeBillSummaryResponse, error)
	DescribeBillDetailWithContext(context.Context, *billing.DescribeBillDetailRequest) (*billing.DescribeBillDetailResponse, error)
}

type rateLimitedTencentBillClient struct {
	inner   tencentBillClient
	limiter *cloudAPILimiter
}

func newRateLimitedTencentBillClient(inner tencentBillClient, credentialID string) tencentBillClient {
	return &rateLimitedTencentBillClient{
		inner:   inner,
		limiter: cloudAPILimiterFor("tencentcloud", credentialID),
	}
}

func (c *rateLimitedTencentBillClient) DescribeAccountBalanceWithContext(ctx context.Context, request *billing.DescribeAccountBalanceRequest) (*billing.DescribeAccountBalanceResponse, error) {
	return callCloudAPI(ctx, c.limiter, func() (*billing.DescribeAccountBalanceResponse, error) {
		return c.inner.DescribeAccountBalanceWithContext(ctx, request)
	})
}

func (c *rateLimitedTencentBillClient) DescribeBillSummaryWithContext(ctx context.Context, request *billing.DescribeBillSummaryRequest) (*billing.DescribeBillSummaryResponse, error) {
	return callCloudAPI(ctx, c.limiter, func() (*billing.DescribeBillSummaryResponse, error) {
		return c.inner.DescribeBillSummaryWithContext(ctx, request)
	})
}

func (c *rateLimitedTencentBillClient) DescribeBillDetailWithContext(ctx context.Context, request *billing.DescribeBillDetailRequest) (*billing.DescribeBillDetailResponse, error) {
	return callCloudAPI(ctx, c.limiter, func() (*billing.DescribeBillDetailResponse, error) {
		return c.inner.DescribeBillDetailWithContext(ctx, request)
	})
}

func newTencentBillClientProfile() *profile.ClientProfile {
	cp := profile.NewClientProfile()
	cp.HttpProfile.Endpoint = "billing.tencentcloudapi.com"
	cp.HttpProfile.ReqTimeout = 90
	cp.NetworkFailureMaxRetries = 2
	cp.NetworkFailureRetryDuration = profile.ConstantDurationFunc(2 * time.Second)
	// Billing queries are read-only, so retrying requests without ClientToken is safe.
	cp.UnsafeRetryOnConnectionFailure = true
	return cp
}

func collectCloudBalance(ctx context.Context, operator, keyID, keySecret string) (*cloudBalanceResult, error) {
	switch normalizeOperator(operator) {
	case "aliyun":
		client, err := bssopenapi.NewClientWithAccessKey("cn-hangzhou", keyID, keySecret)
		if err != nil {
			return nil, fmt.Errorf("创建阿里云账单客户端失败: %w", err)
		}
		client.SetConnectTimeout(10 * time.Second)
		client.SetReadTimeout(45 * time.Second)
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		limiter := cloudAPILimiterFor("aliyun", keyID)
		resp, err := callCloudAPI(ctx, limiter, func() (*bssopenapi.QueryAccountBalanceResponse, error) {
			response, callErr := client.QueryAccountBalance(bssopenapi.CreateQueryAccountBalanceRequest())
			if callErr == nil && response != nil && !response.Success {
				callErr = fmt.Errorf("%s: %s", response.Code, response.Message)
			}
			return response, callErr
		})
		if err != nil {
			return nil, fmt.Errorf("查询阿里云余额失败: %w", err)
		}
		if !resp.Success {
			return nil, fmt.Errorf("查询阿里云余额失败: %s", resp.Message)
		}
		value := strings.TrimSpace(resp.Data.AvailableAmount)
		if value == "" {
			value = strings.TrimSpace(resp.Data.AvailableCashAmount)
		}
		balance, err := parseCloudAmount(value)
		if err != nil {
			return nil, fmt.Errorf("解析阿里云余额失败: %w", err)
		}
		currency := strings.ToUpper(strings.TrimSpace(resp.Data.Currency))
		if len(currency) != 3 {
			currency = "CNY"
		}
		return &cloudBalanceResult{Balance: balance, Currency: currency}, nil
	case "tencentcloud":
		cp := newTencentBillClientProfile()
		client, err := billing.NewClient(tencentcommon.NewCredential(keyID, keySecret), "ap-guangzhou", cp)
		if err != nil {
			return nil, fmt.Errorf("创建腾讯云账单客户端失败: %w", err)
		}
		limitedClient := newRateLimitedTencentBillClient(client, keyID)
		resp, err := limitedClient.DescribeAccountBalanceWithContext(ctx, billing.NewDescribeAccountBalanceRequest())
		if err != nil {
			return nil, fmt.Errorf("查询腾讯云余额失败: %w", err)
		}
		if resp == nil {
			return nil, fmt.Errorf("腾讯云余额响应为空")
		}
		value := tencentBalanceFen(resp.Response)
		if value == nil {
			return nil, fmt.Errorf("腾讯云余额响应缺少余额字段")
		}
		return &cloudBalanceResult{Balance: *value / 100, Currency: "CNY"}, nil
	case "baidu":
		return collectBaiduBalance(ctx, keyID, keySecret)
	case "huawei":
		return collectHuaweiBalance(ctx, keyID, keySecret)
	case "volcengine":
		return collectVolcengineBalance(ctx, keyID, keySecret)
	default:
		return nil, fmt.Errorf("暂不支持云厂商 %s", operator)
	}
}

func collectCloudBills(ctx context.Context, operator, keyID, keySecret string, months []string) (*cloudCollectionResult, error) {
	switch normalizeOperator(operator) {
	case "aliyun":
		return collectAliyunBills(ctx, keyID, keySecret, months)
	case "tencentcloud":
		return collectTencentBills(ctx, keyID, keySecret, months)
	case "baidu":
		return collectBaiduBills(ctx, keyID, keySecret, months)
	case "huawei":
		return collectHuaweiBills(ctx, keyID, keySecret, months)
	case "volcengine":
		return collectVolcengineBills(ctx, keyID, keySecret, months)
	default:
		return nil, fmt.Errorf("暂不支持云厂商 %s", operator)
	}
}

func collectTencentBills(ctx context.Context, secretID, secretKey string, months []string) (*cloudCollectionResult, error) {
	cp := newTencentBillClientProfile()
	client, err := billing.NewClient(tencentcommon.NewCredential(secretID, secretKey), "ap-guangzhou", cp)
	if err != nil {
		return nil, fmt.Errorf("创建腾讯云账单客户端失败: %w", err)
	}
	return collectTencentBillsWithClient(ctx, newRateLimitedTencentBillClient(client, secretID), months), nil
}

func collectTencentBillsWithClient(ctx context.Context, client tencentBillClient, months []string) *cloudCollectionResult {
	result := &cloudCollectionResult{Currency: "CNY", Months: make([]collectedMonth, 0, len(months))}
	balanceResp, err := client.DescribeAccountBalanceWithContext(ctx, billing.NewDescribeAccountBalanceRequest())
	if err != nil {
		result.BalanceErr = fmt.Errorf("查询腾讯云余额失败: %w", err)
	} else if balanceResp == nil {
		result.BalanceErr = fmt.Errorf("腾讯云余额响应为空")
	} else if value := tencentBalanceFen(balanceResp.Response); value != nil {
		balance := *value / 100
		result.Balance = &balance
	} else {
		result.BalanceErr = fmt.Errorf("腾讯云余额响应缺少余额字段")
	}

	for _, month := range months {
		if err := ctx.Err(); err != nil {
			result.PartialErr = appendCollectionError(result.PartialErr, err)
			break
		}
		req := billing.NewDescribeBillSummaryRequest()
		req.Month = tencentcommon.StringPtr(month)
		req.GroupType = tencentcommon.StringPtr("business")
		resp, err := client.DescribeBillSummaryWithContext(ctx, req)
		if err != nil {
			result.PartialErr = appendCollectionError(result.PartialErr, fmt.Errorf("查询腾讯云 %s 账单失败: %w", month, err))
			continue
		}
		if resp == nil || resp.Response == nil || (resp.Response.Ready != nil && *resp.Response.Ready == 0) {
			result.PartialErr = appendCollectionError(result.PartialErr, fmt.Errorf("腾讯云 %s 账单尚未生成", month))
			continue
		}
		item := collectedMonth{Month: month, Currency: "CNY"}
		amountValid := true
		for _, detail := range resp.Response.SummaryDetail {
			if detail == nil {
				continue
			}
			amount, err := parseCloudAmount(stringValue(detail.RealTotalCost))
			if err != nil {
				result.PartialErr = appendCollectionError(result.PartialErr, fmt.Errorf("解析腾讯云 %s 账单金额失败: %w", month, err))
				amountValid = false
				break
			}
			item.Amount += amount
		}
		if !amountValid {
			continue
		}
		result.Months = append(result.Months, item)
		details, err := collectTencentBillDetails(ctx, client, month)
		if err != nil {
			result.PartialErr = appendCollectionError(result.PartialErr, fmt.Errorf("查询腾讯云 %s 账单明细失败: %w", month, err))
			continue
		}
		result.DetailMonths = append(result.DetailMonths, month)
		result.Details = append(result.Details, details...)
	}
	return result
}

func collectAliyunBills(ctx context.Context, accessKeyID, accessKeySecret string, months []string) (*cloudCollectionResult, error) {
	client, err := bssopenapi.NewClientWithAccessKey("cn-hangzhou", accessKeyID, accessKeySecret)
	if err != nil {
		return nil, fmt.Errorf("创建阿里云账单客户端失败: %w", err)
	}
	client.SetConnectTimeout(10 * time.Second)
	client.SetReadTimeout(45 * time.Second)
	limiter := cloudAPILimiterFor("aliyun", accessKeyID)
	result := &cloudCollectionResult{Currency: "CNY", Months: make([]collectedMonth, 0, len(months))}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	balanceResp, err := callCloudAPI(ctx, limiter, func() (*bssopenapi.QueryAccountBalanceResponse, error) {
		response, callErr := client.QueryAccountBalance(bssopenapi.CreateQueryAccountBalanceRequest())
		if callErr == nil && response != nil && !response.Success {
			callErr = fmt.Errorf("%s: %s", response.Code, response.Message)
		}
		return response, callErr
	})
	if err != nil {
		result.BalanceErr = fmt.Errorf("查询阿里云余额失败: %w", err)
	} else if !balanceResp.Success {
		result.BalanceErr = fmt.Errorf("查询阿里云余额失败: %s", balanceResp.Message)
	} else {
		balanceText := strings.TrimSpace(balanceResp.Data.AvailableAmount)
		if balanceText == "" {
			balanceText = strings.TrimSpace(balanceResp.Data.AvailableCashAmount)
		}
		balance, parseErr := parseCloudAmount(balanceText)
		if parseErr != nil {
			result.BalanceErr = fmt.Errorf("解析阿里云余额失败: %w", parseErr)
		} else {
			result.Balance = &balance
		}
		if len(balanceResp.Data.Currency) == 3 {
			result.Currency = strings.ToUpper(balanceResp.Data.Currency)
		}
	}

	for _, month := range months {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		req := bssopenapi.CreateQueryBillOverviewRequest()
		req.BillingCycle = month
		resp, err := callCloudAPI(ctx, limiter, func() (*bssopenapi.QueryBillOverviewResponse, error) {
			response, callErr := client.QueryBillOverview(req)
			if callErr == nil && response != nil && !response.Success {
				callErr = fmt.Errorf("%s: %s", response.Code, response.Message)
			}
			return response, callErr
		})
		if err != nil {
			return nil, fmt.Errorf("查询阿里云 %s 账单失败: %w", month, err)
		}
		if !resp.Success {
			return nil, fmt.Errorf("查询阿里云 %s 账单失败: %s", month, resp.Message)
		}
		item := collectedMonth{Month: month, Currency: result.Currency}
		for _, detail := range resp.Data.Items.Item {
			item.Amount += detail.PretaxAmount
		}
		result.Months = append(result.Months, item)
		details, err := collectAliyunBillDetails(ctx, client, limiter, month)
		if err != nil {
			result.PartialErr = appendCollectionError(result.PartialErr, fmt.Errorf("查询阿里云 %s 账单明细失败: %w", month, err))
			continue
		}
		result.DetailMonths = append(result.DetailMonths, month)
		result.Details = append(result.Details, details...)
	}
	return result, nil
}

func saveCloudCollection(ctx context.Context, db *gorm.DB, account *model.BillCloudAccount, collected *cloudCollectionResult) ([]string, error) {
	detailMonths := make(map[string]struct{}, len(collected.DetailMonths))
	for _, month := range collected.DetailMonths {
		detailMonths[month] = struct{}{}
	}
	detailsByMonth := make(map[string][]model.BillDetail, len(detailMonths))
	for _, detail := range collected.Details {
		detailsByMonth[detail.Month] = append(detailsByMonth[detail.Month], detail)
	}

	savedMonths := make([]string, 0, len(collected.Months))
	var saveErr error
	for _, month := range collected.Months {
		_, replaceDetails := detailMonths[month.Month]
		if err := saveCloudMonthWithRetry(ctx, db, account, month, detailsByMonth[month.Month], replaceDetails); err != nil {
			saveErr = appendCollectionError(saveErr, fmt.Errorf("保存 %s 账单失败: %w", month.Month, err))
			if ctx.Err() != nil {
				break
			}
			continue
		}
		savedMonths = append(savedMonths, month.Month)
	}
	return savedMonths, saveErr
}

func saveCloudMonthWithRetry(ctx context.Context, db *gorm.DB, account *model.BillCloudAccount, month collectedMonth, details []model.BillDetail, replaceDetails bool) error {
	var lastErr error
	for attempt := 1; attempt <= cloudBillSaveMaxAttempts; attempt++ {
		lastErr = saveCloudMonth(ctx, db, account, month, details, replaceDetails)
		if lastErr == nil {
			return nil
		}
		if !isRetryableCloudBillSaveError(lastErr) {
			return lastErr
		}
		if attempt == cloudBillSaveMaxAttempts {
			return fmt.Errorf("重试 %d 次后仍失败: %w", cloudBillSaveMaxAttempts, lastErr)
		}
		delay := cloudBillSaveRetryBaseDelay * time.Duration(1<<(attempt-1))
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	return lastErr
}

func saveCloudMonth(ctx context.Context, db *gorm.DB, account *model.BillCloudAccount, month collectedMonth, details []model.BillDetail, replaceDetails bool) error {
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		summary := model.BillMonthlySummary{
			CloudAccountID: account.ID,
			Operator:       normalizeOperator(account.Operator),
			Month:          month.Month,
			TotalCost:      month.Amount,
			Currency:       month.Currency,
			BillStatus:     billStatus(month.Month),
		}
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "cloud_account_id"}, {Name: "month"}},
			DoUpdates: clause.AssignmentColumns([]string{"operator", "total_cost", "currency", "bill_status", "updated_at"}),
		}).Create(&summary).Error; err != nil {
			return err
		}
		if !replaceDetails {
			return nil
		}
		return syncBillDetails(tx, account, month.Month, details, shouldReplaceBillDetails(month.Month))
	}, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
}

func isRetryableCloudBillSaveError(err error) bool {
	var mysqlErr *mysqlDriver.MySQLError
	return errors.As(err, &mysqlErr) && (mysqlErr.Number == 1205 || mysqlErr.Number == 1213)
}

func collectTencentBillDetails(ctx context.Context, client tencentBillClient, month string) ([]model.BillDetail, error) {
	const pageSize uint64 = 100
	result := make([]model.BillDetail, 0)
	for offset := uint64(0); ; offset += pageSize {
		request := billing.NewDescribeBillDetailRequest()
		request.Month = tencentcommon.StringPtr(month)
		request.Offset = tencentcommon.Uint64Ptr(offset)
		request.Limit = tencentcommon.Uint64Ptr(pageSize)
		request.NeedRecordNum = tencentcommon.Int64Ptr(1)
		response, err := client.DescribeBillDetailWithContext(ctx, request)
		if err != nil {
			return nil, err
		}
		if response == nil || response.Response == nil {
			return nil, fmt.Errorf("账单明细响应为空")
		}
		for _, detail := range response.Response.DetailSet {
			result = append(result, convertTencentBillDetail(detail, month)...)
		}
		count := uint64(len(response.Response.DetailSet))
		if count < pageSize || (response.Response.Total != nil && offset+count >= *response.Response.Total) {
			break
		}
	}
	return result, nil
}

func convertTencentBillDetail(detail *billing.BillDetail, month string) []model.BillDetail {
	if detail == nil {
		return nil
	}
	components := detail.ComponentSet
	if len(components) == 0 {
		components = []*billing.BillDetailComponent{nil}
	}
	result := make([]model.BillDetail, 0, len(components))
	for index, component := range components {
		itemName, itemCode, usageUnit := "-", "-", "-"
		usageQuantity, originalAmount, payableAmount := 0.0, 0.0, 0.0
		componentCode := ""
		if component != nil {
			itemName = firstNonBlank(stringValue(component.ItemCodeName), stringValue(component.ComponentCodeName), "-")
			itemCode = firstNonBlank(stringValue(component.ItemCode), stringValue(component.ComponentCode), "-")
			componentCode = stringValue(component.ComponentCode)
			usageQuantity = cloudAmountOrZero(stringValue(component.UsedAmount))
			usageUnit = firstNonBlank(stringValue(component.UsedAmountUnit), "-")
			originalAmount = cloudAmountOrZero(stringValue(component.Cost))
			payableAmount = cloudAmountOrZero(stringValue(component.RealCost))
		}
		resourceID := firstNonBlank(stringValue(detail.ResourceId), "-")
		resourceName := firstNonBlank(stringValue(detail.ResourceName), resourceID, "-")
		externalID := firstNonBlank(stringValue(detail.Id), stringValue(detail.BillId), stringValue(detail.OrderId))
		recordID := stableRecordID("tencentcloud", month, externalID, resourceID, componentCode, itemCode, fmt.Sprint(index))
		result = append(result, model.BillDetail{
			Operator: "tencentcloud", Month: month, RecordID: recordID,
			ResourceID: resourceID, ResourceName: resourceName,
			ProductCode:     stringValue(detail.BusinessCode),
			ProductName:     firstNonBlank(stringValue(detail.BusinessCodeName), stringValue(detail.BusinessCode), "未分类"),
			SubProductName:  firstNonBlank(stringValue(detail.ProductCodeName), stringValue(detail.ProductCode), "-"),
			BillingItemCode: itemCode, BillingItemName: itemName,
			UsageQuantity: usageQuantity, UsageUnit: usageUnit,
			OriginalAmount: originalAmount, DiscountAmount: positiveDifference(originalAmount, payableAmount), PayableAmount: payableAmount,
			Currency: "CNY", ChargeType: stringValue(detail.PayModeName), TransactionType: stringValue(detail.ActionTypeName),
			Region: firstNonBlank(stringValue(detail.RegionName), stringValue(detail.RegionId), "-"),
			Zone:   firstNonBlank(stringValue(detail.ZoneName), "-"), ProjectName: firstNonBlank(stringValue(detail.ProjectName), "-"),
			OrderID: stringValue(detail.OrderId), TransactionID: stringValue(detail.BillId),
			DeductedAt: parseCloudTime(stringValue(detail.PayTime)), BillingStartAt: parseCloudTime(stringValue(detail.FeeBeginTime)), BillingEndAt: parseCloudTime(stringValue(detail.FeeEndTime)),
		})
	}
	return result
}

func collectAliyunBillDetails(ctx context.Context, client *bssopenapi.Client, limiter *cloudAPILimiter, month string) ([]model.BillDetail, error) {
	const pageSize = 300
	result := make([]model.BillDetail, 0)
	for page := 1; ; page++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		request := bssopenapi.CreateQueryInstanceBillRequest()
		request.BillingCycle = month
		request.PageNum = requests.NewInteger(page)
		request.PageSize = requests.NewInteger(pageSize)
		request.IsBillingItem = requests.NewBoolean(true)
		request.IsHideZeroCharge = requests.NewBoolean(false)
		response, err := callCloudAPI(ctx, limiter, func() (*bssopenapi.QueryInstanceBillResponse, error) {
			result, callErr := client.QueryInstanceBill(request)
			if callErr == nil && result != nil && !result.Success {
				callErr = fmt.Errorf("%s: %s", result.Code, result.Message)
			}
			return result, callErr
		})
		if err != nil {
			return nil, err
		}
		if !response.Success {
			return nil, fmt.Errorf("%s", response.Message)
		}
		for _, item := range response.Data.Items.Item {
			result = append(result, convertAliyunBillDetail(item, month))
		}
		if len(response.Data.Items.Item) < pageSize || page*pageSize >= response.Data.TotalCount {
			break
		}
	}
	return result, nil
}

func convertAliyunBillDetail(item bssopenapi.Item, month string) model.BillDetail {
	resourceID := firstNonBlank(item.ResourceInstanceId, item.InstanceID, item.InstanceId, "-")
	resourceName := firstNonBlank(item.CommodityName, item.ProductDetail, resourceID, "-")
	recordID := stableRecordID("aliyun", month, item.RecordID, item.PaymentTransactionID, resourceID, item.BillingItemCode, item.BillingDate)
	discountAmount := item.PretaxGrossAmount - item.PretaxAmount
	if discountAmount < 0 {
		discountAmount = 0
	}
	currency := normalizedCurrency(firstNonBlank(item.Currency, item.PaymentCurrency, "CNY"))
	usageText := firstNonBlank(item.Usage, strconv.FormatFloat(item.TotalQuantity, 'f', 8, 64))
	return model.BillDetail{
		Operator: "aliyun", Month: month, RecordID: recordID,
		ResourceID: resourceID, ResourceName: resourceName, ProductCode: item.ProductCode,
		ProductName:     firstNonBlank(item.ProductName, item.ProductCode, "未分类"),
		SubProductName:  firstNonBlank(item.ProductType, item.ProductDetail, "-"),
		BillingItemName: firstNonBlank(item.BillingItem, item.ItemName, "-"),
		BillingItemCode: firstNonBlank(item.BillingItemCode, item.Item, "-"),
		UsageQuantity:   cloudAmountOrZero(usageText), UsageUnit: firstNonBlank(item.UsageUnit, "-"),
		OriginalAmount: item.PretaxGrossAmount, DiscountAmount: discountAmount, PayableAmount: item.PretaxAmount,
		Currency: currency, ChargeType: firstNonBlank(item.SubscriptionType, item.BillingType, "-"),
		TransactionType: firstNonBlank(item.BizType, item.StatusName, "-"),
		Region:          firstNonBlank(item.Region, item.RegionNo, "-"), Zone: firstNonBlank(item.ZoneName, item.Zone, "-"),
		ProjectName: firstNonBlank(item.ResourceGroup, item.CostUnit, "-"),
		OrderID:     item.SubOrderId, TransactionID: item.PaymentTransactionID,
		DeductedAt:     parseCloudTime(firstNonBlank(item.PaymentTime, item.BillingDate)),
		BillingStartAt: parseCloudTime(firstNonBlank(item.UsageStartTime, item.StartTime)),
		BillingEndAt:   parseCloudTime(firstNonBlank(item.UsageEndTime, item.EndTime)),
	}
}

func syncBillDetails(tx *gorm.DB, account *model.BillCloudAccount, month string, details []model.BillDetail, replaceExisting bool) error {
	if replaceExisting {
		if err := tx.Where("cloud_account_id = ? AND month = ?", account.ID, month).Delete(&model.BillDetail{}).Error; err != nil {
			return err
		}
	}
	if len(details) == 0 {
		return nil
	}
	operator := normalizeOperator(account.Operator)
	for index := range details {
		details[index].CloudAccountID = account.ID
		details[index].Operator = operator
		details[index].Currency = normalizedCurrency(details[index].Currency)
	}
	bulkTx := tx.Session(&gorm.Session{Logger: tx.Logger.LogMode(logger.Silent)})
	return bulkTx.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "cloud_account_id"}, {Name: "month"}, {Name: "record_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"operator", "month", "resource_id", "resource_name", "product_code", "product_name", "sub_product_name",
			"billing_item_code", "billing_item_name", "usage_quantity", "usage_unit", "original_amount",
			"discount_amount", "payable_amount", "currency", "charge_type", "transaction_type", "region", "zone",
			"project_name", "order_id", "transaction_id", "deducted_at", "billing_start_at", "billing_end_at", "updated_at",
		}),
	}).CreateInBatches(details, cloudBillDetailBatchSize).Error
}

func shouldReplaceBillDetails(month string) bool {
	return billStatus(month) == "issued"
}

func appendCollectionError(current error, next error) error {
	if current == nil {
		return next
	}
	return fmt.Errorf("%v; %w", current, next)
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func stableRecordID(parts ...string) string {
	text := strings.Join(parts, "|")
	if len(text) <= 190 {
		return text
	}
	return fmt.Sprintf("%x", sha1.Sum([]byte(text)))
}

func positiveDifference(original, payable float64) float64 {
	if original <= payable {
		return 0
	}
	return original - payable
}

func cloudAmountOrZero(value string) float64 {
	amount, err := parseCloudAmount(value)
	if err != nil {
		return 0
	}
	return amount
}

func parseCloudTime(value string) *time.Time {
	value = strings.TrimSpace(value)
	if value == "" || value == "-" {
		return nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02T15:04:05", "2006-01-02", "20060102"} {
		parsed, err := time.ParseInLocation(layout, value, time.Local)
		if err == nil {
			return &parsed
		}
	}
	return nil
}

func normalizeOperator(operator string) string {
	return strings.ToLower(strings.TrimSpace(operator))
}

func operatorForKeyType(keyType int) (string, error) {
	switch keyType {
	case 1:
		return "aliyun", nil
	case 2:
		return "tencentcloud", nil
	case 3:
		return "baidu", nil
	case 4:
		return "huawei", nil
	case 6:
		return "volcengine", nil
	default:
		return "", fmt.Errorf("密钥类型 %d 暂不支持账单采集", keyType)
	}
}

func providerDisplay(operator string) string {
	switch normalizeOperator(operator) {
	case "aliyun":
		return "阿里云"
	case "tencentcloud":
		return "腾讯云"
	case "baidu":
		return "百度云"
	case "huawei":
		return "华为云"
	case "volcengine":
		return "火山引擎"
	default:
		return operator
	}
}

func parseCloudAmount(value string) (float64, error) {
	cleaned := strings.NewReplacer(",", "", "，", "", "¥", "", "￥", "", "CNY", "", "RMB", "", "元", "", " ", "").Replace(strings.TrimSpace(value))
	if cleaned == "" || cleaned == "-" {
		return 0, nil
	}
	return strconv.ParseFloat(cleaned, 64)
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func tencentBalanceFen(response *billing.DescribeAccountBalanceResponseParams) *float64 {
	if response == nil {
		return nil
	}
	if response.RealCreditBalance != nil {
		return response.RealCreditBalance
	}
	if response.RealBalance != nil {
		return response.RealBalance
	}
	return response.CreditBalance
}

func billStatus(month string) string {
	if month == time.Now().Format("2006-01") {
		return "estimating"
	}
	return "issued"
}
