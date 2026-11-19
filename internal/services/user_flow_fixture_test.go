package services

import (
	"gorm.io/gorm"
	"wx_channel/internal/database/model"
)

// These composite operations explicitly create both entities in one transaction.
func (s *AutomationService) CreateUserFlowWithInstance(input CreateUserFlowInput) (*model.UserFlowInstance, error) {
	var instance *model.UserFlowInstance
	err := s.db.Transaction(func(tx *gorm.DB) error {
		scoped := &AutomationService{db: tx, cron_parser: s.cron_parser, custom_nodes: s.custom_nodes}
		flow, err := scoped.CreateFlowDefinition(input)
		if err != nil {
			return err
		}
		instance, err = scoped.CreateUserFlowInstance(flow.ID)
		if err != nil {
			return err
		}
		trigger_type, event_key, err := normalize_user_flow_trigger(input.TriggerType, input.EventKey)
		if err != nil {
			return err
		}
		return tx.Model(instance).Updates(map[string]interface{}{"name": input.Name, "trigger_type": trigger_type, "event_key": event_key}).Error
	})
	if err != nil {
		return nil, err
	}
	s.load_user_flows(instance.FlowID)
	return s.GetUserFlowInstance(instance.ID)
}
func (s *AutomationService) ImportUserFlowWithInstance(raw string) (*model.UserFlowInstance, error) {
	var instance *model.UserFlowInstance
	err := s.db.Transaction(func(tx *gorm.DB) error {
		scoped := &AutomationService{db: tx, cron_parser: s.cron_parser, custom_nodes: s.custom_nodes}
		flow, err := scoped.ImportFlowDefinition(raw)
		if err != nil {
			return err
		}
		instance, err = scoped.CreateUserFlowInstance(flow.ID)
		if err != nil {
			return err
		}
		if err := tx.Model(instance).Update("name", flow.Name).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.load_user_flows(instance.FlowID)
	return s.GetUserFlowInstance(instance.ID)
}
