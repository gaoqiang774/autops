package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	baidubilling "github.com/baidubce/bce-sdk-go/services/billing"
	bssmodel "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/bss/v2/model"
	"github.com/shopspring/decimal"
)

type fakeVolcengineClient struct{}

type fakeIncompleteVolcengineClient struct{ fakeVolcengineClient }

func testStringPointer(value string) *string { return &value }

type fakeBaiduClient struct{}

func (fakeBaiduClient) QueryBalance(context.Context) (*baiduBalanceResponse, error) {
	return &baiduBalanceResponse{CashBalance: json.RawMessage(`"100.50"`), CreditBalance: json.RawMessage(`20`)}, nil
}

func (fakeBaiduClient) ListBillDetails(context.Context, string, int, int) (*baidubilling.ResourceChargeItemBillResponse, error) {
	return &baidubilling.ResourceChargeItemBillResponse{
		TotalCount: 1,
		Bills: []baidubilling.ResourceChargeItemBill{{
			BillId: "bill-1", ServiceType: "BCC", ServiceTypeName: "云服务器 BCC", ProductType: "postpay",
			InstanceId: "i-baidu-1", ShortId: "app-server", ChargeItem: "CPU", ChargeItemDesc: "CPU",
			Amount: "2", AmountUnit: "核", OriginPrice: 30, FinancePrice: 25, Region: "华北-北京",
		}},
	}, nil
}

func TestCollectBaiduBillsIncludesBalanceAndDetails(t *testing.T) {
	month := time.Now().AddDate(0, -1, 0).Format("2006-01")
	result := collectBaiduBillsWithClient(t.Context(), fakeBaiduClient{}, []string{month})
	if result.Balance == nil || *result.Balance != 120.5 {
		t.Fatalf("balance = %v", result.Balance)
	}
	if len(result.Months) != 1 || result.Months[0].Amount != 25 {
		t.Fatalf("months = %#v", result.Months)
	}
	if len(result.Details) != 1 || result.Details[0].Operator != "baidu" || result.Details[0].ProductCode != "BCC" {
		t.Fatalf("details = %#v", result.Details)
	}
}

func TestCollectBaiduBillsSkipsCurrentMonth(t *testing.T) {
	result := collectBaiduBillsWithClient(t.Context(), fakeBaiduClient{}, []string{time.Now().Format("2006-01")})
	if len(result.Months) != 0 || len(result.Details) != 0 || result.PartialErr != nil {
		t.Fatalf("current month should be skipped: %#v", result)
	}
}

func (fakeVolcengineClient) QueryBalance(context.Context) (*volcengineBalance, error) {
	return &volcengineBalance{AvailableBalance: "1,234.50"}, nil
}

func (fakeVolcengineClient) ListBillOverview(context.Context, string, int, int) (*volcengineBillOverviewPage, error) {
	return &volcengineBillOverviewPage{
		List:  []*volcengineBillOverviewItem{{PayableAmount: "18.75", Currency: "CNY"}},
		Total: 1,
	}, nil
}

func (fakeVolcengineClient) ListBillDetails(context.Context, string, int, int) (*volcengineBillDetailPage, error) {
	return &volcengineBillDetailPage{
		List: []*volcengineBillDetailItem{{
			BillDetailID: "detail-1", Product: "ECS", ProductZh: "云服务器", InstanceNo: "i-1",
			InstanceName: "app-server", ElementCode: "cpu", Element: "计算规格", Count: "2", Unit: "台",
			OriginalBillAmount: "20", PayableAmount: "18.75", Currency: "CNY", BillingMode: "按量计费",
		}},
		Total: 1,
	}, nil
}

func (fakeIncompleteVolcengineClient) ListBillDetails(context.Context, string, int, int) (*volcengineBillDetailPage, error) {
	return &volcengineBillDetailPage{
		List:  []*volcengineBillDetailItem{{BillDetailID: "free-package", PayableAmount: "0", Currency: "CNY"}},
		Total: 1,
	}, nil
}

func TestCollectVolcengineBillsIncludesDetails(t *testing.T) {
	result := collectVolcengineBillsWithClient(t.Context(), fakeVolcengineClient{}, []string{"2026-07"})
	if result.Balance == nil || *result.Balance != 1234.5 {
		t.Fatalf("balance = %v", result.Balance)
	}
	if len(result.Months) != 1 || result.Months[0].Amount != 18.75 {
		t.Fatalf("months = %#v", result.Months)
	}
	if len(result.Details) != 1 || result.Details[0].ProductName != "云服务器" || result.Details[0].PayableAmount != 18.75 {
		t.Fatalf("details = %#v", result.Details)
	}
	if len(result.DetailMonths) != 1 || result.PartialErr != nil {
		t.Fatalf("detail months/error = %#v / %v", result.DetailMonths, result.PartialErr)
	}
}

