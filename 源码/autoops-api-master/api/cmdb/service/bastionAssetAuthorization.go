package service

import (
	cmdbdao "dodevops-api/api/cmdb/dao"
	cmdbmodel "dodevops-api/api/cmdb/model"
	k8sdao "dodevops-api/api/k8s/dao"
	logtaildao "dodevops-api/api/logtail/dao"
	"dodevops-api/common"
	"dodevops-api/common/constant"
	"dodevops-api/common/result"
	"dodevops-api/common/util"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type BastionAssetAuthorizationServiceInterface interface {
	AuthorizeAssetsToUser(c *gin.Context, dto *cmdbmodel.AuthorizeAssetsToUserDto)
	AuthorizeAssetsToGroup(c *gin.Context, dto *cmdbmodel.AuthorizeAssetsToGroupDto)
	RevokeAssetsFromUser(c *gin.Context, dto *cmdbmodel.RevokeAssetsFromUserDto)
	RevokeAssetsFromGroup(c *gin.Context, dto *cmdbmodel.RevokeAssetsFromGroupDto)
	GetAuthorizedAssets(c *gin.Context, dto *cmdbmodel.GetAuthorizedAssetsDto)
	CheckUserAssetPermission(c *gin.Context, userID uint, assetType string, assetID uint, scopeType, scopeValue string)
}

type BastionAssetAuthorizationServiceImpl struct {
	authDao    cmdbdao.BastionAssetAuthorizationDao
	hostDao    cmdbdao.CmdbHostDao
	groupDao   cmdbdao.BastionUserGroupDao
	clusterDao *k8sdao.KubeClusterDao
	projectDao *logtaildao.LogProjectDAO
	storeDao   *logtaildao.LogStoreDAO
}

type assetAuthorizationInput struct {
	AssetID       uint
	ScopeType     string
	ScopeValue    string
	EffectiveTime *util.HTime
	ExpireTime    *util.HTime
}

func normalizeAssetType(assetType string) string {
	return strings.ToLower(strings.TrimSpace(assetType))
}

func normalizeAssetScopeValues(scopeType string, scopeValues []string, allScopes bool) [][2]string {
	if allScopes || len(scopeValues) == 0 {
		return [][2]string{{cmdbmodel.AssetScopeAll, ""}}
	}
	scopeType = strings.TrimSpace(scopeType)
	scopes := make([][2]string, 0, len(scopeValues))
	seen := map[string]struct{}{}
	for _, value := range scopeValues {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := scopeType + "\x00" + value
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		scopes = append(scopes, [2]string{scopeType, value})
	}
	if len(scopes) == 0 {
		return [][2]string{{cmdbmodel.AssetScopeAll, ""}}
	}
	return scopes
}

func buildAssetAuthorizationInputs(assetIDs []uint, sharedScopeType string, sharedScopeValues []string, sharedAllScopes bool, sharedEffectiveTime, sharedExpireTime *util.HTime, resources []cmdbmodel.AssetAuthorizationResourceDto) ([]assetAuthorizationInput, error) {
	if len(assetIDs) == 0 && len(resources) == 0 {
		return nil, fmt.Errorf("assetIds and authorizations cannot both be empty")
	}

	inputs := make([]assetAuthorizationInput, 0)
	for _, assetID := range assetIDs {
		if assetID == 0 {
			return nil, fmt.Errorf("assetId cannot be empty")
		}
		for _, scope := range normalizeAssetScopeValues(sharedScopeType, sharedScopeValues, sharedAllScopes) {
			inputs = append(inputs, assetAuthorizationInput{
				AssetID:       assetID,
				ScopeType:     scope[0],
				ScopeValue:    scope[1],
				EffectiveTime: sharedEffectiveTime,
				ExpireTime:    sharedExpireTime,
			})
		}
	}

	for _, resource := range resources {
		if resource.AssetID == 0 {
			return nil, fmt.Errorf("authorizations.assetId cannot be empty")
		}
		effectiveTime := resource.EffectiveTime
		if effectiveTime == nil {
			effectiveTime = sharedEffectiveTime
		}
		expireTime := resource.ExpireTime
		if expireTime == nil {
			expireTime = sharedExpireTime
		}
		scopeType := resource.ScopeType
		if strings.TrimSpace(scopeType) == "" {
			scopeType = sharedScopeType
		}
		scopeValues := resource.ScopeValues
		if len(scopeValues) == 0 {
			scopeValues = sharedScopeValues
		}
		allScopes := resource.AllScopes || sharedAllScopes

		for _, scope := range normalizeAssetScopeValues(scopeType, scopeValues, allScopes) {
			inputs = append(inputs, assetAuthorizationInput{
				AssetID:       resource.AssetID,
				ScopeType:     scope[0],
				ScopeValue:    scope[1],
				EffectiveTime: effectiveTime,
				ExpireTime:    expireTime,
			})
		}
	}

	return inputs, nil
}

func (s *BastionAssetAuthorizationServiceImpl) normalizeInputs(inputs []assetAuthorizationInput) ([]assetAuthorizationInput, error) {
	normalized := make([]assetAuthorizationInput, 0, len(inputs))
	seen := map[string]struct{}{}
	for _, input := range inputs {
		effectiveTime, expireTime, err := normalizeAuthorizationWindow(input.EffectiveTime, input.ExpireTime)
		if err != nil {
			return nil, fmt.Errorf("asset %d authorization window invalid: %w", input.AssetID, err)
		}

		scopeType := strings.TrimSpace(input.ScopeType)
		scopeValue := strings.TrimSpace(input.ScopeValue)
		if scopeValue == "" {
			scopeType = cmdbmodel.AssetScopeAll
		}

		key := fmt.Sprintf("%d\x00%s\x00%s", input.AssetID, scopeType, scopeValue)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}

		normalized = append(normalized, assetAuthorizationInput{
			AssetID:       input.AssetID,
			ScopeType:     scopeType,
			ScopeValue:    scopeValue,
			EffectiveTime: effectiveTime,
			ExpireTime:    expireTime,
		})
	}
	return normalized, nil
}

