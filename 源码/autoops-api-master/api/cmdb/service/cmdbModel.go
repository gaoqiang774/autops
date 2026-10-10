package service

import (
	"context"
	"dodevops-api/api/cmdb/dao"
	"dodevops-api/api/cmdb/model"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type CmdbService struct {
	dao *dao.CmdbDao
}

func NewCmdbService(dao *dao.CmdbDao) *CmdbService {
	return &CmdbService{dao: dao}
}

func getRelationLabel(relationType string) string {
	labelMap := map[string]string{
		"runs_on":      "运行于",
		"connects_to":  "连接",
		"depends_on":   "依赖",
		"contains":     "包含",
		"deployed_on":  "部署于",
		"uses":         "使用",
		"emits_log_to": "产生日志",
	}
	if label, ok := labelMap[relationType]; ok {
		return label
	}
	return relationType
}

// CreateBusinessModel 创建业务模型，并同步创建一个业务图谱根绑定节点。
func (s *CmdbService) CreateBusinessModel(dto model.CreateBusinessModelDto) (*model.BusinessModelSummaryVo, error) {
	name := strings.TrimSpace(dto.Name)
	if name == "" {
		return nil, errors.New("业务名称不能为空")
	}

	displayName := strings.TrimSpace(dto.DisplayName)
	if displayName == "" {
		displayName = name
	}

	if err := s.dao.PurgeDeletedBusinessModelByName(name); err != nil {
		return nil, err
	}

	exists, err := s.dao.BusinessModelExists(name)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, errors.New("业务模型已存在")
	}

	propsJSON, err := dao.JSONMapToDatatype(dto.Properties)
	if err != nil {
		return nil, err
	}

	business := &model.CmdbBusinessModel{
		Name:        name,
		DisplayName: displayName,
		Status:      "active",
		Env:         strings.TrimSpace(dto.Env),
		Owner:       strings.TrimSpace(dto.Owner),
		BizCode:     strings.TrimSpace(dto.BizCode),
		Properties:  propsJSON,
	}
	root := &model.CmdbBusinessModelBinding{
		Name:         name,
		DisplayName:  displayName,
		ResourceType: model.CmdbResourceTypeBusinessSystem,
		Status:       "active",
		Env:          business.Env,
		Owner:        business.Owner,
		BizCode:      business.BizCode,
		Properties:   propsJSON,
	}

	if err := s.dao.CreateBusinessModelWithRoot(business, root); err != nil {
		return nil, err
	}

	if err := s.upsertBusinessDefaultHealth(business.ID, root.ID); err != nil {
		return nil, err
	}

	return s.GetBusinessModelSummary(business.ID)
}

// ListBusinessModels 获取业务模型列表，用于领导视图入口页。
func (s *CmdbService) ListBusinessModels(query model.BusinessModelQuery) (map[string]interface{}, error) {
	if query.Page <= 0 {
		query.Page = 1
	}
	if query.PageSize <= 0 {
		query.PageSize = 20
	}
	if query.PageSize > 100 {
		query.PageSize = 100
	}

	businesses, total, err := s.dao.ListBusinessModels(query)
	if err != nil {
		return nil, err
	}

	rootByBusinessID := make(map[uint]*model.CmdbBusinessModelBinding, len(businesses))
	ids := make([]uint, 0, len(businesses))
	for i := range businesses {
		root, err := s.dao.GetBusinessRootBinding(businesses[i].ID)
		if err != nil {
			return nil, err
		}
		rootByBusinessID[businesses[i].ID] = root
		ids = append(ids, root.ID)
	}

	healthSnapshots, err := s.dao.GetBusinessHealthSnapshotsByResourceIDs(0, ids)
	if err != nil {
		return nil, err
	}
	snapshotMap := groupSnapshotsByResourceID(healthSnapshots)

	list := make([]model.BusinessModelSummaryVo, 0, len(businesses))
	for i := range businesses {
		root := rootByBusinessID[businesses[i].ID]
		summary := s.buildBusinessSummaryVo(businessToSummaryResource(&businesses[i]), snapshotMap[root.ID])
		summary.ID = businesses[i].ID
		list = append(list, summary)
	}

	return map[string]interface{}{
		"list":     list,
		"total":    total,
		"page":     query.Page,
		"pageSize": query.PageSize,
	}, nil
}

func (s *CmdbService) GetBusinessModelSummary(businessID uint) (*model.BusinessModelSummaryVo, error) {
	business, err := s.dao.GetBusinessModelByID(businessID)
	if err != nil {
		return nil, err
	}
	root, err := s.dao.GetBusinessRootBinding(businessID)
	if err != nil {
		return nil, err
	}
	snapshots, err := s.dao.GetBusinessHealthSnapshotsByResourceIDs(businessID, []uint{root.ID})
	if err != nil {
		return nil, err
	}
	summary := s.buildBusinessSummaryVo(businessToSummaryResource(business), snapshots)
	summary.ID = business.ID
	return &summary, nil
}

// DeleteBusinessModel 删除业务模型入口资源，并清理业务自身诊断快照与拓扑关系。
// 绑定的资产资源可能被其它业务复用，这里只解除业务拓扑，不删除资产本体。
func (s *CmdbService) DeleteBusinessModel(businessID uint) (map[string]interface{}, error) {
	graph, err := s.collectBusinessGraph(businessID, 8, "downstream")
	if err != nil {
		return nil, err
	}
	if graph.Root.ResourceType != model.CmdbResourceTypeBusinessSystem {
		return nil, errors.New("资源不是业务模型")
	}

	if err := s.dao.DeleteBusinessModel(businessID); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"businessId":       businessID,
		"deletedRelations": len(graph.Relations),
		"deletedSnapshots": "business",
		"deletedBindings":  len(graph.Resources),
	}, nil
}

// BindBusinessModelResources 将业务模型和已有/新增资源串起来，形成可下钻的关系拓扑。
func (s *CmdbService) BindBusinessModelResources(businessID uint, dto model.BusinessModelBindingDto) (map[string]interface{}, error) {
	if _, err := s.dao.GetBusinessModelByID(businessID); err != nil {
		return nil, err
	}
	root, err := s.dao.GetBusinessRootBinding(businessID)
	if err != nil {
		return nil, err
	}
	if len(dto.Resources) == 0 {
		return nil, errors.New("绑定资源不能为空")
	}

	parentID := dto.ParentID
	if parentID == 0 {
		parentID = root.ID
	}
	parent, err := s.dao.GetBusinessBindingByID(businessID, parentID)
	if err != nil {
		return nil, fmt.Errorf("父节点不存在: %w", err)
	}

	boundResources := make([]map[string]interface{}, 0, len(dto.Resources))
	for _, item := range dto.Resources {
		resource, created, err := s.resolveBindingResource(businessID, parentID, item)
		if err != nil {
			return nil, err
		}

		relationType := normalizeRelationType(item.RelationType)
		if relationType == "" {
			relationType = defaultRelationType(parent.ResourceType, resource.ResourceType)
		}

		relationProps, err := dao.JSONMapToDatatype(item.Properties)
		if err != nil {
			return nil, err
		}
		relation := &model.CmdbBusinessModelRelation{
			BusinessID:   businessID,
			SrcID:        parentID,
			DstID:        resource.ID,
			RelationType: relationType,
			Properties:   relationProps,
			Status:       1,
		}
		if err := s.dao.CreateBusinessRelationIfNotExists(relation); err != nil {
			return nil, err
		}

		boundResources = append(boundResources, map[string]interface{}{
			"id":           resource.ID,
			"name":         resource.Name,
			"displayName":  resourceDisplayName(resource),
			"resourceType": resource.ResourceType,
			"sourceType":   resource.SourceType,
			"sourceId":     resource.SourceID,
			"sourceKey":    resource.SourceKey,
			"relationType": relationType,
			"created":      created,
		})
	}

	summary, err := s.RefreshBusinessModelHealth(businessID)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"business":  summary,
		"parentId":  parentID,
		"resources": boundResources,
		"count":     len(boundResources),
	}, nil
}

