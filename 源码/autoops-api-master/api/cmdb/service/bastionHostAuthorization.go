package service

import (
	"dodevops-api/api/cmdb/dao"
	"dodevops-api/api/cmdb/model"
	configDao "dodevops-api/api/configcenter/dao"
	"dodevops-api/common/constant"
	"dodevops-api/common/result"
	"dodevops-api/common/util"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
)

type BastionHostAuthorizationServiceInterface interface {
	AuthorizeHostsToUser(c *gin.Context, dto *model.AuthorizeHostsToUserDto)
	AuthorizeHostsToGroup(c *gin.Context, dto *model.AuthorizeHostsToGroupDto)
	RevokeHostsFromUser(c *gin.Context, dto *model.RevokeHostsFromUserDto)
	RevokeHostsFromGroup(c *gin.Context, dto *model.RevokeHostsFromGroupDto)
	GetAuthorizedHosts(c *gin.Context, dto *model.GetAuthorizedHostsDto)
	GetUserAuthorizedHosts(c *gin.Context, userID uint, page, pageSize int)
	CheckUserHostPermission(c *gin.Context, userID, hostID, authID uint)
}

type BastionHostAuthorizationServiceImpl struct {
	authDao     dao.BastionHostAuthorizationDao
	hostDao     dao.CmdbHostDao
	hostAuthDao dao.CmdbHostAuthDao
	groupDao    dao.BastionUserGroupDao
	ecsAuthDao  configDao.EcsAuthDao
	keyDao      *configDao.KeyManageDao
}

type authorizationInput struct {
	HostID        uint
	AuthIDs       []uint
	EffectiveTime *util.HTime
	ExpireTime    *util.HTime
}

func cloneHTime(src *util.HTime) *util.HTime {
	if src == nil {
		return nil
	}
	cloned := *src
	return &cloned
}

func sameHTime(left, right *util.HTime) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return left.Time.Equal(right.Time)
}

