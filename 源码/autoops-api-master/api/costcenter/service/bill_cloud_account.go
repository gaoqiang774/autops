package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	configmodel "dodevops-api/api/configcenter/model"
	configservice "dodevops-api/api/configcenter/service"
	"dodevops-api/api/costcenter/dao"
	costmodel "dodevops-api/api/costcenter/model"

	"gorm.io/gorm"
)

type BillCloudAccountService struct {
	dao     *dao.BillCloudAccountDAO
	db      *gorm.DB
	keys    *configservice.KeyManageService
	alerts  *BalanceAlertService
	workers chan struct{}
}

func NewBillCloudAccountService(db *gorm.DB) *BillCloudAccountService {
	return &BillCloudAccountService{
		dao: dao.NewBillCloudAccountDAO(db), db: db,
		keys: configservice.NewKeyManageService(), alerts: NewBalanceAlertService(db),
		workers: make(chan struct{}, 3),
	}
}

type CreateBillReq struct {
	KeyManageID                uint   `json:"keyManageId" binding:"required"`
	AccountName                string `json:"accountName"`
	AccountID                  string `json:"accountId"`
	Enabled                    *int   `json:"enabled"`
	BillSyncEnabled            *int   `json:"billSyncEnabled"`
	BalanceSyncEnabled         *int   `json:"balanceSyncEnabled"`
	BalanceSyncIntervalMinutes int    `json:"balanceSyncIntervalMinutes"`
	SyncCycle                  string `json:"syncCycle"`
	LookbackMonths             int    `json:"lookbackMonths"`
}

type UpdateBillReq struct {
	KeyManageID                uint   `json:"keyManageId"`
	AccountName                string `json:"accountName"`
	AccountID                  string `json:"accountId"`
	Enabled                    *int   `json:"enabled"`
	BillSyncEnabled            *int   `json:"billSyncEnabled"`
	BalanceSyncEnabled         *int   `json:"balanceSyncEnabled"`
	BalanceSyncIntervalMinutes int    `json:"balanceSyncIntervalMinutes"`
	SyncCycle                  string `json:"syncCycle"`
	LookbackMonths             int    `json:"lookbackMonths"`
}

type BillCloudAccountResp struct {
	ID                         uint     `json:"id"`
	Operator                   string   `json:"operator"`
	ProviderName               string   `json:"providerName"`
	AccountName                string   `json:"accountName"`
	AccountID                  string   `json:"accountId"`
	KeyManageID                uint     `json:"keyManageId"`
	KeyName                    string   `json:"keyName"`
	Enabled                    int      `json:"enabled"`
	BillSyncEnabled            int      `json:"billSyncEnabled"`
	BalanceSyncEnabled         int      `json:"balanceSyncEnabled"`
	BalanceSyncIntervalMinutes int      `json:"balanceSyncIntervalMinutes"`
	SyncCycle                  string   `json:"syncCycle"`
	LookbackMonths             int      `json:"lookbackMonths"`
	SyncStatus                 string   `json:"syncStatus"`
	LastSyncTime               string   `json:"lastSyncTime"`
	LastSyncError              string   `json:"lastSyncError"`
	CurrentMonthCost           float64  `json:"currentMonthCost"`
	AvailableBalance           float64  `json:"availableBalance"`
	Currency                   string   `json:"currency"`
	BalanceStatus              string   `json:"balanceStatus"`
	LastBalanceTime            string   `json:"lastBalanceTime"`
	LastBalanceError           string   `json:"lastBalanceError"`
	EstimatedAvailableDays     *float64 `json:"estimatedAvailableDays"`
	BalanceAlertEnabled        int      `json:"balanceAlertEnabled"`
	BalanceAlertThreshold      float64  `json:"balanceAlertThreshold"`
}