func (s *CmdbService) UpdateBusinessModelResource(resourceID uint, dto model.BusinessModelBindingResourceDto) (map[string]interface{}, error) {
	if resourceID == 0 {
		return nil, errors.New("resourceId 不能为空")
	}

	resource, err := s.dao.GetBusinessBinding(resourceID)
	if err != nil {
		return nil, err
	}

	dto.ResourceID = resourceID
	if err := s.applyBindingResourceFields(resource, dto); err != nil {
		return nil, err
	}
	if err := s.syncRootBusinessModel(resource); err != nil {
		return nil, err
	}

	snapshots, err := s.dao.GetBusinessHealthSnapshotsByResourceIDs(resource.BusinessID, []uint{resource.ID})
	if err != nil {
		return nil, err
	}
	health := s.resolveNodeHealth(resource, snapshots)
	updatedResource := s.buildBusinessTopologyNode(resource, health, snapshots)

	topology, err := s.GetBusinessModelTopology(resource.BusinessID, 4, "downstream")
	if err != nil {
		return nil, err
	}
	topology["resource"] = updatedResource
	for key, value := range updatedResource {
		if _, exists := topology[key]; !exists {
			topology[key] = value
		}
	}
	return topology, nil
}

func (s *CmdbService) DeleteBusinessModelRelation(relationID uint) (map[string]interface{}, error) {
	if relationID == 0 {
		return nil, errors.New("relationId 不能为空")
	}

	relation, err := s.dao.GetBusinessRelationByID(relationID)
	if err != nil {
		return nil, err
	}
	if err := s.dao.DeleteBusinessRelation(relationID); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"id":           relation.ID,
		"businessId":   relation.BusinessID,
		"source":       relation.SrcID,
		"target":       relation.DstID,
		"sourceId":     relation.SrcID,
		"targetId":     relation.DstID,
		"srcId":        relation.SrcID,
		"dstId":        relation.DstID,
		"relationType": relation.RelationType,
		"deleted":      true,
	}, nil
}

func (s *CmdbService) resolveBindingResource(businessID, parentID uint, item model.BusinessModelBindingResourceDto) (*model.CmdbBusinessModelBinding, bool, error) {
	if item.ResourceID > 0 {
		resource, err := s.dao.GetBusinessBindingByID(businessID, item.ResourceID)
		if err != nil {
			return nil, false, err
		}
		if err := s.applyBindingResourceFields(resource, item); err != nil {
			return nil, false, err
		}
		return resource, false, nil
	}

	if strings.TrimSpace(item.SourceType) != "" && (item.SourceID > 0 || strings.TrimSpace(item.SourceKey) != "") {
		resource, err := s.dao.FindBusinessBindingBySource(businessID, item.SourceType, item.SourceID, item.SourceKey)
		if err == nil {
			if err := s.applyBindingResourceFields(resource, item); err != nil {
				return nil, false, err
			}
			return resource, false, nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, false, err
		}
	}

	name := bindingResourceName(item)
	if name == "" {
		name = fallbackResourceName(item)
	}
	if name == "" {
		return nil, false, errors.New("绑定新资源时 name 不能为空")
	}

	resourceType := bindingResourceType(item)
	if resourceType == "" {
		resourceType = inferResourceTypeFromSource(item.SourceType)
	}
	if resourceType == "" {
		return nil, false, errors.New("绑定新资源时 resourceType 不能为空")
	}

	propsJSON, err := dao.JSONMapToDatatype(item.Properties)
	if err != nil {
		return nil, false, err
	}

	displayName := bindingResourceDisplayName(item)
	if displayName == "" {
		displayName = name
	}

	resource := &model.CmdbBusinessModelBinding{
		BusinessID:   businessID,
		ParentID:     parentID,
		Name:         name,
		DisplayName:  displayName,
		ResourceType: resourceType,
		Status:       "active",
		SourceType:   strings.TrimSpace(item.SourceType),
		SourceID:     item.SourceID,
		SourceKey:    strings.TrimSpace(item.SourceKey),
		Env:          strings.TrimSpace(item.Env),
		Owner:        strings.TrimSpace(item.Owner),
		BizCode:      strings.TrimSpace(item.BizCode),
		Properties:   propsJSON,
	}

	if err := s.dao.CreateBusinessBinding(resource); err != nil {
		return nil, false, err
	}
	return resource, true, nil
}

func (s *CmdbService) applyBindingResourceFields(resource *model.CmdbBusinessModelBinding, item model.BusinessModelBindingResourceDto) error {
	changed := false
	if value := bindingResourceName(item); value != "" && resource.Name != value {
		resource.Name = value
		changed = true
	}
	if value := bindingResourceDisplayName(item); value != "" && resource.DisplayName != value {
		resource.DisplayName = value
		changed = true
	}
	if value := bindingResourceType(item); value != "" && resource.ResourceType != value {
		resource.ResourceType = value
		changed = true
	}
	if value := strings.TrimSpace(item.SourceType); value != "" && resource.SourceType != value {
		resource.SourceType = value
		changed = true
	}
	if item.SourceID > 0 && resource.SourceID != item.SourceID {
		resource.SourceID = item.SourceID
		changed = true
	}
	if value := strings.TrimSpace(item.SourceKey); value != "" && resource.SourceKey != value {
		resource.SourceKey = value
		changed = true
	}
	if value := strings.TrimSpace(item.Env); value != "" && resource.Env != value {
		resource.Env = value
		changed = true
	}
	if value := strings.TrimSpace(item.Owner); value != "" && resource.Owner != value {
		resource.Owner = value
		changed = true
	}
	if value := strings.TrimSpace(item.BizCode); value != "" && resource.BizCode != value {
		resource.BizCode = value
		changed = true
	}
	if value := strings.TrimSpace(item.Status); value != "" && resource.Status != value {
		resource.Status = value
		changed = true
	}
	if len(item.Properties) > 0 {
		propsJSON, err := dao.JSONMapToDatatype(item.Properties)
		if err != nil {
			return err
		}
		resource.Properties = propsJSON
		changed = true
	}
	if !changed {
		return nil
	}
	return s.dao.UpdateBusinessBinding(resource)
}

func (s *CmdbService) syncRootBusinessModel(resource *model.CmdbBusinessModelBinding) error {
	if resource == nil || resource.ParentID != 0 || resource.ResourceType != model.CmdbResourceTypeBusinessSystem {
		return nil
	}

	business, err := s.dao.GetBusinessModelByID(resource.BusinessID)
	if err != nil {
		return err
	}

	name := strings.TrimSpace(resource.Name)
	if name == "" {
		return errors.New("业务名称不能为空")
	}
	if business.Name != name {
		exists, err := s.dao.BusinessModelNameExistsExcept(name, business.ID)
		if err != nil {
			return err
		}
		if exists {
			return errors.New("业务模型已存在")
		}
	}

	business.Name = name
	business.DisplayName = resourceDisplayName(resource)
	business.Env = resource.Env
	business.Owner = resource.Owner
	business.BizCode = resource.BizCode
	business.Properties = resource.Properties
	if strings.TrimSpace(resource.Status) != "" {
		business.Status = resource.Status
	}
	return s.dao.UpdateBusinessModel(business)
}