func sameAuthIDs(left, right []uint) bool {
	left = model.NormalizeAuthorizationAuthIDs(left)
	right = model.NormalizeAuthorizationAuthIDs(right)
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func normalizeAuthorizationWindow(effectiveTime, expireTime *util.HTime) (*util.HTime, *util.HTime, error) {
	now := time.Now()
	effectiveAt := cloneHTime(effectiveTime)
	expireAt := cloneHTime(expireTime)

	if effectiveAt == nil {
		effectiveAt = &util.HTime{Time: now}
	}

	if expireAt != nil {
		if !expireAt.Time.After(now) {
			return nil, nil, fmt.Errorf("expire time must be after current time")
		}
		if !expireAt.Time.After(effectiveAt.Time) {
			return nil, nil, fmt.Errorf("expire time must be after effective time")
		}
	}

	return effectiveAt, expireAt, nil
}

func mergeAuthorizationInputs(hostIDs []uint, sharedAuthIDs []uint, sharedEffectiveTime, sharedExpireTime *util.HTime, authorizations []model.AuthorizationHostLifecycleDto) ([]authorizationInput, error) {
	if len(hostIDs) == 0 && len(authorizations) == 0 {
		return nil, fmt.Errorf("hostIds and authorizations cannot both be empty")
	}

	inputsByHost := make(map[uint]authorizationInput)
	order := make([]uint, 0, len(hostIDs)+len(authorizations))

	for _, hostID := range hostIDs {
		if hostID == 0 {
			return nil, fmt.Errorf("hostId cannot be empty")
		}
		if _, exists := inputsByHost[hostID]; !exists {
			order = append(order, hostID)
		}
		inputsByHost[hostID] = authorizationInput{
			HostID:        hostID,
			AuthIDs:       sharedAuthIDs,
			EffectiveTime: sharedEffectiveTime,
			ExpireTime:    sharedExpireTime,
		}
	}

	detailedSeen := make(map[uint]struct{}, len(authorizations))
	for _, item := range authorizations {
		if item.HostID == 0 {
			return nil, fmt.Errorf("authorizations.hostId cannot be empty")
		}
		if _, exists := detailedSeen[item.HostID]; exists {
			return nil, fmt.Errorf("duplicate authorizations hostId %d", item.HostID)
		}
		detailedSeen[item.HostID] = struct{}{}

		if _, exists := inputsByHost[item.HostID]; !exists {
			order = append(order, item.HostID)
		}

		effectiveTime := item.EffectiveTime
		if effectiveTime == nil {
			effectiveTime = sharedEffectiveTime
		}

		expireTime := item.ExpireTime
		if expireTime == nil {
			expireTime = sharedExpireTime
		}
		authIDs := model.NormalizeAuthorizationAuthIDs(item.AuthIDs, item.SSHKeyIDs)
		if len(authIDs) == 0 {
			authIDs = sharedAuthIDs
		}

		inputsByHost[item.HostID] = authorizationInput{
			HostID:        item.HostID,
			AuthIDs:       authIDs,
			EffectiveTime: effectiveTime,
			ExpireTime:    expireTime,
		}
	}

	inputs := make([]authorizationInput, 0, len(order))
	for _, hostID := range order {
		inputs = append(inputs, inputsByHost[hostID])
	}

	return inputs, nil
}

func (s *BastionHostAuthorizationServiceImpl) normalizeAuthorizationInputs(inputs []authorizationInput) ([]authorizationInput, error) {
	normalized := make([]authorizationInput, 0, len(inputs))
	for _, input := range inputs {
		effectiveTime, expireTime, err := normalizeAuthorizationWindow(input.EffectiveTime, input.ExpireTime)
		if err != nil {
			return nil, fmt.Errorf("婵犵數鍋為崹鍫曞箰閸撗冨灊妞ゆ牗鍩冮弨锕傛煃瑜忛幃渚?%d 闂傚倷鐒﹂惇褰掑礉瀹€鈧埀顒佸嚬閸樺ジ顢欒箛娑橀敜婵°倐鍋撶紒鐙欏洦鐓曢柟鑸妽濞呭棙銇勮熁閸ャ劉鎷婚梺绋挎湰缁海寮ч埀顒傜磽娴ｈ娈旀い锕傛涧閻ｇ兘濡烽埡渚€鍞堕梺缁樻⒐娴滀粙鍩€? %w", input.HostID, err)
		}
		normalized = append(normalized, authorizationInput{
			HostID:        input.HostID,
			AuthIDs:       model.NormalizeAuthorizationAuthIDs(input.AuthIDs),
			EffectiveTime: effectiveTime,
			ExpireTime:    expireTime,
		})
	}
	return normalized, nil
}

func (s *BastionHostAuthorizationServiceImpl) boundHostAuthIDs(host model.CmdbHost) ([]uint, error) {
	rows, err := s.hostAuthDao.GetHostAuths(host.ID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return model.NormalizeAuthorizationAuthIDs([]uint{host.SSHKeyID}), nil
	}

	authIDs := make([]uint, 0, len(rows))
	for _, row := range rows {
		authIDs = append(authIDs, row.AuthID)
	}
	return model.NormalizeAuthorizationAuthIDs(authIDs), nil
}

func containsAuthID(authIDs []uint, authID uint) bool {
	for _, id := range authIDs {
		if id == authID {
			return true
		}
	}
	return false
}

func (s *BastionHostAuthorizationServiceImpl) validateHostIDs(inputs []authorizationInput) error {
	if len(inputs) == 0 {
		return fmt.Errorf("hostIds cannot be empty")
	}

	hostIDs := make([]uint, 0, len(inputs))
	for _, input := range inputs {
		hostIDs = append(hostIDs, input.HostID)
	}

	hosts, err := s.hostDao.GetCmdbHostsByIds(hostIDs)
	if err != nil {
		return err
	}

	existingHosts := make(map[uint]model.CmdbHost, len(hosts))
	for _, host := range hosts {
		existingHosts[host.ID] = host
	}

	for i := range inputs {
		hostID := inputs[i].HostID
		host, exists := existingHosts[hostID]
		if !exists {
			return fmt.Errorf("host ID %d does not exist", hostID)
		}

		boundAuthIDs, err := s.boundHostAuthIDs(host)
		if err != nil {
			return fmt.Errorf("get host %d login accounts failed: %w", hostID, err)
		}

		inputs[i].AuthIDs = model.NormalizeAuthorizationAuthIDs(inputs[i].AuthIDs)
		if len(inputs[i].AuthIDs) == 0 {
			if len(boundAuthIDs) > 1 {
				return fmt.Errorf("host ID %d has multiple login accounts, authIds is required", hostID)
			}
			inputs[i].AuthIDs = boundAuthIDs
			continue
		}

		for _, authID := range inputs[i].AuthIDs {
			if !containsAuthID(boundAuthIDs, authID) {
				return fmt.Errorf("host ID %d is not bound to login account ID %d", hostID, authID)
			}
		}
	}

	return nil
}

func (s *BastionHostAuthorizationServiceImpl) prepareAuthorizationInputs(hostIDs []uint, sharedAuthIDs []uint, sharedEffectiveTime, sharedExpireTime *util.HTime, authorizations []model.AuthorizationHostLifecycleDto) ([]authorizationInput, error) {
	inputs, err := mergeAuthorizationInputs(hostIDs, sharedAuthIDs, sharedEffectiveTime, sharedExpireTime, authorizations)
	if err != nil {
		return nil, err
	}

	inputs, err = s.normalizeAuthorizationInputs(inputs)
	if err != nil {
		return nil, err
	}

	if err := s.validateHostIDs(inputs); err != nil {
		return nil, err
	}

	return inputs, nil
}

func commonAuthorizationWindow(inputs []authorizationInput) (*util.HTime, *util.HTime, bool) {
	if len(inputs) == 0 {
		return nil, nil, false
	}

	effectiveTime := inputs[0].EffectiveTime
	expireTime := inputs[0].ExpireTime
	for _, input := range inputs[1:] {
		if !sameHTime(input.EffectiveTime, effectiveTime) || !sameHTime(input.ExpireTime, expireTime) {
			return nil, nil, false
		}
	}

	return effectiveTime, expireTime, true
}

func (s *BastionHostAuthorizationServiceImpl) buildCmdbHostVo(host model.CmdbHost) model.CmdbHostVo {
	sshKeyName := ""
	if host.SSHKeyID > 0 {
		keyManage, err := s.keyDao.GetByID(host.SSHKeyID)
		if err == nil {
			sshKeyName = keyManage.KeyName
		}
	}

	return model.CmdbHostVo{
		ID:          host.ID,
		HostName:    host.HostName,
		Name:        host.Name,
		GroupID:     host.GroupID,
		GroupName:   host.Group.Name,
		PrivateIP:   host.PrivateIP,
		PublicIP:    host.PublicIP,
		SSHIP:       host.SSHIP,
		SSHName:     host.SSHName,
		SSHKeyID:    host.SSHKeyID,
		SSHKeyName:  sshKeyName,
		SSHPort:     host.SSHPort,
		Remark:      host.Remark,
		Tag:         host.Tag,
		Vendor:      host.Vendor,
		HostType:    host.HostType,
		Region:      host.Region,
		InstanceID:  host.InstanceID,
		OS:          host.OS,
		Status:      host.Status,
		CPU:         host.CPU,
		Memory:      host.Memory,
		Disk:        host.Disk,
		BillingType: host.BillingType,
		CreateTime:  host.CreateTime,
		ExpireTime:  host.ExpireTime,
		UpdateTime:  host.UpdateTime,
	}
}

func (s *BastionHostAuthorizationServiceImpl) effectiveAuthorizationAuthIDs(host model.CmdbHost, rawAuthIDs string) []uint {
	authIDs := model.DecodeAuthorizationAuthIDs(rawAuthIDs)
	if len(authIDs) > 0 {
		return authIDs
	}

	boundAuthIDs, err := s.boundHostAuthIDs(host)
	if err != nil || len(boundAuthIDs) > 1 {
		return nil
	}
	return boundAuthIDs
}

func (s *BastionHostAuthorizationServiceImpl) buildHostAuthOptions(authIDs []uint, defaultAuthID uint) []model.CmdbHostAuthOptionVo {
	options := make([]model.CmdbHostAuthOptionVo, 0, len(authIDs))
	for _, authID := range authIDs {
		auth, err := s.ecsAuthDao.GetEcsAuthById(authID)
		if err != nil {
			continue
		}
		options = append(options, model.CmdbHostAuthOptionVo{
			ID:           auth.ID,
			AuthID:       auth.ID,
			Name:         auth.Name,
			Type:         auth.Type,
			Username:     auth.Username,
			Port:         auth.Port,
			Remark:       auth.Remark,
			IsDefault:    auth.ID == defaultAuthID,
			HasPassword:  auth.HasPassword(),
			HasPublicKey: auth.HasPublicKey(),
		})
	}
	return options
}

func (s *BastionHostAuthorizationServiceImpl) buildAuthorizedHostVo(host model.CmdbHost, authIDs []uint) model.CmdbHostVo {
	vo := s.buildCmdbHostVo(host)
	vo.SSHKeyIDs = authIDs
	vo.SSHAuths = s.buildHostAuthOptions(authIDs, host.SSHKeyID)
	return vo
}

func (s *BastionHostAuthorizationServiceImpl) upsertAuthorizations(c *gin.Context, subjectType string, subjectID uint, inputs []authorizationInput) {
	hostIDs := make([]uint, 0, len(inputs))
	for _, input := range inputs {
		hostIDs = append(hostIDs, input.HostID)
	}

	existingAuths, err := s.authDao.GetAuthorizationsBySubjectAndHosts(subjectType, subjectID, hostIDs)
	if err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "闂傚倷绀侀幖顐ゆ偖椤愶箑纾块柟缁㈠櫘閺佸淇婇妶鍌氫壕濡炪倖鏌ㄧ换妯虹暦閿濆棗绶為悗锝庡墰閺嗐垽姊绘担鐑樺殌闁圭⒈鍋婇幃褎绻濋崒銈囧姺闂佸壊鍋嗛崰搴ｆ閻愬搫绾ч柛顐亜娴滎垳鎮? "+err.Error())
		return
	}

	existingAuthMap := make(map[uint]*model.BastionHostAuthorization, len(existingAuths))
	for i := range existingAuths {
		auth := &existingAuths[i]
		if _, exists := existingAuthMap[auth.HostID]; exists {
			continue
		}
		existingAuthMap[auth.HostID] = auth
	}

	nowHTime := util.HTime{Time: time.Now()}
	createAuths := make([]model.BastionHostAuthorization, 0)
	createdHostIDs := make([]uint, 0)
	updatedHostIDs := make([]uint, 0)
	skippedHostIDs := make([]uint, 0)

	for _, input := range inputs {
		existing := existingAuthMap[input.HostID]
		if existing != nil {
			existingAuthIDs := model.DecodeAuthorizationAuthIDs(existing.AuthIDs)
			if sameHTime(existing.EffectiveTime, input.EffectiveTime) && sameHTime(existing.ExpireTime, input.ExpireTime) && sameAuthIDs(existingAuthIDs, input.AuthIDs) {
				skippedHostIDs = append(skippedHostIDs, input.HostID)
				continue
			}

			existing.AuthIDs = model.EncodeAuthorizationAuthIDs(input.AuthIDs)
			existing.EffectiveTime = cloneHTime(input.EffectiveTime)
			existing.ExpireTime = cloneHTime(input.ExpireTime)
			existing.UpdateTime = cloneHTime(&nowHTime)
			if err := s.authDao.SaveAuthorization(existing); err != nil {
				result.Failed(c, constant.INVALID_PARAMS, "闂傚倷绀侀幖顐⒚洪妶澶嬪仱闁靛ň鏅涢拑鐔封攽閻樺弶鎼愮紒鐘冲▕閺屾盯顢曢妶鍛€剧紓鍌氱М閸嬫挸鈹戦悩顔肩仾闁稿氦鍋愰崚鎺楀礈瑜庨崰? "+err.Error())
				return
			}
			updatedHostIDs = append(updatedHostIDs, input.HostID)
			continue
		}

		createAuths = append(createAuths, model.BastionHostAuthorization{
			SubjectType:   subjectType,
			SubjectID:     subjectID,
			HostID:        input.HostID,
			AuthIDs:       model.EncodeAuthorizationAuthIDs(input.AuthIDs),
			EffectiveTime: cloneHTime(input.EffectiveTime),
			ExpireTime:    cloneHTime(input.ExpireTime),
			CreateTime:    nowHTime,
			UpdateTime:    cloneHTime(&nowHTime),
		})
		createdHostIDs = append(createdHostIDs, input.HostID)
	}

	if len(createAuths) > 0 {
		if err := s.authDao.BatchAuthorizeHosts(createAuths); err != nil {
			result.Failed(c, constant.INVALID_PARAMS, "闂傚倷鑳堕幊鎾诲箹椤愶附鍋嬪┑鐘插亞閻掍粙鏌涢銈呮瀻缂佸墎鍋ゅ鍫曞醇椤愵澀鍑介悶? "+err.Error())
			return
		}
	}

	resp := gin.H{
		"message":        fmt.Sprintf("authorize success: created %d, updated %d, skipped %d", len(createdHostIDs), len(updatedHostIDs), len(skippedHostIDs)),
		"createdCount":   len(createdHostIDs),
		"updatedCount":   len(updatedHostIDs),
		"skipCount":      len(skippedHostIDs),
		"createdHostIds": createdHostIDs,
		"updatedHostIds": updatedHostIDs,
		"skipHostIds":    skippedHostIDs,
	}

	if effectiveTime, expireTime, sameWindow := commonAuthorizationWindow(inputs); sameWindow {
		resp["effectiveTime"] = effectiveTime
		resp["expireTime"] = expireTime
	}

	result.Success(c, resp)
}