func (s *BillCloudAccountService) Create(req *CreateBillReq) (*costmodel.BillCloudAccount, error) {
	key, operator, err := s.validateKey(req.KeyManageID)
	if err != nil {
		return nil, err
	}
	exists, err := s.dao.KeyManageExists(req.KeyManageID, 0)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, errors.New("该密钥已接入账单中心")
	}
	cycle, err := normalizeSyncCycle(req.SyncCycle)
	if err != nil {
		return nil, err
	}
	accountName := strings.TrimSpace(req.AccountName)
	if accountName == "" {
		accountName = strings.TrimSpace(req.AccountID)
	}
	if accountName == "" {
		accountName = key.KeyName
	}
	item := &costmodel.BillCloudAccount{
		Operator: operator, AccountName: accountName, AccountID: strings.TrimSpace(req.AccountID),
		KeyManageID: req.KeyManageID,
		Enabled:     intDefaultOne(req.Enabled), BillSyncEnabled: intDefaultOne(req.BillSyncEnabled),
		BalanceSyncEnabled:         intDefaultOne(req.BalanceSyncEnabled),
		BalanceSyncIntervalMinutes: normalizeBalanceInterval(req.BalanceSyncIntervalMinutes),
		SyncCycle:                  cycle, LookbackMonths: normalizeLookback(req.LookbackMonths),
		SyncStatus: "pending", BalanceStatus: costmodel.BalanceStatusUnknown, Currency: "CNY",
		BalanceAlertOnQueryError: 1, BalanceAlertSeverity: "warning",
		BalanceAlertRepeatMinutes: 1440, BalanceAlertNotifyRecovered: 1,
	}
	if item.Enabled == 0 || item.BalanceSyncEnabled == 0 {
		item.BalanceStatus = costmodel.BalanceStatusDisabled
	}
	if err := s.dao.Create(item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *BillCloudAccountService) Update(id uint, req *UpdateBillReq) error {
	current, err := s.dao.GetByID(id)
	if err != nil {
		return errors.New("云账号不存在")
	}
	updates := map[string]interface{}{}
	if req.KeyManageID > 0 && req.KeyManageID != current.KeyManageID {
		_, operator, err := s.validateKey(req.KeyManageID)
		if err != nil {
			return err
		}
		exists, err := s.dao.KeyManageExists(req.KeyManageID, id)
		if err != nil {
			return err
		}
		if exists {
			return errors.New("该密钥已接入账单中心")
		}
		updates["key_manage_id"] = req.KeyManageID
		updates["operator"] = operator
	}
	if value := strings.TrimSpace(req.AccountName); value != "" {
		updates["account_name"] = value
	}
	if value := strings.TrimSpace(req.AccountID); value != "" {
		updates["account_id"] = value
	}
	if req.Enabled != nil {
		updates["enabled"] = normalizeSwitch(*req.Enabled)
	}
	if req.BillSyncEnabled != nil {
		updates["bill_sync_enabled"] = normalizeSwitch(*req.BillSyncEnabled)
	}
	if req.BalanceSyncEnabled != nil {
		updates["balance_sync_enabled"] = normalizeSwitch(*req.BalanceSyncEnabled)
	}
	if req.BalanceSyncIntervalMinutes > 0 {
		updates["balance_sync_interval_minutes"] = normalizeBalanceInterval(req.BalanceSyncIntervalMinutes)
	}
	if req.SyncCycle != "" {
		cycle, err := normalizeSyncCycle(req.SyncCycle)
		if err != nil {
			return err
		}
		updates["sync_cycle"] = cycle
	}
	if req.LookbackMonths > 0 {
		updates["lookback_months"] = normalizeLookback(req.LookbackMonths)
	}
	enabled := current.Enabled
	if req.Enabled != nil {
		enabled = normalizeSwitch(*req.Enabled)
	}
	balanceEnabled := current.BalanceSyncEnabled
	if req.BalanceSyncEnabled != nil {
		balanceEnabled = normalizeSwitch(*req.BalanceSyncEnabled)
	}
	if enabled == 0 || balanceEnabled == 0 {
		updates["balance_status"] = costmodel.BalanceStatusDisabled
	} else if current.BalanceStatus == costmodel.BalanceStatusDisabled {
		updates["balance_status"] = costmodel.BalanceStatusUnknown
	}
	if len(updates) == 0 {
		return nil
	}
	return s.dao.Update(id, updates)
}

func (s *BillCloudAccountService) Delete(id uint) error {
	if _, err := s.dao.GetByID(id); err != nil {
		return errors.New("云账号不存在")
	}
	return s.dao.Delete(id)
}

