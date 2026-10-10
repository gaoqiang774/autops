package service

import (
	"bytes"
	"errors"
	"fmt"
	"time"

	"dodevops-api/api/costcenter/dao"
	"dodevops-api/api/costcenter/model"

	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

const maxBillDetailExportRows = 100000

type BillDetailQuery struct {
	Month          string
	Operator       string
	CloudAccountID uint
	Keyword        string
	ProductName    string
	ChargeType     string
	Region         string
	Page           int
	PageSize       int
}

type BillDetailService struct {
	dao *dao.BillDetailDAO
}

func NewBillDetailService(db *gorm.DB) *BillDetailService {
	return &BillDetailService{dao: dao.NewBillDetailDAO(db)}
}

func (s *BillDetailService) List(query BillDetailQuery) ([]model.BillDetail, int64, error) {
	return s.dao.List(query.Month, query.Operator, query.CloudAccountID, query.Keyword, query.ProductName, query.ChargeType, query.Region, query.Page, query.PageSize)
}

func (s *BillDetailService) GetByID(id uint) (*model.BillDetail, error) {
	return s.dao.GetByID(id)
}

func (s *BillDetailService) Export(query BillDetailQuery) (*bytes.Buffer, error) {
	rows, total, err := s.dao.ExportRows(query.Month, query.Operator, query.CloudAccountID, query.Keyword, query.ProductName, query.ChargeType, query.Region, maxBillDetailExportRows)
	if err != nil {
		return nil, err
	}
	if total == 0 {
		return nil, errors.New("没有符合条件的账单明细")
	}
	if total > maxBillDetailExportRows {
		return nil, fmt.Errorf("导出记录超过 %d 条，请缩小筛选范围", maxBillDetailExportRows)
	}

	file := excelize.NewFile()
	defer file.Close()
	const sheet = "账单明细"
	file.SetSheetName("Sheet1", sheet)

	headers := []string{
		"云账号ID", "云厂商", "账单月份", "资源ID", "资源名称", "产品类型", "子产品", "计费组件",
		"计费项", "用量", "用量单位", "原价", "优惠金额", "实际费用", "币种", "付费模式", "地域",
		"可用区", "项目", "订单ID", "交易ID", "扣费时间", "开始时间", "结束时间",
	}
	headerStyle, err := file.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "#FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#2468C9"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	if err != nil {
		return nil, err
	}
	for column, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(column+1, 1)
		file.SetCellValue(sheet, cell, header)
		file.SetCellStyle(sheet, cell, cell, headerStyle)
	}

	for index, row := range rows {
		values := []interface{}{
			row.CloudAccountID, row.Operator, row.Month, row.ResourceID, row.ResourceName, row.ProductName,
			row.SubProductName, row.BillingItemName, row.BillingItemCode, row.UsageQuantity, row.UsageUnit,
			row.OriginalAmount, row.DiscountAmount, row.PayableAmount, row.Currency,
			row.ChargeType, row.Region, row.Zone, row.ProjectName, row.OrderID, row.TransactionID, formatExcelTime(row.DeductedAt),
			formatExcelTime(row.BillingStartAt), formatExcelTime(row.BillingEndAt),
		}
		for column, value := range values {
			cell, _ := excelize.CoordinatesToCellName(column+1, index+2)
			file.SetCellValue(sheet, cell, value)
		}
	}

	file.SetColWidth(sheet, "A", "C", 14)
	file.SetColWidth(sheet, "D", "I", 22)
	file.SetColWidth(sheet, "J", "Q", 14)
	file.SetColWidth(sheet, "R", "U", 20)
	file.SetColWidth(sheet, "V", "X", 20)
	if err := file.SetPanes(sheet, &excelize.Panes{Freeze: true, Split: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"}); err != nil {
		return nil, err
	}
	return file.WriteToBuffer()
}

func formatExcelTime(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.Format("2006-01-02 15:04:05")
}
