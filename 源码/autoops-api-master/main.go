package main

import (
	"context"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	alertengine "dodevops-api/api/alertmanager/engine"
	alertmanagerservice "dodevops-api/api/alertmanager/service"
	taskservice "dodevops-api/api/task/service"
	ticketservice "dodevops-api/api/ticket/service"
	"dodevops-api/common"
	"dodevops-api/common/config"
	"dodevops-api/common/snmp"
	"dodevops-api/pkg/db"
	"dodevops-api/pkg/log"
	"dodevops-api/pkg/redis"
	"dodevops-api/router"

	"github.com/gin-gonic/gin"
)

// @title devops运维管理系统
// @version 1.0
// @description devops运维管理系统API接口文档
// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name Authorization
func main() {
	if err := config.LoadConfig("./config.yaml"); err != nil {
		panic("Failed to load config: " + err.Error())
	}

	if err := initServices(); err != nil {
		panic("Failed to initialize services: " + err.Error())
	}
	initTaskRuntime()

	snmp.GetManager().Start()

	appLog := log.Log()
	gin.SetMode(config.Config.Server.Model)
	httpRouter := router.InitRouter()
	srv := &http.Server{
		Addr:    config.Config.Server.Address,
		Handler: httpRouter,
	}
	engineCtx, stopEngine := context.WithCancel(context.Background())
	alertEngine := alertengine.New(common.GetDB())
	alertengine.SetDefault(alertEngine)
	go alertEngine.Run(engineCtx)
	go alertmanagerservice.StartFeishuWSWatcher(engineCtx)
	go alertmanagerservice.StartDomainMonitor(engineCtx)
	go ticketservice.StartReleaseSynchronizer(engineCtx, common.GetDB())

	go func() {
		if config.Config.Server.EnableSwagger {
			appLog.Info("API文档地址: " + swaggerDocURL())
		}
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			appLog.Infof("listen: %s \n", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	shutdownSignal := <-quit
	signal.Stop(quit)
	appLog.Infof("Shutdown Server ... signal=%s", shutdownSignal)

	snmp.GetManager().Stop()
	stopEngine()
	taskservice.StopGlobalScheduler()
	taskservice.StopTaskQueue()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		appLog.Infof("Server Shutdown: %v", err)
	}
	appLog.Info("Server exiting")
}

func initTaskRuntime() {
	if common.GetRedisClient() != nil {
		if err := taskservice.InitTaskQueue(taskservice.DefaultConfig()); err != nil {
			log.Log().Errorf("Failed to initialize task queue: %v", err)
		}
	} else {
		log.Log().Warn("Task queue disabled because Redis is unavailable")
	}

	if err := taskservice.InitGlobalScheduler(); err != nil {
		log.Log().Errorf("Failed to initialize task scheduler: %v", err)
	}
}

func initServices() error {
	if err := db.SetupDBLink(); err != nil {
		return err
	}

	if err := common.InitRedis(); err != nil {
		log.Log().Warnf("Redis initialization failed: %v", err)
	}
	redis.SetupRedisDb()

	return nil
}

func swaggerDocURL() string {
	baseURL := strings.TrimRight(strings.TrimSpace(config.Config.Server.PublicURL), "/")
	if baseURL == "" {
		baseURL = "http://" + swaggerDocAccessHost(config.Config.Server.Address)
	}
	return baseURL + "/swagger/index.html"
}

func swaggerDocAccessHost(bindAddr string) string {
	host, port, err := net.SplitHostPort(bindAddr)
	if err != nil {
		return bindAddr
	}

	host = strings.TrimSpace(host)
	switch host {
	case "", "0.0.0.0", "::":
		if localIP := firstLocalIPv4(); localIP != "" {
			host = localIP
		} else {
			host = "127.0.0.1"
		}
	case "::1":
		host = "127.0.0.1"
	}

	return net.JoinHostPort(host, port)
}

func firstLocalIPv4() string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return ""
	}

	var fallback string
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			default:
				continue
			}

			ip = ip.To4()
			if ip == nil || ip.IsLoopback() {
				continue
			}
			if ip.IsPrivate() {
				return ip.String()
			}
			if fallback == "" {
				fallback = ip.String()
			}
		}
	}

	return fallback
}