func (s *BastionHostAuthorizationServiceImpl) AuthorizeHostsToUser(c *gin.Context, dto *model.AuthorizeHostsToUserDto) {
	if dto.UserID == 0 {
		result.Failed(c, constant.INVALID_PARAMS, "userId cannot be empty")
		return
	}

	sharedAuthIDs := model.NormalizeAuthorizationAuthIDs(dto.AuthIDs, dto.SSHKeyIDs)
	inputs, err := s.prepareAuthorizationInputs(dto.HostIDs, sharedAuthIDs, dto.EffectiveTime, dto.ExpireTime, dto.Authorizations)
	if err != nil {
		result.Failed(c, constant.INVALID_PARAMS, err.Error())
		return
	}

	s.upsertAuthorizations(c, "user", dto.UserID, inputs)
}

func (s *BastionHostAuthorizationServiceImpl) AuthorizeHostsToGroup(c *gin.Context, dto *model.AuthorizeHostsToGroupDto) {
	if dto.GroupID == 0 {
		result.Failed(c, constant.INVALID_PARAMS, "groupId cannot be empty")
		return
	}

	if _, err := s.groupDao.GetUserGroupById(dto.GroupID); err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "user group does not exist")
		return
	}

	sharedAuthIDs := model.NormalizeAuthorizationAuthIDs(dto.AuthIDs, dto.SSHKeyIDs)
	inputs, err := s.prepareAuthorizationInputs(dto.HostIDs, sharedAuthIDs, dto.EffectiveTime, dto.ExpireTime, dto.Authorizations)
	if err != nil {
		result.Failed(c, constant.INVALID_PARAMS, err.Error())
		return
	}

	s.upsertAuthorizations(c, "group", dto.GroupID, inputs)
}

