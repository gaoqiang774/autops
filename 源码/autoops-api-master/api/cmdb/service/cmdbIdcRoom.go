package service

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	cmdbDao "dodevops-api/api/cmdb/dao"
	"dodevops-api/api/cmdb/model"
	"dodevops-api/common/constant"
	"dodevops-api/common/result"
	"dodevops-api/common/util"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type CmdbIdcRoomServiceInterface interface {
	CreateRoom(c *gin.Context, dto *model.CreateCmdbIdcRoomDto)
	UpdateRoom(c *gin.Context, dto *model.UpdateCmdbIdcRoomDto)
	DeleteRoom(c *gin.Context, id uint)
	GetRoomById(c *gin.Context, id uint)
	GetRoomList(c *gin.Context)
	GetRoomLayout(c *gin.Context, roomID uint)
	CreateCabinet(c *gin.Context, dto *model.CreateCmdbIdcCabinetDto)
	UpdateCabinet(c *gin.Context, dto *model.UpdateCmdbIdcCabinetDto)
	DeleteCabinet(c *gin.Context, id uint)
	GetCabinetList(c *gin.Context, roomID uint)
	GetCabinetLayout(c *gin.Context, cabinetID, roomID uint, cabinetName string)
}

type CmdbIdcRoomServiceImpl struct {
	roomDao  cmdbDao.CmdbIdcRoomDao
	assetDao cmdbDao.CmdbIdcAssetDao
}

type roomLayoutCabinetItem struct {
	cabinet   model.CmdbIdcCabinet
	layout    model.CmdbIdcCabinetLayout
	rowNo     int
	cabinetNo int
	matched   bool
}

type idcAssetHardwareState struct {
	Status  string
	Metrics []string
}

type idcHardwareStatusMetric struct {
	Label                string
	Name                 string
	Kind                 string
	Job                  string
	ComponentIndex       string
	RequireInstalledDisk bool
}

var cabinetNamePositionRegexp = regexp.MustCompile(`^\s*0*(\d+)\s*列\s*0*(\d+)\s*(?:柜|架|机柜)`)

func GetCmdbIdcRoomService() CmdbIdcRoomServiceInterface {
	return &CmdbIdcRoomServiceImpl{
		roomDao:  cmdbDao.NewCmdbIdcRoomDao(),
		assetDao: cmdbDao.NewCmdbIdcAssetDao(),
	}
}

func (s *CmdbIdcRoomServiceImpl) CreateRoom(c *gin.Context, dto *model.CreateCmdbIdcRoomDto) {
	name := strings.TrimSpace(dto.Name)
	if name == "" {
		result.Failed(c, constant.INVALID_PARAMS, "机房名称不能为空")
		return
	}
	if s.roomDao.CheckRoomNameExists(name, 0) {
		result.Failed(c, constant.INVALID_PARAMS, "机房名称已存在")
		return
	}

	cabinetNames := deduplicateNames(dto.CabinetNames)
	now := util.HTime{Time: time.Now()}
	room := model.CmdbIdcRoom{
		Name:       name,
		Remark:     strings.TrimSpace(dto.Remark),
		CreateTime: now,
		UpdateTime: now,
	}

	if err := s.roomDao.CreateRoomWithCabinets(&room, cabinetNames); err != nil {
		result.Failed(c, constant.IDC_ROOM_CREATE_FAILED, err.Error())
		return
	}

	result.Success(c, gin.H{
		"id":           room.ID,
		"name":         room.Name,
		"cabinetCount": len(cabinetNames),
	})
}

func (s *CmdbIdcRoomServiceImpl) UpdateRoom(c *gin.Context, dto *model.UpdateCmdbIdcRoomDto) {
	room, err := s.roomDao.GetRoomById(dto.ID)
	if err != nil {
		result.Failed(c, constant.IDC_ROOM_NOT_FOUND, "机房不存在")
		return
	}

	name := strings.TrimSpace(dto.Name)
	if name == "" {
		result.Failed(c, constant.INVALID_PARAMS, "机房名称不能为空")
		return
	}
	if s.roomDao.CheckRoomNameExists(name, room.ID) {
		result.Failed(c, constant.INVALID_PARAMS, "机房名称已存在")
		return
	}

	updateData := model.CmdbIdcRoom{
		Name:       name,
		Remark:     strings.TrimSpace(dto.Remark),
		UpdateTime: util.HTime{Time: time.Now()},
	}
	if err := s.roomDao.UpdateRoom(room.ID, &updateData); err != nil {
		result.Failed(c, constant.IDC_ROOM_UPDATE_FAILED, err.Error())
		return
	}

	result.Success(c, true)
}

