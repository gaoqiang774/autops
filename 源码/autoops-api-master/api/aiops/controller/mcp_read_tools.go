package controller

import (
	"encoding/json"
	"errors"
	"sort"
	"strings"

	aiopsmodel "dodevops-api/api/aiops/model"
	aiopsservice "dodevops-api/api/aiops/service"
	alertdao "dodevops-api/api/alertmanager/dao"
	alertmodel "dodevops-api/api/alertmanager/model"
	cmdbdao "dodevops-api/api/cmdb/dao"
	cmdbmodel "dodevops-api/api/cmdb/model"
	dbmodel "dodevops-api/api/db/model"
	dbservice "dodevops-api/api/db/service"
	k8smodel "dodevops-api/api/k8s/model"
	logtailmodel "dodevops-api/api/logtail/model"
	logtailservice "dodevops-api/api/logtail/service"
	systemauthz "dodevops-api/api/system/authz"
	systemdao "dodevops-api/api/system/dao"
	"dodevops-api/common"
	"dodevops-api/pkg/jwt"

	"github.com/gin-gonic/gin"
)

const mcpReadHandlerKey = "mcp_read"

func invokeMCPReadTool(c *gin.Context, tool aiopsmodel.MCPToolView, args map[string]interface{}, runID uint64) (interface{}, error) {
	switch tool.Name {
	case "ecs_list_hosts", "ecs_search_hosts":
		return invokeMCPHostSearch(c, args)
	case "ecs_get_host", "ecs_get_configuration":
		if err := authorizeMCPHostIdentifier(c, args); err != nil {
			return nil, err
		}
	case "ecs_list_filesystems", "ecs_inspect_path", "ecs_analyze_disk_usage", "ecs_list_log_files", "ecs_request_restart_service", "ecs_request_start_service", "ecs_request_stop_service", "ecs_request_cleanup_logs":
		return aiopsservice.NewMCPHostOperationService().Invoke(c, tool.Name, args)
	case "k8s_request_rollout_restart", "k8s_request_scale_workload", "k8s_request_rollback_workload", "k8s_request_update_image":
		return aiopsservice.NewMCPK8sActionService().Preview(c, tool.Name, args)
	case "k8s_list_clusters", "k8s_check_cluster", "k8s_list_namespaces", "k8s_list_nodes", "k8s_get_node_status", "k8s_search_resources", "k8s_get_resource", "k8s_get_workload_status", "k8s_get_workload_history", "k8s_get_pod_status", "k8s_get_service_endpoints", "k8s_query_events", "k8s_query_logs", "k8s_query_metrics", "k8s_diagnose_workload", "k8s_diagnose_pod", "k8s_diagnose_ingress":
		return aiopsservice.InvokeMCPK8sTool(c, tool.Name, args)
	case "db_search_assets":
		return invokeMCPDatabaseAssetSearch(c, args)
	case "db_search_objects", "db_get_table_schema", "db_execute_readonly_sql", "db_explain_sql", "db_get_monitor_snapshot", "db_get_slow_sql_top", "db_get_instance_health":
		return invokeMCPDatabaseAITool(c, tool.Name, args)
	case "log_list_business_groups", "log_list_projects":
		return invokeMCPLogProjects(c, args)
	case "log_list_stores":
		return invokeMCPLogStores(c, args)
	case "log_check_pipeline":
		return logtailservice.NewLogtailService().VictoriaLogsHealth()
	case "log_search":
		return invokeMCPLogSearch(c, args)
	case "log_aggregate":
		return invokeMCPLogAggregate(c, args)
	case "alert_list_active":
		return invokeMCPAlertList(c, args, false)
	case "alert_get_history":
		return invokeMCPAlertList(c, args, true)
	case "alert_get_detail":
		return invokeMCPAlertDetail(c, args)
	case "alert_get_timeline":
		return invokeMCPAlertTimeline(c, args)
	case "alert_get_notification_status":
		return invokeMCPAlertNotifications(c, args)
	case "alert_list_silences":
		return invokeMCPAlertSilences(c, args)
	case "knowledge_list_groups":
		return aiopsservice.NewKnowledgeGroupService().ListTree()
	case "knowledge_list_categories":
		return invokeMCPKnowledgeCategories(args)
	case "knowledge_get_document", "knowledge_get_citation":
		return invokeMCPKnowledgeDocument(args, tool.Name == "knowledge_get_citation")
	case "knowledge_find_runbooks":
		return invokeMCPKnowledgeSearch(args, "runbook")
	case "knowledge_find_known_issues":
		return invokeMCPKnowledgeSearch(args, "known_issue")
	case "asset_search":
		return invokeMCPAssetSearch(c, args)
	default:
		return aiopsservice.NewAgentToolGatewayService().Invoke(c, &aiopsmodel.AgentToolInvokeRequest{
			RunID: runID, ToolName: tool.HandlerKey, Arguments: args,
			Context: aiopsmodel.AgentToolInvokeContext{AgentRuntime: "mcp", MCPToolName: tool.Name, UserPrompt: mcpStringArg(args, "question", "query")},
		})
	}
	return aiopsservice.NewAgentToolGatewayService().Invoke(c, &aiopsmodel.AgentToolInvokeRequest{
		RunID: runID, ToolName: tool.HandlerKey, Arguments: args,
		Context: aiopsmodel.AgentToolInvokeContext{AgentRuntime: "mcp", MCPToolName: tool.Name, UserPrompt: mcpStringArg(args, "question", "query")},
	})
}