// UpsertHealthSnapshot 写入某个资源在某个维度的健康快照。其它模块可以用这个接口把监控/日志/DB/K8S 的结论喂给图谱。
func (s *CmdbService) UpsertHealthSnapshot(dto model.UpsertHealthSnapshotDto) (*model.HealthDimensionVo, error) {
	if dto.ResourceID == 0 {
		return nil, errors.New("resourceId 不能为空")
	}
	binding, err := s.dao.GetBusinessBinding(dto.ResourceID)
	if err != nil {
		return nil, err
	}

	dimension := normalizeHealthDimension(dto.Dimension)
	if dimension == "" {
		dimension = model.CmdbHealthDimensionOverall
	}

	status, score := normalizeHealthInput(dto.Status, dto.Score)
	evidenceJSON, err := dao.JSONMapToDatatype(dto.Evidence)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	snapshot := &model.CmdbBusinessModelHealthSnapshot{
		BusinessID: binding.BusinessID,
		ResourceID: dto.ResourceID,
		Dimension:  dimension,
		Score:      score,
		Status:     status,
		Source:     strings.TrimSpace(dto.Source),
		Message:    strings.TrimSpace(dto.Message),
		Evidence:   evidenceJSON,
		UpdatedAt:  now,
	}

	if err := s.dao.UpsertBusinessHealthSnapshot(snapshot); err != nil {
		return nil, err
	}

	vo := s.buildHealthDimensionVo(snapshot)
	return &vo, nil
}

// RefreshBusinessModelHealth 先按业务拓扑刷新节点健康，再聚合成业务健康评分。
func (s *CmdbService) RefreshBusinessModelHealth(businessID uint) (*model.BusinessModelSummaryVo, error) {
	graph, err := s.collectBusinessGraph(businessID, 8, "downstream")
	if err != nil {
		return nil, err
	}
	if graph.Root.ResourceType != model.CmdbResourceTypeBusinessSystem {
		return nil, errors.New("资源不是业务模型")
	}

	engine := newBusinessHealthEngine(s)
	report, err := engine.Analyze(context.Background(), graph)
	if err != nil {
		return nil, err
	}
	for resourceID, healthItems := range report.Snapshots {
		for _, health := range healthItems {
			if _, err := s.upsertResourceHealth(businessID, resourceID, health); err != nil {
				return nil, err
			}
		}
	}

	ids := graph.resourceIDs()
	snapshots, err := s.dao.GetBusinessHealthSnapshotsByResourceIDs(businessID, ids)
	if err != nil {
		return nil, err
	}
	snapshotMap := groupSnapshotsByResourceID(snapshots)
	aggregateResources := graph.descendantResources()
	if len(aggregateResources) == 0 {
		aggregateResources = []*model.CmdbBusinessModelBinding{graph.Root}
	}

	upsertedSnapshots := make([]model.CmdbBusinessModelHealthSnapshot, 0, 6)
	for _, dimension := range businessHealthDimensions() {
		health := s.aggregateDimensionHealth(aggregateResources, snapshotMap, dimension)
		snapshot, err := s.upsertAggregatedBusinessHealth(businessID, graph.Root.ID, health, dimension)
		if err != nil {
			return nil, err
		}
		upsertedSnapshots = append(upsertedSnapshots, *snapshot)
	}

	overall := s.aggregateOverallHealth(aggregateResources, snapshotMap)
	overallSnapshot, err := s.upsertAggregatedBusinessHealth(businessID, graph.Root.ID, overall, model.CmdbHealthDimensionOverall)
	if err != nil {
		return nil, err
	}
	upsertedSnapshots = append(upsertedSnapshots, *overallSnapshot)

	summary := s.buildBusinessSummaryVo(graph.Root, upsertedSnapshots)
	return &summary, nil
}

func (s *CmdbService) GetBusinessModelOverview(businessID uint) (map[string]interface{}, error) {
	summary, err := s.RefreshBusinessModelHealth(businessID)
	if err != nil {
		return nil, err
	}

	graph, err := s.collectBusinessGraph(businessID, 8, "downstream")
	if err != nil {
		return nil, err
	}
	ids := graph.resourceIDs()
	snapshots, err := s.dao.GetBusinessHealthSnapshotsByResourceIDs(businessID, ids)
	if err != nil {
		return nil, err
	}
	snapshotMap := groupSnapshotsByResourceID(snapshots)

	assetStats := make(map[string]int)
	statusStats := make(map[string]int)
	riskNodes := make([]map[string]interface{}, 0)

	for _, resource := range graph.descendantResources() {
		assetStats[resource.ResourceType]++
		health := s.resolveNodeHealth(resource, snapshotMap[resource.ID])
		statusStats[health.Status]++
		if health.Status == model.CmdbHealthStatusHealthy {
			continue
		}
		riskNodes = append(riskNodes, map[string]interface{}{
			"id":             resource.ID,
			"name":           resource.Name,
			"displayName":    resourceDisplayName(resource),
			"resourceType":   resource.ResourceType,
			"healthScore":    health.Score,
			"healthStatus":   health.Status,
			"healthColor":    healthColor(health.Status),
			"dimension":      health.Dimension,
			"dimensionLabel": healthDimensionLabel(health.Dimension),
			"message":        health.Message,
			"sourceType":     resource.SourceType,
			"sourceId":       resource.SourceID,
			"sourceKey":      resource.SourceKey,
		})
	}

	sort.Slice(riskNodes, func(i, j int) bool {
		leftStatus, _ := riskNodes[i]["healthStatus"].(string)
		rightStatus, _ := riskNodes[j]["healthStatus"].(string)
		if healthSeverity(leftStatus) != healthSeverity(rightStatus) {
			return healthSeverity(leftStatus) > healthSeverity(rightStatus)
		}
		leftScore, _ := riskNodes[i]["healthScore"].(int)
		rightScore, _ := riskNodes[j]["healthScore"].(int)
		return leftScore < rightScore
	})
	if len(riskNodes) > 10 {
		riskNodes = riskNodes[:10]
	}

	nodeSnapshotMap := groupNodeHealthByResourceID(snapshots)
	engine := newBusinessHealthEngine(s)
	return map[string]interface{}{
		"summary":        summary,
		"dimensions":     summary.Dimensions,
		"assetStats":     assetStats,
		"statusStats":    statusStats,
		"riskNodes":      riskNodes,
		"questions":      s.buildLeadershipQuestions(summary, assetStats, statusStats),
		"coverage":       engine.buildCoverage(graph, nodeSnapshotMap),
		"scoreBreakdown": engine.buildScoreBreakdown(graph, nodeSnapshotMap),
		"evidence":       s.buildBusinessOverviewEvidence(graph, snapshotMap),
		"analysisWindow": businessHealthAnalysisWindow,
	}, nil
}

func (s *CmdbService) GetBusinessModelTopology(businessID uint, depth int, direction string) (map[string]interface{}, error) {
	return s.getBusinessModelTopology(businessID, depth, direction, true)
}

func (s *CmdbService) getBusinessModelTopology(businessID uint, depth int, direction string, refreshHealth bool) (map[string]interface{}, error) {
	if depth <= 0 {
		depth = 4
	}
	if depth > 8 {
		depth = 8
	}
	direction = normalizeGraphDirection(direction)

	if refreshHealth {
		if _, err := s.RefreshBusinessModelHealth(businessID); err != nil {
			return nil, err
		}
	}

	graph, err := s.collectBusinessGraph(businessID, depth, direction)
	if err != nil {
		return nil, err
	}

	ids := graph.resourceIDs()
	snapshots, err := s.dao.GetBusinessHealthSnapshotsByResourceIDs(businessID, ids)
	if err != nil {
		return nil, err
	}
	snapshotMap := groupSnapshotsByResourceID(snapshots)

	nodes := make([]map[string]interface{}, 0, len(graph.Resources))
	resources := graph.sortedResources()
	for _, resource := range resources {
		health := s.resolveNodeHealth(resource, snapshotMap[resource.ID])
		nodes = append(nodes, s.buildBusinessTopologyNode(resource, health, snapshotMap[resource.ID]))
	}

	edges := make([]map[string]interface{}, 0, len(graph.Relations))
	relations := graph.sortedRelations()
	for _, relation := range relations {
		edges = append(edges, map[string]interface{}{
			"id":           relation.ID,
			"source":       relation.SrcID,
			"target":       relation.DstID,
			"sourceId":     relation.SrcID,
			"targetId":     relation.DstID,
			"srcId":        relation.SrcID,
			"dstId":        relation.DstID,
			"relationType": relation.RelationType,
			"label":        getRelationLabel(relation.RelationType),
		})
	}

	return map[string]interface{}{
		"businessId": businessID,
		"rootId":     graph.Root.ID,
		"nodes":      nodes,
		"edges":      edges,
		"depth":      depth,
		"direction":  direction,
	}, nil
}

