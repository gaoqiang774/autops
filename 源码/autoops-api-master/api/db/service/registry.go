package service

import (
	"context"
	"fmt"

	cmdbmodel "dodevops-api/api/cmdb/model"
	dbmodel "dodevops-api/api/db/model"
)

type ConnConfig struct {
	AssetID      uint
	Engine       int
	Name         string
	Host         string
	Port         int
	Endpoint     string
	Username     string
	Password     string
	DatabaseName string
	SSLEnabled   bool
}

type Driver interface {
	Type() int
	Test(ctx context.Context, cfg ConnConfig) error
	Tree(ctx context.Context, cfg ConnConfig, req dbmodel.TreeQuery) ([]dbmodel.TreeNode, error)
	Columns(ctx context.Context, cfg ConnConfig, req dbmodel.ObjectRequest) ([]dbmodel.ColumnInfo, error)
	Indexes(ctx context.Context, cfg ConnConfig, req dbmodel.ObjectRequest) ([]dbmodel.IndexInfo, error)
	DDL(ctx context.Context, cfg ConnConfig, req dbmodel.ObjectRequest) (string, error)
	Query(ctx context.Context, cfg ConnConfig, req dbmodel.DataQueryRequest) (*dbmodel.DataResult, error)
	Create(ctx context.Context, cfg ConnConfig, req dbmodel.DataCreateRequest) (*dbmodel.MutationResult, error)
	Update(ctx context.Context, cfg ConnConfig, req dbmodel.DataUpdateRequest) (*dbmodel.MutationResult, error)
	Delete(ctx context.Context, cfg ConnConfig, req dbmodel.DataDeleteRequest) (*dbmodel.MutationResult, error)
	ExecuteSQL(ctx context.Context, cfg ConnConfig, req dbmodel.SQLExecuteRequest) (*dbmodel.SQLExecuteResult, error)
	Completion(ctx context.Context, cfg ConnConfig, req dbmodel.SQLCompletionQuery) (*dbmodel.SQLCompletionResponse, error)
}

var drivers = map[int]Driver{}

func init() {
	RegisterDriver(NewMySQLDriver())
	RegisterDriver(NewPostgreSQLDriver())
	RegisterDriver(NewRedisDriver())
	for _, dbType := range []int{
		cmdbmodel.DatabaseTypeOracle,
		cmdbmodel.DatabaseTypeMongoDB,
		cmdbmodel.DatabaseTypeSQLServer,
		cmdbmodel.DatabaseTypeElasticsearch,
	} {
		RegisterDriver(unsupportedDriver{dbType: dbType})
	}
}

func RegisterDriver(driver Driver) {
	if driver == nil {
		return
	}
	drivers[driver.Type()] = driver
}

func getDriver(dbType int) (Driver, error) {
	driver, ok := drivers[dbType]
	if !ok {
		return nil, fmt.Errorf("unsupported database type: %d", dbType)
	}
	return driver, nil
}

type unsupportedDriver struct {
	dbType int
}

func (d unsupportedDriver) Type() int {
	return d.dbType
}

func (d unsupportedDriver) err() error {
	return fmt.Errorf("database type %d is registered but not implemented yet", d.dbType)
}

func (d unsupportedDriver) Test(context.Context, ConnConfig) error {
	return d.err()
}

func (d unsupportedDriver) Tree(context.Context, ConnConfig, dbmodel.TreeQuery) ([]dbmodel.TreeNode, error) {
	return nil, d.err()
}

func (d unsupportedDriver) Columns(context.Context, ConnConfig, dbmodel.ObjectRequest) ([]dbmodel.ColumnInfo, error) {
	return nil, d.err()
}

func (d unsupportedDriver) Indexes(context.Context, ConnConfig, dbmodel.ObjectRequest) ([]dbmodel.IndexInfo, error) {
	return nil, d.err()
}

func (d unsupportedDriver) DDL(context.Context, ConnConfig, dbmodel.ObjectRequest) (string, error) {
	return "", d.err()
}

func (d unsupportedDriver) Query(context.Context, ConnConfig, dbmodel.DataQueryRequest) (*dbmodel.DataResult, error) {
	return nil, d.err()
}

func (d unsupportedDriver) Create(context.Context, ConnConfig, dbmodel.DataCreateRequest) (*dbmodel.MutationResult, error) {
	return nil, d.err()
}

func (d unsupportedDriver) Update(context.Context, ConnConfig, dbmodel.DataUpdateRequest) (*dbmodel.MutationResult, error) {
	return nil, d.err()
}

func (d unsupportedDriver) Delete(context.Context, ConnConfig, dbmodel.DataDeleteRequest) (*dbmodel.MutationResult, error) {
	return nil, d.err()
}

func (d unsupportedDriver) ExecuteSQL(context.Context, ConnConfig, dbmodel.SQLExecuteRequest) (*dbmodel.SQLExecuteResult, error) {
	return nil, d.err()
}

func (d unsupportedDriver) Completion(context.Context, ConnConfig, dbmodel.SQLCompletionQuery) (*dbmodel.SQLCompletionResponse, error) {
	return nil, d.err()
}
