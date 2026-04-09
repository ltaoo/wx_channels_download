package database

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"wx_channel/pkg/util"
)

type DatabaseConfig struct {
	DBType     string
	DBPath     string
	DBUser     string
	DBPassword string
	DBHost     string
	DBPort     string
	DBName     string
}

func NewDatabase(cfg *DatabaseConfig, parent_logger *zerolog.Logger) (*gorm.DB, error) {
	var dialector gorm.Dialector
	// logger.Printf("[INTERNAL]db/database - NewDatabase: DBType=%s, DBPath=%s\n", cfg.DBType, cfg.DBPath)

	switch cfg.DBType {
	case "mysql":
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			cfg.DBUser, cfg.DBPassword, cfg.DBHost, cfg.DBPort, cfg.DBName)
		dialector = mysql.Open(dsn)
	case "postgres":
		dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=UTC",
			cfg.DBHost, cfg.DBUser, cfg.DBPassword, cfg.DBName, cfg.DBPort)
		dialector = postgres.Open(dsn)
	case "sqlite":
		// 确保数据库目录存在
		dbDir := cfg.DBPath
		if lastSlash := strings.LastIndex(dbDir, "/"); lastSlash != -1 {
			dbDir = dbDir[:lastSlash]
			if err := os.MkdirAll(dbDir, 0755); err != nil {
				return nil, fmt.Errorf("failed to create database directory %s: %w", dbDir, err)
			}
		}
		// fmt.Printf("[INTERNAL]db/database - SQLite database path: %s\n", cfg.DBPath)
		dialector = sqlite.Open(cfg.DBPath + "?_busy_timeout=5000&_journal=WAL&_synchronous=NORMAL")
	default:
		return nil, fmt.Errorf("unsupported database type: %s", cfg.DBType)
	}

	// 配置GORM - 使用 zerolog
	gormConfig := &gorm.Config{}

	// 连接数据库
	db, err := gorm.Open(dialector, gormConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	registerTimestampCallbacks(db)
	return db, nil
}

func registerTimestampCallbacks(db *gorm.DB) {
	db.Callback().Create().Before("gorm:create").Register("routes:set_timestamps:create", func(db *gorm.DB) {
		if db.Statement == nil || db.Statement.Schema == nil {
			return
		}
		now := util.NowMillis()
		setIfEmpty(db, "created_at", now)
		setField(db, "updated_at", now)
	})
	db.Callback().Update().Before("gorm:update").Register("routes:set_timestamps:update", func(db *gorm.DB) {
		if db.Statement == nil || db.Statement.Schema == nil {
			return
		}
		now := util.NowMillis()
		setField(db, "updated_at", now)
	})
}

func setIfEmpty(db *gorm.DB, col string, millis int64) {
	f, ok := db.Statement.Schema.FieldsByDBName[col]
	if !ok {
		return
	}
	_, isZero := f.ValueOf(db.Statement.Context, db.Statement.ReflectValue)
	if isZero {
		f.Set(db.Statement.Context, db.Statement.ReflectValue, valueForFieldType(f.FieldType, millis))
	}
}

func setField(db *gorm.DB, col string, millis int64) {
	f, ok := db.Statement.Schema.FieldsByDBName[col]
	if !ok {
		return
	}
	f.Set(db.Statement.Context, db.Statement.ReflectValue, valueForFieldType(f.FieldType, millis))
}

var timeType = reflect.TypeOf(time.Time{})

func valueForFieldType(t reflect.Type, millis int64) any {
	if t == nil {
		return millis
	}
	if t == timeType {
		return time.UnixMilli(millis).UTC()
	}
	if t.Kind() == reflect.Pointer && t.Elem() == timeType {
		v := time.UnixMilli(millis).UTC()
		return &v
	}

	switch t.Kind() {
	case reflect.String:
		return strconv.FormatInt(millis, 10)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return millis
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if millis < 0 {
			return uint64(0)
		}
		return uint64(millis)
	default:
		return millis
	}
}