func (s *CmdbIdcRoomServiceImpl) DeleteRoom(c *gin.Context, id uint) {
	if _, err := s.roomDao.GetRoomById(id); err != nil {
		result.Failed(c, constant.IDC_ROOM_NOT_FOUND, "机房不存在")
		return
	}

	assetCount, err := s.assetDao.CountAssetsByRoomID(id)
	if err != nil {
		result.Failed(c, constant.IDC_ROOM_DELETE_FAILED, err.Error())
		return
	}
	if assetCount > 0 {
		result.Failed(c, constant.IDC_ROOM_DELETE_FAILED, "机房下存在资产，无法删除")
		return
	}

	if err := s.roomDao.DeleteRoom(id); err != nil {
		result.Failed(c, constant.IDC_ROOM_DELETE_FAILED, err.Error())
		return
	}
	result.Success(c, true)
}

func (s *CmdbIdcRoomServiceImpl) GetRoomById(c *gin.Context, id uint) {
	room, err := s.roomDao.GetRoomById(id)
	if err != nil {
		result.Failed(c, constant.IDC_ROOM_NOT_FOUND, "机房不存在")
		return
	}

	roomCounts, cabinetCounts, err := s.assetDao.GetRoomAndCabinetAssetCount()
	if err == nil {
		enrichRoomAssetStats(&room, roomCounts, cabinetCounts)
	}
	result.Success(c, room)
}

func (s *CmdbIdcRoomServiceImpl) GetRoomList(c *gin.Context) {
	rooms, err := s.roomDao.ListRooms()
	if err != nil {
		result.Failed(c, constant.IDC_ROOM_NOT_FOUND, err.Error())
		return
	}

	roomCounts, cabinetCounts, err := s.assetDao.GetRoomAndCabinetAssetCount()
	if err == nil {
		for i := range rooms {
			enrichRoomAssetStats(&rooms[i], roomCounts, cabinetCounts)
		}
	}

	result.Success(c, rooms)
}

func (s *CmdbIdcRoomServiceImpl) GetRoomLayout(c *gin.Context, roomID uint) {
	room, err := s.roomDao.GetRoomById(roomID)
	if err != nil {
		result.Failed(c, constant.IDC_ROOM_NOT_FOUND, "机房不存在")
		return
	}

	assets, err := s.assetDao.ListAssetsByRoomID(room.ID)
	if err != nil {
		result.Failed(c, constant.IDC_ASSET_NOT_FOUND, err.Error())
		return
	}

	layout := buildRoomLayoutWithHardware(room, assets)
	result.Success(c, layout)
}

func (s *CmdbIdcRoomServiceImpl) CreateCabinet(c *gin.Context, dto *model.CreateCmdbIdcCabinetDto) {
	if _, err := s.roomDao.GetRoomById(dto.RoomID); err != nil {
		result.Failed(c, constant.IDC_ROOM_NOT_FOUND, "机房不存在")
		return
	}

	name := strings.TrimSpace(dto.Name)
	if name == "" {
		result.Failed(c, constant.INVALID_PARAMS, "机柜名称不能为空")
		return
	}
	if s.roomDao.CheckCabinetNameExists(dto.RoomID, name, 0) {
		result.Failed(c, constant.INVALID_PARAMS, "当前机房下机柜名称已存在")
		return
	}

	totalU, err := normalizeCabinetTotalU(dto.TotalU)
	if err != nil {
		result.Failed(c, constant.INVALID_PARAMS, err.Error())
		return
	}

	now := util.HTime{Time: time.Now()}
	cabinet := model.CmdbIdcCabinet{
		RoomID:     dto.RoomID,
		Name:       name,
		TotalU:     totalU,
		Remark:     strings.TrimSpace(dto.Remark),
		CreateTime: now,
		UpdateTime: now,
	}
	if err := s.roomDao.CreateCabinet(&cabinet); err != nil {
		result.Failed(c, constant.IDC_CABINET_CREATE_FAILED, err.Error())
		return
	}

	result.Success(c, cabinet)
}

