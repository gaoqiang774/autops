package service

import (
	"errors"
	"sort"
	"strings"
	"time"

	configservice "dodevops-api/api/configcenter/service"
	"dodevops-api/api/costcenter/model"

	"gorm.io/gorm"
)

type CostComparison struct {
	CurrentMonth  string   `json:"currentMonth"`
	CurrentCost   float64  `json:"currentCost"`
	PreviousMonth string   `json:"previousMonth"`
	PreviousCost  float64  `json:"previousCost"`
	LastYearMonth string   `json:"lastYearMonth"`
	LastYearCost  float64  `json:"lastYearCost"`
	MoMRate       *float64 `json:"momRate"`
	YoYRate       *float64 `json:"yoyRate"`
}

type ProviderOverview struct {
	ProviderCode      string                 `json:"providerCode"`
	ProviderName      string                 `json:"providerName"`
	AccountCount      int                    `json:"accountCount"`
	AccountNames      []string               `json:"accountNames"`
	AvailableBalance  float64                `json:"availableBalance"`
	CurrentMonthCost  float64                `json:"currentMonthCost"`
	PreviousMonthCost float64                `json:"previousMonthCost"`
	LastYearCost      float64                `json:"lastYearCost"`
	MoMRate           *float64               `json:"momRate"`
	YoYRate           *float64               `json:"yoyRate"`
	Currency          string                 `json:"currency"`
	BalanceTotals     []BalanceCurrencyTotal `json:"balanceTotals"`
	LowBalanceCount   int                    `json:"lowBalanceCount"`
	BalanceErrorCount int                    `json:"balanceErrorCount"`
	SyncStatus        string                 `json:"syncStatus"`
	LastSyncTime      string                 `json:"lastSyncTime"`
	LastSyncError     string                 `json:"lastSyncError"`
}

type BillingOverview struct {
	Month             string                 `json:"month"`
	PreviousMonth     string                 `json:"previousMonth"`
	LastYearMonth     string                 `json:"lastYearMonth"`
	TotalCost         float64                `json:"totalCost"`
	PreviousTotalCost float64                `json:"previousTotalCost"`
	LastYearTotalCost float64                `json:"lastYearTotalCost"`
	MoMRate           *float64               `json:"momRate"`
	YoYRate           *float64               `json:"yoyRate"`
	Currency          string                 `json:"currency"`
	BalanceTotals     []BalanceCurrencyTotal `json:"balanceTotals"`
	AccountCount      int                    `json:"accountCount"`
	LowBalanceCount   int                    `json:"lowBalanceCount"`
	BalanceErrorCount int                    `json:"balanceErrorCount"`
	Providers         []ProviderOverview     `json:"providers"`
}

type TrendPoint struct {
	Month  string  `json:"month"`
	Amount float64 `json:"amount"`
}

type ProductCost struct {
	ProductCode    string   `json:"productCode"`
	ProductName    string   `json:"productName"`
	Amount         float64  `json:"amount"`
	PreviousAmount float64  `json:"previousAmount"`
	MoMRate        *float64 `json:"momRate"`
	Percent        float64  `json:"percent"`
}

type ProviderAccount struct {
	ID               uint    `json:"id"`
	AccountName      string  `json:"accountName"`
	AccountID        string  `json:"accountId"`
	KeyManageID      uint    `json:"keyManageId"`
	KeyName          string  `json:"keyName"`
	AvailableBalance float64 `json:"availableBalance"`
	SyncStatus       string  `json:"syncStatus"`
	LastSyncTime     string  `json:"lastSyncTime"`
	BalanceStatus    string  `json:"balanceStatus"`
	LastBalanceTime  string  `json:"lastBalanceTime"`
	LastBalanceError string  `json:"lastBalanceError"`
}