func (s *BastionAssetAuthorizationServiceImpl) validateAssets(assetType string, inputs []assetAuthorizationInput) error {
	assetIDs := map[uint]struct{}{}
	for _, input := range inputs {
		assetIDs[input.AssetID] = struct{}{}
	}

	switch assetType {
	case cmdbmodel.AssetTypeHost:
		ids := make([]uint, 0, len(assetIDs))
		for id := range assetIDs {
			ids = append(ids, id)
		}
		hosts, err := s.hostDao.GetCmdbHostsByIds(ids)
		if err != nil {
			return err
		}
		if len(hosts) != len(assetIDs) {
			return fmt.Errorf("some host assets do not exist")
		}
	case cmdbmodel.AssetTypeK8SCluster:
		for id := range assetIDs {
			if _, err := s.clusterDao.GetByID(id); err != nil {
				return fmt.Errorf("k8s cluster %d does not exist", id)
			}
		}
	case cmdbmodel.AssetTypeLogProject:
		for id := range assetIDs {
			if _, err := s.projectDao.GetByID(id); err != nil {
				return fmt.Errorf("log project %d does not exist", id)
			}
		}
		for _, input := range inputs {
			if strings.TrimSpace(input.ScopeValue) == "" {
				continue
			}
			if input.ScopeType != cmdbmodel.AssetScopeLogStore {
				return fmt.Errorf("log project only supports scopeType %s", cmdbmodel.AssetScopeLogStore)
			}
			storeID := util.StringToUint(input.ScopeValue)
			if storeID == 0 {
				return fmt.Errorf("log store scopeValue must be store ID")
			}
			store, err := s.storeDao.GetByID(storeID)
			if err != nil {
				return fmt.Errorf("log store %d does not exist", storeID)
			}
			if store.ProjectID != input.AssetID {
				return fmt.Errorf("log store %d does not belong to project %d", storeID, input.AssetID)
			}
		}
	default:
		return fmt.Errorf("unsupported assetType: %s", assetType)
	}
	return nil
}

