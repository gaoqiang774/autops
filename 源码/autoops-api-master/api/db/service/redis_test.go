package service

import "testing"

func TestSplitRedisCommand(t *testing.T) {
	args, err := splitRedisCommand(`GET "user:1 profile"`)
	if err != nil {
		t.Fatalf("splitRedisCommand returned error: %v", err)
	}
	want := []string{"GET", "user:1 profile"}
	if len(args) != len(want) {
		t.Fatalf("len(args) = %d, want %d: %#v", len(args), len(want), args)
	}
	for i := range want {
		if args[i] != want[i] {
			t.Fatalf("args[%d] = %q, want %q", i, args[i], want[i])
		}
	}
}

func TestSplitRedisCommandUnterminatedQuote(t *testing.T) {
	if _, err := splitRedisCommand(`GET "missing`); err == nil {
		t.Fatalf("splitRedisCommand should reject unterminated quote")
	}
}

func TestIsReadOnlyRedisCommand(t *testing.T) {
	readonly := [][]string{
		{"GET", "k"},
		{"HGETALL", "h"},
		{"MEMORY", "USAGE", "k"},
	}
	for _, args := range readonly {
		if !isReadOnlyRedisCommand(args) {
			t.Fatalf("%v should be read-only", args)
		}
	}

	mutations := [][]string{
		{"SET", "k", "v"},
		{"DEL", "k"},
		{"MEMORY", "PURGE"},
	}
	for _, args := range mutations {
		if isReadOnlyRedisCommand(args) {
			t.Fatalf("%v should not be read-only", args)
		}
	}
}

func TestParseRedisKeyspace(t *testing.T) {
	keyspace := parseRedisKeyspace("# Keyspace\r\ndb0:keys=2,expires=1,avg_ttl=1000\r\ndb3:keys=15,expires=0,avg_ttl=0\r\n")
	if keyspace[0] != 2 {
		t.Fatalf("db0 keys = %d, want 2", keyspace[0])
	}
	if keyspace[3] != 15 {
		t.Fatalf("db3 keys = %d, want 15", keyspace[3])
	}
}

func TestRedisChildPrefix(t *testing.T) {
	got, ok := redisChildPrefix("", "pcp:desc:series:0798")
	if !ok {
		t.Fatalf("redisChildPrefix should detect first segment")
	}
	if got != "pcp:" {
		t.Fatalf("redisChildPrefix = %q, want pcp:", got)
	}

	got, ok = redisChildPrefix("pcp:desc:", "pcp:desc:series:0798")
	if !ok {
		t.Fatalf("redisChildPrefix should detect nested segment")
	}
	if got != "pcp:desc:series:" {
		t.Fatalf("redisChildPrefix = %q, want pcp:desc:series:", got)
	}

	if _, ok := redisChildPrefix("pcp:desc:series:", "pcp:desc:series:0798"); ok {
		t.Fatalf("redisChildPrefix should return false for leaf key")
	}
}

func TestRedisPrefixLabel(t *testing.T) {
	if got := redisPrefixLabel("pcp:desc:series:"); got != "series" {
		t.Fatalf("redisPrefixLabel = %q, want series", got)
	}
}

func TestFormatRedisBytes(t *testing.T) {
	if got := formatRedisBytes([]byte("中文 value")); got != "中文 value" {
		t.Fatalf("formatRedisBytes text = %q", got)
	}
	if got := formatRedisBytes([]byte{0xff, 0x00, 0x10, 0x41}); got != "hex:ff001041" {
		t.Fatalf("formatRedisBytes binary = %q, want hex:ff001041", got)
	}
}
