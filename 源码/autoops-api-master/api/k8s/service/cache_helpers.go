package service

import (
	"context"
	"fmt"
	"time"

	"dodevops-api/common"
	"dodevops-api/common/cache"
)

const (
	nodesListCacheTTL     = 20 * time.Second
	servicesListCacheTTL  = 20 * time.Second
	workloadsListCacheTTL = 15 * time.Second
	workloadsListCacheVer = "v2"
)

func newK8sCacheService() cache.ICacheService {
	redisClient := common.GetRedisClient()
	if redisClient == nil {
		return nil
	}

	return cache.NewRedisCache(redisClient)
}

func getCachedJSON(ctx context.Context, cacheService cache.ICacheService, key string, dest interface{}) bool {
	if cacheService == nil {
		return false
	}

	return cacheService.GetJSON(ctx, key, dest) == nil
}

func setCachedJSON(ctx context.Context, cacheService cache.ICacheService, key string, value interface{}, ttl time.Duration) {
	if cacheService == nil {
		return
	}

	_ = cacheService.SetJSON(ctx, key, value, ttl)
}

func invalidateCachedJSON(ctx context.Context, cacheService cache.ICacheService, keys ...string) {
	if cacheService == nil || len(keys) == 0 {
		return
	}

	_ = cacheService.Del(ctx, keys...)
}

func nodesListCacheKey(clusterID uint) string {
	return fmt.Sprintf("k8s:nodes:list:%d", clusterID)
}

func servicesListCacheKey(clusterID uint, namespaceName string) string {
	return fmt.Sprintf("k8s:services:list:%d:%s", clusterID, namespaceName)
}

func workloadsListCacheKey(clusterID uint, namespaceName, workloadType string) string {
	return fmt.Sprintf("k8s:workloads:list:%s:%d:%s:%s", workloadsListCacheVer, clusterID, namespaceName, workloadType)
}

func invalidateWorkloadListCaches(ctx context.Context, cacheService cache.ICacheService, clusterID uint, namespaceName string) {
	invalidateCachedJSON(ctx, cacheService, workloadListCacheKeys(clusterID, namespaceName)...)
}

func workloadListCacheKeys(clusterID uint, namespaceName string) []string {
	workloadTypes := []string{
		"", "all",
		"pod", "pods",
		"deployment", "deployments",
		"statefulset", "statefulsets",
		"daemonset", "daemonsets",
		"job", "jobs",
		"cronjob", "cronjobs",
	}
	namespaces := []string{namespaceName}
	if namespaceName != "" {
		namespaces = append(namespaces, "")
	}
	keys := make([]string, 0, len(workloadTypes)*len(namespaces))
	for _, namespace := range namespaces {
		for _, workloadType := range workloadTypes {
			keys = append(keys, workloadsListCacheKey(clusterID, namespace, workloadType))
		}
	}
	return keys
}
