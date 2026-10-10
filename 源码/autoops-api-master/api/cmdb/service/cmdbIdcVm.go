package service

import (
	cmdbDao "dodevops-api/api/cmdb/dao"
	"dodevops-api/api/cmdb/model"
	"dodevops-api/common/constant"
	"dodevops-api/common/result"
	"dodevops-api/common/util"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type CmdbIdcVmServiceInterface interface {
	CreateVm(c *gin.Context, dto *model.CreateCmdbIdcVmDto)
	UpdateVm(c *gin.Context, dto *model.UpdateCmdbIdcVmDto)
	DeleteVm(c *gin.Context, id uint)
	GetVmById(c *gin.Context, id uint)
	GetVmList(c *gin.Context, query *model.CmdbIdcVmQuery)
	ImportVmsFromExcel(c *gin.Context, rows []model.ExcelCmdbIdcVmRow)
}

type CmdbIdcVmServiceImpl struct {
	vmDao    cmdbDao.CmdbIdcVmDao
	assetDao cmdbDao.CmdbIdcAssetDao
}

func GetCmdbIdcVmService() CmdbIdcVmServiceInterface {
	return &CmdbIdcVmServiceImpl{
		vmDao:    cmdbDao.NewCmdbIdcVmDao(),
		assetDao: cmdbDao.NewCmdbIdcAssetDao(),
	}
}

func (s *CmdbIdcVmServiceImpl) CreateVm(c *gin.Context, dto *model.CreateCmdbIdcVmDto) {
	vmIP := strings.TrimSpace(dto.VmIP)
	if vmIP == "" {
		result.Failed(c, constant.INVALID_PARAMS, "虚拟机IP不能为空")
		return
	}

	physicalAsset, err := s.resolvePhysicalAsset(dto.PhysicalAssetID, dto.PhysicalIP)
	if err != nil {
		result.Failed(c, constant.INVALID_PARAMS, err.Error())
		return
	}

	if exist, err := s.vmDao.GetVmByPhysicalAssetAndIP(physicalAsset.ID, vmIP, 0); err == nil && exist != nil {
		result.Failed(c, constant.INVALID_PARAMS, "同一物理机下虚拟机IP已存在")
		return
	} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		result.Failed(c, constant.IDC_VM_CREATE_FAILED, err.Error())
		return
	}

	now := util.HTime{Time: time.Now()}
	vm := model.CmdbIdcVm{
		PhysicalAssetID: physicalAsset.ID,
		VmIP:            vmIP,
		Username:        strings.TrimSpace(dto.Username),
		VmName:          strings.TrimSpace(dto.VmName),
		Remark:          strings.TrimSpace(dto.Remark),
		CreateTime:      now,
		UpdateTime:      now,
	}

	if err := s.vmDao.CreateVm(&vm); err != nil {
		result.Failed(c, constant.IDC_VM_CREATE_FAILED, err.Error())
		return
	}

	result.Success(c, s.buildVmVO(vm, physicalAsset.IP))
}

func (s *CmdbIdcVmServiceImpl) UpdateVm(c *gin.Context, dto *model.UpdateCmdbIdcVmDto) {
	existVm, err := s.vmDao.GetVmById(dto.ID)
	if err != nil {
		result.Failed(c, constant.IDC_VM_NOT_FOUND, "虚拟机关联不存在")
		return
	}

	vmIP := strings.TrimSpace(dto.VmIP)
	if vmIP == "" {
		result.Failed(c, constant.INVALID_PARAMS, "虚拟机IP不能为空")
		return
	}

	physicalAssetID := dto.PhysicalAssetID
	physicalIP := dto.PhysicalIP
	if physicalAssetID == 0 && strings.TrimSpace(physicalIP) == "" {
		physicalAssetID = existVm.PhysicalAssetID
	}

	physicalAsset, err := s.resolvePhysicalAsset(physicalAssetID, physicalIP)
	if err != nil {
		result.Failed(c, constant.INVALID_PARAMS, err.Error())
		return
	}

	if sameIPVm, err := s.vmDao.GetVmByPhysicalAssetAndIP(physicalAsset.ID, vmIP, existVm.ID); err == nil && sameIPVm != nil {
		result.Failed(c, constant.INVALID_PARAMS, "同一物理机下虚拟机IP已存在")
		return
	} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		result.Failed(c, constant.IDC_VM_UPDATE_FAILED, err.Error())
		return
	}

	updateData := model.CmdbIdcVm{
		PhysicalAssetID: physicalAsset.ID,
		VmIP:            vmIP,
		Username:        strings.TrimSpace(dto.Username),
		VmName:          strings.TrimSpace(dto.VmName),
		Remark:          strings.TrimSpace(dto.Remark),
		UpdateTime:      util.HTime{Time: time.Now()},
	}

	if err := s.vmDao.UpdateVm(existVm.ID, &updateData); err != nil {
		result.Failed(c, constant.IDC_VM_UPDATE_FAILED, err.Error())
		return
	}

	result.Success(c, true)
}

