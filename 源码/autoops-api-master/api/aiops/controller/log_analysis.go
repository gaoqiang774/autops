package controller

import (
	"encoding/json"
	"net/http"
	"strings"

	"dodevops-api/api/aiops/model"
	"dodevops-api/api/aiops/service"
	"dodevops-api/common/constant"
	"dodevops-api/common/result"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

func ListHostLogFiles(c *gin.Context) {
	var query model.HostLogFilesQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "参数错误")
		return
	}

	data, err := service.NewLogAnalysisService().ListHostLogFiles(c, query)
	if err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "获取主机日志文件列表失败: "+err.Error())
		return
	}
	result.Success(c, data)
}

func ListLogSessions(c *gin.Context) {
	var query model.LogSessionsQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "参数错误")
		return
	}
	result.Success(c, service.NewLogAnalysisService().ListSessions(query))
}

func GetLogSessionStatus(c *gin.Context) {
	var query model.LogSessionStatusQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "参数错误")
		return
	}

	data, err := service.NewLogAnalysisService().GetSessionStatus(query.SessionID)
	if err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "查询日志分析会话状态失败: "+err.Error())
		return
	}
	result.Success(c, data)
}

func QueryHostLogs(c *gin.Context) {
	var req model.HostLogQueryRequest
	if err := c.BindJSON(&req); err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "参数错误")
		return
	}
	if req.SourceType == "" {
		req.SourceType = model.LogSourceTypeHost
	}
	if req.SourceType != model.LogSourceTypeHost {
		result.Failed(c, constant.INVALID_PARAMS, "当前仅支持主机日志查询")
		return
	}

	data, err := service.NewLogAnalysisService().QueryHostLogs(c, &req)
	if err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "查询主机日志失败: "+err.Error())
		return
	}
	result.Success(c, data)
}

func StreamHostLogs(c *gin.Context) {
	var req model.HostLogStreamQuery
	if err := c.ShouldBindQuery(&req); err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "参数错误")
		return
	}
	if req.SourceType == "" {
		req.SourceType = model.LogSourceTypeHost
	}
	if req.SourceType != model.LogSourceTypeHost {
		result.Failed(c, constant.INVALID_PARAMS, "当前仅支持主机日志实时追踪")
		return
	}
	if c.Query("follow") == "" {
		req.Follow = true
	}

	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	wsConn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer wsConn.Close()

	if err := service.NewLogAnalysisService().StreamHostLogs(c, &req, wsConn); err != nil {
		_ = wsConn.WriteJSON(model.LogStreamEvent{
			Type:    "error",
			Message: err.Error(),
		})
	}
}

func QueryK8sLogs(c *gin.Context) {
	var req model.K8sLogQueryRequest
	if err := c.BindJSON(&req); err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "参数错误")
		return
	}
	if req.SourceType == "" {
		req.SourceType = model.LogSourceTypeK8s
	}
	if req.SourceType != model.LogSourceTypeK8s {
		result.Failed(c, constant.INVALID_PARAMS, "当前仅支持 K8s 日志查询")
		return
	}

	data, err := service.NewLogAnalysisService().QueryK8sLogs(c, &req)
	if err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "查询 K8s 日志失败: "+err.Error())
		return
	}
	result.Success(c, data)
}

func StreamK8sLogs(c *gin.Context) {
	var req model.K8sLogStreamQuery
	if err := c.ShouldBindQuery(&req); err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "参数错误")
		return
	}
	if req.SourceType == "" {
		req.SourceType = model.LogSourceTypeK8s
	}
	if req.SourceType != model.LogSourceTypeK8s {
		result.Failed(c, constant.INVALID_PARAMS, "当前仅支持 K8s 日志实时追踪")
		return
	}
	if c.Query("follow") == "" {
		req.Follow = true
	}

	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	wsConn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer wsConn.Close()

	if err := service.NewLogAnalysisService().StreamK8sLogs(c, &req, wsConn); err != nil {
		_ = wsConn.WriteJSON(model.LogStreamEvent{
			Type:    "error",
			Message: err.Error(),
		})
	}
}

func AnalyzeLogs(c *gin.Context) {
	var req model.LogAnalyzeRequest
	if err := c.BindJSON(&req); err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "参数错误")
		return
	}

	data, err := service.NewLogAnalysisService().AnalyzeLogs(c, &req)
	if err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "AI 分析日志失败: "+err.Error())
		return
	}
	result.Success(c, data)
}