func (s *CmdbService) upsertBusinessDefaultHealth(businessID, resourceID uint) error {
	defaults := []model.CmdbBusinessModelHealthSnapshot{
		{
			BusinessID: businessID,
			ResourceID: resourceID,
			Dimension:  model.CmdbHealthDimensionOverall,
			Score:      0,
			Status:     model.CmdbHealthStatusUnknown,
			Source:     "cmdb_model",
			Message:    "业务模型已创建，等待绑定资产和健康数据",
		},
	}
	for i := range defaults {
		defaults[i].UpdatedAt = time.Now()
		if err := s.dao.UpsertBusinessHealthSnapshot(&defaults[i]); err != nil {
			return err
		}
	}
	return nil
}

func (s *CmdbService) upsertAggregatedBusinessHealth(businessID, resourceID uint, health nodeHealth, dimension string) (*model.CmdbBusinessModelHealthSnapshot, error) {
	evidence, err := dao.JSONMapToDatatype(health.Evidence)
	if err != nil {
		return nil, err
	}
	snapshot := &model.CmdbBusinessModelHealthSnapshot{
		BusinessID: businessID,
		ResourceID: resourceID,
		Dimension:  dimension,
		Score:      health.Score,
		Status:     health.Status,
		Source:     "cmdb_model",
		Message:    health.Message,
		Evidence:   evidence,
		UpdatedAt:  time.Now(),
	}
	return snapshot, s.dao.UpsertBusinessHealthSnapshot(snapshot)
}

func (s *CmdbService) upsertResourceHealth(businessID, resourceID uint, health nodeHealth) (*model.CmdbBusinessModelHealthSnapshot, error) {
	evidence, err := dao.JSONMapToDatatype(health.Evidence)
	if err != nil {
		return nil, err
	}
	source := strings.TrimSpace(health.Source)
	if source == "" {
		source = businessHealthEngineSource
	}
	dimension := normalizeHealthDimension(health.Dimension)
	if dimension == "" {
		dimension = model.CmdbHealthDimensionOverall
	}
	status, score := normalizeHealthInput(health.Status, health.Score)
	snapshot := &model.CmdbBusinessModelHealthSnapshot{
		BusinessID: businessID,
		ResourceID: resourceID,
		Dimension:  dimension,
		Score:      clampScore(score),
		Status:     status,
		Source:     source,
		Message:    health.Message,
		Evidence:   evidence,
		UpdatedAt:  time.Now(),
	}
	return snapshot, s.dao.UpsertBusinessHealthSnapshot(snapshot)
}

type businessGraphData struct {
	Root      *model.CmdbBusinessModelBinding
	Resources map[uint]*model.CmdbBusinessModelBinding
	Relations map[string]*model.CmdbBusinessModelRelation
}

