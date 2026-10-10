package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/aliyun/alibaba-cloud-sdk-go/services/bssopenapi"
	mysqlDriver "github.com/go-sql-driver/mysql"
	billing "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/billing/v20180709"
	tencentcommon "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
)

type fakeTencentBillClient struct{}

func (fakeTencentBillClient) DescribeAccountBalanceWithContext(context.Context, *billing.DescribeAccountBalanceRequest) (*billing.DescribeAccountBalanceResponse, error) {
	return &billing.DescribeAccountBalanceResponse{Response: &billing.DescribeAccountBalanceResponseParams{
		RealBalance: tencentcommon.Float64Ptr(10000),
	}}, nil
}

func (fakeTencentBillClient) DescribeBillSummaryWithContext(_ context.Context, request *billing.DescribeBillSummaryRequest) (*billing.DescribeBillSummaryResponse, error) {
	if stringValue(request.Month) == "2026-06" {
		return nil, errors.New("账单未开通")
	}
	return &billing.DescribeBillSummaryResponse{Response: &billing.DescribeBillSummaryResponseParams{
		Ready: tencentcommon.Uint64Ptr(1),
		SummaryDetail: []*billing.SummaryDetail{
			{RealTotalCost: tencentcommon.StringPtr("35.60")},
		},
	}}, nil
}

func (fakeTencentBillClient) DescribeBillDetailWithContext(context.Context, *billing.DescribeBillDetailRequest) (*billing.DescribeBillDetailResponse, error) {
	return &billing.DescribeBillDetailResponse{Response: &billing.DescribeBillDetailResponseParams{
		DetailSet: []*billing.BillDetail{},
		Total:     tencentcommon.Uint64Ptr(0),
	}}, nil
}

func TestConvertAliyunBillDetail(t *testing.T) {
	item := bssopenapi.Item{
		RecordID: "record-1", ResourceInstanceId: "i-1", ProductName: "云服务器 ECS",
		BillingItem: "云盘", BillingItemCode: "disk", PretaxGrossAmount: 120,
		PretaxAmount: 90, Currency: "CNY", Region: "cn-shenzhen", Usage: "20", UsageUnit: "GB",
	}
	detail := convertAliyunBillDetail(item, "2026-07")
	if detail.RecordID == "" || detail.ResourceID != "i-1" || detail.ProductName != "云服务器 ECS" {
		t.Fatalf("unexpected aliyun detail identity: %#v", detail)
	}
	if detail.DiscountAmount != 30 || detail.PayableAmount != 90 || detail.Currency != "CNY" {
		t.Fatalf("unexpected aliyun detail amount: %#v", detail)
	}
}

func TestConvertTencentBillDetailCreatesStableComponentRows(t *testing.T) {
	detail := &billing.BillDetail{
		Id: tencentcommon.StringPtr("detail-1"), ResourceId: tencentcommon.StringPtr("ins-1"),
		ResourceName: tencentcommon.StringPtr("app-server"), BusinessCodeName: tencentcommon.StringPtr("云服务器 CVM"),
		ComponentSet: []*billing.BillDetailComponent{
			{ItemCode: tencentcommon.StringPtr("cpu"), ItemCodeName: tencentcommon.StringPtr("CPU"), Cost: tencentcommon.StringPtr("20"), RealCost: tencentcommon.StringPtr("16")},
			{ItemCode: tencentcommon.StringPtr("mem"), ItemCodeName: tencentcommon.StringPtr("内存"), Cost: tencentcommon.StringPtr("10"), RealCost: tencentcommon.StringPtr("8")},
		},
	}
	rows := convertTencentBillDetail(detail, "2026-07")
	if len(rows) != 2 {
		t.Fatalf("got %d component rows, want 2", len(rows))
	}
	if rows[0].RecordID == rows[1].RecordID {
		t.Fatal("component rows must have different record IDs")
	}
	if rows[0].DiscountAmount != 4 || rows[1].DiscountAmount != 2 {
		t.Fatalf("unexpected discount amounts: %v, %v", rows[0].DiscountAmount, rows[1].DiscountAmount)
	}
}

func TestStableRecordIDBoundsLongValues(t *testing.T) {
	long := make([]byte, 300)
	for index := range long {
		long[index] = 'a'
	}
	got := stableRecordID(string(long), string(long))
	if len(got) != 40 {
		t.Fatalf("hashed record ID length = %d, want 40", len(got))
	}
}

func TestTencentCollectionKeepsSuccessfulMonths(t *testing.T) {
	result := collectTencentBillsWithClient(t.Context(), fakeTencentBillClient{}, []string{"2026-07", "2026-06"})
	if result == nil || len(result.Months) != 1 {
		t.Fatalf("months = %#v", result)
	}
	if result.Months[0].Month != "2026-07" || result.Months[0].Amount != 35.6 {
		t.Fatalf("successful month = %#v", result.Months[0])
	}
	if result.Balance == nil || *result.Balance != 100 {
		t.Fatalf("balance = %v", result.Balance)
	}
	if result.PartialErr == nil || !strings.Contains(result.PartialErr.Error(), "2026-06") {
		t.Fatalf("partial error = %v", result.PartialErr)
	}
}

func TestTencentClientProfileRetriesNetworkFailures(t *testing.T) {
	cp := newTencentBillClientProfile()
	if cp.HttpProfile.ReqTimeout != 90 {
		t.Fatalf("request timeout = %d", cp.HttpProfile.ReqTimeout)
	}
	if cp.NetworkFailureMaxRetries != 2 || !cp.UnsafeRetryOnConnectionFailure {
		t.Fatalf("network retry profile = %#v", cp)
	}
}

func TestIsRetryableCloudBillSaveError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "lock wait timeout", err: &mysqlDriver.MySQLError{Number: 1205}, want: true},
		{name: "wrapped deadlock", err: fmt.Errorf("save month: %w", &mysqlDriver.MySQLError{Number: 1213}), want: true},
		{name: "duplicate key", err: &mysqlDriver.MySQLError{Number: 1062}, want: false},
		{name: "generic error", err: errors.New("database unavailable"), want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := isRetryableCloudBillSaveError(test.err); got != test.want {
				t.Fatalf("isRetryableCloudBillSaveError() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestShouldReplaceBillDetailsOnlyForIssuedMonths(t *testing.T) {
	currentMonth := time.Now().Format("2006-01")
	previousMonth := time.Now().AddDate(0, -1, 0).Format("2006-01")
	if shouldReplaceBillDetails(currentMonth) {
		t.Fatal("current month details must be merged without deleting existing rows")
	}
	if !shouldReplaceBillDetails(previousMonth) {
		t.Fatal("issued month details should be replaced atomically")
	}
}