func (s *BastionHostAuthorizationServiceImpl) RevokeHostsFromUser(c *gin.Context, dto *model.RevokeHostsFromUserDto) {
	if err := s.authDao.BatchRevokeHostAuthorization("user", dto.UserID, dto.HostIDs); err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "revoke authorization failed: "+err.Error())
		return
	}

	result.Success(c, gin.H{
		"message": fmt.Sprintf("revoked %d host authorizations", len(dto.HostIDs)),
	})
}

func (s *BastionHostAuthorizationServiceImpl) RevokeHostsFromGroup(c *gin.Context, dto *model.RevokeHostsFromGroupDto) {
	if _, err := s.groupDao.GetUserGroupById(dto.GroupID); err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "user group does not exist")
		return
	}

	if err := s.authDao.BatchRevokeHostAuthorization("group", dto.GroupID, dto.HostIDs); err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "revoke authorization failed: "+err.Error())
		return
	}

	result.Success(c, gin.H{
		"message": fmt.Sprintf("revoked %d host authorizations", len(dto.HostIDs)),
	})
}

func (s *BastionHostAuthorizationServiceImpl) GetAuthorizedHosts(c *gin.Context, dto *model.GetAuthorizedHostsDto) {
	if dto.SubjectType == "group" {
		if _, err := s.groupDao.GetUserGroupById(dto.SubjectID); err != nil {
			result.Failed(c, constant.INVALID_PARAMS, "user group does not exist")
			return
		}
	}

	auths, total, err := s.authDao.GetAuthorizedHostsWithPage(dto.SubjectType, dto.SubjectID, dto.Page, dto.PageSize, dto.IncludeExpired)
	if err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "get authorized hosts failed: "+err.Error())
		return
	}

	if len(auths) == 0 {
		result.SuccessWithPage(c, []model.BastionAuthorizedHostVo{}, 0, dto.Page, dto.PageSize)
		return
	}

	hostIDs := make([]uint, 0, len(auths))
	for _, auth := range auths {
		hostIDs = append(hostIDs, auth.HostID)
	}

	hosts, err := s.hostDao.GetCmdbHostsByIds(hostIDs)
	if err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "闂傚倷绀侀幖顐ゆ偖椤愶箑纾块柟缁㈠櫘閺佸淇婇妶鍕厡闁崇粯姊荤槐鎾存媴閼测剝鍨归懞鍗烆潩椤戙垹閰ｅ畷鍫曟晲閸涱厸鎷ゅ┑鐐差嚟閸樠囧箠韫囨搩鍤楅柛鏇ㄥ墯缂嶅洦銇勯幇鈺佸壘? "+err.Error())
		return
	}

	hostMap := make(map[uint]model.CmdbHost, len(hosts))
	for _, host := range hosts {
		hostMap[host.ID] = host
	}

	now := time.Now()
	vos := make([]model.BastionAuthorizedHostVo, 0, len(auths))
	for _, auth := range auths {
		host, ok := hostMap[auth.HostID]
		if !ok {
			continue
		}
		authIDs := s.effectiveAuthorizationAuthIDs(host, auth.AuthIDs)

		vos = append(vos, model.BastionAuthorizedHostVo{
			CmdbHostVo:          s.buildAuthorizedHostVo(host, authIDs),
			AuthorizationID:     auth.ID,
			SubjectType:         auth.SubjectType,
			SubjectID:           auth.SubjectID,
			AuthorizedAuthIDs:   authIDs,
			AuthorizedSSHAuths:  s.buildHostAuthOptions(authIDs, host.SSHKeyID),
			AuthorizationStatus: model.ResolveAuthorizationStatus(auth.EffectiveTime, auth.ExpireTime, now),
			EffectiveTime:       auth.EffectiveTime,
			AuthExpireTime:      auth.ExpireTime,
			AuthCreateTime:      auth.CreateTime,
			AuthUpdateTime:      auth.UpdateTime,
		})
	}

	result.SuccessWithPage(c, vos, total, dto.Page, dto.PageSize)
}