type ProviderBillingDetail struct {
	ProviderCode string            `json:"providerCode"`
	ProviderName string            `json:"providerName"`
	Currency     string            `json:"currency"`
	Balance      float64           `json:"balance"`
	Comparison   CostComparison    `json:"comparison"`
	Trend        []TrendPoint      `json:"trend"`
	Products     []ProductCost     `json:"products"`
	Accounts     []ProviderAccount `json:"accounts"`
}

type CloudBillDashboardService struct {
	db   *gorm.DB
	keys *configservice.KeyManageService
}

func NewCloudBillDashboardService(db *gorm.DB) *CloudBillDashboardService {
	return &CloudBillDashboardService{db: db, keys: configservice.NewKeyManageService()}
}

func (s *CloudBillDashboardService) Overview(month string) (*BillingOverview, error) {
	month, err := normalizeMonth(month)
	if err != nil {
		return nil, err
	}
	previous := shiftMonth(month, -1)
	lastYear := shiftMonth(month, -12)
	var sources []model.BillCloudAccount
	if err := s.db.Where("enabled = ?", 1).Order("operator ASC, id ASC").Find(&sources).Error; err != nil {
		return nil, err
	}
	costs, err := s.costsByOperator([]string{month, previous, lastYear}, "", 0)
	if err != nil {
		return nil, err
	}
	providers := map[string]*ProviderOverview{}
	providerBalances := map[string]map[string]float64{}
	totalBalances := map[string]float64{}
	for _, source := range sources {
		code := normalizeOperator(source.Operator)
		item := providers[code]
		if item == nil {
			item = &ProviderOverview{ProviderCode: code, ProviderName: providerDisplay(code), Currency: normalizedCurrency(source.Currency), AccountNames: []string{}, BalanceTotals: []BalanceCurrencyTotal{}, SyncStatus: source.SyncStatus}
			providers[code] = item
			providerBalances[code] = map[string]float64{}
		}
		item.AccountCount++
		accountName := firstNonBlank(source.AccountName, source.AccountID)
		item.AccountNames = append(item.AccountNames, accountName)
		item.AvailableBalance += source.AvailableBalance
		currency := normalizedCurrency(source.Currency)
		providerBalances[code][currency] += source.AvailableBalance
		totalBalances[currency] += source.AvailableBalance
		item.CurrentMonthCost = costs[code][month]
		item.PreviousMonthCost = costs[code][previous]
		item.LastYearCost = costs[code][lastYear]
		item.MoMRate = changeRate(item.CurrentMonthCost, item.PreviousMonthCost)
		item.YoYRate = changeRate(item.CurrentMonthCost, item.LastYearCost)
		if source.BalanceStatus == model.BalanceStatusLow {
			item.LowBalanceCount++
		}
		if source.BalanceStatus == model.BalanceStatusError {
			item.BalanceErrorCount++
		}
		item.SyncStatus = mergeSyncStatus(item.SyncStatus, source.SyncStatus)
		if source.LastSyncError != "" {
			item.LastSyncError = source.LastSyncError
		}
		if source.LastSyncTime != nil {
			formatted := source.LastSyncTime.Format("2006-01-02 15:04:05")
			if formatted > item.LastSyncTime {
				item.LastSyncTime = formatted
			}
		}
		if item.Currency == "" {
			item.Currency = "CNY"
		}
	}
	list := make([]ProviderOverview, 0, len(providers))
	for _, item := range providers {
		for currency, balance := range providerBalances[item.ProviderCode] {
			item.BalanceTotals = append(item.BalanceTotals, BalanceCurrencyTotal{Currency: currency, Balance: balance})
		}
		sort.Slice(item.BalanceTotals, func(i, j int) bool { return item.BalanceTotals[i].Currency < item.BalanceTotals[j].Currency })
		list = append(list, *item)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].CurrentMonthCost > list[j].CurrentMonthCost })
	overview := &BillingOverview{Month: month, PreviousMonth: previous, LastYearMonth: lastYear, Currency: "CNY", Providers: list, BalanceTotals: []BalanceCurrencyTotal{}}
	for _, item := range list {
		overview.TotalCost += item.CurrentMonthCost
		overview.PreviousTotalCost += item.PreviousMonthCost
		overview.LastYearTotalCost += item.LastYearCost
		overview.AccountCount += item.AccountCount
		overview.LowBalanceCount += item.LowBalanceCount
		overview.BalanceErrorCount += item.BalanceErrorCount
	}
	for currency, balance := range totalBalances {
		overview.BalanceTotals = append(overview.BalanceTotals, BalanceCurrencyTotal{Currency: currency, Balance: balance})
	}
	sort.Slice(overview.BalanceTotals, func(i, j int) bool { return overview.BalanceTotals[i].Currency < overview.BalanceTotals[j].Currency })
	overview.MoMRate = changeRate(overview.TotalCost, overview.PreviousTotalCost)
	overview.YoYRate = changeRate(overview.TotalCost, overview.LastYearTotalCost)
	return overview, nil
}

