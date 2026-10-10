package snmp

import (
	"log"
	"sync"
)

type Manager struct{}

var (
	instance *Manager
	once     sync.Once
)

func GetManager() *Manager {
	once.Do(func() {
		instance = &Manager{}
	})
	return instance
}

func (m *Manager) Start() {
	log.Println("[SNMP Manager] disabled: collector module has been removed")
}

func (m *Manager) Reload() {
	log.Println("[SNMP Manager] reload skipped: collector module has been removed")
}

func (m *Manager) Stop() {
	log.Println("[SNMP Manager] stopped")
}