func (s *BastionAssetAuthorizationServiceImpl) prepareInputs(assetType string, assetIDs []uint, scopeType string, scopeValues []string, allScopes bool, effectiveTime, expireTime *util.HTime, resources []cmdbmodel.AssetAuthorizationResourceDto) ([]assetAuthorizationInput, error) {
	inputs, err := buildAssetAuthorizationInputs(assetIDs, scopeType, scopeValues, allScopes, effectiveTime, expireTime, resources)
	if err != nil {
		return nil, err
	}
	inputs, err = s.normalizeInputs(inputs)
	if err != nil {
		return nil, err
	}
	if err := s.validateAssets(assetType, inputs); err != nil {
		return nil, err
	}
	return inputs, nil
}

func (s *BastionAssetAuthorizationServiceImpl) upsertAuthorizations(c *gin.Context, subjectType string, subjectID uint, assetType string, inputs []assetAuthorizationInput) {
	nowHTime := util.HTime{Time: time.Now()}
	created := 0
	updated := 0
	skipped := 0

	for _, input := range inputs {
		scopes := [][2]string{{input.ScopeType, input.ScopeValue}}
		existingAuths, err := s.authDao.GetAuthorizationsBySubjectAssetScopes(subjectType, subjectID, assetType, input.AssetID, scopes)
		if err != nil {
			result.Failed(c, constant.INVALID_PARAMS, "get existing asset authorization failed: "+err.Error())
			return
		}

		if len(existingAuths) > 0 {
			auth := existingAuths[0]
			if sameHTime(auth.EffectiveTime, input.EffectiveTime) && sameHTime(auth.ExpireTime, input.ExpireTime) {
				skipped++
				continue
			}
			auth.EffectiveTime = cloneHTime(input.EffectiveTime)
			auth.ExpireTime = cloneHTime(input.ExpireTime)
			auth.UpdateTime = cloneHTime(&nowHTime)
			if err := s.authDao.SaveAuthorization(&auth); err != nil {
				result.Failed(c, constant.INVALID_PARAMS, "update asset authorization failed: "+err.Error())
				return
			}
			updated++
			continue
		}

		auth := cmdbmodel.BastionAssetAuthorization{
			SubjectType:   subjectType,
			SubjectID:     subjectID,
			AssetType:     assetType,
			AssetID:       input.AssetID,
			ScopeType:     input.ScopeType,
			ScopeValue:    input.ScopeValue,
			EffectiveTime: cloneHTime(input.EffectiveTime),
			ExpireTime:    cloneHTime(input.ExpireTime),
			CreateTime:    nowHTime,
			UpdateTime:    cloneHTime(&nowHTime),
		}
		if err := s.authDao.BatchAuthorizeAssets([]cmdbmodel.BastionAssetAuthorization{auth}); err != nil {
			result.Failed(c, constant.INVALID_PARAMS, "create asset authorization failed: "+err.Error())
			return
		}
		created++
	}

	result.Success(c, gin.H{
		"message":      fmt.Sprintf("authorize success: created %d, updated %d, skipped %d", created, updated, skipped),
		"createdCount": created,
		"updatedCount": updated,
		"skipCount":    skipped,
	})
}