func (s *CmdbIdcVmServiceImpl) DeleteVm(c *gin.Context, id uint) {
	if _, err := s.vmDao.GetVmById(id); err != nil {
		result.Failed(c, constant.IDC_VM_NOT_FOUND, "虚拟机关联不存在")
		return
	}
	if err := s.vmDao.DeleteVm(id); err != nil {
		result.Failed(c, constant.IDC_VM_DELETE_FAILED, err.Error())
		return
	}
	result.Success(c, true)
}

func (s *CmdbIdcVmServiceImpl) GetVmById(c *gin.Context, id uint) {
	vm, err := s.vmDao.GetVmById(id)
	if err != nil {
		result.Failed(c, constant.IDC_VM_NOT_FOUND, "虚拟机关联不存在")
		return
	}
	result.Success(c, s.buildVmVO(vm, vm.PhysicalAsset.IP))
}

func (s *CmdbIdcVmServiceImpl) GetVmList(c *gin.Context, query *model.CmdbIdcVmQuery) {
	if query.SortBy == "" {
		query.SortBy = "id"
	}
	if query.SortOrder == "" {
		query.SortOrder = "desc"
	}

	list, total, err := s.vmDao.GetVmList(query)
	if err != nil {
		result.Failed(c, constant.IDC_VM_NOT_FOUND, err.Error())
		return
	}

	vos := make([]model.CmdbIdcVmVo, 0, len(list))
	for _, item := range list {
		vos = append(vos, s.buildVmVO(item, item.PhysicalAsset.IP))
	}

	result.SuccessWithPage(c, vos, total, query.Page, query.PageSize)
}

func (s *CmdbIdcVmServiceImpl) ImportVmsFromExcel(c *gin.Context, rows []model.ExcelCmdbIdcVmRow) {
	if len(rows) == 0 {
		result.Failed(c, constant.INVALID_PARAMS, "Excel中没有可导入的数据")
		return
	}

	successCount := 0
	failCount := 0
	failedRows := make([]string, 0)

	for index, row := range rows {
		rowNo := index + 2
		vmIP := strings.TrimSpace(row.VmIP)
		if vmIP == "" {
			failCount++
			failedRows = append(failedRows, fmt.Sprintf("第%d行 虚拟机IP不能为空", rowNo))
			continue
		}

		physicalAsset, err := s.resolvePhysicalAsset(0, row.PhysicalIP)
		if err != nil {
			failCount++
			failedRows = append(failedRows, fmt.Sprintf("第%d行 %s", rowNo, err.Error()))
			continue
		}

		if exist, err := s.vmDao.GetVmByPhysicalAssetAndIP(physicalAsset.ID, vmIP, 0); err == nil && exist != nil {
			failCount++
			failedRows = append(failedRows, fmt.Sprintf("第%d行 同一物理机下虚拟机IP已存在", rowNo))
			continue
		} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			failCount++
			failedRows = append(failedRows, fmt.Sprintf("第%d行 %s", rowNo, err.Error()))
			continue
		}

		now := util.HTime{Time: time.Now()}
		vm := model.CmdbIdcVm{
			PhysicalAssetID: physicalAsset.ID,
			VmIP:            vmIP,
			Username:        strings.TrimSpace(row.Username),
			VmName:          strings.TrimSpace(row.VmName),
			Remark:          strings.TrimSpace(row.Remark),
			CreateTime:      now,
			UpdateTime:      now,
		}
		if err := s.vmDao.CreateVm(&vm); err != nil {
			failCount++
			failedRows = append(failedRows, fmt.Sprintf("第%d行 %s", rowNo, err.Error()))
			continue
		}
		successCount++
	}

	result.Success(c, gin.H{
		"success":    successCount,
		"fail":       failCount,
		"total":      len(rows),
		"failedRows": failedRows,
	})
}

func (s *CmdbIdcVmServiceImpl) resolvePhysicalAsset(physicalAssetID uint, physicalIP string) (model.CmdbIdcAsset, error) {
	trimmedIP := strings.TrimSpace(physicalIP)
	var asset model.CmdbIdcAsset

	if physicalAssetID > 0 {
		assetByID, err := s.assetDao.GetAssetById(physicalAssetID)
		if err != nil {
			return asset, errors.New("物理机资产不存在")
		}
		asset = assetByID
		if trimmedIP != "" && trimmedIP != asset.IP {
			return asset, errors.New("物理机ID和物理机IP不匹配")
		}
		return asset, nil
	}

	if trimmedIP == "" {
		return asset, errors.New("物理机IP不能为空")
	}

	assetByIP, err := s.assetDao.GetAssetByIP(trimmedIP)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return asset, errors.New("物理机资产不存在")
		}
		return asset, err
	}
	return *assetByIP, nil
}

func (s *CmdbIdcVmServiceImpl) buildVmVO(vm model.CmdbIdcVm, physicalIP string) model.CmdbIdcVmVo {
	return model.CmdbIdcVmVo{
		ID:              vm.ID,
		PhysicalAssetID: vm.PhysicalAssetID,
		PhysicalIP:      physicalIP,
		VmIP:            vm.VmIP,
		Username:        vm.Username,
		VmName:          vm.VmName,
		Remark:          vm.Remark,
		CreateTime:      vm.CreateTime,
		UpdateTime:      vm.UpdateTime,
	}
}
