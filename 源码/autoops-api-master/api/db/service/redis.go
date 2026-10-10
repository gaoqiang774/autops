package service

import (
	"context"
	"encoding/hex"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	cmdbmodel "dodevops-api/api/cmdb/model"
	dbmodel "dodevops-api/api/db/model"

	"github.com/go-redis/redis/v8"
)

const (
	redisDefaultDBCount = 16
	redisTreeKeyLimit   = 200
	redisTreeScanLimit  = 2000
	redisTreeDelimiter  = ":"
	redisTreeTimeout    = 5 * time.Second
)

var redisKeywords = []string{
	"GET", "MGET", "EXISTS", "TYPE", "TTL", "PTTL", "STRLEN",
	"HGET", "HGETALL", "HLEN", "HKEYS", "HVALS", "HMGET",
	"LRANGE", "LLEN", "LINDEX",
	"SMEMBERS", "SCARD", "SSCAN", "SISMEMBER",
	"ZRANGE", "ZREVRANGE", "ZCARD", "ZSCORE",
	"SCAN", "INFO", "DBSIZE", "MEMORY USAGE",
	"XLEN", "XRANGE", "XREVRANGE",
}

type RedisDriver struct{}

func NewRedisDriver() *RedisDriver {
	return &RedisDriver{}
}

func (d *RedisDriver) Type() int {
	return cmdbmodel.DatabaseTypeRedis
}

func (d *RedisDriver) Test(ctx context.Context, cfg ConnConfig) error {
	client, err := openRedisClient(ctx, cfg, 0)
	if err != nil {
		return err
	}
	defer client.Close()
	return nil
}

func (d *RedisDriver) Tree(ctx context.Context, cfg ConnConfig, req dbmodel.TreeQuery) ([]dbmodel.TreeNode, error) {
	level := strings.TrimSpace(req.Level)
	if level == "" || level == "root" {
		return d.databaseNodes(ctx, cfg, req.Keyword)
	}
	if level == dbmodel.ObjectTypeDatabase || level == dbmodel.ObjectTypeRedisDB {
		dbIndex, err := parseRedisDatabase(req.Database)
		if err != nil {
			return nil, err
		}
		return d.keyNodes(ctx, cfg, dbIndex, req.ObjectName, req.Keyword)
	}
	if level == dbmodel.ObjectTypeRedisKey {
		return nil, nil
	}
	return nil, fmt.Errorf("unsupported tree level for Redis: %s", level)
}

func (d *RedisDriver) Columns(ctx context.Context, cfg ConnConfig, req dbmodel.ObjectRequest) ([]dbmodel.ColumnInfo, error) {
	if strings.TrimSpace(req.ObjectName) == "" {
		return redisKeyColumns(""), nil
	}
	dbIndex, err := parseRedisDatabase(req.Database)
	if err != nil {
		return nil, err
	}
	client, err := openRedisClient(ctx, cfg, dbIndex)
	if err != nil {
		return nil, err
	}
	defer client.Close()
	keyType, err := client.Type(ctx, req.ObjectName).Result()
	if err != nil {
		return nil, err
	}
	if keyType == "none" {
		return nil, fmt.Errorf("redis key not found: %s", req.ObjectName)
	}
	return redisKeyColumns(keyType), nil
}

func (d *RedisDriver) Indexes(context.Context, ConnConfig, dbmodel.ObjectRequest) ([]dbmodel.IndexInfo, error) {
	return []dbmodel.IndexInfo{}, nil
}

func (d *RedisDriver) DDL(ctx context.Context, cfg ConnConfig, req dbmodel.ObjectRequest) (string, error) {
	if strings.TrimSpace(req.ObjectName) == "" {
		return "", fmt.Errorf("objectName is required")
	}
	dbIndex, err := parseRedisDatabase(req.Database)
	if err != nil {
		return "", err
	}
	client, err := openRedisClient(ctx, cfg, dbIndex)
	if err != nil {
		return "", err
	}
	defer client.Close()
	keyType, err := client.Type(ctx, req.ObjectName).Result()
	if err != nil {
		return "", err
	}
	if keyType == "none" {
		return "", fmt.Errorf("redis key not found: %s", req.ObjectName)
	}
	ttl := client.TTL(ctx, req.ObjectName).Val()
	memory := redisMemoryUsage(ctx, client, req.ObjectName)
	lines := []string{
		fmt.Sprintf("KEY %q", req.ObjectName),
		fmt.Sprintf("TYPE %s", keyType),
		fmt.Sprintf("DB %d", dbIndex),
		fmt.Sprintf("TTL %s", formatRedisTTL(ttl)),
	}
	if memory >= 0 {
		lines = append(lines, fmt.Sprintf("MEMORY %s", formatBytes(memory)))
	}
	return strings.Join(lines, "\n"), nil
}

