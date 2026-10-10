package controller

import (
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"dodevops-api/api/cmdb/dao"
	cmdbservice "dodevops-api/api/cmdb/service"
	systemModel "dodevops-api/api/system/model"
	"dodevops-api/common/config"
	"dodevops-api/common/constant"
	"dodevops-api/common/result"
	"dodevops-api/pkg/jwt"
)

type RDPController struct {
	service     *cmdbservice.RDPService
	fileService *cmdbservice.WindowsFileTransferService
}

func NewRDPController(service *cmdbservice.RDPService) *RDPController {
	return &RDPController{
		service:     service,
		fileService: cmdbservice.NewWindowsFileTransferService(),
	}
}

// ConnectRDPNative handles RDP websocket connection.
func (c *RDPController) ConnectRDPNative(ctx *gin.Context) {
	claims, err := getRDPTokenClaims(ctx)
	if err != nil {
		ctx.String(http.StatusUnauthorized, err.Error())
		return
	}
	if strings.EqualFold(strings.TrimSpace(claims.Username), "test") {
		result.Failed(ctx, int(result.ApiCode.TESTUSERFORBIDDEN), result.ApiCode.GetMessage(result.ApiCode.TESTUSERFORBIDDEN))
		return
	}
	ctx.Set(constant.ContextKeyUserObj, claims)

	sessionID := strings.TrimSpace(ctx.Query("sessionId"))
	var session *cmdbservice.RDPSessionInfo

	if sessionID == "" {
		rdpID, hostID, err := getRDPHostID(ctx)
		if err != nil {
			ctx.String(http.StatusBadRequest, err.Error())
			return
		}

		allowed, err := hasRDPHostPermission(claims, hostID)
		if err != nil {
			ctx.String(http.StatusInternalServerError, "failed to check host permission")
			return
		}
		if !allowed {
			ctx.String(http.StatusForbidden, "no permission for this host")
			return
		}

		session, err = c.service.OpenSession(rdpID, claims.ID, claims.Username)
		if err != nil {
			ctx.String(http.StatusBadRequest, err.Error())
			return
		}
		sessionID = session.SessionID
	} else {
		session, err = c.service.GetSession(sessionID)
		if err != nil {
			ctx.String(http.StatusNotFound, err.Error())
			return
		}

		if !canAccessRDPSession(claims, session) {
			ctx.String(http.StatusForbidden, "no permission for this session")
			return
		}

		allowed, err := hasRDPHostPermission(claims, session.HostID)
		if err != nil {
			ctx.String(http.StatusInternalServerError, "failed to check host permission")
			return
		}
		if !allowed {
			ctx.String(http.StatusForbidden, "no permission for this host")
			return
		}
	}

	width, _ := strconv.Atoi(ctx.Query("w"))
	height, _ := strconv.Atoi(ctx.Query("h"))
	if width <= 0 {
		width = 1920
	}
	if height <= 0 {
		height = 1080
	}

	handler, sessionInfo, err := c.service.GetOrCreateWebHandlerBySession(sessionID, width, height)
	if err != nil {
		ctx.String(http.StatusBadRequest, err.Error())
		return
	}

	ctx.Writer.Header().Set("X-RDP-Session-Id", sessionInfo.SessionID)
	ctx.Request.URL.Path = "/ws"
	handler.ServeHTTP(ctx.Writer, ctx.Request)
}

// OpenSession creates or reuses a session for one host.
func (c *RDPController) OpenSession(ctx *gin.Context) {
	admin, err := getRDPAdmin(ctx)
	if err != nil {
		result.Failed(ctx, http.StatusUnauthorized, err.Error())
		return
	}

	rdpID, hostID, err := getRDPHostID(ctx)
	if err != nil {
		result.Failed(ctx, http.StatusBadRequest, err.Error())
		return
	}

	allowed, err := hasRDPHostPermission(admin, hostID)
	if err != nil {
		result.Failed(ctx, http.StatusInternalServerError, "failed to check host permission")
		return
	}
	if !allowed {
		result.Failed(ctx, http.StatusForbidden, "no permission for this host")
		return
	}

	session, err := c.service.OpenSession(rdpID, admin.ID, admin.Username)
	if err != nil {
		result.Failed(ctx, http.StatusBadRequest, err.Error())
		return
	}

	result.Success(ctx, gin.H{
		"session": session,
		"message": "session ready",
	})
}