func invokeMCPHostSearch(c *gin.Context, args map[string]interface{}) (interface{}, error) {
	keyword := strings.TrimSpace(mcpStringArg(args, "keyword"))
	limit := mcpIntArg(args, "limit", 10)
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	hostDao := cmdbdao.NewCmdbHostDao()
	var hosts []cmdbmodel.CmdbHost
	if keyword == "" {
		hosts = hostDao.GetCmdbHostList()
	} else {
		hosts = hostDao.SearchCmdbHosts(keyword)
	}
	isAdmin, err := systemauthz.IsSystemAdmin(c)
	if err != nil {
		return nil, err
	}
	allowed := map[uint]struct{}{}
	if !isAdmin {
		admin, err := jwt.GetAdmin(c)
		if err != nil || admin == nil {
			return nil, aiopsservice.ErrMCPUnauthorized
		}
		authDao := cmdbdao.NewBastionHostAuthorizationDao()
		ids, err := authDao.GetUserAuthorizedHostIds(admin.ID)
		if err != nil {
			return nil, err
		}
		allowed = uintSet(ids)
	}
	visible := make([]cmdbmodel.CmdbHost, 0, len(hosts))
	for _, host := range hosts {
		if !isAdmin {
			if _, ok := allowed[host.ID]; !ok {
				continue
			}
		}
		visible = append(visible, host)
	}
	items := make([]map[string]interface{}, 0, limit)
	for _, host := range visible {
		items = append(items, mcpHostView(host))
		if len(items) >= limit {
			break
		}
	}
	return gin.H{"keyword": keyword, "items": items, "count": len(items), "total": len(visible), "limit": limit}, nil
}

func authorizeMCPHostIdentifier(c *gin.Context, args map[string]interface{}) error {
	if mcpUint64Arg(args, "hostId") > 0 {
		return nil // The executor already checked an explicit hostId.
	}
	isAdmin, err := systemauthz.IsSystemAdmin(c)
	if err != nil || isAdmin {
		return err
	}
	identifier := mcpStringArg(args, "ip", "hostname", "instance")
	if identifier == "" {
		return errors.New("one of hostId, ip, hostname or instance is required")
	}
	hostDao := cmdbdao.NewCmdbHostDao()
	host, err := hostDao.GetCmdbHostByInstanceIdentifier(identifier)
	if err != nil {
		return err
	}
	admin, err := jwt.GetAdmin(c)
	if err != nil || admin == nil {
		return aiopsservice.ErrMCPUnauthorized
	}
	authDao := cmdbdao.NewBastionHostAuthorizationDao()
	ids, err := authDao.GetUserAuthorizedHostIds(admin.ID)
	if err != nil {
		return err
	}
	if _, ok := uintSet(ids)[host.ID]; !ok {
		return aiopsservice.ErrMCPPermissionDenied
	}
	return nil
}

