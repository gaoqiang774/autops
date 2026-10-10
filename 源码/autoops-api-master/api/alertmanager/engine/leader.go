package engine

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"time"

	"dodevops-api/pkg/log"

	"github.com/go-redis/redis/v8"
)

const (
	alertEngineLeaderKey     = "alertmanager:engine:leader"
	alertEngineLeaderTTL     = 15 * time.Second
	alertEngineLeaderRenew   = 5 * time.Second
	alertEngineLeaderRetry   = 3 * time.Second
	alertEngineLeaderTimeout = 2 * time.Second
	alertEngineDBLeaderLock  = "autoops:alertmanager:engine:leader"
)

type alertEngineLeaderRuntime struct {
	cancel context.CancelFunc
	done   chan struct{}
}

type alertEngineDBLeaderLease struct {
	conn *sql.Conn
}

var renewAlertEngineLeadershipScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
	return redis.call("PEXPIRE", KEYS[1], ARGV[2])
end
return 0
`)

var releaseAlertEngineLeadershipScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
	return redis.call("DEL", KEYS[1])
end
return 0
`)

func (e *Engine) runWithLeaderElection(ctx context.Context, client *redis.Client) {
	token := newAlertEngineLeaderToken()
	var redisUnavailableSince time.Time
	var redisFailureCount int
	var redisWasAvailable bool
	var fallbackLease *alertEngineDBLeaderLease
	var fallbackRuntime *alertEngineLeaderRuntime

	stopFallback := func() {
		if fallbackRuntime != nil {
			e.stopLeaderRuntime(fallbackRuntime)
			fallbackRuntime = nil
		}
		if fallbackLease != nil {
			fallbackLease.release()
			fallbackLease = nil
		}
	}
	defer stopFallback()

	for {
		if ctx.Err() != nil {
			return
		}

		redisAvailable := false
		if client != nil {
			acquired, err := acquireAlertEngineLeadership(ctx, client, token)
			if err == nil {
				redisAvailable = true
				redisWasAvailable = true
				redisUnavailableSince = time.Time{}
				redisFailureCount = 0
				stopFallback()
				if acquired {
					log.Log().Info("alert engine Redis leader lease acquired")
					if e.holdAlertEngineLeadership(ctx, client, token) {
						return
					}
					continue
				}
			} else {
				redisFailureCount++
				if redisUnavailableSince.IsZero() {
					redisUnavailableSince = time.Now()
				}
				if redisFailureCount == 1 || redisFailureCount%20 == 0 {
					log.Log().Warnf("alert engine Redis leader lease unavailable, database fallback will be used: %v", err)
				}
			}
		}

		// A healthy Redis lease is authoritative. If another instance owns it,
		// this process must remain idle instead of also running via the database.
		if redisAvailable {
			if waitAlertEngineRetry(ctx) {
				return
			}
			continue
		}

		// When a running Redis instance disappears, wait for the old lease to
		// expire before falling back so two former Redis peers cannot overlap.
		if redisWasAvailable && time.Since(redisUnavailableSince) < alertEngineLeaderTTL {
			if waitAlertEngineRetry(ctx) {
				return
			}
			continue
		}

		if fallbackLease == nil {
			lease, acquired, err := e.acquireDBLeaderLease(ctx)
			if err != nil {
				log.Log().Warnf("alert engine database fallback lease failed: %v", err)
			} else if acquired {
				fallbackLease = lease
				fallbackRuntime = e.startLeaderRuntime(ctx)
				log.Log().Warn("alert engine is running with database leader lease because Redis is unavailable")
			}
		} else if held, err := fallbackLease.held(ctx); err != nil || !held {
			if err != nil {
				log.Log().Warnf("alert engine database fallback lease check failed: %v", err)
			} else {
				log.Log().Warn("alert engine database fallback lease lost")
			}
			stopFallback()
		}

		if waitAlertEngineRetry(ctx) {
			return
		}
	}
}

// holdAlertEngineLeadership runs workers until the lease is lost. Returning
// true means the parent context was canceled and the election loop can exit.
func (e *Engine) holdAlertEngineLeadership(ctx context.Context, client *redis.Client, token string) bool {
	runtime := e.startLeaderRuntime(ctx)

	ticker := time.NewTicker(alertEngineLeaderRenew)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			e.stopLeaderRuntime(runtime)
			releaseAlertEngineLeadership(client, token)
			return true
		case <-ticker.C:
			renewed, err := renewAlertEngineLeadership(ctx, client, token)
			if err == nil && renewed {
				continue
			}
			e.stopLeaderRuntime(runtime)
			if err != nil {
				log.Log().Warnf("alert engine renew leader lease failed: %v", err)
			} else {
				log.Log().Warn("alert engine leader lease lost")
			}
			return false
		}
	}
}