// UploadHostFile uploads one local file to the target Windows host Downloads directory.
func (c *RDPController) UploadHostFile(ctx *gin.Context) {
	admin, err := getRDPAdmin(ctx)
	if err != nil {
		result.Failed(ctx, http.StatusUnauthorized, err.Error())
		return
	}

	hostIDStr := strings.TrimSpace(ctx.Param("hostId"))
	hostID, err := strconv.ParseUint(hostIDStr, 10, 64)
	if err != nil || hostID == 0 {
		result.Failed(ctx, http.StatusBadRequest, "invalid hostId")
		return
	}

	allowed, err := hasRDPHostPermission(admin, uint(hostID))
	if err != nil {
		result.Failed(ctx, http.StatusInternalServerError, "failed to check host permission")
		return
	}
	if !allowed {
		result.Failed(ctx, http.StatusForbidden, "no permission for this host")
		return
	}

	fileHeader, err := ctx.FormFile("file")
	if err != nil {
		result.Failed(ctx, http.StatusBadRequest, "missing file: "+err.Error())
		return
	}

	requestedName := ctx.PostForm("fileName")
	log.Printf("[RDPUpload] request received: hostId=%d userId=%d user=%s file=%s requestedName=%s", hostID, admin.ID, admin.Username, fileHeader.Filename, requestedName)
	task, err := c.fileService.CreateUploadTask(
		uint(hostID),
		fileHeader,
		requestedName,
		resolvePublicBaseURL(ctx),
		admin.ID,
		admin.Username,
	)
	if err != nil {
		log.Printf("[RDPUpload] request failed: hostId=%d file=%s err=%v", hostID, fileHeader.Filename, err)
		result.Failed(ctx, http.StatusInternalServerError, err.Error())
		return
	}
	log.Printf("[RDPUpload] request accepted: hostId=%d file=%s taskId=%s", hostID, task.FileName, task.TaskID)

	result.Success(ctx, gin.H{
		"hostId":   hostID,
		"fileName": task.FileName,
		"task":     task,
		"message":  "文件已接收，后台上传任务已启动",
	})
}

// GetUploadTaskStatus returns WinRM upload task status.
func (c *RDPController) GetUploadTaskStatus(ctx *gin.Context) {
	admin, err := getRDPAdmin(ctx)
	if err != nil {
		result.Failed(ctx, http.StatusUnauthorized, err.Error())
		return
	}

	taskID := strings.TrimSpace(ctx.Param("taskId"))
	if taskID == "" {
		result.Failed(ctx, http.StatusBadRequest, "missing taskId")
		return
	}

	task, err := c.fileService.GetUploadTask(taskID)
	if err != nil {
		result.Failed(ctx, http.StatusNotFound, err.Error())
		return
	}

	if !isRDPAdmin(admin) && task.CreatedBy != admin.ID {
		result.Failed(ctx, http.StatusForbidden, "no permission for this task")
		return
	}

	allowed, err := hasRDPHostPermission(admin, task.HostID)
	if err != nil {
		result.Failed(ctx, http.StatusInternalServerError, "failed to check host permission")
		return
	}
	if !allowed {
		result.Failed(ctx, http.StatusForbidden, "no permission for this host")
		return
	}

	result.Success(ctx, gin.H{
		"task": task,
	})
}

// DisconnectRDP closes one session or all sessions of a host.
func (c *RDPController) DisconnectRDP(ctx *gin.Context) {
	admin, err := getRDPAdmin(ctx)
	if err != nil {
		result.Failed(ctx, http.StatusUnauthorized, err.Error())
		return
	}

	sessionID := strings.TrimSpace(ctx.Query("sessionId"))
	if sessionID != "" {
		session, err := c.service.GetSession(sessionID)
		if err != nil {
			result.Failed(ctx, http.StatusNotFound, err.Error())
			return
		}
		if !canAccessRDPSession(admin, session) {
			result.Failed(ctx, http.StatusForbidden, "no permission for this session")
			return
		}
		allowed, err := hasRDPHostPermission(admin, session.HostID)
		if err != nil {
			result.Failed(ctx, http.StatusInternalServerError, "failed to check host permission")
			return
		}
		if !allowed {
			result.Failed(ctx, http.StatusForbidden, "no permission for this host")
			return
		}
		if err := c.service.DisconnectSession(sessionID); err != nil {
			result.Failed(ctx, http.StatusNotFound, err.Error())
			return
		}
		result.Success(ctx, gin.H{
			"message":   "RDP session disconnected",
			"sessionId": sessionID,
		})
		return
	}

	rdpID, hostID, err := getRDPHostID(ctx)
	if err != nil {
		result.Failed(ctx, http.StatusBadRequest, err.Error())
		return
	}
	allowed, err := hasRDPHostPermission(admin, hostID)
	if err != nil {
		result.Failed(ctx, http.StatusInternalServerError, "failed to check host permission")
		return
	}
	if !allowed {
		result.Failed(ctx, http.StatusForbidden, "no permission for this host")
		return
	}

	ids := c.service.FindSessionIDsByHost(rdpID, admin.ID, isRDPAdmin(admin))
	if len(ids) == 0 {
		result.Failed(ctx, http.StatusNotFound, "no session found for host")
		return
	}

	disconnected := 0
	for _, id := range ids {
		if err := c.service.DisconnectSession(id); err == nil {
			disconnected++
		}
	}

	result.Success(ctx, gin.H{
		"message":      "RDP session disconnected",
		"rdpid":        rdpID,
		"disconnected": disconnected,
	})
}