func (s *CmdbIdcRoomServiceImpl) UpdateCabinet(c *gin.Context, dto *model.UpdateCmdbIdcCabinetDto) {
	if _, err := s.roomDao.GetRoomById(dto.RoomID); err != nil {
		result.Failed(c, constant.IDC_ROOM_NOT_FOUND, "机房不存在")
		return
	}

	cabinet, err := s.roomDao.GetCabinetById(dto.ID)
	if err != nil {
		result.Failed(c, constant.IDC_CABINET_NOT_FOUND, "机柜不存在")
		return
	}

	name := strings.TrimSpace(dto.Name)
	if name == "" {
		result.Failed(c, constant.INVALID_PARAMS, "机柜名称不能为空")
		return
	}
	if s.roomDao.CheckCabinetNameExists(dto.RoomID, name, dto.ID) {
		result.Failed(c, constant.INVALID_PARAMS, "当前机房下机柜名称已存在")
		return
	}

	totalU := cabinet.TotalU
	if dto.TotalU > 0 {
		totalU, err = normalizeCabinetTotalU(dto.TotalU)
		if err != nil {
			result.Failed(c, constant.INVALID_PARAMS, err.Error())
			return
		}
	}

	updateData := model.CmdbIdcCabinet{
		RoomID:     dto.RoomID,
		Name:       name,
		TotalU:     totalU,
		Remark:     strings.TrimSpace(dto.Remark),
		UpdateTime: util.HTime{Time: time.Now()},
	}
	if err := s.roomDao.UpdateCabinet(cabinet.ID, &updateData); err != nil {
		result.Failed(c, constant.IDC_CABINET_UPDATE_FAILED, err.Error())
		return
	}

	result.Success(c, true)
}

func (s *CmdbIdcRoomServiceImpl) DeleteCabinet(c *gin.Context, id uint) {
	if _, err := s.roomDao.GetCabinetById(id); err != nil {
		result.Failed(c, constant.IDC_CABINET_NOT_FOUND, "机柜不存在")
		return
	}

	assetCount, err := s.assetDao.CountAssetsByCabinetID(id)
	if err != nil {
		result.Failed(c, constant.IDC_CABINET_DELETE_FAILED, err.Error())
		return
	}
	if assetCount > 0 {
		result.Failed(c, constant.IDC_CABINET_DELETE_FAILED, "机柜下存在资产，无法删除")
		return
	}

	if err := s.roomDao.DeleteCabinet(id); err != nil {
		result.Failed(c, constant.IDC_CABINET_DELETE_FAILED, err.Error())
		return
	}
	result.Success(c, true)
}

func (s *CmdbIdcRoomServiceImpl) GetCabinetList(c *gin.Context, roomID uint) {
	if roomID > 0 {
		if _, err := s.roomDao.GetRoomById(roomID); err != nil {
			result.Failed(c, constant.IDC_ROOM_NOT_FOUND, "机房不存在")
			return
		}
	}

	cabinets, err := s.roomDao.ListCabinets(roomID)
	if err != nil {
		result.Failed(c, constant.IDC_CABINET_NOT_FOUND, err.Error())
		return
	}

	_, cabinetCounts, err := s.assetDao.GetRoomAndCabinetAssetCount()
	if err == nil {
		for i := range cabinets {
			cabinets[i].AssetCount = cabinetCounts[cabinets[i].ID]
		}
	}
	result.Success(c, cabinets)
}

func (s *CmdbIdcRoomServiceImpl) GetCabinetLayout(c *gin.Context, cabinetID, roomID uint, cabinetName string) {
	cabinet, err := s.resolveCabinet(cabinetID, roomID, cabinetName)
	if err != nil {
		result.Failed(c, constant.INVALID_PARAMS, err.Error())
		return
	}

	room, err := s.roomDao.GetRoomById(cabinet.RoomID)
	if err != nil {
		result.Failed(c, constant.IDC_ROOM_NOT_FOUND, "机房不存在")
		return
	}

	assets, err := s.assetDao.ListAssetsByCabinetID(cabinet.ID)
	if err != nil {
		result.Failed(c, constant.IDC_ASSET_NOT_FOUND, err.Error())
		return
	}

	layout := buildCabinetLayoutWithHardware(cabinet, room.Name, assets, aggregateIDCAssetHardwareStates(assets, 5))
	result.Success(c, layout)
}

func (s *CmdbIdcRoomServiceImpl) resolveCabinet(cabinetID, roomID uint, cabinetName string) (model.CmdbIdcCabinet, error) {
	if cabinetID > 0 {
		cabinet, err := s.roomDao.GetCabinetById(cabinetID)
		if err != nil {
			return model.CmdbIdcCabinet{}, errors.New("机柜不存在")
		}
		return cabinet, nil
	}

	name := strings.TrimSpace(cabinetName)
	if name == "" {
		return model.CmdbIdcCabinet{}, errors.New("请传入cabinetId或cabinetName")
	}

	if roomID > 0 {
		cabinet, err := s.roomDao.GetCabinetByRoomAndName(roomID, name)
		if err != nil {
			return model.CmdbIdcCabinet{}, errors.New("机柜不存在")
		}
		return *cabinet, nil
	}

	cabinets, err := s.roomDao.FindCabinetsByName(name)
	if err != nil {
		return model.CmdbIdcCabinet{}, err
	}
	if len(cabinets) == 0 {
		return model.CmdbIdcCabinet{}, errors.New("机柜不存在")
	}
	if len(cabinets) > 1 {
		return model.CmdbIdcCabinet{}, errors.New("机柜名称重复，请传入roomId")
	}
	return cabinets[0], nil
}

