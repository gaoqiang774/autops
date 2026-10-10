// dao/rdp_target_dao.go
package dao

import (
	"errors"
	"fmt"
	"log"
	"strconv"
	"sync"

	rdp "github.com/bouncyball-git/gopher-rdp"
)

// RDPTarget RDP 目标配置
type RDPTarget struct {
	ID             string `json:"id"`
	Host           string `json:"host"`
	Port           int    `json:"port"`
	Username       string `json:"username"`
	Password       string `json:"password"`
	Domain         string `json:"domain,omitempty"`
	Width          int    `json:"width"`
	Height         int    `json:"height"`
	Depth          int    `json:"depth"`
	KeyboardMode   string `json:"keyboard_mode"`
	Wallpaper      bool   `json:"wallpaper"`
	FullWindowDrag bool   `json:"full_window_drag"`
	FontSmoothing  bool   `json:"font_smoothing"`
	Clipboard      bool   `json:"clipboard"`
}

// ToRDPOptions 转换为 RDP Options
func (t *RDPTarget) ToRDPOptions() *rdp.Options {
	return &rdp.Options{
		Host:     t.Host,
		Port:     t.Port,
		Username: t.Username,
		Password: t.Password,
		Domain:   t.Domain,
		Width:    uint16(t.Width),
		Height:   uint16(t.Height),
		Depth:    uint16(t.Depth),

		Wallpaper:      t.Wallpaper,
		FullWindowDrag: t.FullWindowDrag,
		FontSmoothing:  t.FontSmoothing,
		Clipboard:      t.Clipboard,
	}
}

// RDPTargetDAO RDP 目标数据访问层
type RDPTargetDAO struct {
	targets map[string]*RDPTarget
	mu      sync.RWMutex
}

// NewRDPTargetDAO 创建 DAO 实例
func NewRDPTargetDAO() *RDPTargetDAO {
	dao := &RDPTargetDAO{
		targets: make(map[string]*RDPTarget),
	}
	// 初始化默认数据
	dao.initDefaultTargets()
	return dao
}

// initDefaultTargets 初始化默认目标
func (d *RDPTargetDAO) initDefaultTargets() {
	d.targets["1"] = &RDPTarget{
		ID:             "1",
		Host:           "172.16.16.32",
		Port:           3389,
		Username:       "Administrator",
		Password:       "leyoujia@123",
		Width:          1920,
		Height:         1080,
		Depth:          32,
		Wallpaper:      true,
		FullWindowDrag: true,
		FontSmoothing:  true,
		Clipboard:      true,
	}
	d.targets["2"] = &RDPTarget{
		ID:             "2",
		Host:           "172.16.16.29",
		Port:           3389,
		Username:       "Administrator",
		Password:       "leyoujia@123",
		Width:          1920,
		Height:         1080,
		Depth:          32,
		Wallpaper:      true,
		FullWindowDrag: true,
		FontSmoothing:  true,
		Clipboard:      true,
	}
}

// FindByID 根据 ID 查找目标
func (d *RDPTargetDAO) FindByID(id string) (*RDPTarget, error) {
	idUint, _ := strconv.ParseUint(id, 10, 64)

	host, err := NewCmdbHostSSHDao().GetHostSSHInfo(uint(idUint))
	if err != nil {
		log.Printf("获取主机ID=%s信息失败: %v", id, err)
		return nil, fmt.Errorf("获取主机信息失败: %v", err)
	}

	credential, err := NewCmdbHostSSHDao().GetSSHCredentials(host.SSHKeyID)
	if err != nil {
		log.Printf("获取SSH凭据失败: %v", err)
		return nil, fmt.Errorf("获取SSH凭据失败: %v", err)
	}
	log.Println(host.SSHIP, credential)
	target := &RDPTarget{
		ID:             id,
		Host:           host.SSHIP,
		Port:           host.SSHPort,
		Username:       host.SSHName,
		Password:       credential,
		Width:          1920,
		Height:         1080,
		Depth:          32,
		Wallpaper:      true,
		FullWindowDrag: true,
		FontSmoothing:  true,
		Clipboard:      true,
	}

	return target, nil
}

// FindAll 获取所有目标
func (d *RDPTargetDAO) FindAll() []*RDPTarget {
	d.mu.RLock()
	defer d.mu.RUnlock()

	targets := make([]*RDPTarget, 0, len(d.targets))
	for _, t := range d.targets {
		// 返回副本，隐藏密码
		targetCopy := *t
		targetCopy.Password = ""
		targets = append(targets, &targetCopy)
	}
	return targets
}

// Save 保存目标
func (d *RDPTargetDAO) Save(target *RDPTarget) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.targets[target.ID] = target
	return nil
}

// Delete 删除目标
func (d *RDPTargetDAO) Delete(id string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if _, exists := d.targets[id]; !exists {
		return errors.New("target not found")
	}
	delete(d.targets, id)
	return nil
}

// Exists 检查目标是否存在
func (d *RDPTargetDAO) Exists(id string) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()

	_, exists := d.targets[id]
	return exists
}
