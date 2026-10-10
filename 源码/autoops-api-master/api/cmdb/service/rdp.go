// service/rdp_service.go
package service

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"dodevops-api/api/cmdb/dao"

	rdp "github.com/bouncyball-git/gopher-rdp"
	"github.com/bouncyball-git/gopher-rdp/display/web"
	"github.com/google/uuid"
)

type RDPSessionInfo struct {
	SessionID    string    `json:"sessionId"`
	RDPID        string    `json:"rdpid"`
	HostID       uint      `json:"hostId"`
	Host         string    `json:"host"`
	Port         int       `json:"port"`
	Username     string    `json:"username"`
	OwnerID      uint      `json:"ownerId"`
	OwnerName    string    `json:"ownerName"`
	Connected    bool      `json:"connected"`
	LastWidth    int       `json:"lastWidth"`
	LastHeight   int       `json:"lastHeight"`
	CreatedAt    time.Time `json:"createdAt"`
	LastActiveAt time.Time `json:"lastActiveAt"`
}

type rdpSession struct {
	info         RDPSessionInfo
	client       *rdp.Client
	keyboardMode rdp.KeyboardMode
}

type RDPService struct {
	dao      *dao.RDPTargetDAO
	sessions map[string]*rdpSession
	mu       sync.RWMutex
}

func NewRDPService(dao *dao.RDPTargetDAO) *RDPService {
	return &RDPService{
		dao:      dao,
		sessions: make(map[string]*rdpSession),
	}
}

func stringToKeyboardMode(mode string) rdp.KeyboardMode {
	switch mode {
	case "unicode":
		return rdp.KeyboardMode(1)
	case "scancode":
		return rdp.KeyboardMode(0)
	default:
		return rdp.KeyboardMode(0)
	}
}

func cloneSessionInfo(info RDPSessionInfo) *RDPSessionInfo {
	cp := info
	return &cp
}

func sanitizeUploadFileName(name string) string {
	name = strings.TrimSpace(name)
	name = filepath.Base(name)
	if name == "." || name == ".." {
		return ""
	}
	return name
}

func (s *RDPService) createSession(rdpID string, ownerID uint, ownerName string) (*RDPSessionInfo, error) {
	hostID, err := strconv.ParseUint(rdpID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("rdpid format error: %w", err)
	}

	target, err := s.dao.FindByID(rdpID)
	if err != nil {
		return nil, fmt.Errorf("failed to load RDP target: %w", err)
	}

	sessionID := uuid.NewString()
	now := time.Now()
	session := &rdpSession{
		info: RDPSessionInfo{
			SessionID:    sessionID,
			RDPID:        rdpID,
			HostID:       uint(hostID),
			Host:         target.Host,
			Port:         target.Port,
			Username:     target.Username,
			OwnerID:      ownerID,
			OwnerName:    ownerName,
			Connected:    false,
			LastWidth:    1920,
			LastHeight:   1080,
			CreatedAt:    now,
			LastActiveAt: now,
		},
		keyboardMode: stringToKeyboardMode(target.KeyboardMode),
	}

	s.mu.Lock()
	s.sessions[sessionID] = session
	s.mu.Unlock()

	log.Printf("[RDP] session created: sessionId=%s rdpid=%s owner=%d", sessionID, rdpID, ownerID)
	return cloneSessionInfo(session.info), nil
}

func (s *RDPService) OpenSession(rdpID string, ownerID uint, ownerName string) (*RDPSessionInfo, error) {
	rdpID = strings.TrimSpace(rdpID)
	if rdpID == "" {
		return nil, errors.New("missing rdpid")
	}

	s.mu.RLock()
	for _, session := range s.sessions {
		if session.info.RDPID == rdpID && session.info.OwnerID == ownerID {
			info := cloneSessionInfo(session.info)
			s.mu.RUnlock()
			return info, nil
		}
	}
	s.mu.RUnlock()

	return s.createSession(rdpID, ownerID, ownerName)
}

func (s *RDPService) GetSession(sessionID string) (*RDPSessionInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	session, exists := s.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("RDP session not found: %s", sessionID)
	}
	return cloneSessionInfo(session.info), nil
}

func (s *RDPService) markSessionDisconnected(sessionID string, client *rdp.Client) {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, exists := s.sessions[sessionID]
	if !exists {
		return
	}
	if session.client != client {
		return
	}
	session.client = nil
	session.info.Connected = false
	session.info.LastActiveAt = time.Now()
}