func buildRoomLayout(room model.CmdbIdcRoom, assets []model.CmdbIdcAsset) model.CmdbIdcRoomLayout {
	return buildRoomLayoutFromHardware(room, assets, nil)
}

func buildRoomLayoutWithHardware(room model.CmdbIdcRoom, assets []model.CmdbIdcAsset) model.CmdbIdcRoomLayout {
	return buildRoomLayoutFromHardware(room, assets, aggregateIDCAssetHardwareStates(assets, 5))
}

func buildRoomLayoutFromHardware(room model.CmdbIdcRoom, assets []model.CmdbIdcAsset, hardwareByAsset map[uint]idcAssetHardwareState) model.CmdbIdcRoomLayout {
	assetsByCabinet := make(map[uint][]model.CmdbIdcAsset)
	for _, asset := range assets {
		if asset.CabinetID == 0 {
			continue
		}
		assetsByCabinet[asset.CabinetID] = append(assetsByCabinet[asset.CabinetID], asset)
	}

	items := make([]roomLayoutCabinetItem, 0, len(room.Cabinets))
	for _, cabinet := range room.Cabinets {
		rowNo, cabinetNo, matched := parseCabinetNamePosition(cabinet.Name)
		layout := buildCabinetLayoutWithHardware(cabinet, room.Name, assetsByCabinet[cabinet.ID], hardwareByAsset)
		items = append(items, roomLayoutCabinetItem{
			cabinet:   cabinet,
			layout:    layout,
			rowNo:     rowNo,
			cabinetNo: cabinetNo,
			matched:   matched,
		})
	}

	sort.SliceStable(items, func(i, j int) bool {
		left, right := items[i], items[j]
		if left.matched != right.matched {
			return left.matched
		}
		if left.rowNo != right.rowNo {
			return left.rowNo < right.rowNo
		}
		if left.cabinetNo != right.cabinetNo {
			return left.cabinetNo < right.cabinetNo
		}
		if left.cabinet.Name != right.cabinet.Name {
			return left.cabinet.Name < right.cabinet.Name
		}
		return left.cabinet.ID < right.cabinet.ID
	})

	rows := make([]model.CmdbIdcRoomLayoutRow, 0)
	unmatched := make([]model.CmdbIdcRoomLayoutCabinet, 0)
	rowIndexByNo := make(map[int]int)

	for _, item := range items {
		if !item.matched {
			unmatched = append(unmatched, buildRoomLayoutCabinet(item, len(unmatched), len(rows), "unmatched"))
			continue
		}

		rowIndex, ok := rowIndexByNo[item.rowNo]
		if !ok {
			rowIndex = len(rows)
			rowIndexByNo[item.rowNo] = rowIndex
			rows = append(rows, model.CmdbIdcRoomLayoutRow{
				RowNo:    item.rowNo,
				RowLabel: fmt.Sprintf("%02d列", item.rowNo),
				Y:        rowIndex,
				Cabinets: make([]model.CmdbIdcRoomLayoutCabinet, 0),
			})
		}

		x := len(rows[rowIndex].Cabinets)
		rows[rowIndex].Cabinets = append(rows[rowIndex].Cabinets, buildRoomLayoutCabinet(item, x, rowIndex, "matched"))
	}

	return model.CmdbIdcRoomLayout{
		RoomID:            room.ID,
		RoomName:          room.Name,
		CabinetCount:      len(room.Cabinets),
		AssetCount:        int64(len(assets)),
		LayoutMode:        "name_order",
		Rows:              rows,
		UnmatchedCabinets: unmatched,
	}
}

func buildRoomLayoutCabinet(item roomLayoutCabinetItem, x, y int, parseStatus string) model.CmdbIdcRoomLayoutCabinet {
	return model.CmdbIdcRoomLayoutCabinet{
		CabinetID:       item.cabinet.ID,
		Name:            item.cabinet.Name,
		RowNo:           item.rowNo,
		CabinetNo:       item.cabinetNo,
		X:               x,
		Y:               y,
		Direction:       "front",
		TotalU:          item.layout.TotalU,
		UsedU:           item.layout.UsedU,
		FreeU:           item.layout.FreeU,
		UsageRate:       item.layout.UsageRate,
		IdleRate:        item.layout.IdleRate,
		AssetCount:      int64(item.layout.AssetCount),
		Status:          roomLayoutCabinetStatus(item.layout),
		HardwareStatus:  item.layout.HardwareStatus,
		HardwareSummary: item.layout.HardwareSummary,
		ParseStatus:     parseStatus,
	}
}