func (e *Engine) startLeaderRuntime(ctx context.Context) *alertEngineLeaderRuntime {
	leaderCtx, cancel := context.WithCancel(ctx)
	runtime := &alertEngineLeaderRuntime{cancel: cancel, done: make(chan struct{})}
	e.setRunContext(leaderCtx)
	e.setLeader(true)
	go func() {
		defer close(runtime.done)
		e.runLeader(leaderCtx)
	}()
	return runtime
}

func (e *Engine) stopLeaderRuntime(runtime *alertEngineLeaderRuntime) {
	if runtime == nil {
		return
	}
	e.setLeader(false)
	runtime.cancel()
	<-runtime.done
	e.setRunContext(nil)
}

func (e *Engine) acquireDBLeaderLease(ctx context.Context) (*alertEngineDBLeaderLease, bool, error) {
	if e == nil || e.repo == nil || e.repo.db == nil {
		return nil, false, fmt.Errorf("database is unavailable")
	}
	db, err := e.repo.db.DB()
	if err != nil {
		return nil, false, err
	}
	opCtx, cancel := context.WithTimeout(ctx, alertEngineLeaderTimeout)
	defer cancel()
	conn, err := db.Conn(opCtx)
	if err != nil {
		return nil, false, err
	}
	var acquired sql.NullInt64
	if err := conn.QueryRowContext(opCtx, "SELECT GET_LOCK(?, 0)", alertEngineDBLeaderLock).Scan(&acquired); err != nil {
		_ = conn.Close()
		return nil, false, err
	}
	if !acquired.Valid || acquired.Int64 != 1 {
		_ = conn.Close()
		return nil, false, nil
	}
	return &alertEngineDBLeaderLease{conn: conn}, true, nil
}

func (l *alertEngineDBLeaderLease) held(ctx context.Context) (bool, error) {
	if l == nil || l.conn == nil {
		return false, nil
	}
	opCtx, cancel := context.WithTimeout(ctx, alertEngineLeaderTimeout)
	defer cancel()
	var held sql.NullInt64
	err := l.conn.QueryRowContext(opCtx,
		"SELECT IF(IS_USED_LOCK(?) = CONNECTION_ID(), 1, 0)", alertEngineDBLeaderLock,
	).Scan(&held)
	return err == nil && held.Valid && held.Int64 == 1, err
}

func (l *alertEngineDBLeaderLease) release() {
	if l == nil || l.conn == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), alertEngineLeaderTimeout)
	defer cancel()
	_, _ = l.conn.ExecContext(ctx, "SELECT RELEASE_LOCK(?)", alertEngineDBLeaderLock)
	_ = l.conn.Close()
	l.conn = nil
}

func waitAlertEngineRetry(ctx context.Context) bool {
	timer := time.NewTimer(alertEngineLeaderRetry)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return true
	case <-timer.C:
		return false
	}
}

func acquireAlertEngineLeadership(ctx context.Context, client *redis.Client, token string) (bool, error) {
	opCtx, cancel := context.WithTimeout(ctx, alertEngineLeaderTimeout)
	defer cancel()
	return client.SetNX(opCtx, alertEngineLeaderKey, token, alertEngineLeaderTTL).Result()
}

func renewAlertEngineLeadership(ctx context.Context, client *redis.Client, token string) (bool, error) {
	opCtx, cancel := context.WithTimeout(ctx, alertEngineLeaderTimeout)
	defer cancel()
	result, err := renewAlertEngineLeadershipScript.Run(opCtx, client, []string{alertEngineLeaderKey}, token, alertEngineLeaderTTL.Milliseconds()).Int64()
	return result == 1, err
}

func releaseAlertEngineLeadership(client *redis.Client, token string) {
	ctx, cancel := context.WithTimeout(context.Background(), alertEngineLeaderTimeout)
	defer cancel()
	if err := releaseAlertEngineLeadershipScript.Run(ctx, client, []string{alertEngineLeaderKey}, token).Err(); err != nil && err != redis.Nil {
		log.Log().Warnf("alert engine release leader lease failed: %v", err)
	}
}

func newAlertEngineLeaderToken() string {
	hostname, _ := os.Hostname()
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err == nil {
		return fmt.Sprintf("%s:%d:%s", hostname, os.Getpid(), hex.EncodeToString(buffer))
	}
	return fmt.Sprintf("%s:%d:%s", hostname, os.Getpid(), time.Now().Format("20060102150405.000000000"))
}