// GetOrCreateWebHandlerBySession gets or creates an RDP web handler by session.
func (s *RDPService) GetOrCreateWebHandlerBySession(sessionID string, width, height int) (http.Handler, *RDPSessionInfo, error) {
	s.mu.RLock()
	session, exists := s.sessions[sessionID]
	if !exists {
		s.mu.RUnlock()
		return nil, nil, fmt.Errorf("RDP session not found: %s", sessionID)
	}
	if session.client != nil {
		s.mu.RUnlock()

		s.mu.Lock()
		session, exists = s.sessions[sessionID]
		if !exists {
			s.mu.Unlock()
			return nil, nil, fmt.Errorf("RDP session not found: %s", sessionID)
		}
		if session.client != nil {
			session.info.LastActiveAt = time.Now()
			session.info.LastWidth = width
			session.info.LastHeight = height
			client := session.client
			kbMode := session.keyboardMode
			info := cloneSessionInfo(session.info)
			s.mu.Unlock()
			return web.NewWebHandler(client, nil, width, height, kbMode), info, nil
		}

		rdpID := session.info.RDPID
		s.mu.Unlock()
		return s.createWebHandlerForSession(sessionID, rdpID, width, height)
	}
	rdpID := session.info.RDPID
	s.mu.RUnlock()

	return s.createWebHandlerForSession(sessionID, rdpID, width, height)
}

func (s *RDPService) createWebHandlerForSession(sessionID, rdpID string, width, height int) (http.Handler, *RDPSessionInfo, error) {
	target, err := s.dao.FindByID(rdpID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load RDP target: %w", err)
	}

	opts := target.ToRDPOptions()
	opts.Width = uint16(width)
	opts.Height = uint16(height)
	opts.Clipboard = true

	client, err := rdp.NewClient(opts)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create RDP client: %w", err)
	}
	if err := client.Connect(); err != nil {
		client.Close()
		return nil, nil, fmt.Errorf("failed to connect RDP server: %w", err)
	}

	kbMode := stringToKeyboardMode(target.KeyboardMode)
	now := time.Now()

	s.mu.Lock()
	session, exists := s.sessions[sessionID]
	if !exists {
		s.mu.Unlock()
		client.Close()
		return nil, nil, fmt.Errorf("RDP session not found: %s", sessionID)
	}
	if session.client != nil {
		existing := session.client
		info := cloneSessionInfo(session.info)
		kb := session.keyboardMode
		s.mu.Unlock()
		client.Close()
		return web.NewWebHandler(existing, nil, width, height, kb), info, nil
	}
	session.client = client
	session.keyboardMode = kbMode
	session.info.Host = target.Host
	session.info.Port = target.Port
	session.info.Username = target.Username
	session.info.Connected = true
	session.info.LastWidth = width
	session.info.LastHeight = height
	session.info.LastActiveAt = now
	info := cloneSessionInfo(session.info)
	s.mu.Unlock()

	go func(id string, c *rdp.Client) {
		<-c.Done()
		s.markSessionDisconnected(id, c)
	}(sessionID, client)

	return web.NewWebHandler(client, nil, width, height, kbMode), info, nil
}

func (s *RDPService) FindSessionIDsByHost(rdpID string, ownerID uint, adminMode bool) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sessionIDs := make([]string, 0)
	for id, session := range s.sessions {
		if session.info.RDPID != rdpID {
			continue
		}
		if adminMode || session.info.OwnerID == ownerID {
			sessionIDs = append(sessionIDs, id)
		}
	}
	return sessionIDs
}

func (s *RDPService) DisconnectSession(sessionID string) error {
	s.mu.Lock()
	session, exists := s.sessions[sessionID]
	if !exists {
		s.mu.Unlock()
		return fmt.Errorf("RDP session not found: %s", sessionID)
	}
	delete(s.sessions, sessionID)
	client := session.client
	s.mu.Unlock()

	if client != nil {
		client.Close()
	}
	return nil
}

// DisconnectClient keeps backward compatibility: disconnect all sessions by rdpid.
func (s *RDPService) DisconnectClient(rdpID string) error {
	sessionIDs := s.FindSessionIDsByHost(rdpID, 0, true)
	if len(sessionIDs) == 0 {
		return fmt.Errorf("RDP session not found for host: %s", rdpID)
	}
	for _, sessionID := range sessionIDs {
		_ = s.DisconnectSession(sessionID)
	}
	return nil
}

// GetActiveSessions returns all known RDP sessions.
func (s *RDPService) GetActiveSessions() []RDPSessionInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sessions := make([]RDPSessionInfo, 0, len(s.sessions))
	for _, session := range s.sessions {
		sessions = append(sessions, session.info)
	}
	return sessions
}
