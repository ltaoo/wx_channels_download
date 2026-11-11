package services

import (
	"encoding/json"
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
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
		return nil, err
	}
	var triggers []UserFlowTriggerInput
	if value, ok := envelope["triggers"]; ok {
		if err := json.Unmarshal(value, &triggers); err != nil {
			return nil, err
		}
		delete(envelope, "triggers")
	}
	var instance *model.UserFlowInstance
	err := s.db.Transaction(func(tx *gorm.DB) error {
		scoped := &AutomationService{db: tx, cron_parser: s.cron_parser, custom_nodes: s.custom_nodes}
		flow, err := scoped.ImportFlowDefinition(marshal_json(envelope))
		if err != nil {
			return err
		}
		instance, err = scoped.CreateUserFlowInstance(flow.ID)
		if err != nil {
			return err
		}
		for index := range triggers {
			triggers[index].ID = ""
		}
		if err := tx.Model(instance).Update("name", flow.Name).Error; err != nil {
			return err
		}
		return scoped.replace_user_flow_triggers(tx, instance.ID, "", triggers)
	})
	if err != nil {
		return nil, err
	}
	s.load_user_flows(instance.FlowID)
	return s.GetUserFlowInstance(instance.ID)
}