func parseCabinetNamePosition(name string) (int, int, bool) {
	matches := cabinetNamePositionRegexp.FindStringSubmatch(strings.TrimSpace(name))
	if len(matches) != 3 {
		return 0, 0, false
	}

	rowNo, rowErr := strconv.Atoi(matches[1])
	cabinetNo, cabinetErr := strconv.Atoi(matches[2])
	if rowErr != nil || cabinetErr != nil || rowNo <= 0 || cabinetNo <= 0 {
		return 0, 0, false
	}
	return rowNo, cabinetNo, true
}

func roomLayoutCabinetStatus(layout model.CmdbIdcCabinetLayout) string {
	if layout.HardwareStatus == "critical" {
		return layout.HardwareStatus
	}
	return "normal"
}

func buildCabinetLayout(cabinet model.CmdbIdcCabinet, roomName string, assets []model.CmdbIdcAsset) model.CmdbIdcCabinetLayout {
	return buildCabinetLayoutWithHardware(cabinet, roomName, assets, nil)
}

func buildCabinetLayoutWithHardware(cabinet model.CmdbIdcCabinet, roomName string, assets []model.CmdbIdcAsset, hardwareByAsset map[uint]idcAssetHardwareState) model.CmdbIdcCabinetLayout {
	totalU, _ := normalizeCabinetTotalU(cabinet.TotalU)
	slots := make([]model.CmdbIdcCabinetSlot, totalU)
	occupied := make([]bool, totalU+1)
	for i := 1; i <= totalU; i++ {
		slots[i-1] = model.CmdbIdcCabinetSlot{
			U:        i,
			Occupied: false,
			AssetIDs: make([]uint, 0),
		}
	}

	layoutAssets := make([]model.CmdbIdcCabinetAssetBlock, 0, len(assets))
	placedCount := 0
	unplacedCount := 0
	hardwareSummary := model.CmdbIdcHardwareSummary{
		Issues: []model.CmdbIdcHardwareIssueBrief{},
	}

	for _, asset := range assets {
		uSize := asset.CabinetUSize
		if uSize <= 0 {
			uSize = 1
		}

		hardwareStatus, hardwareMetrics := resolveIDCAssetHardwareState(asset, hardwareByAsset)
		applyIDCHardwareSummary(&hardwareSummary, asset, hardwareStatus, hardwareMetrics)

		block := model.CmdbIdcCabinetAssetBlock{
			AssetID:              asset.ID,
			IP:                   asset.IP,
			Brand:                asset.Brand,
			Model:                asset.Model,
			Purpose:              asset.Purpose,
			RemoteManagementCard: asset.RemoteManagementCard,
			HardwareStatus:       hardwareStatus,
			HardwareMetrics:      hardwareMetrics,
			CabinetPosition:      asset.CabinetPosition,
			CabinetUSize:         uSize,
			StartU:               asset.CabinetPosition,
			EndU:                 asset.CabinetPosition + uSize - 1,
			PlacementStatus:      "unplaced",
		}

		if asset.CabinetPosition <= 0 {
			unplacedCount++
			layoutAssets = append(layoutAssets, block)
			continue
		}

		placedCount++
		block.PlacementStatus = "placed"
		if block.EndU > totalU {
			block.PlacementStatus = "out_of_range"
		}

		hasOverlap := false
		start := maxInt(block.StartU, 1)
		end := minInt(block.EndU, totalU)
		for u := start; u <= end; u++ {
			if slots[u-1].Occupied {
				hasOverlap = true
			}
			slots[u-1].Occupied = true
			slots[u-1].AssetIDs = append(slots[u-1].AssetIDs, asset.ID)
			occupied[u] = true
		}
		if hasOverlap {
			block.PlacementStatus = "overlap"
		}

		layoutAssets = append(layoutAssets, block)
	}

	sort.Slice(layoutAssets, func(i, j int) bool {
		if layoutAssets[i].CabinetPosition == layoutAssets[j].CabinetPosition {
			return layoutAssets[i].AssetID < layoutAssets[j].AssetID
		}
		if layoutAssets[i].CabinetPosition <= 0 {
			return false
		}
		if layoutAssets[j].CabinetPosition <= 0 {
			return true
		}
		return layoutAssets[i].CabinetPosition < layoutAssets[j].CabinetPosition
	})

	usedU := 0
	for u := 1; u <= totalU; u++ {
		if occupied[u] {
			usedU++
		}
	}
	freeU := totalU - usedU
	if freeU < 0 {
		freeU = 0
	}

	usageRate := 0.0
	if totalU > 0 {
		usageRate = float64(usedU) * 100 / float64(totalU)
	}
	hardwareStatus := resolveIDCHardwareSummaryStatus(hardwareSummary)

	return model.CmdbIdcCabinetLayout{
		CabinetID:          cabinet.ID,
		CabinetName:        cabinet.Name,
		RoomID:             cabinet.RoomID,
		RoomName:           roomName,
		TotalU:             totalU,
		UsedU:              usedU,
		FreeU:              freeU,
		UsageRate:          roundRate(usageRate),
		IdleRate:           roundRate(100 - usageRate),
		IsFull:             freeU == 0,
		AssetCount:         len(assets),
		PlacedAssetCount:   placedCount,
		UnplacedAssetCount: unplacedCount,
		HardwareStatus:     hardwareStatus,
		HardwareSummary:    hardwareSummary,
		Slots:              slots,
		Assets:             layoutAssets,
	}
}