func mcpHostView(host cmdbmodel.CmdbHost) map[string]interface{} {
	return map[string]interface{}{
		"id": host.ID, "hostName": host.HostName, "name": host.Name,
		"privateIp": host.PrivateIP, "publicIp": host.PublicIP, "sshIp": host.SSHIP,
		"hostType": host.HostType, "os": host.OS, "status": host.Status,
		"vendor": host.Vendor, "region": host.Region, "instanceId": host.InstanceID,
		"tag": host.Tag,
	}
}

func invokeMCPDatabaseAITool(c *gin.Context, toolName string, args map[string]interface{}) (interface{}, error) {
	var req dbmodel.DBAIToolRequest
	if err := decodeMCPArguments(args, &req); err != nil {
		return nil, err
	}
	req.Tool = toolName
	if req.PageSize <= 0 || req.PageSize > 50 {
		req.PageSize = 50
	}
	return dbservice.NewDatabaseService().RunAITool(c, req)
}

func invokeMCPDatabaseAssetSearch(c *gin.Context, args map[string]interface{}) (interface{}, error) {
	var query dbmodel.AssetQuery
	if err := decodeMCPArguments(args, &query); err != nil {
		return nil, err
	}
	requestedPageSize := query.PageSize
	if requestedPageSize <= 0 || requestedPageSize > 100 {
		requestedPageSize = 20
	}
	query.Page = 1
	query.PageSize = 200
	items, total, err := dbservice.NewDatabaseService().ListAssets(query)
	if err != nil {
		return nil, err
	}
	isAdmin, err := systemauthz.IsSystemAdmin(c)
	if err != nil {
		return nil, err
	}
	if !isAdmin {
		admin, err := jwt.GetAdmin(c)
		if err != nil || admin == nil {
			return nil, aiopsservice.ErrMCPUnauthorized
		}
		authDao := cmdbdao.NewBastionAssetAuthorizationDao()
		allowedIDs, err := authDao.GetUserAuthorizedAssetIDs(admin.ID, "database")
		if err != nil {
			return nil, err
		}
		allowed := uintSet(allowedIDs)
		filtered := make([]dbmodel.AssetVO, 0, len(items))
		for _, item := range items {
			if _, ok := allowed[item.ID]; ok {
				filtered = append(filtered, item)
			}
		}
		items = filtered
		total = int64(len(items))
	}
	if len(items) > requestedPageSize {
		items = items[:requestedPageSize]
	}
	return gin.H{"list": items, "total": total, "page": 1, "pageSize": requestedPageSize}, nil
}

func invokeMCPLogProjects(c *gin.Context, args map[string]interface{}) (interface{}, error) {
	var query logtailmodel.LogProjectQuery
	if err := decodeMCPArguments(args, &query); err != nil {
		return nil, err
	}
	items, total, normalized, err := logtailservice.NewLogtailService().ListProjects(c, query)
	return gin.H{"list": items, "total": total, "page": normalized.Page, "pageSize": normalized.PageSize}, err
}

func invokeMCPLogStores(c *gin.Context, args map[string]interface{}) (interface{}, error) {
	var query logtailmodel.LogStoreQuery
	if err := decodeMCPArguments(args, &query); err != nil {
		return nil, err
	}
	items, total, normalized, err := logtailservice.NewLogtailService().ListStores(c, query)
	return gin.H{"list": items, "total": total, "page": normalized.Page, "pageSize": normalized.PageSize}, err
}

func invokeMCPLogSearch(c *gin.Context, args map[string]interface{}) (interface{}, error) {
	var req logtailmodel.LogQueryLogsRequest
	if err := decodeMCPArguments(args, &req); err != nil {
		return nil, err
	}
	if req.Limit <= 0 || req.Limit > 1000 {
		req.Limit = 200
	}
	return logtailservice.NewLogtailService().QueryLogs(c, req)
}

func invokeMCPLogAggregate(c *gin.Context, args map[string]interface{}) (interface{}, error) {
	var req logtailmodel.LogQueryHitsRequest
	if err := decodeMCPArguments(args, &req); err != nil {
		return nil, err
	}
	return logtailservice.NewLogtailService().QueryHits(c, req)
}