// GetActiveSessions returns visible sessions for current user.
func (c *RDPController) GetActiveSessions(ctx *gin.Context) {
	admin, err := getRDPAdmin(ctx)
	if err != nil {
		result.Failed(ctx, http.StatusUnauthorized, err.Error())
		return
	}

	sessions := c.service.GetActiveSessions()
	if isRDPAdmin(admin) {
		result.Success(ctx, gin.H{
			"sessions": sessions,
			"count":    len(sessions),
		})
		return
	}

	authDao := dao.NewBastionHostAuthorizationDao()
	authorizedHostIDs, err := authDao.GetUserAuthorizedHostIds(admin.ID)
	if err != nil {
		result.Failed(ctx, http.StatusInternalServerError, "failed to load authorized hosts")
		return
	}

	authorizedSet := make(map[uint]struct{}, len(authorizedHostIDs))
	for _, hostID := range authorizedHostIDs {
		authorizedSet[hostID] = struct{}{}
	}

	filteredSessions := make([]cmdbservice.RDPSessionInfo, 0, len(sessions))
	for _, session := range sessions {
		if session.OwnerID != admin.ID {
			continue
		}
		if _, allowed := authorizedSet[session.HostID]; allowed {
			filteredSessions = append(filteredSessions, session)
		}
	}

	result.Success(ctx, gin.H{
		"sessions": filteredSessions,
		"count":    len(filteredSessions),
	})
}

type rdpHostRequest struct {
	RDPID string `json:"rdpid" form:"rdpid"`
	ID    string `json:"id" form:"id"`
}

func getRDPHostID(ctx *gin.Context) (string, uint, error) {
	rdpID := strings.TrimSpace(ctx.Query("rdpid"))
	if rdpID == "" {
		rdpID = strings.TrimSpace(ctx.Query("id"))
	}
	if rdpID == "" {
		rdpID = strings.TrimSpace(ctx.PostForm("rdpid"))
	}
	if rdpID == "" {
		rdpID = strings.TrimSpace(ctx.PostForm("id"))
	}
	if rdpID == "" {
		var req rdpHostRequest
		if err := ctx.ShouldBindJSON(&req); err == nil {
			rdpID = strings.TrimSpace(req.RDPID)
			if rdpID == "" {
				rdpID = strings.TrimSpace(req.ID)
			}
		}
	}
	if rdpID == "" {
		return "", 0, errors.New("missing rdpid parameter")
	}

	hostID, err := strconv.ParseUint(rdpID, 10, 64)
	if err != nil {
		return "", 0, errors.New("invalid rdpid parameter")
	}

	return rdpID, uint(hostID), nil
}

func getRDPTokenClaims(ctx *gin.Context) (*systemModel.JwtAdmin, error) {
	token := strings.TrimSpace(ctx.Query("token"))
	if token == "" {
		authHeader := strings.TrimSpace(ctx.GetHeader("Authorization"))
		if authHeader == "" {
			return nil, errors.New("missing token or Authorization header")
		}
		if !strings.HasPrefix(authHeader, "Bearer ") {
			return nil, errors.New("invalid Authorization header format")
		}
		token = strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
	}
	claims, err := jwt.ValidateToken(token)
	if err != nil {
		return nil, err
	}
	return claims, nil
}

func getRDPAdmin(ctx *gin.Context) (*systemModel.JwtAdmin, error) {
	return jwt.GetAdmin(ctx)
}

func hasRDPHostPermission(admin *systemModel.JwtAdmin, hostID uint) (bool, error) {
	if admin == nil {
		return false, errors.New("failed to get user info")
	}
	if isRDPAdmin(admin) {
		return true, nil
	}

	authDao := dao.NewBastionHostAuthorizationDao()
	return authDao.CheckUserHostPermission(admin.ID, hostID)
}

func canAccessRDPSession(admin *systemModel.JwtAdmin, session *cmdbservice.RDPSessionInfo) bool {
	if admin == nil || session == nil {
		return false
	}
	if isRDPAdmin(admin) {
		return true
	}
	return session.OwnerID == admin.ID
}

func isRDPAdmin(admin *systemModel.JwtAdmin) bool {
	return admin != nil && (admin.Username == "admin" || admin.ID == 1)
}

func resolvePublicBaseURL(ctx *gin.Context) string {
	scheme := "http"
	if ctx.Request != nil && ctx.Request.TLS != nil {
		scheme = "https"
	}
	if proto := strings.TrimSpace(ctx.GetHeader("X-Forwarded-Proto")); proto != "" {
		scheme = proto
	}

	host := strings.TrimSpace(ctx.GetHeader("X-Forwarded-Host"))
	if host == "" && ctx.Request != nil {
		host = strings.TrimSpace(ctx.Request.Host)
	}
	if host != "" {
		return scheme + "://" + host
	}

	if config.Config != nil {
		publicURL := strings.TrimRight(strings.TrimSpace(config.Config.Server.PublicURL), "/")
		if publicURL != "" {
			return publicURL
		}
	}

	return "http://127.0.0.1:8000"
}