func resolveIDCAssetHardwareState(asset model.CmdbIdcAsset, hardwareByAsset map[uint]idcAssetHardwareState) (string, []string) {
	if hardwareByAsset == nil {
		return "unknown", []string{}
	}
	state, ok := hardwareByAsset[asset.ID]
	if !ok || strings.TrimSpace(asset.RemoteManagementCard) == "" {
		return "unknown", []string{}
	}
	if state.Status == "" {
		state.Status = "unknown"
	}
	if state.Metrics == nil {
		state.Metrics = []string{}
	}
	return state.Status, state.Metrics
}

func applyIDCHardwareSummary(summary *model.CmdbIdcHardwareSummary, asset model.CmdbIdcAsset, status string, metrics []string) {
	switch status {
	case "critical":
		summary.CriticalAssetCount++
	case "warning", "normal", "healthy":
		summary.NormalAssetCount++
	default:
		summary.UnknownAssetCount++
	}

	if status != "critical" {
		return
	}
	summary.AlertAssetCount++
	summary.Issues = append(summary.Issues, model.CmdbIdcHardwareIssueBrief{
		AssetID:              asset.ID,
		IP:                   asset.IP,
		RemoteManagementCard: asset.RemoteManagementCard,
		Level:                status,
		Metrics:              metrics,
	})
}

func resolveIDCHardwareSummaryStatus(summary model.CmdbIdcHardwareSummary) string {
	if summary.CriticalAssetCount > 0 {
		return "critical"
	}
	return "normal"
}

func aggregateIDCAssetHardwareStates(assets []model.CmdbIdcAsset, lookbackMinutes int) map[uint]idcAssetHardwareState {
	result := make(map[uint]idcAssetHardwareState, len(assets))
	if len(assets) == 0 {
		return result
	}
	if lookbackMinutes <= 0 {
		lookbackMinutes = 5
	}

	hardwareSvc := NewCmdbIdcAssetHardwareMonitorService()
	account, err := hardwareSvc.resolvePrometheusAccount()
	if err != nil {
		return result
	}

	assetByCandidate := make(map[string]*model.CmdbIdcAsset)
	for _, asset := range assets {
		if strings.TrimSpace(asset.RemoteManagementCard) == "" {
			result[asset.ID] = idcAssetHardwareState{Status: "unknown", Metrics: []string{}}
			continue
		}
		result[asset.ID] = idcAssetHardwareState{Status: "normal", Metrics: []string{}}
		for _, candidate := range normalizeManagementTargets(asset.RemoteManagementCard) {
			assetCopy := asset
			assetByCandidate[candidate] = &assetCopy
		}
	}
	if len(assetByCandidate) == 0 {
		return result
	}
	candidateRegex := buildIDCHardwareCandidateRegex(assetByCandidate)

	for _, spec := range hardwareVendorSpecs {
		installedDiskByAsset := map[string]struct{}{}
		if spec.vendor == "huawei" {
			installedDiskByAsset = collectIDCInstalledDiskComponents(hardwareSvc, account, assetByCandidate, candidateRegex, lookbackMinutes)
		}
		seenMetrics := map[string]struct{}{}
		for _, item := range idcHardwareStatusMetrics(spec) {
			if strings.TrimSpace(item.Name) == "" {
				continue
			}
			if _, exists := seenMetrics[item.Name]; exists {
				continue
			}
			seenMetrics[item.Name] = struct{}{}

			for _, label := range []string{"target", "instance"} {
				selector := fmt.Sprintf(`%s=~%q`, label, candidateRegex)
				if item.Job != "" {
					selector = fmt.Sprintf(`job=%q,%s`, item.Job, selector)
				}
				query := fmt.Sprintf(`last_over_time(%s{%s}[%dm])`, item.Name, selector, lookbackMinutes)
				response, err := hardwareSvc.queryPrometheusInstant(account, query)
				if err != nil {
					continue
				}
				applyIDCHardwareMetricResults(result, assetByCandidate, response, spec.vendor, item, installedDiskByAsset)
			}
		}
	}

	return result
}