func (d *RedisDriver) Query(ctx context.Context, cfg ConnConfig, req dbmodel.DataQueryRequest) (*dbmodel.DataResult, error) {
	if strings.TrimSpace(req.ObjectName) == "" {
		return nil, fmt.Errorf("objectName is required")
	}
	if req.ObjectType != "" && req.ObjectType != dbmodel.ObjectTypeRedisKey {
		return nil, fmt.Errorf("unsupported objectType for Redis query: %s", req.ObjectType)
	}
	dbIndex, err := parseRedisDatabase(req.Database)
	if err != nil {
		return nil, err
	}
	page, pageSize := dbmodel.NormalizePage(req.Page, req.PageSize)
	client, err := openRedisClient(ctx, cfg, dbIndex)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	key := strings.TrimSpace(req.ObjectName)
	keyType, err := client.Type(ctx, key).Result()
	if err != nil {
		return nil, err
	}
	if keyType == "none" {
		return nil, fmt.Errorf("redis key not found: %s", key)
	}

	columns := redisKeyColumns(keyType)
	rows, total, err := queryRedisKey(ctx, client, key, keyType, page, pageSize)
	if err != nil {
		return nil, err
	}
	return &dbmodel.DataResult{
		Columns:     columns,
		Rows:        rows,
		PrimaryKeys: []string{},
		Page:        page,
		PageSize:    pageSize,
		Total:       total,
		Editable:    false,
	}, nil
}

func (d *RedisDriver) Create(context.Context, ConnConfig, dbmodel.DataCreateRequest) (*dbmodel.MutationResult, error) {
	return nil, fmt.Errorf("Redis工作台暂不支持新增数据")
}

func (d *RedisDriver) Update(context.Context, ConnConfig, dbmodel.DataUpdateRequest) (*dbmodel.MutationResult, error) {
	return nil, fmt.Errorf("Redis工作台暂不支持更新数据")
}

func (d *RedisDriver) Delete(context.Context, ConnConfig, dbmodel.DataDeleteRequest) (*dbmodel.MutationResult, error) {
	return nil, fmt.Errorf("Redis工作台暂不支持删除数据")
}

func (d *RedisDriver) ExecuteSQL(ctx context.Context, cfg ConnConfig, req dbmodel.SQLExecuteRequest) (*dbmodel.SQLExecuteResult, error) {
	commandText := normalizeSQLText(req.SQL)
	if commandText == "" {
		return nil, fmt.Errorf("redis command is required")
	}
	args, err := splitRedisCommand(commandText)
	if err != nil {
		return nil, err
	}
	if len(args) == 0 {
		return nil, fmt.Errorf("redis command is required")
	}
	if !isReadOnlyRedisCommand(args) {
		return nil, fmt.Errorf("only Redis read-only commands are allowed")
	}
	dbIndex, err := parseRedisDatabase(req.Database)
	if err != nil {
		return nil, err
	}
	client, err := openRedisClient(ctx, cfg, dbIndex)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	cmdArgs := make([]interface{}, 0, len(args))
	for _, arg := range args {
		cmdArgs = append(cmdArgs, arg)
	}
	start := time.Now()
	value, err := client.Do(ctx, cmdArgs...).Result()
	if err != nil {
		if err != redis.Nil {
			return nil, err
		}
		value = nil
	}
	rows := redisCommandRows(value)
	columns := redisCommandColumns(rows)
	page, pageSize := dbmodel.NormalizePage(req.Page, req.PageSize)
	total := int64(len(rows))
	rows = paginateRows(rows, page, pageSize)
	return &dbmodel.SQLExecuteResult{
		Columns:    columns,
		Rows:       rows,
		Page:       page,
		PageSize:   pageSize,
		Total:      total,
		DurationMS: time.Since(start).Milliseconds(),
		SQL:        commandText,
		ReadOnly:   true,
		Estimate: dbmodel.SQLExecutionEstimate{
			EstimatedRows:    total,
			EstimatedSeconds: 1,
			RiskLevel:        "low",
			Message:          "本条 Redis 只读命令预计执行较快",
		},
	}, nil
}

