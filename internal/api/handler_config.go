package api

import (
	"reflect"
	"sort"

	"github.com/gin-gonic/gin"

	result "wx_channel/internal/apiresult"
	"wx_channel/internal/config"
)

type application_config_update_body struct {
	Values map[string]interface{} `json:"values"`
}

type application_config_field struct {
	config.ConfigField
	Value          interface{} `json:"value,omitempty"`
	EffectiveValue interface{} `json:"effective_value,omitempty"`
	Configured     bool        `json:"configured"`
}

func (c *APIClient) handle_application_config_get(ctx *gin.Context) {
	if c.cfg == nil || c.cfg.Original == nil {
		result.Err(ctx, 500, "配置未初始化")
		return
	}

	fields := make([]application_config_field, 0)
	application_fields := make([]application_config_field, 0)
	plugin_fields := make([]application_config_field, 0)
	for _, field := range unique_config_schema() {
		response_field := field
		if response_field.Sensitive {
			response_field.Default = nil
		}
		entry := application_config_field{
			ConfigField: response_field,
			Configured:  c.cfg.Original.IsInConfig(field.Key),
		}
		if !field.Sensitive {
			entry.Value = c.cfg.Original.GetRaw(field.Key)
			entry.EffectiveValue = c.cfg.Original.Get(field.Key)
		}
		fields = append(fields, entry)
		if field.Source == config.ConfigFieldSourcePlugin {
			plugin_fields = append(plugin_fields, entry)
		} else {
			application_fields = append(application_fields, entry)
		}
	}

	result.Ok(ctx, gin.H{
		"config_file":        c.cfg.Original.FullPath,
		"fields":             fields,
		"application_fields": application_fields,
		"plugin_fields":      plugin_fields,
	})
}

func (c *APIClient) handle_application_config_update(ctx *gin.Context) {
	if c.cfg == nil || c.cfg.Original == nil {
		result.Err(ctx, 500, "配置未初始化")
		return
	}
	if c.application_restart_service == nil {
		result.Err(ctx, 503, "应用重启服务未初始化")
		return
	}

	var body application_config_update_body
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, 400, "不合法的参数")
		return
	}
	if len(body.Values) == 0 {
		result.Err(ctx, 400, "缺少配置项")
		return
	}

	keys := make([]string, 0, len(body.Values))
	for key := range body.Values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	updated_values := make(map[string]interface{}, len(keys))
	updated_keys := make([]string, 0, len(keys))
	for _, key := range keys {
		field, ok := config.Lookup(key)
		if !ok {
			result.Err(ctx, 400, "未知配置项: "+key)
			return
		}
		normalized_value, err := config.NormalizeValue(field, body.Values[key])
		if err != nil {
			result.Err(ctx, 400, err.Error())
			return
		}
		if reflect.DeepEqual(c.cfg.Original.GetRaw(key), normalized_value) {
			continue
		}
		updated_values[key] = normalized_value
		updated_keys = append(updated_keys, key)
	}

	if len(updated_values) == 0 {
		result.Ok(ctx, gin.H{
			"changed":           false,
			"updated_keys":      []string{},
			"restart_scheduled": false,
		})
		return
	}
	if err := c.cfg.Original.UpdateAndSave(updated_values); err != nil {
		result.Err(ctx, 500, "保存配置失败: "+err.Error())
		return
	}
	config_revision, err := c.cfg.Original.Revision()
	if err != nil {
		result.Err(ctx, 500, "配置已保存，但生成配置摘要失败: "+err.Error())
		return
	}
	restart_token, err := c.application_restart_service.NewConfirmationToken(config_revision)
	if err != nil {
		result.Err(ctx, 500, "配置已保存，但生成重启确认令牌失败: "+err.Error())
		return
	}
	if err := c.application_restart_service.Schedule(func(restart_err error) {
		if c.logger != nil {
			c.logger.Error().Err(restart_err).Msg("restart after configuration update failed")
		}
	}); err != nil {
		result.Err(ctx, 500, "配置已保存，但安排重启失败: "+err.Error())
		return
	}

	result.Ok(ctx, gin.H{
		"changed":               true,
		"updated_keys":          updated_keys,
		"restart_scheduled":     true,
		"restart_status":        "scheduled",
		"restart_token":         restart_token,
		"confirmation_required": true,
		"message":               "配置已保存并安排重启，但尚未确认重启完成；请在连接恢复后检查重启状态",
		"next_action": gin.H{
			"tool": "get_restart_status",
			"arguments": gin.H{
				"restart_token": restart_token,
			},
			"complete_when": "status=completed、restart_completed=true 且 config_applied=true",
		},
	})
}

func (c *APIClient) handle_application_restart_status(ctx *gin.Context) {
	if c.cfg == nil || c.cfg.Original == nil {
		result.Err(ctx, 500, "配置未初始化")
		return
	}
	if c.application_restart_service == nil {
		result.Err(ctx, 503, "应用重启服务未初始化")
		return
	}
	restart_token := ctx.Query("restart_token")
	config_revision, err := c.cfg.Original.Revision()
	if err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}
	confirmation, err := c.application_restart_service.CheckConfirmation(restart_token, config_revision)
	if err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}
	result.Ok(ctx, confirmation)
}

func unique_config_schema() []config.ConfigField {
	schema := config.GetSchema()
	seen := make(map[string]struct{}, len(schema))
	unique_reversed := make([]config.ConfigField, 0, len(schema))
	for index := len(schema) - 1; index >= 0; index-- {
		field := schema[index]
		if _, ok := seen[field.Key]; ok {
			continue
		}
		seen[field.Key] = struct{}{}
		unique_reversed = append(unique_reversed, field)
	}
	unique := make([]config.ConfigField, len(unique_reversed))
	for index := range unique_reversed {
		unique[len(unique_reversed)-1-index] = unique_reversed[index]
	}
	return unique
}
