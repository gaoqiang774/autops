package model

import "sort"

// CmdbGroupHostDto is the group tree with simplified host data.
type CmdbGroupHostDto struct {
	ID        uint               `json:"id"`
	ParentID  uint               `json:"parentId"`
	Name      string             `json:"name"`
	HostCount int                `json:"hostCount"`
	Hosts     []SimpleHostVo     `json:"hosts"`
	Children  []CmdbGroupHostDto `json:"children"`
}

// SimpleHostVo is the host payload embedded under the group tree.
type SimpleHostVo struct {
	ID       uint   `json:"id"`
	HostName string `json:"hostName"`
	SSHIP    string `json:"sshIp"`
	SSHName  string `json:"sshName"`
	SSHPort  int    `json:"sshPort"`
	HostType int    `json:"hostType"`
}

// BuildCmdbGroupHostTree builds a group tree with hosts attached to leaf groups.
func BuildCmdbGroupHostTree(groups []CmdbGroup, hosts []CmdbHostVo) []CmdbGroupHostDto {
	groupMap := make(map[uint]CmdbGroupHostDto)
	hostsByGroup := make(map[uint][]SimpleHostVo)

	for _, host := range hosts {
		hostsByGroup[host.GroupID] = append(hostsByGroup[host.GroupID], SimpleHostVo{
			ID:       host.ID,
			HostName: host.HostName,
			SSHIP:    host.SSHIP,
			SSHName:  host.SSHName,
			SSHPort:  host.SSHPort,
			HostType: host.HostType,
		})
	}

	for _, group := range groups {
		groupMap[group.ID] = CmdbGroupHostDto{
			ID:       group.ID,
			ParentID: group.ParentID,
			Name:     group.Name,
			Hosts:    hostsByGroup[group.ID],
		}
	}

	var tree []CmdbGroupHostDto
	for _, group := range groups {
		if group.ParentID == 0 {
			tree = append(tree, buildGroupHostSubTree(groupMap[group.ID], groupMap))
		}
	}
	sortCmdbGroupHostDtosRecursive(tree, groups)
	return tree
}

func buildGroupHostSubTree(group CmdbGroupHostDto, groupMap map[uint]CmdbGroupHostDto) CmdbGroupHostDto {
	hostCount := len(group.Hosts)

	for _, child := range groupMap {
		if child.ParentID == group.ID {
			childNode := buildGroupHostSubTree(child, groupMap)
			group.Children = append(group.Children, childNode)
			hostCount += childNode.HostCount
		}
	}

	group.HostCount = hostCount
	return group
}

func sortCmdbGroupHostDtosRecursive(groups []CmdbGroupHostDto, sourceGroups []CmdbGroup) {
	groupMeta := make(map[uint]CmdbGroup, len(sourceGroups))
	for _, group := range sourceGroups {
		groupMeta[group.ID] = group
	}

	sortCmdbGroupHostDtos(groups, groupMeta)
}

func sortCmdbGroupHostDtos(groups []CmdbGroupHostDto, groupMeta map[uint]CmdbGroup) {
	sort.SliceStable(groups, func(i, j int) bool {
		return cmdbGroupLess(groupMeta[groups[i].ID], groupMeta[groups[j].ID])
	})

	for i := range groups {
		sortCmdbGroupHostDtos(groups[i].Children, groupMeta)
	}
}