func (d *RedisDriver) Completion(context.Context, ConnConfig, dbmodel.SQLCompletionQuery) (*dbmodel.SQLCompletionResponse, error) {
	return &dbmodel.SQLCompletionResponse{
		Keywords: append([]string{}, redisKeywords...),
		Databases: []dbmodel.SQLCompletionItem{
			{Label: "db0", Value: "0", Type: dbmodel.ObjectTypeRedisDB, Database: "0"},
		},
	}, nil
}

func (d *RedisDriver) databaseNodes(ctx context.Context, cfg ConnConfig, keyword string) ([]dbmodel.TreeNode, error) {
	client, err := openRedisClient(ctx, cfg, 0)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	keyspace := map[int]int64{}
	info, err := client.Info(ctx, "keyspace").Result()
	if err == nil {
		keyspace = parseRedisKeyspace(info)
	}
	dbCount := redisConfiguredDBCount(ctx, client)
	if dbCount <= 0 {
		dbCount = redisDefaultDBCount
	}

	dbIndexes := make([]int, 0, len(keyspace))
	for dbIndex := range keyspace {
		dbIndexes = append(dbIndexes, dbIndex)
	}
	sort.Ints(dbIndexes)
	if len(dbIndexes) == 0 {
		for i := 0; i < dbCount; i++ {
			dbIndexes = append(dbIndexes, i)
		}
	}

	nodes := make([]dbmodel.TreeNode, 0, len(dbIndexes))
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	for _, dbIndex := range dbIndexes {
		label := fmt.Sprintf("db%d", dbIndex)
		if keyword != "" && !strings.Contains(strings.ToLower(label), keyword) {
			continue
		}
		keyCount := keyspace[dbIndex]
		nodes = append(nodes, dbmodel.TreeNode{
			Key:      fmt.Sprintf("redis:%d:db%d", cfg.AssetID, dbIndex),
			Label:    label,
			Type:     dbmodel.ObjectTypeRedisDB,
			AssetID:  cfg.AssetID,
			Engine:   cfg.Engine,
			Database: strconv.Itoa(dbIndex),
			Leaf:     false,
			Extra: map[string]interface{}{
				"keyCount": keyCount,
			},
		})
	}
	return nodes, nil
}

func (d *RedisDriver) keyNodes(ctx context.Context, cfg ConnConfig, dbIndex int, prefix, keyword string) ([]dbmodel.TreeNode, error) {
	treeCtx, cancel := context.WithTimeout(ctx, redisTreeTimeout)
	defer cancel()

	client, err := openRedisClient(ctx, cfg, dbIndex)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	prefix = strings.TrimSpace(prefix)
	if prefix != "" && !strings.HasSuffix(prefix, redisTreeDelimiter) {
		prefix += redisTreeDelimiter
	}
	match := prefix + "*"
	if strings.TrimSpace(keyword) != "" {
		match = prefix + "*" + strings.TrimSpace(keyword) + "*"
	}
	keys, err := scanRedisKeys(treeCtx, client, match, redisTreeScanLimit)
	if err != nil {
		return nil, err
	}
	sort.Strings(keys)

	nodes := make([]dbmodel.TreeNode, 0, redisTreeKeyLimit)
	prefixSet := map[string]struct{}{}
	for _, key := range keys {
		if len(nodes) >= redisTreeKeyLimit {
			break
		}
		if strings.TrimSpace(keyword) == "" {
			if childPrefix, ok := redisChildPrefix(prefix, key); ok {
				if _, exists := prefixSet[childPrefix]; exists {
					continue
				}
				prefixSet[childPrefix] = struct{}{}
				nodes = append(nodes, dbmodel.TreeNode{
					Key:        fmt.Sprintf("redis:%d:db%d:%s", cfg.AssetID, dbIndex, childPrefix),
					Label:      redisPrefixLabel(childPrefix),
					Type:       dbmodel.ObjectTypeRedisDB,
					AssetID:    cfg.AssetID,
					Engine:     cfg.Engine,
					Database:   strconv.Itoa(dbIndex),
					ObjectName: childPrefix,
					Leaf:       false,
					Extra: map[string]interface{}{
						"prefix": childPrefix,
					},
				})
				continue
			}
		}
		nodes = append(nodes, dbmodel.TreeNode{
			Key:        fmt.Sprintf("redis:%d:db%d:%s", cfg.AssetID, dbIndex, key),
			Label:      key,
			Type:       dbmodel.ObjectTypeRedisKey,
			AssetID:    cfg.AssetID,
			Engine:     cfg.Engine,
			Database:   strconv.Itoa(dbIndex),
			ObjectName: key,
			Leaf:       true,
			Extra: map[string]interface{}{
				"prefix": prefix,
			},
		})
	}
	return nodes, nil
}

