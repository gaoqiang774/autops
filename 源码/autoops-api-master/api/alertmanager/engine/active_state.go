package engine

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"dodevops-api/api/alertmanager/model"
	"dodevops-api/pkg/log"

	"github.com/go-redis/redis/v8"
)

const (
	activeStateEventKeyPrefix = "alertmanager:active:event:"
	activeStateRuleKeyPrefix  = "alertmanager:active:rule:"
	activeStateFingerprintSet = "alertmanager:active:fingerprints"
	activeStateRuleSet        = "alertmanager:active:rules"
	activeStateTimeout        = 2 * time.Second
)

type activeStateStore struct {
	client *redis.Client
}

func newActiveStateStore(client *redis.Client) *activeStateStore {
	return &activeStateStore{client: client}
}

func (s *activeStateStore) enabled() bool {
	return s != nil && s.client != nil
}

func (s *activeStateStore) listByRules(ruleIDs []uint) (map[uint][]model.AlertActiveEvent, []uint, bool) {
	grouped := make(map[uint][]model.AlertActiveEvent, len(ruleIDs))
	if !s.enabled() || len(ruleIDs) == 0 {
		return grouped, ruleIDs, false
	}

	ctx, cancel := context.WithTimeout(context.Background(), activeStateTimeout)
	defer cancel()

	pipe := s.client.Pipeline()
	ruleCommands := make(map[uint]*redis.StringSliceCmd, len(ruleIDs))
	for _, ruleID := range ruleIDs {
		ruleCommands[ruleID] = pipe.SMembers(ctx, activeStateRuleKey(ruleID))
	}
	if _, err := pipe.Exec(ctx); err != nil {
		log.Log().Warnf("alert engine redis active state smembers pipeline failed: err=%v", err)
		return grouped, ruleIDs, false
	}

	missingRuleIDs := make([]uint, 0)
	eventKeys := make([]string, 0)
	eventFingerprints := make([]string, 0)
	eventRuleIDs := make([]uint, 0)
	for _, ruleID := range ruleIDs {
		fingerprints, err := ruleCommands[ruleID].Result()
		if err != nil {
			log.Log().Warnf("alert engine redis active state smembers failed: rule=%d err=%v", ruleID, err)
			return grouped, ruleIDs, false
		}
		if len(fingerprints) == 0 {
			missingRuleIDs = append(missingRuleIDs, ruleID)
			continue
		}
		for _, fingerprint := range fingerprints {
			if fingerprint == "" {
				continue
			}
			eventKeys = append(eventKeys, activeStateEventKey(fingerprint))
			eventFingerprints = append(eventFingerprints, fingerprint)
			eventRuleIDs = append(eventRuleIDs, ruleID)
		}
	}
	if len(eventKeys) == 0 {
		return grouped, missingRuleIDs, true
	}

	values, err := s.client.MGet(ctx, eventKeys...).Result()
	if err != nil {
		log.Log().Warnf("alert engine redis active state mget failed: err=%v", err)
		return grouped, ruleIDs, false
	}

	staleFingerprints := make([]interface{}, 0)
	staleEventKeys := make([]string, 0)
	staleRuleMembers := make(map[uint][]interface{})
	loadedByRule := make(map[uint]int, len(ruleIDs))
	staleByRule := make(map[uint]int, len(ruleIDs))
	for i, value := range values {
		ruleID := eventRuleIDs[i]
		fingerprint := eventFingerprints[i]
		text, ok := value.(string)
		if !ok || text == "" {
			staleFingerprints = append(staleFingerprints, fingerprint)
			staleRuleMembers[ruleID] = append(staleRuleMembers[ruleID], fingerprint)
			staleByRule[ruleID]++
			continue
		}
		var event model.AlertActiveEvent
		if err := json.Unmarshal([]byte(text), &event); err != nil {
			log.Log().Warnf("alert engine redis active state unmarshal failed: rule=%d fingerprint=%s err=%v", ruleID, fingerprint, err)
			staleFingerprints = append(staleFingerprints, fingerprint)
			staleEventKeys = append(staleEventKeys, activeStateEventKey(fingerprint))
			staleRuleMembers[ruleID] = append(staleRuleMembers[ruleID], fingerprint)
			staleByRule[ruleID]++
			continue
		}
		if event.Status == model.EventStatusResolved {
			staleFingerprints = append(staleFingerprints, fingerprint)
			staleEventKeys = append(staleEventKeys, activeStateEventKey(fingerprint))
			staleRuleMembers[ruleID] = append(staleRuleMembers[ruleID], fingerprint)
			staleByRule[ruleID]++
			continue
		}
		grouped[ruleID] = append(grouped[ruleID], event)
		loadedByRule[ruleID]++
	}
	s.cleanupStaleState(ctx, staleFingerprints, staleEventKeys, staleRuleMembers)
	for ruleID, staleCount := range staleByRule {
		if staleCount > 0 && loadedByRule[ruleID] == 0 {
			missingRuleIDs = append(missingRuleIDs, ruleID)
		}
	}
	return grouped, missingRuleIDs, true
}