func AnalyzeK8sPod(c *gin.Context) {
	var req model.K8sPodAnalyzeRequest
	if err := c.BindJSON(&req); err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "参数错误")
		return
	}

	data, err := service.NewLogAnalysisService().AnalyzeK8sPod(c, &req)
	if err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "AI 分析 K8s Pod 失败: "+err.Error())
		return
	}
	result.Success(c, data)
}

func AnalyzeK8sIngress(c *gin.Context) {
	var req model.K8sIngressAnalyzeRequest
	if c.Request != nil && c.Request.Method == http.MethodGet {
		if err := c.ShouldBindQuery(&req); err != nil {
			result.Failed(c, constant.INVALID_PARAMS, "参数错误")
			return
		}
	} else {
		if err := c.BindJSON(&req); err != nil {
			result.Failed(c, constant.INVALID_PARAMS, "参数错误")
			return
		}
	}

	data, err := service.NewLogAnalysisService().AnalyzeK8sIngress(c, &req)
	if err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "AI 分析 K8s Ingress 失败: "+err.Error())
		return
	}
	result.Success(c, data)
}

func StreamK8sIngressAnalyze(c *gin.Context) {
	var req model.K8sIngressAnalyzeRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "参数错误")
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache, no-transform")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Header("Transfer-Encoding", "chunked")
	c.Status(http.StatusOK)
	c.Writer.WriteHeaderNow()

	flusher, _ := c.Writer.(http.Flusher)
	writeSSE := func(eventName string, event model.K8sIngressAnalyzeProgressEvent) error {
		payload, err := json.Marshal(event)
		if err != nil {
			return err
		}
		if _, err := c.Writer.Write([]byte("event: " + eventName + "\n")); err != nil {
			return err
		}
		if _, err := c.Writer.Write([]byte("data: " + string(payload) + "\n\n")); err != nil {
			return err
		}
		if flusher != nil {
			flusher.Flush()
		}
		return nil
	}
	_, _ = c.Writer.Write([]byte(": ingress analyze stream opened " + strings.Repeat(" ", 2048) + "\n\n"))
	if flusher != nil {
		flusher.Flush()
	}
	err := service.NewLogAnalysisService().StreamK8sIngressAnalyze(c.Request.Context(), &req, func(event model.K8sIngressAnalyzeProgressEvent) error {
		return writeSSE(event.Type, event)
	})
	if err != nil {
		_ = writeSSE("error", model.K8sIngressAnalyzeProgressEvent{
			Type:      "error",
			Status:    "failed",
			Progress:  100,
			Message:   "分析进度流中断",
			Error:     err.Error(),
			Timestamp: "",
		})
	}
}

func StartK8sPodAnalyzeTask(c *gin.Context) {
	var req model.K8sPodAnalyzeRequest
	if err := c.BindJSON(&req); err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "参数错误")
		return
	}

	data, err := service.NewLogAnalysisService().StartK8sPodAnalyzeTask(&req)
	if err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "创建 K8s Pod AI 分析任务失败: "+err.Error())
		return
	}
	result.Success(c, data)
}

func GetK8sPodAnalyzeTask(c *gin.Context) {
	taskID := c.Param("taskId")
	data, err := service.NewLogAnalysisService().GetK8sPodAnalyzeTask(taskID)
	if err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "查询 K8s Pod AI 分析任务失败: "+err.Error())
		return
	}
	result.Success(c, data)
}

func StreamK8sPodAnalyzeTask(c *gin.Context) {
	taskID := c.Param("taskId")
	if _, err := service.NewLogAnalysisService().GetK8sPodAnalyzeTask(taskID); err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "查询 K8s Pod AI 分析任务失败: "+err.Error())
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)

	flusher, _ := c.Writer.(http.Flusher)
	err := service.NewLogAnalysisService().StreamK8sPodAnalyzeTask(c.Request.Context(), taskID, func(event model.K8sPodAnalyzeProgressEvent) error {
		c.SSEvent(event.Type, event)
		if flusher != nil {
			flusher.Flush()
		}
		return nil
	})
	if err != nil {
		c.SSEvent("error", model.K8sPodAnalyzeProgressEvent{
			Type:    "error",
			Status:  "failed",
			Message: "分析进度流中断",
			Error:   err.Error(),
		})
		if flusher != nil {
			flusher.Flush()
		}
	}
}
