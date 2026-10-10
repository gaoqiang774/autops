// pkg/db/db.go
package db

import (
	"dodevops-api/common/config"
	"fmt"
	"log"
	"os"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var Db *gorm.DB

// NewGormLogger writes warning, slow SQL, and error logs to stdout.
func NewGormLogger() logger.Interface {
	return logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags),
		logger.Config{
			SlowThreshold:             time.Second,
			LogLevel:                  logger.Warn,
			IgnoreRecordNotFoundError: true,
			Colorful:                  true,
		},
	)
}

// SetupDBLink initializes the database connection.
func SetupDBLink() error {
	var err error
	var dbConfig = config.Config.Db
	url := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&collation=utf8mb4_general_ci&parseTime=True&loc=Local&sql_mode=''",
		dbConfig.Username,
		dbConfig.Password,
		dbConfig.Host,
		dbConfig.Port,
		dbConfig.Db)

	Db, err = gorm.Open(mysql.Open(url), &gorm.Config{
		Logger:                                   NewGormLogger(),
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		panic(err)
	}
	if Db.Error != nil {
		panic(Db.Error)
	}

	if dbConfig.AutoMigrate {
		if err := AutoMigrate(Db); err != nil {
			panic(err)
		}
	} else {
		log.Println("AutoMigrate disabled by config, skipping schema migration")
	}

	sqlDB, err := Db.DB()
	sqlDB.SetMaxIdleConns(dbConfig.MaxIdle)
	sqlDB.SetMaxOpenConns(dbConfig.MaxOpen)
	return nil
}