func (s *activeStateStore) listAll() ([]model.AlertActiveEvent, bool) {
	if !s.enabled() {
		return nil, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), activeStateTimeout)
	defer cancel()

	fingerprints, err := s.client.SMembers(ctx, activeStateFingerprintSet).Result()
	if err != nil {
		log.Log().Warnf("alert engine redis active state fingerprints failed: err=%v", err)
		return nil, false
	}
	if len(fingerprints) == 0 {
		return nil, false
	}

	validFingerprints := make([]string, 0, len(fingerprints))
	keys := make([]string, 0, len(fingerprints))
	for _, fingerprint := range fingerprints {
		if fingerprint == "" {
			continue
		}
		validFingerprints = append(validFingerprints, fingerprint)
		keys = append(keys, activeStateEventKey(fingerprint))
	}
	if len(keys) == 0 {
		return nil, false
	}
	values, err := s.client.MGet(ctx, keys...).Result()
	if err != nil {
		log.Log().Warnf("alert engine redis active state list all mget failed: err=%v", err)
		return nil, false
	}

	events := make([]model.AlertActiveEvent, 0, len(values))
	staleFingerprints := make([]interface{}, 0)
	staleEventKeys := make([]string, 0)
	staleRuleMembers := make(map[uint][]interface{})
	for i, value := range values {
		fingerprint := validFingerprints[i]
		text, ok := value.(string)
		if !ok || text == "" {
			staleFingerprints = append(staleFingerprints, fingerprint)
			continue
		}
		var event model.AlertActiveEvent
		if err := json.Unmarshal([]byte(text), &event); err != nil {
			log.Log().Warnf("alert engine redis active state list all unmarshal failed: fingerprint=%s err=%v", fingerprint, err)
			staleFingerprints = append(staleFingerprints, fingerprint)
			staleEventKeys = append(staleEventKeys, activeStateEventKey(fingerprint))
			continue
		}
		if event.Status == model.EventStatusResolved {
			staleFingerprints = append(staleFingerprints, fingerprint)
			staleEventKeys = append(staleEventKeys, activeStateEventKey(fingerprint))
			if event.RuleID > 0 {
				staleRuleMembers[event.RuleID] = append(staleRuleMembers[event.RuleID], fingerprint)
			}
			continue
		}
		events = append(events, event)
	}
	s.cleanupStaleState(ctx, staleFingerprints, staleEventKeys, staleRuleMembers)
	return events, true
}

func (s *activeStateStore) save(event model.AlertActiveEvent) {
	if !s.enabled() || event.Fingerprint == "" || event.RuleID == 0 || event.Status == model.EventStatusResolved {
		return
	}
	payload, err := json.Marshal(event)
	if err != nil {
		log.Log().Warnf("alert engine marshal active state failed: event=%d fingerprint=%s err=%v", event.ID, event.Fingerprint, err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), activeStateTimeout)
	defer cancel()
	pipe := s.client.Pipeline()
	pipe.Set(ctx, activeStateEventKey(event.Fingerprint), payload, 0)
	pipe.SAdd(ctx, activeStateRuleKey(event.RuleID), event.Fingerprint)
	pipe.SAdd(ctx, activeStateFingerprintSet, event.Fingerprint)
	pipe.SAdd(ctx, activeStateRuleSet, activeStateRuleMember(event.RuleID))
	if _, err := pipe.Exec(ctx); err != nil {
		log.Log().Warnf("alert engine save redis active state failed: event=%d fingerprint=%s err=%v", event.ID, event.Fingerprint, err)
	}
}