func newRedisClient(cfg ConnConfig, dbIndex int, includeUsername bool) *redis.Client {
	username := ""
	if includeUsername && shouldUseRedisACLUsername(cfg.Username) {
		username = cfg.Username
	}
	return redis.NewClient(&redis.Options{
		Addr:         net.JoinHostPort(cfg.Host, fmt.Sprint(cfg.Port)),
		Username:     username,
		Password:     cfg.Password,
		DB:           dbIndex,
		DialTimeout:  10 * time.Second,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	})
}

func openRedisClient(ctx context.Context, cfg ConnConfig, dbIndex int) (*redis.Client, error) {
	client := newRedisClient(cfg, dbIndex, true)
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		fallback := newRedisFallbackClient(cfg, dbIndex, err)
		if fallback == nil {
			return nil, err
		}
		if fallbackErr := fallback.Ping(ctx).Err(); fallbackErr != nil {
			_ = fallback.Close()
			return nil, fallbackErr
		}
		return fallback, nil
	}
	return client, nil
}

func newRedisFallbackClient(cfg ConnConfig, dbIndex int, err error) *redis.Client {
	if !shouldUseRedisACLUsername(cfg.Username) || !isRedisAuthUsernameError(err) {
		return nil
	}
	return newRedisClient(cfg, dbIndex, false)
}

func shouldUseRedisACLUsername(username string) bool {
	username = strings.ToLower(strings.TrimSpace(username))
	if username == "" {
		return false
	}
	switch username {
	case "default", "redis", "root", "admin":
		return false
	default:
		return true
	}
}

func isRedisAuthUsernameError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "wrongpass") ||
		strings.Contains(message, "invalid username-password pair") ||
		strings.Contains(message, "user is disabled")
}

func parseRedisDatabase(value string) (int, error) {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.TrimPrefix(value, "db")
	if value == "" {
		return 0, nil
	}
	dbIndex, err := strconv.Atoi(value)
	if err != nil || dbIndex < 0 {
		return 0, fmt.Errorf("invalid redis database: %s", value)
	}
	return dbIndex, nil
}

func parseRedisKeyspace(info string) map[int]int64 {
	result := map[int]int64{}
	for _, line := range strings.Split(info, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "db") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		dbIndex, err := strconv.Atoi(strings.TrimPrefix(parts[0], "db"))
		if err != nil {
			continue
		}
		for _, field := range strings.Split(parts[1], ",") {
			keyValue := strings.SplitN(field, "=", 2)
			if len(keyValue) != 2 || keyValue[0] != "keys" {
				continue
			}
			count, err := strconv.ParseInt(keyValue[1], 10, 64)
			if err == nil {
				result[dbIndex] = count
			}
		}
	}
	return result
}