func (s *BastionHostAuthorizationServiceImpl) GetUserAuthorizedHosts(c *gin.Context, userID uint, page, pageSize int) {
	hostIDs, total, err := s.authDao.GetUserAuthorizedHostsWithPage(userID, page, pageSize)
	if err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "闂傚倷绀侀崥瀣磿閹惰棄搴婇柤鑹扮堪娴滃綊鏌涢妷顔煎缂佺姵濞婇弻娑㈩敃閵堝懏鐎剧紓鍌氱М閸嬫挸鈹戦悙鏉戠仸闁圭澧介崚鎺楊敍濮橈絾鏁犻梺閫炲苯澧い顓炴健楠炲棝骞嶉鍓у嫎闂? "+err.Error())
		return
	}

	if len(hostIDs) == 0 {
		result.SuccessWithPage(c, []model.CmdbHostVo{}, 0, page, pageSize)
		return
	}

	hosts, err := s.hostDao.GetCmdbHostsByIds(hostIDs)
	if err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "闂傚倷绀侀幖顐ゆ偖椤愶箑纾块柟缁㈠櫘閺佸淇婇妶鍕厡闁崇粯姊荤槐鎾存媴閼测剝鍨归懞鍗烆潩椤戙垹閰ｅ畷鍫曟晲閸涱厸鎷ゅ┑鐐差嚟閸樠囧箠韫囨搩鍤楅柛鏇ㄥ墯缂嶅洦銇勯幇鈺佸壘? "+err.Error())
		return
	}

	hostMap := make(map[uint]model.CmdbHost, len(hosts))
	for _, host := range hosts {
		hostMap[host.ID] = host
	}

	vos := make([]model.CmdbHostVo, 0, len(hostIDs))
	for _, hostID := range hostIDs {
		host, ok := hostMap[hostID]
		if !ok {
			continue
		}
		authIDs, err := s.authDao.GetUserHostAuthorizedAuthIDs(userID, hostID)
		if err != nil {
			result.Failed(c, constant.INVALID_PARAMS, "闂傚倷绀侀崥瀣磿閹惰棄搴婇柤鑹扮堪娴滃綊鏌涢妷顔煎缂佺姵濞婇弻娑㈩敃閵堝懏鐎剧紓鍌氱М閸嬫捇姊洪懡銈呅ｉ柛鏂炲懐涓嶉柟杈剧祷娴滃綊鏌熺粙鍧楊€楃紒鍓佸仱瀵爼宕奸顫嚱閻? "+err.Error())
			return
		}
		vos = append(vos, s.buildAuthorizedHostVo(host, authIDs))
	}

	result.SuccessWithPage(c, vos, total, page, pageSize)
}