func TestCollectVolcengineBillsMarksIncompleteCurrentMonthDetails(t *testing.T) {
	month := time.Now().Format("2006-01")
	result := collectVolcengineBillsWithClient(t.Context(), fakeIncompleteVolcengineClient{}, []string{month})
	if len(result.Details) != 1 || len(result.DetailMonths) != 1 {
		t.Fatalf("details = %#v, detail months = %#v", result.Details, result.DetailMonths)
	}
	if result.PartialErr == nil || !strings.Contains(result.PartialErr.Error(), "当前月明细尚未完整生成") {
		t.Fatalf("partial error = %v", result.PartialErr)
	}
}

func TestCollectVolcengineBillsDoesNotReplaceIncompleteIssuedMonth(t *testing.T) {
	month := time.Now().AddDate(0, -1, 0).Format("2006-01")
	result := collectVolcengineBillsWithClient(t.Context(), fakeIncompleteVolcengineClient{}, []string{month})
	if len(result.Months) != 1 {
		t.Fatalf("months = %#v", result.Months)
	}
	if len(result.Details) != 0 || len(result.DetailMonths) != 0 {
		t.Fatalf("incomplete issued details must not be saved: %#v / %#v", result.Details, result.DetailMonths)
	}
	if result.PartialErr == nil || !strings.Contains(result.PartialErr.Error(), "历史月明细不完整") {
		t.Fatalf("partial error = %v", result.PartialErr)
	}
}

func TestConvertHuaweiBillDetail(t *testing.T) {
	chargingMode := int32(3)
	billType := int32(5)
	item := bssmodel.NvlCostAnalysedBillDetail{
		ServiceTypeCode:  testStringPointer("hws.service.type.ec2"),
		ServiceTypeName:  testStringPointer("弹性云服务器"),
		ResourceTypeCode: testStringPointer("hws.resource.type.vm"),
		ResourceTypeName: testStringPointer("云主机"),
		ResourceId:       testStringPointer("ecs-1"), ResourceName: testStringPointer("app-server"),
		RegionName: testStringPointer("华南-广州"), ChargingMode: &chargingMode, BillType: &billType,
	}
	detail := convertHuaweiBillDetail(item, "2026-07", "CNY", 0)
	if detail.Operator != "huawei" || detail.ProductName != "弹性云服务器" || detail.ResourceID != "ecs-1" {
		t.Fatalf("detail = %#v", detail)
	}
	if detail.ChargeType != "按需计费" || detail.TransactionType != "使用" {
		t.Fatalf("billing labels = %q / %q", detail.ChargeType, detail.TransactionType)
	}
}

type fakeHuaweiFallbackClient struct{}

func (fakeHuaweiFallbackClient) ShowCustomerAccountBalances(*bssmodel.ShowCustomerAccountBalancesRequest) (*bssmodel.ShowCustomerAccountBalancesResponse, error) {
	return &bssmodel.ShowCustomerAccountBalancesResponse{AccountBalances: &[]bssmodel.AccountBalanceV3{}}, nil
}

func (fakeHuaweiFallbackClient) ShowCustomerMonthlySum(*bssmodel.ShowCustomerMonthlySumRequest) (*bssmodel.ShowCustomerMonthlySumResponse, error) {
	return &bssmodel.ShowCustomerMonthlySumResponse{}, nil
}

func (fakeHuaweiFallbackClient) ListCustomerBillsMonthlyBreakDown(*bssmodel.ListCustomerBillsMonthlyBreakDownRequest) (*bssmodel.ListCustomerBillsMonthlyBreakDownResponse, error) {
	amount1 := decimal.NewFromFloat(12.5)
	amount2 := decimal.NewFromFloat(-2.5)
	rows := []bssmodel.NvlCostAnalysedBillDetail{{ConsumeAmount: &amount1}, {ConsumeAmount: &amount2}}
	total := int32(len(rows))
	return &bssmodel.ListCustomerBillsMonthlyBreakDownResponse{Details: &rows, TotalCount: &total}, nil
}

func TestHuaweiCollectionFallsBackToDetailTotal(t *testing.T) {
	result := collectHuaweiBillsWithClient(t.Context(), fakeHuaweiFallbackClient{}, []string{"2025-11"})
	if len(result.Months) != 1 || result.Months[0].Amount != 10 {
		t.Fatalf("months = %#v", result.Months)
	}
	if len(result.Details) != 2 || len(result.DetailMonths) != 1 {
		t.Fatalf("details = %#v / %#v", result.Details, result.DetailMonths)
	}
	if result.PartialErr != nil {
		t.Fatalf("partial error = %v", result.PartialErr)
	}
}

func TestHuaweiMonthlyAmountFallsBackToBillSums(t *testing.T) {
	amount1 := decimal.NewFromFloat(20.5)
	amount2 := decimal.NewFromFloat(-1)
	rows := []bssmodel.BillSumRecordInfoV2{{ConsumeAmount: &amount1}, {ConsumeAmount: &amount2}}
	total := int32(len(rows))
	amount, available, err := huaweiMonthlyAmount(&bssmodel.ShowCustomerMonthlySumResponse{
		BillSums:   &rows,
		TotalCount: &total,
	})
	if err != nil || !available || amount != 19.5 {
		t.Fatalf("amount/available/error = %v / %v / %v", amount, available, err)
	}
}
