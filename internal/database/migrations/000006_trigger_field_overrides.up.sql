ALTER TABLE `user_flow_instance` ADD COLUMN `trigger_overrides` TEXT NOT NULL DEFAULT '{}';

-- Separate declared input keys from runtime progress when defaults are refreshed.
UPDATE `flow_schedule` AS schedule
SET `initial_data` = json_set(initial_data, '$.__automation.configured_input_keys', json((
  SELECT json_group_array(input.key)
  FROM user_flow AS flow, json_each(flow.default_triggers) AS trigger,
       json_each(json_extract(trigger.value, '$.initial_data')) AS input
  WHERE flow.id = schedule.flow_id
    AND json_extract(trigger.value, '$.id') = json_extract(schedule.initial_data, '$.__automation.default_trigger_id')
)))
WHERE json_valid(initial_data)
  AND json_extract(CASE WHEN json_valid(initial_data) THEN initial_data ELSE '{}' END, '$.__automation.default_trigger_id') IS NOT NULL;
