package service

import (
	"reflect"
	"testing"

	dbmodel "dodevops-api/api/db/model"
)

func TestQuotePostgresIdentifier(t *testing.T) {
	got := quotePostgresIdentifier(`user"name`)
	want := `"user""name"`
	if got != want {
		t.Fatalf("quotePostgresIdentifier() = %q, want %q", got, want)
	}
}

func TestBuildPostgresWhere(t *testing.T) {
	columnMap := map[string]dbmodel.ColumnInfo{
		"id":     {Name: "id", Type: "integer"},
		"name":   {Name: "name", Type: "character varying"},
		"status": {Name: "status", Type: "text"},
	}
	gotSQL, gotArgs, err := buildPostgresWhere([]dbmodel.DataFilter{
		{Column: "id", Operator: "=", Value: 1},
		{Column: "name", Operator: "like", Value: "ops"},
		{Column: "status", Operator: "in", Value: []interface{}{"open", "closed"}},
	}, columnMap)
	if err != nil {
		t.Fatalf("buildPostgresWhere() error = %v", err)
	}
	wantSQL := ` WHERE "id" = $1 AND "name"::text ILIKE $2 AND "status" IN ($3, $4)`
	if gotSQL != wantSQL {
		t.Fatalf("buildPostgresWhere() SQL = %q, want %q", gotSQL, wantSQL)
	}
	wantArgs := []interface{}{1, "%ops%", "open", "closed"}
	if !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("buildPostgresWhere() args = %#v, want %#v", gotArgs, wantArgs)
	}
}

func TestBuildPostgresWhereInvalidIntegerTextUsesTextComparison(t *testing.T) {
	columnMap := map[string]dbmodel.ColumnInfo{
		"id": {Name: "id", Type: "integer"},
	}
	gotSQL, gotArgs, err := buildPostgresWhere([]dbmodel.DataFilter{
		{Column: "id", Operator: "=", Value: "赵六"},
	}, columnMap)
	if err != nil {
		t.Fatalf("buildPostgresWhere() error = %v", err)
	}
	wantSQL := ` WHERE "id"::text = $1`
	if gotSQL != wantSQL {
		t.Fatalf("buildPostgresWhere() SQL = %q, want %q", gotSQL, wantSQL)
	}
	wantArgs := []interface{}{"赵六"}
	if !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("buildPostgresWhere() args = %#v, want %#v", gotArgs, wantArgs)
	}
}

func TestBuildPostgresWhereInvalidIntegerRangeReturnsEmptyCondition(t *testing.T) {
	columnMap := map[string]dbmodel.ColumnInfo{
		"id": {Name: "id", Type: "integer"},
	}
	gotSQL, gotArgs, err := buildPostgresWhere([]dbmodel.DataFilter{
		{Column: "id", Operator: ">", Value: "赵六"},
	}, columnMap)
	if err != nil {
		t.Fatalf("buildPostgresWhere() error = %v", err)
	}
	wantSQL := ` WHERE 1 = 0`
	if gotSQL != wantSQL {
		t.Fatalf("buildPostgresWhere() SQL = %q, want %q", gotSQL, wantSQL)
	}
	if len(gotArgs) != 0 {
		t.Fatalf("buildPostgresWhere() args = %#v, want empty", gotArgs)
	}
}

func TestParsePostgresArrayText(t *testing.T) {
	got := parsePostgresArrayText(`{status,id}`)
	want := []string{"id", "status"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parsePostgresArrayText() = %#v, want %#v", got, want)
	}
}

func TestPostgresPoolKeyIncludesDatabase(t *testing.T) {
	cfg := ConnConfig{AssetID: 3, Host: "127.0.0.1", Port: 5432, Username: "postgres"}
	rootKey := postgresPoolKey(cfg, postgresMaintenanceDatabase)
	assetNameKey := postgresPoolKey(cfg, "aiops-pgsql")
	if rootKey == assetNameKey {
		t.Fatalf("postgres pool key should distinguish maintenance database and asset name database")
	}
}

func TestPostgresDefaultDatabaseIsVisible(t *testing.T) {
	if isPostgresSystemDatabase("postgres") {
		t.Fatalf("postgres default database should be visible in workbench tree")
	}
	if !isPostgresSystemDatabase("template0") || !isPostgresSystemDatabase("template1") {
		t.Fatalf("template databases should be hidden in workbench tree")
	}
}