func redisConfiguredDBCount(ctx context.Context, client *redis.Client) int {
	value, err := client.ConfigGet(ctx, "databases").Result()
	if err != nil || len(value) < 2 {
		return redisDefaultDBCount
	}
	count, err := strconv.Atoi(fmt.Sprint(value[1]))
	if err != nil || count <= 0 {
		return redisDefaultDBCount
	}
	return count
}

func scanRedisKeys(ctx context.Context, client *redis.Client, match string, limit int) ([]string, error) {
	keys := make([]string, 0, limit)
	var cursor uint64
	for {
		if err := ctx.Err(); err != nil {
			return keys, nil
		}
		batch, next, err := client.Scan(ctx, cursor, match, 100).Result()
		if err != nil {
			if ctx.Err() != nil {
				return keys, nil
			}
			return nil, err
		}
		keys = append(keys, batch...)
		if len(keys) >= limit {
			return keys[:limit], nil
		}
		if next == 0 {
			return keys, nil
		}
		cursor = next
	}
}

func redisChildPrefix(prefix, key string) (string, bool) {
	if !strings.HasPrefix(key, prefix) {
		return "", false
	}
	rest := strings.TrimPrefix(key, prefix)
	if rest == "" {
		return "", false
	}
	index := strings.Index(rest, redisTreeDelimiter)
	if index < 0 {
		return "", false
	}
	return prefix + rest[:index+1], true
}

func redisPrefixLabel(prefix string) string {
	prefix = strings.TrimSuffix(prefix, redisTreeDelimiter)
	parts := strings.Split(prefix, redisTreeDelimiter)
	if len(parts) == 0 {
		return prefix
	}
	return parts[len(parts)-1]
}

func redisKeyColumns(keyType string) []dbmodel.ColumnInfo {
	switch strings.ToLower(keyType) {
	case "string":
		return []dbmodel.ColumnInfo{
			{Name: "key", Type: "string"},
			{Name: "type", Type: "string"},
			{Name: "value", Type: "string"},
			{Name: "ttlSeconds", Type: "integer"},
		}
	case "hash":
		return []dbmodel.ColumnInfo{{Name: "field", Type: "string"}, {Name: "value", Type: "string"}}
	case "list":
		return []dbmodel.ColumnInfo{{Name: "index", Type: "integer"}, {Name: "value", Type: "string"}}
	case "set":
		return []dbmodel.ColumnInfo{{Name: "member", Type: "string"}}
	case "zset":
		return []dbmodel.ColumnInfo{{Name: "member", Type: "string"}, {Name: "score", Type: "float"}}
	case "stream":
		return []dbmodel.ColumnInfo{{Name: "id", Type: "string"}, {Name: "fields", Type: "json"}}
	default:
		return []dbmodel.ColumnInfo{{Name: "key", Type: "string"}, {Name: "type", Type: "string"}, {Name: "value", Type: "string"}}
	}
}

