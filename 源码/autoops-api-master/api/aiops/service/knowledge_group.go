package service

import (
	"errors"
	"strings"

	aiopsdao "dodevops-api/api/aiops/dao"
	aiopsmodel "dodevops-api/api/aiops/model"
	"dodevops-api/pkg/jwt"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type KnowledgeGroupService struct {
	dao *aiopsdao.KnowledgeGroupDao
}

func NewKnowledgeGroupService() *KnowledgeGroupService {
	return &KnowledgeGroupService{dao: aiopsdao.NewKnowledgeGroupDao()}
}

func (s *KnowledgeGroupService) ListTree() ([]aiopsmodel.KnowledgeGroupTreeVO, error) {
	groups, counts, err := s.dao.List()
	if err != nil {
		return nil, err
	}
	return buildKnowledgeGroupTree(groups, counts), nil
}

func (s *KnowledgeGroupService) Create(c *gin.Context, dto *aiopsmodel.KnowledgeGroupDTO) (*aiopsmodel.KnowledgeGroup, error) {
	if dto == nil {
		return nil, errors.New("request is nil")
	}
	name := strings.TrimSpace(dto.Name)
	if name == "" {
		return nil, errors.New("name is required")
	}
	if dto.ParentID > 0 {
		if _, err := s.dao.Get(dto.ParentID); err != nil {
			return nil, errors.New("parent group not found")
		}
	}
	userID := knowledgeUserID(c)
	group := &aiopsmodel.KnowledgeGroup{
		ParentID: dto.ParentID, Name: name, Description: strings.TrimSpace(dto.Description),
		SortOrder: dto.SortOrder, CreatedBy: userID, UpdatedBy: userID,
	}
	if err := s.dao.Create(group); err != nil {
		return nil, err
	}
	return group, nil
}

func (s *KnowledgeGroupService) Update(c *gin.Context, id uint, dto *aiopsmodel.KnowledgeGroupDTO) (*aiopsmodel.KnowledgeGroup, error) {
	if id == 0 || dto == nil {
		return nil, errors.New("id and request are required")
	}
	name := strings.TrimSpace(dto.Name)
	if name == "" {
		return nil, errors.New("name is required")
	}
	groups, _, err := s.dao.List()
	if err != nil {
		return nil, err
	}
	if !knowledgeGroupExists(groups, id) {
		return nil, gorm.ErrRecordNotFound
	}
	if dto.ParentID == id || knowledgeGroupIsDescendant(groups, id, dto.ParentID) {
		return nil, errors.New("group cannot be moved under itself or its descendants")
	}
	if dto.ParentID > 0 && !knowledgeGroupExists(groups, dto.ParentID) {
		return nil, errors.New("parent group not found")
	}
	updates := map[string]interface{}{
		"parent_id": dto.ParentID, "name": name, "description": strings.TrimSpace(dto.Description),
		"sort_order": dto.SortOrder, "updated_by": knowledgeUserID(c),
	}
	if err := s.dao.Update(id, updates); err != nil {
		return nil, err
	}
	return s.dao.Get(id)
}

func (s *KnowledgeGroupService) Delete(id uint) error {
	if id == 0 {
		return errors.New("id is required")
	}
	return s.dao.Delete(id)
}

func (s *KnowledgeGroupService) ResolveGroup(groupID uint, groupName, category string, userID uint) (uint, string, string, string, error) {
	if groupID == 0 {
		var err error
		groupID, err = s.dao.EnsurePath(groupName, category, userID)
		if err != nil {
			return 0, "", "", "", err
		}
	}
	if groupID == 0 {
		return 0, strings.TrimSpace(groupName), strings.TrimSpace(category), "", nil
	}
	groups, _, err := s.dao.List()
	if err != nil {
		return 0, "", "", "", err
	}
	labels, ok := knowledgeGroupLabels(groups, groupID)
	if !ok {
		return 0, "", "", "", errors.New("knowledge group not found")
	}
	rootName := labels[0]
	categoryName := ""
	if len(labels) > 1 {
		categoryName = labels[len(labels)-1]
	}
	return groupID, rootName, categoryName, strings.Join(labels, " / "), nil
}

func (s *KnowledgeGroupService) DescendantIDs(groupID uint) ([]uint, error) {
	groups, _, err := s.dao.List()
	if err != nil {
		return nil, err
	}
	if groupID == 0 {
		return nil, nil
	}
	if !knowledgeGroupExists(groups, groupID) {
		return nil, gorm.ErrRecordNotFound
	}
	result := []uint{groupID}
	for i := 0; i < len(result); i++ {
		parentID := result[i]
		for _, group := range groups {
			if group.ParentID == parentID {
				result = append(result, group.ID)
			}
		}
	}
	return result, nil
}

func buildKnowledgeGroupTree(groups []aiopsmodel.KnowledgeGroup, directCounts map[uint]int64) []aiopsmodel.KnowledgeGroupTreeVO {
	children := make(map[uint][]aiopsmodel.KnowledgeGroup)
	known := make(map[uint]struct{}, len(groups))
	for _, group := range groups {
		known[group.ID] = struct{}{}
	}
	for _, group := range groups {
		parentID := group.ParentID
		if _, ok := known[parentID]; parentID > 0 && !ok {
			parentID = 0
		}
		children[parentID] = append(children[parentID], group)
	}
	var build func(uint, []string) ([]aiopsmodel.KnowledgeGroupTreeVO, int64)
	build = func(parentID uint, parentPath []string) ([]aiopsmodel.KnowledgeGroupTreeVO, int64) {
		nodes := make([]aiopsmodel.KnowledgeGroupTreeVO, 0, len(children[parentID]))
		var total int64
		for _, group := range children[parentID] {
			pathParts := append(append([]string{}, parentPath...), group.Name)
			childNodes, childCount := build(group.ID, pathParts)
			directCount := directCounts[group.ID]
			docCount := directCount + childCount
			nodes = append(nodes, aiopsmodel.KnowledgeGroupTreeVO{
				ID: group.ID, ParentID: group.ParentID, Name: group.Name, Description: group.Description,
				SortOrder: group.SortOrder, Path: strings.Join(pathParts, " / "), DirectCount: directCount,
				DocCount: docCount, Children: childNodes,
			})
			total += docCount
		}
		return nodes, total
	}
	result, _ := build(0, nil)
	return result
}

func knowledgeGroupExists(groups []aiopsmodel.KnowledgeGroup, id uint) bool {
	for _, group := range groups {
		if group.ID == id {
			return true
		}
	}
	return false
}

func knowledgeGroupIsDescendant(groups []aiopsmodel.KnowledgeGroup, ancestorID, candidateID uint) bool {
	if candidateID == 0 {
		return false
	}
	parents := make(map[uint]uint, len(groups))
	for _, group := range groups {
		parents[group.ID] = group.ParentID
	}
	seen := map[uint]struct{}{}
	for current := candidateID; current > 0; current = parents[current] {
		if current == ancestorID {
			return true
		}
		if _, duplicate := seen[current]; duplicate {
			return false
		}
		seen[current] = struct{}{}
	}
	return false
}

func knowledgeGroupLabels(groups []aiopsmodel.KnowledgeGroup, id uint) ([]string, bool) {
	byID := make(map[uint]aiopsmodel.KnowledgeGroup, len(groups))
	for _, group := range groups {
		byID[group.ID] = group
	}
	labels := []string{}
	seen := map[uint]struct{}{}
	for id > 0 {
		group, ok := byID[id]
		if !ok {
			return nil, false
		}
		if _, duplicate := seen[id]; duplicate {
			return nil, false
		}
		seen[id] = struct{}{}
		labels = append([]string{group.Name}, labels...)
		id = group.ParentID
	}
	return labels, len(labels) > 0
}

func knowledgeUserID(c *gin.Context) uint {
	if c != nil {
		if admin, err := jwt.GetAdmin(c); err == nil && admin != nil {
			return admin.ID
		}
	}
	return 0
}