func (s *BillCloudAccountService) GetList(operator, syncStatus string, enabled *int, page, pageSize int) ([]BillCloudAccountResp, int64, error) {
	operator = normalizeOperator(operator)
	list, total, err := s.dao.GetList(operator, syncStatus, enabled, page, pageSize)
	if err != nil {
		return nil, 0, err
	}
	keyNames := map[uint]string{}
	if keys, err := s.keys.List(); err == nil {
		for _, key := range keys {
			keyNames[key.ID] = key.KeyName
		}
	}
	result := make([]BillCloudAccountResp, 0, len(list))
	for _, item := range list {
		name := item.AccountName
		if name == "" {
			name = item.AccountID
		}
		result = append(result, BillCloudAccountResp{
			ID: item.ID, Operator: normalizeOperator(item.Operator), ProviderName: providerDisplay(item.Operator),
			AccountName: name, AccountID: item.AccountID, KeyManageID: item.KeyManageID, KeyName: keyNames[item.KeyManageID],
			Enabled: item.Enabled, BillSyncEnabled: item.BillSyncEnabled, BalanceSyncEnabled: item.BalanceSyncEnabled,
			BalanceSyncIntervalMinutes: item.BalanceSyncIntervalMinutes, SyncCycle: item.SyncCycle, LookbackMonths: item.LookbackMonths,
			SyncStatus: item.SyncStatus, LastSyncTime: formatTimePointer(item.LastSyncTime), LastSyncError: item.LastSyncError,
			CurrentMonthCost: item.CurrentMonthCost, AvailableBalance: item.AvailableBalance, Currency: normalizedCurrency(item.Currency),
			BalanceStatus: item.BalanceStatus, LastBalanceTime: formatTimePointer(item.LastBalanceTime), LastBalanceError: item.LastBalanceError,
			EstimatedAvailableDays: s.alerts.EstimatedAvailableDays(item.ID, item.AvailableBalance, time.Now()),
			BalanceAlertEnabled:    item.BalanceAlertEnabled, BalanceAlertThreshold: item.BalanceAlertThreshold,
		})
	}
	return result, total, nil
}

func (s *BillCloudAccountService) ManualSync(id uint) error {
	return s.startBillSync(id, true)
}

func (s *BillCloudAccountService) startBillSync(id uint, forceLookback bool) error {
	claimed, err := s.dao.ClaimSync(id)
	if err != nil {
		return err
	}
	if !claimed {
		return errors.New("云账号已停用、账单采集未开启或正在同步")
	}
	go func() {
		s.workers <- struct{}{}
		defer func() { <-s.workers }()
		s.runBillSync(id, forceLookback)
	}()
	return nil
}

func (s *BillCloudAccountService) RefreshBalance(id uint) error {
	return s.startBalanceRefresh(id)
}

func (s *BillCloudAccountService) startBalanceRefresh(id uint) error {
	claimed, err := s.dao.ClaimBalanceRefresh(id)
	if err != nil {
		return err
	}
	if !claimed {
		return errors.New("云账号已停用、余额监控未开启或正在刷新")
	}
	go func() {
		s.workers <- struct{}{}
		defer func() { <-s.workers }()
		s.runBalanceRefresh(id)
	}()
	return nil
}

func (s *BillCloudAccountService) SyncAll() (int, error) {
	items, err := s.dao.ActiveList()
	if err != nil {
		return 0, err
	}
	accepted := 0
	for _, item := range items {
		if item.BillSyncEnabled != 1 {
			continue
		}
		if err := s.startBillSync(item.ID, false); err == nil {
			accepted++
		}
	}
	return accepted, nil
}

func (s *BillCloudAccountService) RefreshAllBalances() (int, error) {
	items, err := s.dao.ActiveList()
	if err != nil {
		return 0, err
	}
	accepted := 0
	for _, item := range items {
		if item.BalanceSyncEnabled != 1 {
			continue
		}
		if err := s.startBalanceRefresh(item.ID); err == nil {
			accepted++
		}
	}
	return accepted, nil
}

