package database

import (
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"wx_channel/internal/config"
	dbpkg "wx_channel/pkg/database"
)

func NewDatabaseConfig(cfg *config.Config) *dbpkg.DatabaseConfig {
	return &dbpkg.DatabaseConfig{
		DBType: cfg.DBType,
		DBPath: cfg.DBPath,
		DBUser: cfg.DBUser,
		DBPassword: cfg.DBPassword,
		DBHost: cfg.DBHost,
		DBPort: cfg.DBPort,
		DBName: cfg.DBName,
	}
}

type ClientDatabase struct {
	cfg *dbpkg.DatabaseConfig
	logger *zerolog.Logger
	db *gorm.DB
}

func NewClientDatabase(cfg *dbpkg.DatabaseConfig, logger *zerolog.Logger) *ClientDatabase {
	return &ClientDatabase{
		cfg: cfg,
		logger: logger,
		db: nil,
	}
}

func (c *ClientDatabase) Setup() error {
	database, err := dbpkg.NewDatabase(c.cfg, c.logger)
	if err != nil {
		c.logger.Error().Err(err).Msg("Failed to connect to database")
		return err
	}
	c.db = database
	migrator := dbpkg.NewMigrator(c.cfg, &migrations)
	if err := migrator.MigrateUp(); err != nil {
		c.logger.Error().Err(err).Msg("Failed to run migrations")
		return err
	}
	return nil
}

func (c *ClientDatabase) Close() error {
	if c.db == nil {
		return nil
	}
	sqlDB, err := c.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

func (c *ClientDatabase) DB() *gorm.DB {
	return c.db
}