func (s *BastionAssetAuthorizationServiceImpl) AuthorizeAssetsToUser(c *gin.Context, dto *cmdbmodel.AuthorizeAssetsToUserDto) {
	assetType := normalizeAssetType(dto.AssetType)
	if dto.UserID == 0 {
		result.Failed(c, constant.INVALID_PARAMS, "userId cannot be empty")
		return
	}

	inputs, err := s.prepareInputs(assetType, dto.AssetIDs, dto.ScopeType, dto.ScopeValues, dto.AllScopes, dto.EffectiveTime, dto.ExpireTime, dto.Authorizations)
	if err != nil {
		result.Failed(c, constant.INVALID_PARAMS, err.Error())
		return
	}
	s.upsertAuthorizations(c, cmdbmodel.AssetAuthorizationSubjectUser, dto.UserID, assetType, inputs)
}

func (s *BastionAssetAuthorizationServiceImpl) AuthorizeAssetsToGroup(c *gin.Context, dto *cmdbmodel.AuthorizeAssetsToGroupDto) {
	assetType := normalizeAssetType(dto.AssetType)
	if dto.GroupID == 0 {
		result.Failed(c, constant.INVALID_PARAMS, "groupId cannot be empty")
		return
	}
	if _, err := s.groupDao.GetUserGroupById(dto.GroupID); err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "user group does not exist")
		return
	}

	inputs, err := s.prepareInputs(assetType, dto.AssetIDs, dto.ScopeType, dto.ScopeValues, dto.AllScopes, dto.EffectiveTime, dto.ExpireTime, dto.Authorizations)
	if err != nil {
		result.Failed(c, constant.INVALID_PARAMS, err.Error())
		return
	}
	s.upsertAuthorizations(c, cmdbmodel.AssetAuthorizationSubjectGroup, dto.GroupID, assetType, inputs)
}

func (s *BastionAssetAuthorizationServiceImpl) RevokeAssetsFromUser(c *gin.Context, dto *cmdbmodel.RevokeAssetsFromUserDto) {
	scopes := normalizeAssetScopeValues(dto.ScopeType, dto.ScopeValues, dto.AllScopes)
	if err := s.authDao.BatchRevokeAssetAuthorization(cmdbmodel.AssetAuthorizationSubjectUser, dto.UserID, normalizeAssetType(dto.AssetType), dto.AssetID, scopes); err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "revoke asset authorization failed: "+err.Error())
		return
	}
	result.Success(c, gin.H{"message": "revoke asset authorization success"})
}

func (s *BastionAssetAuthorizationServiceImpl) RevokeAssetsFromGroup(c *gin.Context, dto *cmdbmodel.RevokeAssetsFromGroupDto) {
	if _, err := s.groupDao.GetUserGroupById(dto.GroupID); err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "user group does not exist")
		return
	}
	scopes := normalizeAssetScopeValues(dto.ScopeType, dto.ScopeValues, dto.AllScopes)
	if err := s.authDao.BatchRevokeAssetAuthorization(cmdbmodel.AssetAuthorizationSubjectGroup, dto.GroupID, normalizeAssetType(dto.AssetType), dto.AssetID, scopes); err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "revoke asset authorization failed: "+err.Error())
		return
	}
	result.Success(c, gin.H{"message": "revoke asset authorization success"})
}

func (s *BastionAssetAuthorizationServiceImpl) assetName(assetType string, assetID uint) string {
	switch assetType {
	case cmdbmodel.AssetTypeK8SCluster:
		cluster, err := s.clusterDao.GetByID(assetID)
		if err == nil && cluster != nil {
			return cluster.Name
		}
	case cmdbmodel.AssetTypeHost:
		hosts, err := s.hostDao.GetCmdbHostsByIds([]uint{assetID})
		if err == nil && len(hosts) > 0 {
			return hosts[0].HostName
		}
	case cmdbmodel.AssetTypeLogProject:
		project, err := s.projectDao.GetByID(assetID)
		if err == nil && project != nil {
			return project.Name
		}
	}
	return ""
}