func (g *businessGraphData) resourceIDs() []uint {
	ids := make([]uint, 0, len(g.Resources))
	for id := range g.Resources {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func (g *businessGraphData) sortedResources() []*model.CmdbBusinessModelBinding {
	resources := make([]*model.CmdbBusinessModelBinding, 0, len(g.Resources))
	if g.Root != nil {
		resources = append(resources, g.Root)
	}
	ids := g.resourceIDs()
	for _, id := range ids {
		if g.Root != nil && id == g.Root.ID {
			continue
		}
		resources = append(resources, g.Resources[id])
	}
	return resources
}

func (g *businessGraphData) descendantResources() []*model.CmdbBusinessModelBinding {
	resources := make([]*model.CmdbBusinessModelBinding, 0, len(g.Resources))
	ids := g.resourceIDs()
	for _, id := range ids {
		if g.Root != nil && id == g.Root.ID {
			continue
		}
		resources = append(resources, g.Resources[id])
	}
	return resources
}

func (g *businessGraphData) sortedRelations() []*model.CmdbBusinessModelRelation {
	keys := make([]string, 0, len(g.Relations))
	for key := range g.Relations {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	relations := make([]*model.CmdbBusinessModelRelation, 0, len(keys))
	for _, key := range keys {
		relations = append(relations, g.Relations[key])
	}
	return relations
}

func (s *CmdbService) collectBusinessGraph(rootID uint, depth int, direction string) (*businessGraphData, error) {
	if depth <= 0 {
		depth = 4
	}
	if depth > 8 {
		depth = 8
	}
	direction = normalizeGraphDirection(direction)

	if _, err := s.dao.GetBusinessModelByID(rootID); err != nil {
		return nil, err
	}
	root, err := s.dao.GetBusinessRootBinding(rootID)
	if err != nil {
		return nil, err
	}
	businessID := root.BusinessID

	graph := &businessGraphData{
		Root:      root,
		Resources: map[uint]*model.CmdbBusinessModelBinding{root.ID: root},
		Relations: make(map[string]*model.CmdbBusinessModelRelation),
	}
	visited := map[uint]bool{root.ID: true}

	var walk func(currentID uint, currentDepth int)
	walk = func(currentID uint, currentDepth int) {
		if currentDepth > depth {
			return
		}

		if direction == "upstream" || direction == "both" {
			relations, err := s.dao.GetBusinessUpstreamRelationsData(businessID, currentID)
			if err == nil {
				for _, relation := range relations {
					relationCopy := relation
					edgeKey := formatEdgeKey(relationCopy.SrcID, relationCopy.DstID, relationCopy.RelationType)
					graph.Relations[edgeKey] = &relationCopy
					if relation.Src != nil {
						resourceCopy := *relation.Src
						graph.Resources[resourceCopy.ID] = &resourceCopy
					}
					if !visited[relationCopy.SrcID] {
						visited[relationCopy.SrcID] = true
						walk(relationCopy.SrcID, currentDepth+1)
					}
				}
			}
		}

		if direction == "downstream" || direction == "both" {
			relations, err := s.dao.GetBusinessDownstreamRelationsData(businessID, currentID)
			if err == nil {
				for _, relation := range relations {
					relationCopy := relation
					edgeKey := formatEdgeKey(relationCopy.SrcID, relationCopy.DstID, relationCopy.RelationType)
					graph.Relations[edgeKey] = &relationCopy
					if relation.Dst != nil {
						resourceCopy := *relation.Dst
						graph.Resources[resourceCopy.ID] = &resourceCopy
					}
					if !visited[relationCopy.DstID] {
						visited[relationCopy.DstID] = true
						walk(relationCopy.DstID, currentDepth+1)
					}
				}
			}
		}
	}

	walk(root.ID, 1)
	return graph, nil
}

type nodeHealth struct {
	Dimension string
	Score     int
	Status    string
	Source    string
	Message   string
	Evidence  map[string]interface{}
	UpdatedAt *time.Time
}

func (s *CmdbService) resolveNodeHealth(resource *model.CmdbBusinessModelBinding, snapshots []model.CmdbBusinessModelHealthSnapshot) nodeHealth {
	if snapshot := findSnapshotByDimension(snapshots, model.CmdbHealthDimensionOverall); snapshot != nil {
		return snapshotToNodeHealth(*snapshot)
	}

	var worst *nodeHealth
	for _, snapshot := range snapshots {
		health := snapshotToNodeHealth(snapshot)
		if worst == nil || isWorseHealth(health, *worst) {
			healthCopy := health
			worst = &healthCopy
		}
	}
	if worst != nil {
		return *worst
	}

	return derivedHealthFromResource(resource)
}

func (s *CmdbService) aggregateOverallHealth(resources []*model.CmdbBusinessModelBinding, snapshots map[uint][]model.CmdbBusinessModelHealthSnapshot) nodeHealth {
	var worst nodeHealth
	hasHealth := false
	affectedCount := 0

	for _, resource := range resources {
		health := s.resolveNodeHealth(resource, snapshots[resource.ID])
		if health.Status != model.CmdbHealthStatusHealthy {
			affectedCount++
		}
		if !hasHealth || isWorseHealth(health, worst) {
			worst = health
			hasHealth = true
		}
	}

	if !hasHealth {
		return unknownHealth(model.CmdbHealthDimensionOverall, "暂无业务健康数据")
	}

	worst.Message = businessHealthMessage(model.CmdbHealthDimensionOverall, worst, affectedCount, len(resources))
	worst.Evidence = mergeEvidence(worst.Evidence, map[string]interface{}{
		"affectedCount": affectedCount,
		"nodeCount":     len(resources),
	})
	return worst
}

func (s *CmdbService) aggregateDimensionHealth(resources []*model.CmdbBusinessModelBinding, snapshots map[uint][]model.CmdbBusinessModelHealthSnapshot, dimension string) nodeHealth {
	var worst nodeHealth
	hasDimension := false
	affectedCount := 0

	for _, resource := range resources {
		snapshot := findSnapshotByDimension(snapshots[resource.ID], dimension)
		if snapshot == nil {
			continue
		}
		health := snapshotToNodeHealth(*snapshot)
		health.Evidence = mergeEvidence(health.Evidence, map[string]interface{}{
			"resourceId":   resource.ID,
			"resourceName": resourceDisplayName(resource),
			"resourceType": resource.ResourceType,
		})
		if health.Status != model.CmdbHealthStatusHealthy {
			affectedCount++
		}
		if !hasDimension || isWorseHealth(health, worst) {
			worst = health
			hasDimension = true
		}
	}

	if !hasDimension {
		return unknownHealth(dimension, fmt.Sprintf("暂无%s健康数据", healthDimensionLabel(dimension)))
	}

	worst.Message = businessHealthMessage(dimension, worst, affectedCount, len(resources))
	worst.Evidence = mergeEvidence(worst.Evidence, map[string]interface{}{
		"affectedCount": affectedCount,
		"nodeCount":     len(resources),
	})
	return worst
}

func (s *CmdbService) buildBusinessTopologyNode(resource *model.CmdbBusinessModelBinding, health nodeHealth, snapshots []model.CmdbBusinessModelHealthSnapshot) map[string]interface{} {
	props := jsonToMap(resource.Properties)

	dimensions := make([]model.HealthDimensionVo, 0, len(snapshots))
	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].Dimension < snapshots[j].Dimension
	})
	for i := range snapshots {
		vo := s.buildHealthDimensionVo(&snapshots[i])
		dimensions = append(dimensions, vo)
	}

	return map[string]interface{}{
		"id":             resource.ID,
		"name":           resource.Name,
		"label":          resourceDisplayName(resource),
		"displayName":    resourceDisplayName(resource),
		"resourceType":   resource.ResourceType,
		"resourceLabel":  resourceTypeLabel(resource.ResourceType),
		"nodeRole":       resourceNodeRole(resource.ResourceType),
		"status":         resource.Status,
		"sourceType":     resource.SourceType,
		"sourceId":       resource.SourceID,
		"sourceKey":      resource.SourceKey,
		"env":            resource.Env,
		"owner":          resource.Owner,
		"bizCode":        resource.BizCode,
		"healthScore":    health.Score,
		"healthStatus":   health.Status,
		"healthColor":    healthColor(health.Status),
		"color":          healthColor(health.Status),
		"typeColor":      resourceTypeColor(resource.ResourceType),
		"problemSummary": health.Message,
		"dimension":      health.Dimension,
		"dimensions":     dimensions,
		"evidence":       health.Evidence,
		"diagnostics":    diagnosisEntries(resource),
		"properties":     props,
	}
}

func (s *CmdbService) buildBusinessSummaryVo(resource *model.CmdbBusinessModelBinding, snapshots []model.CmdbBusinessModelHealthSnapshot) model.BusinessModelSummaryVo {
	snapshotMap := groupSnapshotsByDimension(snapshots)
	overall := unknownHealth(model.CmdbHealthDimensionOverall, "暂无业务健康数据")
	if snapshot, ok := snapshotMap[model.CmdbHealthDimensionOverall]; ok {
		overall = snapshotToNodeHealth(snapshot)
	} else {
		overall = derivedHealthFromResource(resource)
	}

	dimensions := make([]model.HealthDimensionVo, 0, 5)
	for _, dimension := range businessHealthDimensions() {
		snapshot, ok := snapshotMap[dimension]
		if !ok {
			dimensions = append(dimensions, model.HealthDimensionVo{
				Dimension: dimension,
				Label:     healthDimensionLabel(dimension),
				Score:     0,
				Status:    model.CmdbHealthStatusUnknown,
				Color:     healthColor(model.CmdbHealthStatusUnknown),
				Message:   fmt.Sprintf("暂无%s健康数据", healthDimensionLabel(dimension)),
			})
			continue
		}
		dimensions = append(dimensions, s.buildHealthDimensionVo(&snapshot))
	}

	return model.BusinessModelSummaryVo{
		ID:             resource.ID,
		Name:           resource.Name,
		DisplayName:    resourceDisplayName(resource),
		ResourceType:   resource.ResourceType,
		Env:            resource.Env,
		Owner:          resource.Owner,
		BizCode:        resource.BizCode,
		HealthScore:    overall.Score,
		HealthStatus:   overall.Status,
		HealthColor:    healthColor(overall.Status),
		ProblemSummary: overall.Message,
		Dimensions:     dimensions,
		UpdatedAt:      resource.UpdatedAt,
	}
}

func (s *CmdbService) buildHealthDimensionVo(snapshot *model.CmdbBusinessModelHealthSnapshot) model.HealthDimensionVo {
	if snapshot == nil {
		return model.HealthDimensionVo{}
	}
	updatedAt := snapshot.UpdatedAt
	return model.HealthDimensionVo{
		Dimension: snapshot.Dimension,
		Label:     healthDimensionLabel(snapshot.Dimension),
		Score:     snapshot.Score,
		Status:    snapshot.Status,
		Color:     healthColor(snapshot.Status),
		Message:   snapshot.Message,
		Evidence:  jsonToMap(snapshot.Evidence),
		UpdatedAt: &updatedAt,
	}
}

func (s *CmdbService) buildLeadershipQuestions(summary *model.BusinessModelSummaryVo, assetStats map[string]int, statusStats map[string]int) []map[string]interface{} {
	infra := findDimensionVo(summary.Dimensions, model.CmdbHealthDimensionInfra)
	business := findDimensionVo(summary.Dimensions, model.CmdbHealthDimensionBusiness)

	totalAssets := 0
	for _, count := range assetStats {
		totalAssets += count
	}
	abnormalAssets := statusStats[model.CmdbHealthStatusWarning] + statusStats[model.CmdbHealthStatusCritical]

	return []map[string]interface{}{
		{
			"question": "业务是否稳定",
			"status":   summary.HealthStatus,
			"color":    summary.HealthColor,
			"answer":   summary.ProblemSummary,
		},
		{
			"question": "资产是否充足",
			"status":   infra.Status,
			"color":    infra.Color,
			"answer":   fmt.Sprintf("当前已关联 %d 个资产节点，异常节点 %d 个；容量充足性由基础资源维度体现", totalAssets, abnormalAssets),
		},
		{
			"question": "是否存在业务压力",
			"status":   business.Status,
			"color":    business.Color,
			"answer":   business.Message,
		},
	}
}