func (s *CloudBillDashboardService) ProviderDetail(operator, month string, cloudAccountID uint) (*ProviderBillingDetail, error) {
	operator = normalizeOperator(operator)
	if operator == "" {
		return nil, errors.New("云厂商不能为空")
	}
	month, err := normalizeMonth(month)
	if err != nil {
		return nil, err
	}
	var sources []model.BillCloudAccount
	sourceQuery := s.db.Where("operator = ? AND enabled = ?", operator, 1)
	if cloudAccountID > 0 {
		sourceQuery = sourceQuery.Where("id = ?", cloudAccountID)
	}
	if err := sourceQuery.Order("id ASC").Find(&sources).Error; err != nil {
		return nil, err
	}
	if len(sources) == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	months := lastMonthsEndingAt(month, 12)
	previous := shiftMonth(month, -1)
	lastYear := shiftMonth(month, -12)
	queryMonths := append(append([]string{}, months...), lastYear)
	costs, err := s.costsByOperator(queryMonths, operator, cloudAccountID)
	if err != nil {
		return nil, err
	}
	detail := &ProviderBillingDetail{
		ProviderCode: operator, ProviderName: providerDisplay(operator), Currency: "CNY",
		Trend: make([]TrendPoint, 0, len(months)), Products: []ProductCost{}, Accounts: []ProviderAccount{},
	}
	for _, value := range months {
		detail.Trend = append(detail.Trend, TrendPoint{Month: value, Amount: costs[operator][value]})
	}
	detail.Comparison = CostComparison{
		CurrentMonth: month, CurrentCost: costs[operator][month],
		PreviousMonth: previous, PreviousCost: costs[operator][previous],
		LastYearMonth: lastYear, LastYearCost: costs[operator][lastYear],
	}
	detail.Comparison.MoMRate = changeRate(detail.Comparison.CurrentCost, detail.Comparison.PreviousCost)
	detail.Comparison.YoYRate = changeRate(detail.Comparison.CurrentCost, detail.Comparison.LastYearCost)
	for _, source := range sources {
		keyName := ""
		if key, err := s.keys.GetByID(source.KeyManageID); err == nil {
			keyName = key.KeyName
		}
		lastSync := ""
		if source.LastSyncTime != nil {
			lastSync = source.LastSyncTime.Format("2006-01-02 15:04:05")
		}
		detail.Balance += source.AvailableBalance
		if source.Currency != "" {
			detail.Currency = source.Currency
		}
		detail.Accounts = append(detail.Accounts, ProviderAccount{
			ID: source.ID, AccountName: firstNonBlank(source.AccountName, source.AccountID), AccountID: source.AccountID,
			KeyManageID: source.KeyManageID, KeyName: keyName, AvailableBalance: source.AvailableBalance,
			SyncStatus: source.SyncStatus, LastSyncTime: lastSync, BalanceStatus: source.BalanceStatus,
			LastBalanceTime: formatTimePointer(source.LastBalanceTime), LastBalanceError: source.LastBalanceError,
		})
	}
	detail.Products, err = s.productCosts(operator, month, cloudAccountID)
	return detail, err
}