func (s *BastionAssetAuthorizationServiceImpl) scopeName(assetType, scopeType, scopeValue string, cache map[string]string) string {
	scopeValue = strings.TrimSpace(scopeValue)
	if scopeValue == "" {
		return "全部"
	}
	key := assetType + "\x00" + scopeType + "\x00" + scopeValue
	if value, ok := cache[key]; ok {
		return value
	}

	name := ""
	switch assetType {
	case cmdbmodel.AssetTypeLogProject:
		if scopeType == cmdbmodel.AssetScopeLogStore {
			storeID := util.StringToUint(scopeValue)
			if storeID > 0 {
				store, err := s.storeDao.GetByID(storeID)
				if err == nil && store != nil {
					name = store.Name
				}
			}
		}
	case cmdbmodel.AssetTypeK8SCluster:
		if scopeType == cmdbmodel.AssetScopeNamespace {
			name = scopeValue
		}
	}
	if name == "" {
		name = scopeValue
	}
	cache[key] = name
	return name
}

func (s *BastionAssetAuthorizationServiceImpl) GetAuthorizedAssets(c *gin.Context, dto *cmdbmodel.GetAuthorizedAssetsDto) {
	auths, total, err := s.authDao.GetAuthorizedAssetsWithPage(dto.SubjectType, dto.SubjectID, normalizeAssetType(dto.AssetType), dto.AssetID, dto.Page, dto.PageSize, dto.IncludeExpired)
	if err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "get authorized assets failed: "+err.Error())
		return
	}

	now := time.Now()
	vos := make([]cmdbmodel.BastionAssetAuthorizationVo, 0, len(auths))
	scopeNameCache := map[string]string{}
	for _, auth := range auths {
		scopeName := s.scopeName(auth.AssetType, auth.ScopeType, auth.ScopeValue, scopeNameCache)
		vos = append(vos, cmdbmodel.BastionAssetAuthorizationVo{
			ID:                  auth.ID,
			SubjectType:         auth.SubjectType,
			SubjectID:           auth.SubjectID,
			AssetType:           auth.AssetType,
			AssetID:             auth.AssetID,
			AssetName:           s.assetName(auth.AssetType, auth.AssetID),
			ScopeType:           auth.ScopeType,
			ScopeValue:          auth.ScopeValue,
			ScopeName:           scopeName,
			ScopeDisplayName:    scopeName,
			AllScopes:           auth.ScopeValue == "",
			AuthorizationStatus: cmdbmodel.ResolveAuthorizationStatus(auth.EffectiveTime, auth.ExpireTime, now),
			EffectiveTime:       auth.EffectiveTime,
			ExpireTime:          auth.ExpireTime,
			CreateTime:          auth.CreateTime,
			UpdateTime:          auth.UpdateTime,
		})
	}

	result.SuccessWithPage(c, vos, total, dto.Page, dto.PageSize)
}

func (s *BastionAssetAuthorizationServiceImpl) CheckUserAssetPermission(c *gin.Context, userID uint, assetType string, assetID uint, scopeType, scopeValue string) {
	hasPermission, err := s.authDao.CheckUserAssetPermission(userID, normalizeAssetType(assetType), assetID, scopeType, scopeValue)
	if err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "check asset permission failed: "+err.Error())
		return
	}
	result.Success(c, gin.H{
		"userId":        userID,
		"assetType":     normalizeAssetType(assetType),
		"assetId":       assetID,
		"scopeType":     scopeType,
		"scopeValue":    scopeValue,
		"hasPermission": hasPermission,
	})
}

func GetBastionAssetAuthorizationService() BastionAssetAuthorizationServiceInterface {
	return &BastionAssetAuthorizationServiceImpl{
		authDao:    cmdbdao.NewBastionAssetAuthorizationDao(),
		hostDao:    cmdbdao.NewCmdbHostDao(),
		groupDao:   cmdbdao.NewBastionUserGroupDao(),
		clusterDao: k8sdao.NewKubeClusterDao(common.GetDB()),
		projectDao: logtaildao.NewLogProjectDAO(),
		storeDao:   logtaildao.NewLogStoreDAO(),
	}
}