func fallbackResourceName(item model.BusinessModelBindingResourceDto) string {
	if value := bindingResourceDisplayName(item); value != "" {
		return value
	}
	if value := strings.TrimSpace(item.SourceKey); value != "" {
		return value
	}
	if strings.TrimSpace(item.SourceType) != "" && item.SourceID > 0 {
		return fmt.Sprintf("%s-%d", strings.TrimSpace(item.SourceType), item.SourceID)
	}
	return ""
}

func bindingResourceName(item model.BusinessModelBindingResourceDto) string {
	return strings.TrimSpace(item.Name)
}

func bindingResourceDisplayName(item model.BusinessModelBindingResourceDto) string {
	if value := strings.TrimSpace(item.DisplayName); value != "" {
		return value
	}
	return strings.TrimSpace(item.Label)
}

func bindingResourceType(item model.BusinessModelBindingResourceDto) string {
	if value := normalizeResourceType(item.ResourceType); value != "" {
		return value
	}
	value := normalizeResourceType(item.Type)
	if isKnownBusinessResourceType(value) {
		return value
	}
	return ""
}

func isKnownBusinessResourceType(resourceType string) bool {
	switch resourceType {
	case model.CmdbResourceTypeBusinessSystem,
		model.CmdbResourceTypeBusinessService,
		model.CmdbResourceTypeApplication,
		model.CmdbResourceTypeNginx,
		model.CmdbResourceTypeGateway,
		model.CmdbResourceTypePhysicalServer,
		model.CmdbResourceTypeHost,
		model.CmdbResourceTypeVM,
		model.CmdbResourceTypeCloudHost,
		model.CmdbResourceTypeK8sCluster,
		model.CmdbResourceTypeK8sNamespace,
		model.CmdbResourceTypeK8sWorkload,
		model.CmdbResourceTypeDatabase,
		model.CmdbResourceTypeMiddleware,
		model.CmdbResourceTypeLogProject,
		model.CmdbResourceTypeLogStore:
		return true
	default:
		return false
	}
}

func normalizeResourceType(resourceType string) string {
	return convertToDbType(strings.TrimSpace(strings.ToLower(resourceType)))
}

func inferResourceTypeFromSource(sourceType string) string {
	switch strings.TrimSpace(strings.ToLower(sourceType)) {
	case "cmdb_host", "host", "server":
		return model.CmdbResourceTypeHost
	case "cmdb_idc_asset", "physical", "physical_server", "physical-server":
		return model.CmdbResourceTypePhysicalServer
	case "cmdb_idc_vm", "vm", "virtual", "virtual_server":
		return model.CmdbResourceTypeVM
	case "db", "database", "mysql", "redis", "postgresql", "pg":
		return model.CmdbResourceTypeDatabase
	case "k8s", "k8s_cluster", "k8s-cluster":
		return model.CmdbResourceTypeK8sCluster
	case "k8s_namespace", "k8s-namespace", "namespace", "ns":
		return model.CmdbResourceTypeK8sNamespace
	case "k8s_workload", "k8s-workload", "pod", "deployment", "statefulset":
		return model.CmdbResourceTypeK8sWorkload
	case "nginx", "openresty", "ingress_nginx", "ingress-nginx":
		return model.CmdbResourceTypeNginx
	case "gateway", "api_gateway", "api-gateway", "网关":
		return model.CmdbResourceTypeGateway
	case "middleware", "kafka", "rocketmq", "rabbitmq":
		return model.CmdbResourceTypeMiddleware
	case "log_project", "log-project", "logproject", "logtail_project", "logtail-project":
		return model.CmdbResourceTypeLogProject
	case "log_store", "log-store", "logstore", "logtail_store", "logtail-store":
		return model.CmdbResourceTypeLogStore
	case "service", "business_service", "business-service":
		return model.CmdbResourceTypeBusinessService
	case "application", "app":
		return model.CmdbResourceTypeApplication
	default:
		return ""
	}
}

func normalizeRelationType(relationType string) string {
	switch strings.TrimSpace(strings.ToLower(relationType)) {
	case "contains", "contain", "include", "包含":
		return model.CmdbRelationContains
	case "runs_on", "runson", "runs-on", "运行于":
		return model.CmdbRelationRunsOn
	case "connects_to", "connects-to", "connect", "连接":
		return model.CmdbRelationConnectsTo
	case "depends_on", "depends-on", "depend", "依赖":
		return model.CmdbRelationDependsOn
	case "deployed_on", "deployed-on", "部署于":
		return model.CmdbRelationDeployedOn
	case "uses", "use", "使用":
		return model.CmdbRelationUses
	case "emits_log_to", "emits-log-to", "emit_log", "emit-log", "log", "logs", "产生日志", "日志":
		return model.CmdbRelationEmitsLogTo
	default:
		return strings.TrimSpace(strings.ToLower(relationType))
	}
}

func defaultRelationType(parentType, childType string) string {
	if childType == model.CmdbResourceTypeLogProject || childType == model.CmdbResourceTypeLogStore {
		return model.CmdbRelationEmitsLogTo
	}
	if parentType == model.CmdbResourceTypeBusinessSystem || parentType == model.CmdbResourceTypeBusinessService {
		return model.CmdbRelationContains
	}
	if childType == model.CmdbResourceTypeHost ||
		childType == model.CmdbResourceTypePhysicalServer ||
		childType == model.CmdbResourceTypeVM ||
		childType == model.CmdbResourceTypeCloudHost ||
		childType == model.CmdbResourceTypeK8sCluster ||
		childType == model.CmdbResourceTypeK8sNamespace ||
		childType == model.CmdbResourceTypeK8sWorkload {
		return model.CmdbRelationDeployedOn
	}
	return model.CmdbRelationDependsOn
}

func normalizeGraphDirection(direction string) string {
	switch strings.TrimSpace(strings.ToLower(direction)) {
	case "upstream", "downstream", "both":
		return strings.TrimSpace(strings.ToLower(direction))
	default:
		return "downstream"
	}
}

func normalizeHealthDimension(dimension string) string {
	switch strings.TrimSpace(strings.ToLower(dimension)) {
	case "", "overall", "all", "总览", "综合":
		return model.CmdbHealthDimensionOverall
	case "infra", "base", "basic", "host", "cmdb", "基础", "基础监控":
		return model.CmdbHealthDimensionInfra
	case "middleware", "mid", "中间件", "中间件监控":
		return model.CmdbHealthDimensionMiddleware
	case "container", "k8s", "pod", "容器", "容器监控":
		return model.CmdbHealthDimensionContainer
	case "business", "biz", "service", "业务", "业务监控":
		return model.CmdbHealthDimensionBusiness
	case "log", "logs", "aiops", "日志", "日志监控":
		return model.CmdbHealthDimensionLog
	default:
		return strings.TrimSpace(strings.ToLower(dimension))
	}
}

func normalizeHealthInput(status string, score int) (string, int) {
	normalizedStatus := normalizeHealthStatusName(status)
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}
	if score == 0 {
		switch normalizedStatus {
		case model.CmdbHealthStatusHealthy:
			score = 100
		case model.CmdbHealthStatusWarning:
			score = 75
		case model.CmdbHealthStatusUnknown:
			score = 0
		}
	}
	if normalizedStatus == "" {
		normalizedStatus = statusByScore(score)
	}
	return normalizedStatus, score
}