func (s *activeStateStore) delete(event model.AlertActiveEvent) {
	if !s.enabled() || event.Fingerprint == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), activeStateTimeout)
	defer cancel()
	pipe := s.client.Pipeline()
	pipe.Del(ctx, activeStateEventKey(event.Fingerprint))
	pipe.SRem(ctx, activeStateFingerprintSet, event.Fingerprint)
	if event.RuleID > 0 {
		pipe.SRem(ctx, activeStateRuleKey(event.RuleID), event.Fingerprint)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		log.Log().Warnf("alert engine delete redis active state failed: event=%d fingerprint=%s err=%v", event.ID, event.Fingerprint, err)
		return
	}
	if event.RuleID > 0 {
		s.removeEmptyRuleIndex(ctx, event.RuleID)
	}
}

func (s *activeStateStore) deleteRule(ruleID uint) {
	if !s.enabled() || ruleID == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), activeStateTimeout)
	defer cancel()

	ruleKey := activeStateRuleKey(ruleID)
	fingerprints, err := s.client.SMembers(ctx, ruleKey).Result()
	if err != nil {
		log.Log().Warnf("alert engine list redis active state for delete failed: rule=%d err=%v", ruleID, err)
		return
	}
	keys := make([]string, 0, len(fingerprints)+1)
	fingerprintMembers := make([]interface{}, 0, len(fingerprints))
	for _, fingerprint := range fingerprints {
		keys = append(keys, activeStateEventKey(fingerprint))
		fingerprintMembers = append(fingerprintMembers, fingerprint)
	}
	keys = append(keys, ruleKey)
	pipe := s.client.Pipeline()
	pipe.Del(ctx, keys...)
	if len(fingerprintMembers) > 0 {
		pipe.SRem(ctx, activeStateFingerprintSet, fingerprintMembers...)
	}
	pipe.SRem(ctx, activeStateRuleSet, activeStateRuleMember(ruleID))
	if _, err := pipe.Exec(ctx); err != nil {
		log.Log().Warnf("alert engine delete redis active state by rule failed: rule=%d err=%v", ruleID, err)
	}
}

func (s *activeStateStore) cleanupStaleState(ctx context.Context, fingerprints []interface{}, eventKeys []string, ruleMembers map[uint][]interface{}) {
	if len(fingerprints) == 0 && len(eventKeys) == 0 && len(ruleMembers) == 0 {
		return
	}
	pipe := s.client.Pipeline()
	if len(fingerprints) > 0 {
		pipe.SRem(ctx, activeStateFingerprintSet, fingerprints...)
	}
	if len(eventKeys) > 0 {
		pipe.Del(ctx, eventKeys...)
	}
	for ruleID, members := range ruleMembers {
		if ruleID > 0 && len(members) > 0 {
			pipe.SRem(ctx, activeStateRuleKey(ruleID), members...)
		}
	}
	if _, err := pipe.Exec(ctx); err != nil {
		log.Log().Warnf("alert engine cleanup stale redis active state failed: err=%v", err)
		return
	}
	for ruleID := range ruleMembers {
		s.removeEmptyRuleIndex(ctx, ruleID)
	}
}

func (s *activeStateStore) removeEmptyRuleIndex(ctx context.Context, ruleID uint) {
	count, err := s.client.SCard(ctx, activeStateRuleKey(ruleID)).Result()
	if err != nil {
		log.Log().Warnf("alert engine redis active state scard failed: rule=%d err=%v", ruleID, err)
		return
	}
	if count == 0 {
		_ = s.client.SRem(ctx, activeStateRuleSet, activeStateRuleMember(ruleID)).Err()
	}
}

func activeStateEventKey(fingerprint string) string {
	return activeStateEventKeyPrefix + fingerprint
}

func activeStateRuleKey(ruleID uint) string {
	return activeStateRuleKeyPrefix + strconv.FormatUint(uint64(ruleID), 10)
}

func activeStateRuleMember(ruleID uint) string {
	return strconv.FormatUint(uint64(ruleID), 10)
}