func (s *BastionHostAuthorizationServiceImpl) CheckUserHostPermission(c *gin.Context, userID, hostID, authID uint) {
	hasPermission, err := s.authDao.CheckUserHostAuthPermission(userID, hostID, authID)
	if err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "濠电姷顣藉Σ鍛村磻閳ь剟鏌涚€ｎ偅宕岄柡宀嬬磿娴狅妇鎷犻幓鎺戭潥婵犵數鍋熼崢褔鎮ч幘鎰佹綎濞寸姴顑呯粻娑欍亜閹存繂缍栭柡澶樺弮閺? "+err.Error())
		return
	}

	result.Success(c, gin.H{
		"userId":        userID,
		"hostId":        hostID,
		"authId":        authID,
		"hasPermission": hasPermission,
	})
}

func GetBastionHostAuthorizationService() BastionHostAuthorizationServiceInterface {
	return &BastionHostAuthorizationServiceImpl{
		authDao:     dao.NewBastionHostAuthorizationDao(),
		hostDao:     dao.NewCmdbHostDao(),
		hostAuthDao: dao.NewCmdbHostAuthDao(),
		groupDao:    dao.NewBastionUserGroupDao(),
		ecsAuthDao:  configDao.NewEcsAuthDao(),
		keyDao:      configDao.NewKeyManageDao(),
	}
}