func collectIDCInstalledDiskComponents(hardwareSvc *CmdbIdcAssetHardwareMonitorService, account *prometheusAccount, assetByCandidate map[string]*model.CmdbIdcAsset, candidateRegex string, lookbackMinutes int) map[string]struct{} {
	installed := map[string]struct{}{}
	collect := func(metricName string, validValue func(float64) bool) {
		for _, label := range []string{"target", "instance"} {
			query := fmt.Sprintf(`last_over_time(%s{%s=~%q}[%dm])`, metricName, label, candidateRegex, lookbackMinutes)
			response, err := hardwareSvc.queryPrometheusInstant(account, query)
			if err != nil || response == nil {
				continue
			}
			for _, resultItem := range response.Data.Result {
				if !validValue(valueFromVector(resultItem.Value)) {
					continue
				}
				index := componentIndex(resultItem.Metric, "hardDiskIndex")
				if strings.TrimSpace(index) == "" {
					continue
				}
				for _, candidate := range idcCandidatesFromMetricLabels(resultItem.Metric) {
					asset, ok := assetByCandidate[candidate]
					if !ok {
						continue
					}
					installed[idcHardwareComponentKey(asset.ID, index)] = struct{}{}
				}
			}
		}
	}

	collect("hardDiskPowerOnHours", func(value float64) bool {
		return value > 1
	})
	collect("hardDiskTemperature", func(value float64) bool {
		return value > 0 && value < 120
	})

	return installed
}

func applyIDCHardwareMetricResults(result map[uint]idcAssetHardwareState, assetByCandidate map[string]*model.CmdbIdcAsset, response *prometheusAPIResponse, vendor string, metricItem idcHardwareStatusMetric, installedDiskByAsset map[string]struct{}) {
	if response == nil {
		return
	}
	for _, resultItem := range response.Data.Result {
		metricStatus := normalizeIDCHardwareAggregationStatus(vendor, metricItem, valueFromVector(resultItem.Value))
		if metricStatus != "critical" {
			continue
		}

		matchedAssetIDs := map[uint]struct{}{}
		for _, candidate := range idcCandidatesFromMetricLabels(resultItem.Metric) {
			asset, ok := assetByCandidate[candidate]
			if !ok {
				continue
			}
			if _, exists := matchedAssetIDs[asset.ID]; exists {
				continue
			}
			matchedAssetIDs[asset.ID] = struct{}{}

			if metricItem.RequireInstalledDisk {
				index := componentIndex(resultItem.Metric, metricItem.ComponentIndex)
				if strings.TrimSpace(index) == "" {
					continue
				}
				if _, installed := installedDiskByAsset[idcHardwareComponentKey(asset.ID, index)]; !installed {
					continue
				}
			}

			state := result[asset.ID]
			state.Status = "critical"
			state.Metrics = appendUniqueIDCHardwareMetric(state.Metrics, fmt.Sprintf("%s:%s", metricItem.Label, metricStatus))
			result[asset.ID] = state
		}
	}
}

func normalizeIDCHardwareAggregationStatus(vendor string, metricItem idcHardwareStatusMetric, value float64) string {
	switch metricItem.Kind {
	case "remaining_life":
		if value <= 0 || value >= 255 {
			return "unknown"
		}
		if value <= 20 {
			return "critical"
		}
		return "normal"
	default:
		return normalizeStatus(vendor, metricItem.Name, value)
	}
}