func (s *BillCloudAccountService) runBillSync(id uint, forceLookback bool) {
	item, err := s.dao.GetByID(id)
	if err != nil {
		s.finishBillFailure(id, err)
		return
	}
	keyID, keySecret, err := s.credentials(item)
	if err != nil {
		s.finishBillFailure(id, err)
		return
	}
	count, err := s.dao.SummaryCount(item.ID)
	if err != nil {
		s.finishBillFailure(id, err)
		return
	}
	monthCount := 2
	lookbackMonths := normalizeLookback(item.LookbackMonths)
	needsLookback := forceLookback || count == 0 || item.SyncStatus == "partial_success"
	if !needsLookback && normalizeOperator(item.Operator) == "volcengine" {
		incomplete, checkErr := s.dao.HasIncompleteIssuedDetails(item.ID, recentMonths(lookbackMonths))
		if checkErr != nil {
			s.finishBillFailure(id, checkErr)
			return
		}
		needsLookback = incomplete
	}
	if needsLookback {
		monthCount = lookbackMonths
	}
	months := recentMonths(monthCount)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	collected, err := collectCloudBills(ctx, item.Operator, keyID, keySecret, months)
	keyID, keySecret = "", ""
	if err != nil {
		s.finishBillFailure(id, err)
		return
	}
	savedMonths, saveErr := saveCloudCollection(ctx, s.db, item, collected)
	if saveErr != nil {
		log.Printf("部分账单保存失败: cloudAccount=%d error=%v", item.ID, saveErr)
	}
	savedMonthSet := make(map[string]struct{}, len(savedMonths))
	for _, month := range savedMonths {
		savedMonthSet[month] = struct{}{}
	}
	currentCost := item.CurrentMonthCost
	for _, month := range collected.Months {
		_, saved := savedMonthSet[month.Month]
		if saved && month.Month == time.Now().Format("2006-01") {
			currentCost = month.Amount
		}
	}
	message := ""
	if collected.PartialErr != nil {
		message = collected.PartialErr.Error()
	}
	if saveErr != nil {
		message = appendStatusMessage(message, saveErr.Error())
	}
	hasSuccessfulPart := len(savedMonths) > 0
	if collected.Balance != nil {
		if err := s.saveAndEvaluateBalance(item, *collected.Balance, collected.Currency, time.Now()); err != nil {
			message = appendStatusMessage(message, err.Error())
		} else {
			hasSuccessfulPart = true
		}
	} else if collected.BalanceErr != nil {
		message = appendStatusMessage(message, collected.BalanceErr.Error())
		_ = s.saveBalanceFailure(item, collected.BalanceErr, time.Now())
	}
	status := "success"
	if message != "" {
		status = "failed"
		if hasSuccessfulPart {
			status = "partial_success"
		}
	}
	if err := s.dao.FinishBillSync(item.ID, status, message, currentCost); err != nil {
		log.Printf("保存账单同步状态失败: cloudAccount=%d error=%v", item.ID, err)
	}
}