func (s *CloudBillDashboardService) costsByOperator(months []string, operator string, cloudAccountID uint) (map[string]map[string]float64, error) {
	type row struct {
		Operator string
		Month    string
		Amount   float64
	}
	var rows []row
	query := s.db.Model(&model.BillMonthlySummary{}).
		Select("operator, month, SUM(total_cost) AS amount").Where("month IN ?", months)
	if operator != "" {
		query = query.Where("operator = ?", operator)
	}
	if cloudAccountID > 0 {
		query = query.Where("cloud_account_id = ?", cloudAccountID)
	}
	if err := query.Group("operator, month").Scan(&rows).Error; err != nil {
		return nil, err
	}
	result := map[string]map[string]float64{}
	for _, row := range rows {
		code := normalizeOperator(row.Operator)
		if result[code] == nil {
			result[code] = map[string]float64{}
		}
		result[code][row.Month] += row.Amount
	}
	if operator != "" && result[operator] == nil {
		result[operator] = map[string]float64{}
	}
	return result, nil
}

func (s *CloudBillDashboardService) productCosts(operator, month string, cloudAccountID uint) ([]ProductCost, error) {
	type row struct {
		ProductCode string
		ProductName string
		Month       string
		Amount      float64
	}
	var rows []row
	previous := shiftMonth(month, -1)
	query := s.db.Model(&model.BillDetail{}).
		Select("product_code, product_name, month, SUM(payable_amount) AS amount").
		Where("operator = ? AND month IN ?", operator, []string{month, previous})
	if cloudAccountID > 0 {
		query = query.Where("cloud_account_id = ?", cloudAccountID)
	}
	err := query.Group("product_code, product_name, month").Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	type productValue struct {
		code, name        string
		current, previous float64
	}
	values := map[string]*productValue{}
	total := 0.0
	for _, row := range rows {
		key := row.ProductCode + "\x00" + row.ProductName
		value := values[key]
		if value == nil {
			value = &productValue{code: row.ProductCode, name: row.ProductName}
			values[key] = value
		}
		if row.Month == month {
			value.current += row.Amount
			total += row.Amount
		} else {
			value.previous += row.Amount
		}
	}
	result := make([]ProductCost, 0, len(values))
	for _, value := range values {
		percent := 0.0
		if total != 0 {
			percent = value.current / total * 100
		}
		result = append(result, ProductCost{ProductCode: value.code, ProductName: value.name, Amount: value.current, PreviousAmount: value.previous, MoMRate: changeRate(value.current, value.previous), Percent: percent})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Amount > result[j].Amount })
	return result, nil
}

func normalizeMonth(value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return time.Now().Format("2006-01"), nil
	}
	parsed, err := time.Parse("2006-01", value)
	if err != nil {
		return "", errors.New("月份格式应为 YYYY-MM")
	}
	return parsed.Format("2006-01"), nil
}

func shiftMonth(month string, delta int) string {
	parsed, _ := time.Parse("2006-01", month)
	return parsed.AddDate(0, delta, 0).Format("2006-01")
}

func lastMonthsEndingAt(month string, count int) []string {
	result := make([]string, 0, count)
	for index := count - 1; index >= 0; index-- {
		result = append(result, shiftMonth(month, -index))
	}
	return result
}

func changeRate(current, baseline float64) *float64 {
	if baseline == 0 {
		return nil
	}
	value := (current - baseline) / baseline * 100
	return &value
}

func mergeSyncStatus(left, right string) string {
	priority := map[string]int{"failed": 5, "syncing": 4, "pending": 3, "partial_success": 2, "success": 1}
	if priority[right] > priority[left] {
		return right
	}
	return left
}
