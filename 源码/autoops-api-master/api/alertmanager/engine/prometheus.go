package engine

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"dodevops-api/api/alertmanager/model"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
)

var promAPICache = struct {
	sync.RWMutex
	items map[string]cachedPromAPI
}{
	items: make(map[string]cachedPromAPI),
}

type cachedPromAPI struct {
	api       v1.API
	expiresAt time.Time
}

func newPromAPI(ds model.AlertDatasource) (v1.API, error) {
	cacheKey := promAPICacheKey(ds)
	now := time.Now()
	promAPICache.RLock()
	cached, ok := promAPICache.items[cacheKey]
	promAPICache.RUnlock()
	if ok && now.Before(cached.expiresAt) {
		return cached.api, nil
	}

	roundTripper := http.DefaultTransport
	switch strings.ToLower(strings.TrimSpace(ds.AuthType)) {
	case "", "none":
	case "basic":
		roundTripper = &basicAuthRoundTripper{
			username: ds.Username,
			password: ds.Password,
			next:     roundTripper,
		}
	case "bearer":
		roundTripper = &bearerAuthRoundTripper{
			token: ds.Token,
			next:  roundTripper,
		}
	default:
		return nil, fmt.Errorf("unsupported auth type: %s", ds.AuthType)
	}

	client, err := api.NewClient(api.Config{
		Address:      normalizeQueryURL(ds.URL),
		RoundTripper: roundTripper,
	})
	if err != nil {
		return nil, err
	}
	promAPI := v1.NewAPI(client)
	promAPICache.Lock()
	promAPICache.items[cacheKey] = cachedPromAPI{
		api:       promAPI,
		expiresAt: now.Add(5 * time.Minute),
	}
	promAPICache.Unlock()
	return promAPI, nil
}

func promAPICacheKey(ds model.AlertDatasource) string {
	return strings.Join([]string{
		fmt.Sprintf("id:%d", ds.ID),
		"url:" + normalizeQueryURL(ds.URL),
		"auth:" + strings.TrimSpace(ds.AuthType),
		"user:" + strings.TrimSpace(ds.Username),
		"password:" + strings.TrimSpace(ds.Password),
		"token:" + strings.TrimSpace(ds.Token),
	}, "|")
}

func normalizeQueryURL(rawURL string) string {
	url := strings.TrimRight(strings.TrimSpace(rawURL), "/")
	for _, suffix := range []string{"/api/v1/import/prometheus", "/api/v1/write", "/write"} {
		if strings.HasSuffix(url, suffix) {
			return strings.TrimRight(strings.TrimSuffix(url, suffix), "/")
		}
	}
	return url
}

func isPromQLDatasource(datasourceType string) bool {
	switch datasourceType {
	case model.AlertDatasourceBuiltin, model.AlertDatasourcePrometheus, model.AlertDatasourceVictoriaMetrics:
		return true
	default:
		return false
	}
}

type basicAuthRoundTripper struct {
	username string
	password string
	next     http.RoundTripper
}

func (rt *basicAuthRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req.SetBasicAuth(rt.username, rt.password)
	return rt.next.RoundTrip(req)
}

type bearerAuthRoundTripper struct {
	token string
	next  http.RoundTripper
}

func (rt *bearerAuthRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", "Bearer "+rt.token)
	return rt.next.RoundTrip(req)
}