func queryRedisKey(ctx context.Context, client *redis.Client, key, keyType string, page, pageSize int) ([]map[string]interface{}, int64, error) {
	offset := int64((page - 1) * pageSize)
	limit := int64(pageSize)
	switch strings.ToLower(keyType) {
	case "string":
		value, err := client.Get(ctx, key).Result()
		if err != nil {
			return nil, 0, err
		}
		ttl := client.TTL(ctx, key).Val()
		return []map[string]interface{}{{
			"key":        key,
			"type":       keyType,
			"value":      formatRedisValue(value),
			"ttlSeconds": ttlSeconds(ttl),
		}}, 1, nil
	case "hash":
		total, err := client.HLen(ctx, key).Result()
		if err != nil {
			return nil, 0, err
		}
		values, err := scanRedisHash(ctx, client, key, offset, limit)
		return values, total, err
	case "list":
		total, err := client.LLen(ctx, key).Result()
		if err != nil {
			return nil, 0, err
		}
		items, err := client.LRange(ctx, key, offset, offset+limit-1).Result()
		if err != nil {
			return nil, 0, err
		}
		rows := make([]map[string]interface{}, 0, len(items))
		for i, item := range items {
			rows = append(rows, map[string]interface{}{"index": offset + int64(i), "value": formatRedisValue(item)})
		}
		return rows, total, nil
	case "set":
		total, err := client.SCard(ctx, key).Result()
		if err != nil {
			return nil, 0, err
		}
		values, err := scanRedisSet(ctx, client, key, offset, limit)
		return values, total, err
	case "zset":
		total, err := client.ZCard(ctx, key).Result()
		if err != nil {
			return nil, 0, err
		}
		items, err := client.ZRangeWithScores(ctx, key, offset, offset+limit-1).Result()
		if err != nil {
			return nil, 0, err
		}
		rows := make([]map[string]interface{}, 0, len(items))
		for _, item := range items {
			rows = append(rows, map[string]interface{}{"member": fmt.Sprint(item.Member), "score": item.Score})
		}
		return rows, total, nil
	case "stream":
		total, err := client.XLen(ctx, key).Result()
		if err != nil {
			return nil, 0, err
		}
		items, err := client.XRangeN(ctx, key, "-", "+", int64(page*pageSize)).Result()
		if err != nil {
			return nil, 0, err
		}
		start := (page - 1) * pageSize
		if start >= len(items) {
			return []map[string]interface{}{}, total, nil
		}
		end := start + pageSize
		if end > len(items) {
			end = len(items)
		}
		rows := make([]map[string]interface{}, 0, end-start)
		for _, item := range items[start:end] {
			rows = append(rows, map[string]interface{}{"id": item.ID, "fields": item.Values})
		}
		return rows, total, nil
	default:
		return nil, 0, fmt.Errorf("unsupported Redis key type: %s", keyType)
	}
}

func scanRedisHash(ctx context.Context, client *redis.Client, key string, offset, limit int64) ([]map[string]interface{}, error) {
	rows := make([]map[string]interface{}, 0, limit)
	var cursor uint64
	var skipped int64
	for {
		values, next, err := client.HScan(ctx, key, cursor, "*", 100).Result()
		if err != nil {
			return nil, err
		}
		for i := 0; i+1 < len(values); i += 2 {
			if skipped < offset {
				skipped++
				continue
			}
			rows = append(rows, map[string]interface{}{"field": values[i], "value": formatRedisValue(values[i+1])})
			if int64(len(rows)) >= limit {
				return rows, nil
			}
		}
		if next == 0 {
			return rows, nil
		}
		cursor = next
	}
}

func scanRedisSet(ctx context.Context, client *redis.Client, key string, offset, limit int64) ([]map[string]interface{}, error) {
	rows := make([]map[string]interface{}, 0, limit)
	var cursor uint64
	var skipped int64
	for {
		values, next, err := client.SScan(ctx, key, cursor, "*", 100).Result()
		if err != nil {
			return nil, err
		}
		for _, value := range values {
			if skipped < offset {
				skipped++
				continue
			}
			rows = append(rows, map[string]interface{}{"member": formatRedisValue(value)})
			if int64(len(rows)) >= limit {
				return rows, nil
			}
		}
		if next == 0 {
			return rows, nil
		}
		cursor = next
	}
}

func ttlSeconds(ttl time.Duration) int64 {
	switch {
	case ttl < 0:
		return int64(ttl / time.Second)
	default:
		return int64(ttl.Seconds())
	}
}

func formatRedisTTL(ttl time.Duration) string {
	switch ttl {
	case -2 * time.Second:
		return "not exists"
	case -1 * time.Second:
		return "persistent"
	default:
		if ttl < 0 {
			return "unknown"
		}
		return ttl.String()
	}
}

func redisMemoryUsage(ctx context.Context, client *redis.Client, key string) int64 {
	value, err := client.Do(ctx, "MEMORY", "USAGE", key).Int64()
	if err != nil {
		return -1
	}
	return value
}

func splitRedisCommand(command string) ([]string, error) {
	var args []string
	var current strings.Builder
	var quote rune
	escaped := false
	for _, ch := range command {
		if escaped {
			current.WriteRune(ch)
			escaped = false
			continue
		}
		if ch == '\\' {
			escaped = true
			continue
		}
		if quote != 0 {
			if ch == quote {
				quote = 0
				continue
			}
			current.WriteRune(ch)
			continue
		}
		if ch == '\'' || ch == '"' {
			quote = ch
			continue
		}
		if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
			if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
			continue
		}
		current.WriteRune(ch)
	}
	if escaped {
		current.WriteRune('\\')
	}
	if quote != 0 {
		return nil, fmt.Errorf("unterminated quote in redis command")
	}
	if current.Len() > 0 {
		args = append(args, current.String())
	}
	return args, nil
}