func invokeMCPAlertList(c *gin.Context, args map[string]interface{}, history bool) (interface{}, error) {
	var query alertmodel.AlertEventQuery
	if err := decodeMCPArguments(args, &query); err != nil {
		return nil, err
	}
	query.Page, query.PageSize = alertmodel.NormalizePage(query.Page, query.PageSize)
	scope, all, err := mcpAlertBusinessScope(c)
	if err != nil {
		return nil, err
	}
	if !all && query.BusinessID != "" && !scopeAllows(scope, query.BusinessID) {
		return nil, aiopsservice.ErrMCPPermissionDenied
	}
	if all || query.BusinessID != "" {
		return queryMCPAlertPage(query, history)
	}
	if len(scope) == 0 {
		return gin.H{"list": []interface{}{}, "total": 0, "page": query.Page, "pageSize": query.PageSize}, nil
	}
	combined := make([]interface{}, 0)
	var total int64
	for _, businessID := range scope {
		scopedQuery := query
		scopedQuery.BusinessID = businessID
		page, err := queryMCPAlertPage(scopedQuery, history)
		if err != nil {
			return nil, err
		}
		total += page["total"].(int64)
		data, _ := json.Marshal(page["list"])
		var rows []interface{}
		_ = json.Unmarshal(data, &rows)
		combined = append(combined, rows...)
	}
	if len(combined) > query.PageSize {
		combined = combined[:query.PageSize]
	}
	return gin.H{"list": combined, "total": total, "page": query.Page, "pageSize": query.PageSize}, nil
}

func queryMCPAlertPage(query alertmodel.AlertEventQuery, history bool) (gin.H, error) {
	dao := alertdao.NewAlertManagerDao()
	if history {
		items, total, err := dao.ListHistoryEvents(query)
		return gin.H{"list": items, "total": total, "page": query.Page, "pageSize": query.PageSize}, err
	}
	query.EventStatus = alertmodel.EventDisplayStatusActive
	items, total, err := dao.ListActiveEvents(query)
	return gin.H{"list": items, "total": total, "page": query.Page, "pageSize": query.PageSize}, err
}

func invokeMCPAlertDetail(c *gin.Context, args map[string]interface{}) (interface{}, error) {
	eventID := mcpUint64Arg(args, "eventId")
	eventType := strings.ToLower(mcpStringArg(args, "eventType"))
	if eventType == "" {
		eventType = "active"
	}
	dao := alertdao.NewAlertManagerDao()
	var event interface{}
	var businessID string
	if eventType == "history" || eventType == "resolved" {
		item, err := dao.GetHistoryEvent(eventID)
		if err != nil {
			return nil, err
		}
		event, businessID = item, item.BusinessID
	} else {
		item, err := dao.GetActiveEvent(eventID)
		if err != nil {
			return nil, err
		}
		event, businessID = item, item.BusinessID
	}
	if err := authorizeMCPAlertBusiness(c, businessID); err != nil {
		return nil, err
	}
	timeline, _ := dao.ListTimeline(eventID)
	notify, _ := dao.ListNotifyRecordsByEvent(eventID, eventType)
	return gin.H{"eventType": eventType, "event": event, "timeline": timeline, "notifyRecords": notify}, nil
}

func invokeMCPAlertTimeline(c *gin.Context, args map[string]interface{}) (interface{}, error) {
	eventID := mcpUint64Arg(args, "eventId")
	dao := alertdao.NewAlertManagerDao()
	event, err := dao.GetActiveEvent(eventID)
	if err != nil {
		history, historyErr := dao.GetHistoryEvent(eventID)
		if historyErr != nil {
			return nil, err
		}
		if err := authorizeMCPAlertBusiness(c, history.BusinessID); err != nil {
			return nil, err
		}
	} else if err := authorizeMCPAlertBusiness(c, event.BusinessID); err != nil {
		return nil, err
	}
	return dao.ListTimeline(eventID)
}

func invokeMCPAlertNotifications(c *gin.Context, args map[string]interface{}) (interface{}, error) {
	if _, err := invokeMCPAlertDetail(c, args); err != nil {
		return nil, err
	}
	eventID := mcpUint64Arg(args, "eventId")
	eventType := mcpStringArg(args, "eventType")
	if eventType == "" {
		eventType = "active"
	}
	return alertdao.NewAlertManagerDao().ListNotifyRecordsByEvent(eventID, eventType)
}

