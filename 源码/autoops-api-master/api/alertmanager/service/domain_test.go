package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"dodevops-api/api/alertmanager/model"
)

func TestProbeDomainSuccess(t *testing.T) {
	item := model.AlertDomain{
		Protocol: "http", Path: "/",
		ExpectedStatus: http.StatusNoContent, TimeoutSeconds: 2, SSLExpireWarnDays: 1,
	}
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer httpServer.Close()
	httpParsed, _ := url.Parse(httpServer.URL)
	item.Domain = httpParsed.Hostname()
	item.Port, _ = strconv.Atoi(httpParsed.Port())
	result := probeDomain(context.Background(), item)
	if result.Status != model.DomainStatusNormal || result.StatusCode != http.StatusNoContent {
		t.Fatalf("unexpected probe result: %#v", result)
	}
}

func TestProbeDomainStatusMismatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()
	parsed, _ := url.Parse(server.URL)
	port, _ := strconv.Atoi(parsed.Port())
	result := probeDomain(context.Background(), model.AlertDomain{
		Domain: parsed.Hostname(), Protocol: "http", Port: port, Path: "/",
		ExpectedStatus: http.StatusOK, TimeoutSeconds: 2, SSLExpireWarnDays: 30,
	})
	if result.Status != model.DomainStatusAbnormal || result.FailureType != "http_status" {
		t.Fatalf("unexpected probe result: %#v", result)
	}
}

func TestNormalizeDomainInputFromURL(t *testing.T) {
	item, err := normalizeDomainInput(model.AlertDomainInput{
		Domain: "http://Example.COM:8443/health?ready=1", Protocol: "https", Path: "/",
		Tags: []string{"prod", "prod"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if item.Domain != "example.com" || item.Protocol != "http" || item.Port != 8443 || item.Path != "/health?ready=1" {
		t.Fatalf("unexpected normalized domain: %#v", item)
	}
	if len(item.Tags) != 1 || item.Tags[0] != "prod" {
		t.Fatalf("unexpected tags: %#v", item.Tags)
	}
}

func TestNormalizeDomainInputRejectsInvalidDomainAndStatus(t *testing.T) {
	for _, input := range []model.AlertDomainInput{
		{Domain: "bad..example.com"},
		{Domain: "example.com", ExpectedStatus: 999},
	} {
		if _, err := normalizeDomainInput(input); err == nil {
			t.Fatalf("expected invalid input error for %#v", input)
		}
	}
}

func TestDomainTargetURLIPv6(t *testing.T) {
	item := model.AlertDomain{Domain: "::1", Protocol: "http", Path: "/health"}
	if got := domainTargetURL(item); got != "http://[::1]/health" {
		t.Fatalf("unexpected IPv6 URL: %s", got)
	}
}