func idcHardwareStatusMetrics(spec vendorMetricSpec) []idcHardwareStatusMetric {
	items := []idcHardwareStatusMetric{
		{Label: "内存/系统稳定性", Name: spec.memoryMetric, Kind: "status"},
		{Label: "磁盘/RAID状态", Name: spec.diskMetric, Kind: "status"},
		{Label: "电源状态", Name: spec.powerMetric, Kind: "status"},
	}

	if spec.vendor == "huawei" {
		items = append(items,
			idcHardwareStatusMetric{Label: "磁盘状态", Name: "hardDiskStatus", Kind: "status", ComponentIndex: "hardDiskIndex", RequireInstalledDisk: true},
			idcHardwareStatusMetric{Label: "磁盘磨损寿命", Name: "hardDiskRemnantWearout", Kind: "remaining_life", ComponentIndex: "hardDiskIndex", RequireInstalledDisk: true},
			idcHardwareStatusMetric{Label: "风扇模块状态", Name: spec.fanMetric, Kind: "status"},
			idcHardwareStatusMetric{Label: "风扇模块状态", Name: "fanStatus", Kind: "status"},
			idcHardwareStatusMetric{Label: "RAID卡健康状态", Name: "raidControllerHealthStatus", Kind: "status"},
			idcHardwareStatusMetric{Label: "逻辑盘一致性状态", Name: "logicalDriveConsistencyCheckStatus", Kind: "status"},
			idcHardwareStatusMetric{Label: "电源模块状态", Name: "powerSupplyStatus", Kind: "status"},
		)
	}
	if spec.vendor == "dell" {
		items = append(items,
			idcHardwareStatusMetric{Label: "物理磁盘状态", Name: "physicalDiskComponentStatus", Kind: "status"},
			idcHardwareStatusMetric{Label: "SSD剩余寿命", Name: "physicalDiskRemainingRatedWriteEndurance", Kind: "remaining_life"},
			idcHardwareStatusMetric{Label: "RAID控制器状态", Name: "controllerComponentStatus", Kind: "status"},
			idcHardwareStatusMetric{Label: "风扇模块状态", Name: spec.fanMetric, Kind: "status"},
			idcHardwareStatusMetric{Label: "电源模块状态", Name: "powerSupplySensorState", Kind: "status"},
		)
	}
	if spec.vendor == "ipmi" {
		items = append(items,
			idcHardwareStatusMetric{Label: "风扇模块状态", Name: "ipmi_fan_speed_state", Kind: "status", Job: spec.requiredJob},
			idcHardwareStatusMetric{Label: "电源状态", Name: "ipmi_power_state", Kind: "status", Job: spec.requiredJob},
			idcHardwareStatusMetric{Label: "电流状态", Name: "ipmi_current_state", Kind: "status", Job: spec.requiredJob},
			idcHardwareStatusMetric{Label: "电压状态", Name: "ipmi_voltage_state", Kind: "status", Job: spec.requiredJob},
		)
	}

	return dedupeIDCHardwareStatusMetrics(items)
}

func dedupeIDCHardwareStatusMetrics(items []idcHardwareStatusMetric) []idcHardwareStatusMetric {
	result := make([]idcHardwareStatusMetric, 0, len(items))
	seen := map[string]struct{}{}
	for _, item := range items {
		if strings.TrimSpace(item.Name) == "" {
			continue
		}
		key := item.Label + ":" + item.Name
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, item)
	}
	return result
}

func idcCandidatesFromMetricLabels(labels map[string]string) []string {
	values := make([]string, 0, 6)
	seen := map[string]struct{}{}
	appendValue := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if _, exists := seen[value]; exists {
			return
		}
		seen[value] = struct{}{}
		values = append(values, value)
	}

	for _, key := range []string{"target", "instance"} {
		raw := labels[key]
		appendValue(raw)
		for _, candidate := range normalizeManagementTargets(raw) {
			appendValue(candidate)
		}
	}
	return values
}

func buildIDCHardwareCandidateRegex(assetByCandidate map[string]*model.CmdbIdcAsset) string {
	values := make([]string, 0, len(assetByCandidate))
	for candidate := range assetByCandidate {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		values = append(values, regexp.QuoteMeta(candidate))
	}
	sort.Strings(values)
	if len(values) == 0 {
		return "$^"
	}
	return "^(?:" + strings.Join(values, "|") + ")$"
}

func appendUniqueIDCHardwareMetric(metrics []string, metric string) []string {
	for _, item := range metrics {
		if item == metric {
			return metrics
		}
	}
	return append(metrics, metric)
}

func idcHardwareComponentKey(assetID uint, index string) string {
	return fmt.Sprintf("%d:%s", assetID, strings.TrimSpace(index))
}

func deduplicateNames(names []string) []string {
	resultNames := make([]string, 0, len(names))
	seen := make(map[string]struct{}, len(names))

	for _, name := range names {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		resultNames = append(resultNames, trimmed)
	}

	return resultNames
}

func enrichRoomAssetStats(room *model.CmdbIdcRoom, roomCounts, cabinetCounts map[uint]int64) {
	room.AssetCount = roomCounts[room.ID]
	room.CabinetCount = len(room.Cabinets)
	for i := range room.Cabinets {
		room.Cabinets[i].AssetCount = cabinetCounts[room.Cabinets[i].ID]
	}
}

func normalizeCabinetTotalU(totalU int) (int, error) {
	if totalU <= 0 {
		return 42, nil
	}
	if totalU > 100 {
		return 0, fmt.Errorf("totalU不能大于100")
	}
	return totalU, nil
}

func roundRate(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return float64(int(v*100+0.5)) / 100
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func isRecordNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}