func invokeMCPAlertSilences(c *gin.Context, args map[string]interface{}) (interface{}, error) {
	isAdmin, err := systemauthz.IsSystemAdmin(c)
	if err != nil {
		return nil, err
	}
	if !isAdmin {
		return nil, aiopsservice.ErrMCPPermissionDenied
	}
	var query alertmodel.AlertSilenceRuleQuery
	if err := decodeMCPArguments(args, &query); err != nil {
		return nil, err
	}
	query.Page, query.PageSize = alertmodel.NormalizePage(query.Page, query.PageSize)
	items, total, err := alertdao.NewAlertManagerDao().ListSilenceRules(query)
	return gin.H{"list": items, "total": total, "page": query.Page, "pageSize": query.PageSize}, err
}

func invokeMCPKnowledgeSearch(args map[string]interface{}, docType string) (interface{}, error) {
	items, err := aiopsservice.NewKnowledgeService().SearchDocuments(&aiopsmodel.KnowledgeSearchRequest{
		Query: mcpStringArg(args, "query"), Limit: mcpIntArg(args, "limit", 10),
		DocType: docType, Status: aiopsmodel.KnowledgeStatusPublished,
		Category: mcpStringArg(args, "category"), ServiceName: mcpStringArg(args, "serviceName"),
	})
	return gin.H{"items": items, "total": len(items)}, err
}

func invokeMCPKnowledgeDocument(args map[string]interface{}, citationOnly bool) (interface{}, error) {
	doc, err := aiopsservice.NewKnowledgeService().GetDocument(uint(mcpUint64Arg(args, "documentId")))
	if err != nil {
		return nil, err
	}
	if doc.Status != aiopsmodel.KnowledgeStatusPublished {
		return nil, aiopsservice.ErrMCPPermissionDenied
	}
	if citationOnly {
		return gin.H{"documentId": doc.ID, "title": doc.Title, "status": doc.Status, "updatedAt": doc.UpdatedAt, "groupPath": doc.GroupPath}, nil
	}
	return doc, nil
}

func invokeMCPKnowledgeCategories(args map[string]interface{}) (interface{}, error) {
	limit := mcpIntArg(args, "limit", 100)
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	items, _, _, err := aiopsservice.NewKnowledgeService().ListDocuments(aiopsmodel.KnowledgeDocumentQuery{Page: 1, PageSize: limit, Status: aiopsmodel.KnowledgeStatusPublished})
	if err != nil {
		return nil, err
	}
	docTypes, categories, services, tags := map[string]struct{}{}, map[string]struct{}{}, map[string]struct{}{}, map[string]struct{}{}
	for _, item := range items {
		addMCPSet(docTypes, item.DocType)
		addMCPSet(categories, item.Category)
		addMCPSet(services, item.ServiceName)
		for _, tag := range item.Tags {
			addMCPSet(tags, tag)
		}
	}
	return gin.H{"docTypes": sortedMCPSet(docTypes), "categories": sortedMCPSet(categories), "services": sortedMCPSet(services), "tags": sortedMCPSet(tags)}, nil
}