func (s *BillCloudAccountService) runBalanceRefresh(id uint) {
	item, err := s.dao.GetByID(id)
	if err != nil {
		s.finishBalanceFailure(id, err)
		return
	}
	keyID, keySecret, err := s.credentials(item)
	if err != nil {
		s.finishBalanceFailure(id, err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	result, err := collectCloudBalance(ctx, item.Operator, keyID, keySecret)
	keyID, keySecret = "", ""
	if err != nil {
		s.finishBalanceFailure(id, err)
		return
	}
	if err := s.saveAndEvaluateBalance(item, result.Balance, result.Currency, time.Now()); err != nil {
		s.finishBalanceFailure(id, err)
		return
	}
}

func (s *BillCloudAccountService) saveAndEvaluateBalance(item *costmodel.BillCloudAccount, balance float64, currency string, now time.Time) error {
	currency = normalizedCurrency(currency)
	if err := s.dao.SaveBalance(item.ID, balance, currency, costmodel.BalanceStatusNormal, now); err != nil {
		return fmt.Errorf("保存余额失败: %w", err)
	}
	return s.alerts.EvaluateBalance(item.ID, balance, currency, now)
}

func (s *BillCloudAccountService) saveBalanceFailure(item *costmodel.BillCloudAccount, balanceErr error, now time.Time) error {
	message := errorMessage(balanceErr)
	if err := s.dao.SaveBalanceError(item.ID, message, now); err != nil {
		return err
	}
	return s.alerts.EvaluateQueryError(item.ID, message, now)
}

func (s *BillCloudAccountService) finishBillFailure(cloudAccountID uint, syncErr error) {
	message := errorMessage(syncErr)
	_ = s.dao.FinishBillSync(cloudAccountID, "failed", message, 0)
}

func (s *BillCloudAccountService) finishBalanceFailure(cloudAccountID uint, balanceErr error) {
	message := errorMessage(balanceErr)
	now := time.Now()
	_ = s.dao.SaveBalanceError(cloudAccountID, message, now)
	_ = s.alerts.EvaluateQueryError(cloudAccountID, message, now)
}

func (s *BillCloudAccountService) credentials(item *costmodel.BillCloudAccount) (string, string, error) {
	keyID, keySecret, keyType, err := s.keys.GetDecryptedKeyForCloudAPI(item.KeyManageID)
	if err != nil {
		return "", "", fmt.Errorf("读取密钥失败: %w", err)
	}
	operator, err := operatorForKeyType(keyType)
	if err != nil {
		return "", "", err
	}
	if operator != normalizeOperator(item.Operator) {
		return "", "", errors.New("密钥类型与云账号厂商不一致")
	}
	return keyID, keySecret, nil
}

func (s *BillCloudAccountService) validateKey(id uint) (*configmodel.KeyManage, string, error) {
	key, err := s.keys.GetByID(id)
	if err != nil {
		return nil, "", errors.New("密钥不存在")
	}
	operator, err := operatorForKeyType(key.KeyType)
	if err != nil {
		return nil, "", err
	}
	return key, operator, nil
}

func normalizeSyncCycle(value string) (string, error) {
	if value == "" {
		return "daily", nil
	}
	switch value {
	case "daily", "weekly", "monthly":
		return value, nil
	default:
		return "", errors.New("同步周期仅支持 daily、weekly、monthly")
	}
}

func normalizeLookback(value int) int {
	if value <= 0 {
		return 12
	}
	if value > 12 {
		return 12
	}
	return value
}

func normalizeBalanceInterval(value int) int {
	if value <= 0 {
		return 60
	}
	if value < 10 {
		return 10
	}
	if value > 1440 {
		return 1440
	}
	return value
}

func normalizeSwitch(value int) int {
	if value == 1 {
		return 1
	}
	return 0
}

func intDefaultOne(value *int) int {
	if value == nil {
		return 1
	}
	return normalizeSwitch(*value)
}

func recentMonths(count int) []string {
	months := make([]string, 0, count)
	now := time.Now()
	for index := 0; index < count; index++ {
		months = append(months, now.AddDate(0, -index, 0).Format("2006-01"))
	}
	return months
}

func formatTimePointer(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.Format("2006-01-02 15:04:05")
}

func normalizedCurrency(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	if len(value) != 3 {
		return "CNY"
	}
	return value
}

func errorMessage(err error) string {
	if err == nil {
		return "未知错误"
	}
	return err.Error()
}

func appendStatusMessage(current, next string) string {
	if strings.TrimSpace(current) == "" {
		return next
	}
	if strings.TrimSpace(next) == "" {
		return current
	}
	return current + "; " + next
}

var cloudBillSchedulerOnce sync.Once

func StartCloudBillScheduler(ctx context.Context, service *BillCloudAccountService) {
	cloudBillSchedulerOnce.Do(func() {
		go func() {
			ticker := time.NewTicker(time.Minute)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case now := <-ticker.C:
					items, err := service.dao.ActiveList()
					if err != nil {
						continue
					}
					for _, item := range items {
						if item.BillSyncEnabled == 1 && syncDue(&item, now) {
							_ = service.startBillSync(item.ID, false)
						}
						if item.BalanceSyncEnabled == 1 && balanceDue(&item, now) {
							_ = service.startBalanceRefresh(item.ID)
						}
					}
				}
			}
		}()
	})
}

func syncDue(item *costmodel.BillCloudAccount, now time.Time) bool {
	if item.Enabled != 1 || item.BillSyncEnabled != 1 || item.SyncStatus == "syncing" {
		return false
	}
	if item.LastSyncTime == nil {
		return true
	}
	interval := 24 * time.Hour
	switch item.SyncCycle {
	case "weekly":
		interval = 7 * 24 * time.Hour
	case "monthly":
		interval = 30 * 24 * time.Hour
	}
	return now.Sub(*item.LastSyncTime) >= interval
}

func balanceDue(item *costmodel.BillCloudAccount, now time.Time) bool {
	if item.Enabled != 1 || item.BalanceSyncEnabled != 1 || item.BalanceStatus == costmodel.BalanceStatusRefreshing {
		return false
	}
	if item.LastBalanceTime == nil {
		return true
	}
	return now.Sub(*item.LastBalanceTime) >= time.Duration(normalizeBalanceInterval(item.BalanceSyncIntervalMinutes))*time.Minute
}
