package db

//数据库配置

import (
	"fmt"
	"gotik/internal/account"
	"gotik/internal/config"
	"gotik/internal/social"
	"gotik/internal/video"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// 创建数据库连接
func NewDB(dbcfg config.DatabaseConfig) (*gorm.DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		dbcfg.User, dbcfg.Password, dbcfg.Host, dbcfg.Port, dbcfg.DBName)

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	sqlDB.SetMaxOpenConns(20)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(time.Hour)

	if err := sqlDB.Ping(); err != nil {
		return nil, err
	}

	return db, nil
}

// 自动迁移表
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(&account.Account{}, &video.Video{}, &video.Like{}, &video.Comment{}, &social.Social{})
}

// 关闭数据库连接
func CloseDB(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