func normalizeHealthStatusName(status string) string {
	switch strings.TrimSpace(strings.ToLower(status)) {
	case "healthy", "success", "green", "ok", "online", "active", "running", "正常", "绿色":
		return model.CmdbHealthStatusHealthy
	case "warning", "warn", "yellow", "degraded", "maintenance", "轻微问题", "告警", "黄色":
		return model.CmdbHealthStatusWarning
	case "critical", "error", "danger", "red", "failed", "offline", "down", "故障", "严重", "红色":
		return model.CmdbHealthStatusCritical
	case "unknown", "gray", "grey", "未知", "无数据":
		return model.CmdbHealthStatusUnknown
	case "":
		return ""
	default:
		return model.CmdbHealthStatusUnknown
	}
}

func statusByScore(score int) string {
	switch {
	case score >= 90:
		return model.CmdbHealthStatusHealthy
	case score >= 60:
		return model.CmdbHealthStatusWarning
	case score > 0:
		return model.CmdbHealthStatusCritical
	default:
		return model.CmdbHealthStatusCritical
	}
}

func healthSeverity(status string) int {
	switch normalizeHealthStatusName(status) {
	case model.CmdbHealthStatusCritical:
		return 4
	case model.CmdbHealthStatusWarning:
		return 3
	case model.CmdbHealthStatusUnknown:
		return 2
	case model.CmdbHealthStatusHealthy:
		return 1
	default:
		return 2
	}
}

func isWorseHealth(candidate, current nodeHealth) bool {
	if healthSeverity(candidate.Status) != healthSeverity(current.Status) {
		return healthSeverity(candidate.Status) > healthSeverity(current.Status)
	}
	return candidate.Score < current.Score
}

func businessHealthDimensions() []string {
	return []string{
		model.CmdbHealthDimensionInfra,
		model.CmdbHealthDimensionMiddleware,
		model.CmdbHealthDimensionContainer,
		model.CmdbHealthDimensionBusiness,
		model.CmdbHealthDimensionLog,
	}
}

func healthDimensionLabel(dimension string) string {
	switch dimension {
	case model.CmdbHealthDimensionOverall:
		return "综合健康"
	case model.CmdbHealthDimensionInfra:
		return "基础资源"
	case model.CmdbHealthDimensionMiddleware:
		return "中间件"
	case model.CmdbHealthDimensionContainer:
		return "容器"
	case model.CmdbHealthDimensionBusiness:
		return "业务监控"
	case model.CmdbHealthDimensionLog:
		return "日志"
	default:
		return dimension
	}
}

func healthColor(status string) string {
	switch normalizeHealthStatusName(status) {
	case model.CmdbHealthStatusHealthy:
		return "#16a34a"
	case model.CmdbHealthStatusWarning:
		return "#f59e0b"
	case model.CmdbHealthStatusCritical:
		return "#dc2626"
	default:
		return "#94a3b8"
	}
}

func resourceTypeLabel(resourceType string) string {
	switch resourceType {
	case model.CmdbResourceTypeBusinessSystem:
		return "业务系统"
	case model.CmdbResourceTypeBusinessService:
		return "业务服务"
	case model.CmdbResourceTypeApplication:
		return "应用"
	case model.CmdbResourceTypeNginx:
		return "Nginx"
	case model.CmdbResourceTypeGateway:
		return "Gateway网关"
	case model.CmdbResourceTypePhysicalServer:
		return "物理机"
	case model.CmdbResourceTypeHost:
		return "主机"
	case model.CmdbResourceTypeVM:
		return "虚拟机"
	case model.CmdbResourceTypeCloudHost:
		return "云主机"
	case model.CmdbResourceTypeK8sCluster:
		return "K8S集群"
	case model.CmdbResourceTypeK8sNamespace:
		return "命名空间"
	case model.CmdbResourceTypeK8sWorkload:
		return "K8S工作负载"
	case model.CmdbResourceTypeDatabase:
		return "数据库"
	case model.CmdbResourceTypeMiddleware:
		return "中间件"
	case model.CmdbResourceTypeLogProject:
		return "日志项目"
	case model.CmdbResourceTypeLogStore:
		return "日志库"
	default:
		return resourceType
	}
}

func resourceTypeColor(resourceType string) string {
	switch resourceType {
	case model.CmdbResourceTypeBusinessSystem:
		return "#0f766e"
	case model.CmdbResourceTypeBusinessService, model.CmdbResourceTypeApplication, model.CmdbResourceTypeNginx, model.CmdbResourceTypeGateway:
		return "#2563eb"
	case model.CmdbResourceTypePhysicalServer, model.CmdbResourceTypeHost:
		return "#475569"
	case model.CmdbResourceTypeVM, model.CmdbResourceTypeCloudHost:
		return "#0891b2"
	case model.CmdbResourceTypeK8sCluster, model.CmdbResourceTypeK8sNamespace, model.CmdbResourceTypeK8sWorkload:
		return "#7c3aed"
	case model.CmdbResourceTypeDatabase:
		return "#b45309"
	case model.CmdbResourceTypeMiddleware:
		return "#c2410c"
	case model.CmdbResourceTypeLogProject, model.CmdbResourceTypeLogStore:
		return "#64748b"
	default:
		return "#64748b"
	}
}

func resourceNodeRole(resourceType string) string {
	switch resourceType {
	case model.CmdbResourceTypeLogProject, model.CmdbResourceTypeLogStore:
		return "observability"
	default:
		return "primary"
	}
}

func businessToSummaryResource(business *model.CmdbBusinessModel) *model.CmdbBusinessModelBinding {
	if business == nil {
		return nil
	}
	return &model.CmdbBusinessModelBinding{
		Model: gorm.Model{
			ID:        business.ID,
			CreatedAt: business.CreatedAt,
			UpdatedAt: business.UpdatedAt,
			DeletedAt: business.DeletedAt,
		},
		BusinessID:   business.ID,
		Name:         business.Name,
		DisplayName:  business.DisplayName,
		ResourceType: model.CmdbResourceTypeBusinessSystem,
		Env:          business.Env,
		Owner:        business.Owner,
		BizCode:      business.BizCode,
		Properties:   business.Properties,
		Status:       business.Status,
	}
}

func resourceDisplayName(resource *model.CmdbBusinessModelBinding) string {
	if resource == nil {
		return ""
	}
	if strings.TrimSpace(resource.DisplayName) != "" {
		return strings.TrimSpace(resource.DisplayName)
	}
	return resource.Name
}

func formatEdgeKey(srcID, dstID uint, relationType string) string {
	return fmt.Sprintf("%d-%d-%s", srcID, dstID, relationType)
}

func groupSnapshotsByResourceID(snapshots []model.CmdbBusinessModelHealthSnapshot) map[uint][]model.CmdbBusinessModelHealthSnapshot {
	result := make(map[uint][]model.CmdbBusinessModelHealthSnapshot)
	for _, snapshot := range snapshots {
		result[snapshot.ResourceID] = append(result[snapshot.ResourceID], snapshot)
	}
	return result
}

func groupNodeHealthByResourceID(snapshots []model.CmdbBusinessModelHealthSnapshot) map[uint][]nodeHealth {
	result := make(map[uint][]nodeHealth)
	for _, snapshot := range snapshots {
		result[snapshot.ResourceID] = append(result[snapshot.ResourceID], snapshotToNodeHealth(snapshot))
	}
	return result
}