func invokeMCPAssetSearch(c *gin.Context, args map[string]interface{}) (interface{}, error) {
	db := common.GetDB()
	if db == nil {
		return nil, errors.New("database is not initialized")
	}
	keyword := strings.TrimSpace(mcpStringArg(args, "keyword"))
	limit := mcpIntArg(args, "limit", 20)
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	typeFilter := strings.ToLower(strings.TrimSpace(mcpStringArg(args, "type")))
	isAdmin, err := systemauthz.IsSystemAdmin(c)
	if err != nil {
		return nil, err
	}
	admin, err := jwt.GetAdmin(c)
	if err != nil || admin == nil {
		return nil, aiopsservice.ErrMCPUnauthorized
	}
	results := make([]map[string]interface{}, 0, limit)
	like := "%" + keyword + "%"

	if typeFilter == "" || typeFilter == "host" || typeFilter == "ecs" {
		query := db.Model(&cmdbmodel.CmdbHost{}).Where("host_name LIKE ? OR name LIKE ? OR private_ip LIKE ? OR public_ip LIKE ? OR ssh_ip LIKE ?", like, like, like, like, like)
		if !isAdmin {
			authDao := cmdbdao.NewBastionHostAuthorizationDao()
			ids, err := authDao.GetUserAuthorizedHostIds(admin.ID)
			if err != nil {
				return nil, err
			}
			query = query.Where("id IN ?", ids)
		}
		var hosts []cmdbmodel.CmdbHost
		if err := query.Limit(limit).Find(&hosts).Error; err != nil {
			return nil, err
		}
		for _, host := range hosts {
			results = append(results, map[string]interface{}{"type": "host", "id": host.ID, "name": host.HostName, "displayName": host.Name, "privateIp": host.PrivateIP, "publicIp": host.PublicIP, "status": host.Status, "environment": host.Tag})
		}
	}
	if len(results) < limit && (typeFilter == "" || typeFilter == "k8s" || typeFilter == "k8s_cluster") {
		query := db.Model(&k8smodel.KubeCluster{}).Where("name LIKE ? OR description LIKE ?", like, like)
		if !isAdmin {
			authDao := cmdbdao.NewBastionAssetAuthorizationDao()
			ids, err := authDao.GetUserAuthorizedAssetIDs(admin.ID, cmdbmodel.AssetTypeK8SCluster)
			if err != nil {
				return nil, err
			}
			query = query.Where("id IN ?", ids)
		}
		var clusters []k8smodel.KubeCluster
		if err := query.Limit(limit - len(results)).Find(&clusters).Error; err != nil {
			return nil, err
		}
		for _, cluster := range clusters {
			results = append(results, map[string]interface{}{"type": "k8s_cluster", "id": cluster.ID, "name": cluster.Name, "version": cluster.Version, "status": cluster.Status, "nodeCount": cluster.NodeCount})
		}
	}
	return gin.H{"keyword": keyword, "items": results, "count": len(results)}, nil
}

func mcpAlertBusinessScope(c *gin.Context) ([]string, bool, error) {
	isAdmin, err := systemauthz.IsSystemAdmin(c)
	if err != nil {
		return nil, false, err
	}
	if isAdmin {
		return nil, true, nil
	}
	admin, err := jwt.GetAdmin(c)
	if err != nil || admin == nil {
		return nil, false, aiopsservice.ErrMCPUnauthorized
	}
	values, err := systemdao.GetUserBusinessScope(admin.ID)
	return values, false, err
}

func authorizeMCPAlertBusiness(c *gin.Context, businessID string) error {
	scope, all, err := mcpAlertBusinessScope(c)
	if err != nil || all {
		return err
	}
	if !scopeAllows(scope, businessID) {
		return aiopsservice.ErrMCPPermissionDenied
	}
	return nil
}

func scopeAllows(scope []string, value string) bool {
	value = strings.TrimSpace(value)
	for _, item := range scope {
		if strings.EqualFold(strings.TrimSpace(item), value) {
			return true
		}
	}
	return false
}

func decodeMCPArguments(args map[string]interface{}, target interface{}) error {
	data, err := json.Marshal(args)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

func mcpUint64Arg(args map[string]interface{}, key string) uint64 {
	value, ok := args[key]
	if !ok {
		return 0
	}
	switch typed := value.(type) {
	case float64:
		return uint64(typed)
	case uint64:
		return typed
	case uint:
		return uint64(typed)
	case int:
		return uint64(typed)
	default:
		return 0
	}
}

func mcpIntArg(args map[string]interface{}, key string, fallback int) int {
	value := mcpUint64Arg(args, key)
	if value == 0 {
		return fallback
	}
	return int(value)
}

func uintSet(items []uint) map[uint]struct{} {
	result := make(map[uint]struct{}, len(items))
	for _, item := range items {
		result[item] = struct{}{}
	}
	return result
}

func addMCPSet(target map[string]struct{}, value string) {
	value = strings.TrimSpace(value)
	if value != "" {
		target[value] = struct{}{}
	}
}

func sortedMCPSet(items map[string]struct{}) []string {
	result := make([]string, 0, len(items))
	for item := range items {
		result = append(result, item)
	}
	sort.Strings(result)
	return result
}
