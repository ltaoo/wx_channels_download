package config

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"wx_channel/pkg/configapi"
)

const runtime_config_document_id = "runtime"

type runtime_config_document struct {
	ID        string `gorm:"primaryKey;size:64"`
	Values    string `gorm:"type:text;not null"`
	UpdatedAt int64  `gorm:"not null"`
}

func (runtime_config_document) TableName() string { return "runtime_config_documents" }

type gorm_config_backend struct {
	db *gorm.DB
}

func new_gorm_config_backend(db *gorm.DB) (*gorm_config_backend, error) {
	if db == nil {
		return nil, errors.New("config database is nil")
	}
	if err := db.AutoMigrate(&runtime_config_document{}); err != nil {
		return nil, err
	}
	return &gorm_config_backend{db: db}, nil
}

func (b *gorm_config_backend) LoadConfig(ctx context.Context) (map[string]any, error) {
	var document runtime_config_document
	query := b.db.WithContext(ctx).Where("id = ?", runtime_config_document_id).Limit(1).Find(&document)
	if query.Error != nil {
		return nil, query.Error
	}
	if query.RowsAffected == 0 {
		return make(map[string]any), nil
	}
	values := make(map[string]any)
	if document.Values == "" {
		return values, nil
	}
	if err := json.Unmarshal([]byte(document.Values), &values); err != nil {
		return nil, err
	}
	return values, nil
}

func (b *gorm_config_backend) SaveConfig(ctx context.Context, values map[string]any) error {
	data, err := json.Marshal(values)
	if err != nil {
		return err
	}
	document := runtime_config_document{
		ID:        runtime_config_document_id,
		Values:    string(data),
		UpdatedAt: time.Now().UnixMilli(),
	}
	return b.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{"values", "updated_at"}),
	}).Create(&document).Error
}

func (c *Config) AttachDatabaseSource(ctx context.Context, db *gorm.DB) error {
	backend, err := new_gorm_config_backend(db)
	if err != nil {
		return err
	}
	source, err := configapi.NewDatabaseSource("database", configapi.PriorityDatabase, backend)
	if err != nil {
		return err
	}
	manager := c.Manager()
	if manager == nil {
		return errors.New("config manager is not initialized")
	}
	if err := manager.AddSource(source); err != nil {
		return err
	}
	if err := manager.SetDefaultWriteSource(source.Name()); err != nil {
		return err
	}
	_, err = manager.Refresh(ctx)
	return err
}

var _ configapi.Backend = (*gorm_config_backend)(nil)