func (s *CmdbService) buildBusinessOverviewEvidence(graph *businessGraphData, snapshots map[uint][]model.CmdbBusinessModelHealthSnapshot) []map[string]interface{} {
	evidence := make([]map[string]interface{}, 0, len(graph.Resources))
	for _, resource := range graph.sortedResources() {
		health := s.resolveNodeHealth(resource, snapshots[resource.ID])
		evidence = append(evidence, map[string]interface{}{
			"resourceId":     resource.ID,
			"resourceName":   resourceDisplayName(resource),
			"resourceType":   resource.ResourceType,
			"resourceLabel":  resourceTypeLabel(resource.ResourceType),
			"healthStatus":   health.Status,
			"healthScore":    health.Score,
			"dimension":      health.Dimension,
			"message":        health.Message,
			"evidence":       health.Evidence,
			"diagnostics":    diagnosisEntries(resource),
			"sourceType":     resource.SourceType,
			"sourceId":       resource.SourceID,
			"sourceKey":      resource.SourceKey,
			"analysisWindow": businessHealthAnalysisWindow,
		})
	}
	return evidence
}

func groupSnapshotsByDimension(snapshots []model.CmdbBusinessModelHealthSnapshot) map[string]model.CmdbBusinessModelHealthSnapshot {
	result := make(map[string]model.CmdbBusinessModelHealthSnapshot)
	for _, snapshot := range snapshots {
		result[snapshot.Dimension] = snapshot
	}
	return result
}

func findSnapshotByDimension(snapshots []model.CmdbBusinessModelHealthSnapshot, dimension string) *model.CmdbBusinessModelHealthSnapshot {
	for i := range snapshots {
		if snapshots[i].Dimension == dimension {
			return &snapshots[i]
		}
	}
	return nil
}

func findDimensionVo(dimensions []model.HealthDimensionVo, dimension string) model.HealthDimensionVo {
	for _, item := range dimensions {
		if item.Dimension == dimension {
			return item
		}
	}
	return model.HealthDimensionVo{
		Dimension: dimension,
		Label:     healthDimensionLabel(dimension),
		Score:     0,
		Status:    model.CmdbHealthStatusUnknown,
		Color:     healthColor(model.CmdbHealthStatusUnknown),
		Message:   fmt.Sprintf("暂无%s健康数据", healthDimensionLabel(dimension)),
	}
}

func snapshotToNodeHealth(snapshot model.CmdbBusinessModelHealthSnapshot) nodeHealth {
	status, score := normalizeHealthInput(snapshot.Status, snapshot.Score)
	updatedAt := snapshot.UpdatedAt
	return nodeHealth{
		Dimension: snapshot.Dimension,
		Score:     score,
		Status:    status,
		Source:    snapshot.Source,
		Message:   snapshot.Message,
		Evidence:  jsonToMap(snapshot.Evidence),
		UpdatedAt: &updatedAt,
	}
}

func derivedHealthFromResource(resource *model.CmdbBusinessModelBinding) nodeHealth {
	status := normalizeHealthStatusName(resource.Status)
	if status == "" {
		status = model.CmdbHealthStatusUnknown
	}
	score := 0
	switch status {
	case model.CmdbHealthStatusHealthy:
		score = 100
	case model.CmdbHealthStatusWarning:
		score = 75
	case model.CmdbHealthStatusCritical:
		score = 40
	default:
		score = 0
	}
	return nodeHealth{
		Dimension: model.CmdbHealthDimensionOverall,
		Score:     score,
		Status:    status,
		Source:    "cmdb_status",
		Message:   fmt.Sprintf("来源于资源状态: %s", resource.Status),
		Evidence: map[string]interface{}{
			"resourceId":     resource.ID,
			"resourceName":   resourceDisplayName(resource),
			"resourceStatus": resource.Status,
		},
	}
}

func unknownHealth(dimension, message string) nodeHealth {
	return nodeHealth{
		Dimension: dimension,
		Score:     0,
		Status:    model.CmdbHealthStatusUnknown,
		Message:   message,
		Evidence:  map[string]interface{}{},
	}
}

func businessHealthMessage(dimension string, health nodeHealth, affectedCount, nodeCount int) string {
	if health.Message != "" && affectedCount == 0 {
		return health.Message
	}
	label := healthDimensionLabel(dimension)
	switch health.Status {
	case model.CmdbHealthStatusHealthy:
		return fmt.Sprintf("%s正常，已覆盖 %d 个节点", label, nodeCount)
	case model.CmdbHealthStatusWarning:
		return fmt.Sprintf("%s存在轻微问题，影响 %d/%d 个节点，最低分 %d", label, affectedCount, nodeCount, health.Score)
	case model.CmdbHealthStatusCritical:
		return fmt.Sprintf("%s存在严重问题，影响 %d/%d 个节点，最低分 %d", label, affectedCount, nodeCount, health.Score)
	default:
		return fmt.Sprintf("%s健康数据不足，已覆盖 %d 个节点", label, nodeCount)
	}
}

func mergeEvidence(base map[string]interface{}, extra map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{}, len(base)+len(extra))
	for key, value := range base {
		result[key] = value
	}
	for key, value := range extra {
		result[key] = value
	}
	return result
}

func jsonToMap(value datatypes.JSON) map[string]interface{} {
	if len(value) == 0 {
		return map[string]interface{}{}
	}
	var result map[string]interface{}
	if err := json.Unmarshal(value, &result); err != nil {
		return map[string]interface{}{}
	}
	if result == nil {
		return map[string]interface{}{}
	}
	return result
}

// convertToDbType 将业务类型转换为数据库类型
func convertToDbType(businessType string) string {
	typeMap := map[string]string{
		"physical_server":  "physical_server",
		"physical":         "physical_server",
		"physical-server":  "physical_server",
		"server":           "host",
		"host":             "host",
		"virtual":          "vm",
		"virtual-server":   "vm",
		"vm":               "vm",
		"cloud-server":     "cloud_host",
		"cloud_host":       "cloud_host",
		"cloud":            "cloud_host",
		"network_device":   "network_device",
		"network":          "network_device",
		"k8s_cluster":      "k8s_cluster",
		"k8s-cluster":      "k8s_cluster",
		"k8s":              "k8s_cluster",
		"k8s_namespace":    "k8s_namespace",
		"k8s-namespace":    "k8s_namespace",
		"namespace":        "k8s_namespace",
		"ns":               "k8s_namespace",
		"k8s_workload":     "k8s_workload",
		"k8s-workload":     "k8s_workload",
		"workload":         "k8s_workload",
		"pod":              "k8s_workload",
		"deployment":       "k8s_workload",
		"statefulset":      "k8s_workload",
		"database":         "database",
		"db":               "database",
		"mysql":            "database",
		"redis":            "database",
		"postgresql":       "database",
		"pg":               "database",
		"service":          "business_service",
		"business-service": "business_service",
		"business_service": "business_service",
		"business-system":  "business_system",
		"business_system":  "business_system",
		"application":      "application",
		"nginx":            "nginx",
		"openresty":        "nginx",
		"ingress_nginx":    "nginx",
		"ingress-nginx":    "nginx",
		"gateway":          "gateway",
		"api_gateway":      "gateway",
		"api-gateway":      "gateway",
		"网关":               "gateway",
		"middleware":       "middleware",
		"log_project":      "log_project",
		"log-project":      "log_project",
		"logproject":       "log_project",
		"logtail_project":  "log_project",
		"logtail-project":  "log_project",
		"log_store":        "log_store",
		"log-store":        "log_store",
		"logstore":         "log_store",
		"logtail_store":    "log_store",
		"logtail-store":    "log_store",
		"idc":              "idc",
		"cdn":              "cdn",
		"domain":           "domain",
		"cos_bucket":       "cos_bucket",
		"dev_app":          "dev_app",
	}
	normalized := strings.TrimSpace(strings.ToLower(businessType))
	if dbType, ok := typeMap[normalized]; ok {
		return dbType
	}
	return normalized
}