func isReadOnlyRedisCommand(args []string) bool {
	if len(args) == 0 {
		return false
	}
	command := strings.ToUpper(args[0])
	if command == "MEMORY" {
		return len(args) >= 2 && strings.EqualFold(args[1], "USAGE")
	}
	switch command {
	case "GET", "MGET", "EXISTS", "TYPE", "TTL", "PTTL", "STRLEN",
		"HGET", "HGETALL", "HLEN", "HKEYS", "HVALS", "HMGET", "HEXISTS",
		"LRANGE", "LLEN", "LINDEX",
		"SMEMBERS", "SCARD", "SSCAN", "SISMEMBER",
		"ZRANGE", "ZREVRANGE", "ZCARD", "ZSCORE", "ZRANK", "ZREVRANK",
		"SCAN", "INFO", "DBSIZE",
		"XLEN", "XRANGE", "XREVRANGE":
		return true
	default:
		return false
	}
}

func redisCommandRows(value interface{}) []map[string]interface{} {
	switch v := value.(type) {
	case nil:
		return []map[string]interface{}{{"result": nil}}
	case string:
		return []map[string]interface{}{{"result": v}}
	case []byte:
		return []map[string]interface{}{{"result": formatRedisBytes(v)}}
	case int64, int, uint64, bool, float64:
		return []map[string]interface{}{{"result": v}}
	case []interface{}:
		return redisSliceRows(v)
	default:
		return []map[string]interface{}{{"result": fmt.Sprint(v)}}
	}
}

func redisSliceRows(values []interface{}) []map[string]interface{} {
	rows := make([]map[string]interface{}, 0, len(values))
	for i, value := range values {
		rows = append(rows, map[string]interface{}{
			"index": i,
			"value": normalizeRedisCommandValue(value),
		})
	}
	return rows
}

func normalizeRedisCommandValue(value interface{}) interface{} {
	switch v := value.(type) {
	case nil:
		return nil
	case []byte:
		return formatRedisBytes(v)
	case string:
		return formatRedisValue(v)
	case []interface{}:
		items := make([]interface{}, 0, len(v))
		for _, item := range v {
			items = append(items, normalizeRedisCommandValue(item))
		}
		return items
	default:
		return v
	}
}

func formatRedisValue(value string) string {
	return formatRedisBytes([]byte(value))
}

func formatRedisBytes(value []byte) string {
	if len(value) == 0 {
		return ""
	}
	if isPrintableUTF8(value) {
		return string(value)
	}
	return "hex:" + hex.EncodeToString(value)
}

func isPrintableUTF8(value []byte) bool {
	if !utf8.Valid(value) {
		return false
	}
	controlCount := 0
	for len(value) > 0 {
		r, size := utf8.DecodeRune(value)
		value = value[size:]
		if r == '\n' || r == '\r' || r == '\t' {
			continue
		}
		if r < 0x20 || r == 0x7f {
			controlCount++
		}
	}
	return controlCount == 0
}

func redisCommandColumns(rows []map[string]interface{}) []dbmodel.ColumnInfo {
	if len(rows) == 0 {
		return []dbmodel.ColumnInfo{{Name: "result", Type: "string"}}
	}
	names := make([]string, 0, len(rows[0]))
	for name := range rows[0] {
		names = append(names, name)
	}
	sort.Strings(names)
	columns := make([]dbmodel.ColumnInfo, 0, len(names))
	for _, name := range names {
		columns = append(columns, dbmodel.ColumnInfo{Name: name, Type: "string"})
	}
	return columns
}

func paginateRows(rows []map[string]interface{}, page, pageSize int) []map[string]interface{} {
	start := (page - 1) * pageSize
	if start >= len(rows) {
		return []map[string]interface{}{}
	}
	end := start + pageSize
	if end > len(rows) {
		end = len(rows)
	}
	return rows[start:end]
}
