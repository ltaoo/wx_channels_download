import { AutomationPageViewModel } from "./automation.model.js";
import {
  AutomationEmptyState,
  AutomationFlowGraph,
  AutomationRunPipelineDialog,
  AutomationWorkspaceToolbar,
  automation_execution_status_class,
} from "./flow.components.js";

function AutomationNodeInspector(props) {
  const vm$ = props.store;
  return View(
    {
      class: "dm-panel automation-node-inspector",
      attributes: { n: "automation-node-inspector" },
    },
    [
      View({ class: "automation-editor-panel__header" }, [
        View({}, [
          View({ class: "automation-editor-panel__title" }, ["节点配置"]),
          View({ class: "automation-editor-panel__hint" }, [
            "修改当前选中节点",
          ]),
        ]),
      ]),
      (() => {
        // const node = (vm$.state.edit_nodes.value || []).find(
        //   (item) => item.id === selected_id,
        // );
        const node = computed(
          vm$.state.selected_edit_node_id,
          (selected_id) => {
            return (vm$.state.edit_nodes.value || []).find(
              (item) => item.id === selected_id,
            );
          },
        );
        return Show({
          when: computed(node, (t) => !t),
          ok() {
            return AutomationEmptyState({
              compact: true,
              name: "automation-node-inspector-empty",
              title: "请选择节点",
              description: "在画布中选择节点后编辑配置。",
            });
          },
          else() {
            const is_start = node.id === vm$.state.edit_start_node_id.value;
            return View(
              { class: "automation-inspector-form" },
              [
                View({ class: "automation-inspector-meta" }, [
                  Tag({ variant: is_start ? "success" : "info" }, [
                    vm$.methods.flowLabel(node.type),
                  ]),
                  View(
                    {
                      class: "automation-code",
                      attributes: { title: node.id },
                    },
                    [node.id],
                  ),
                ]),
                View({ class: "automation-form__field" }, [
                  Label({ class: "automation-form__label" }, ["节点名称"]),
                  Input({
                    store: vm$.ui.input_edit_node_name$,
                    attributes: {
                      n: "automation-edit-node-name",
                      "aria-label": "节点名称",
                    },
                  }),
                ]),
                View({ class: "automation-form__field" }, [
                  Label({ class: "automation-form__label" }, ["节点配置 JSON"]),
                  Textarea({
                    store: vm$.ui.input_edit_node_config$,
                    class: "automation-node-config-input",
                    attributes: {
                      n: "automation-edit-node-config",
                      rows: "12",
                      spellcheck: "false",
                      "aria-label": "节点配置 JSON",
                    },
                  }),
                ]),
                Button(
                  {
                    store: vm$.ui.btn_apply_node_config$,
                    attributes: {
                      n: "automation-apply-node-config",
                      type: "button",
                    },
                  },
                  ["应用节点配置"],
                ),
                View({ class: "automation-inspector-connections" }, [
                  View({ class: "automation-property__label" }, ["后续节点"]),
                  View({ class: "automation-code automation-code--wrap" }, [
                    (node.next_ids || []).join("、") || "无",
                  ]),
                ]),
                !is_start
                  ? Button(
                      {
                        store: vm$.ui.btn_remove_selected_node$,
                        class: "automation-remove-selected-node",
                        attributes: {
                          n: "automation-remove-selected-node",
                          type: "button",
                        },
                      },
                      ["删除当前节点"],
                    )
                  : null,
              ].filter(Boolean),
            );
          },
        });
      })(),
    ],
  );
}

function AutomationExecutionPanel(props) {
  const vm$ = props.store;
  const recent_logs_ = computed(vm$.state.execution_logs, (logs) =>
    (logs || []).slice(-50).reverse(),
  );
  return View(
    {
      class: "automation-execution-panel",
      attributes: {
        n: "automation-execution-panel",
        "aria-label": "执行过程日志",
      },
    },
    [
      View({ class: "automation-execution-panel__header" }, [
        View({}, [
          View({ class: "automation-editor-panel__title" }, ["执行过程"]),
          View({ class: "automation-editor-panel__hint" }, [
            computed(vm$.state.execution_run_id, (run_id) =>
              run_id ? `Run ID：${run_id}` : "等待触发 Pipeline",
            ),
          ]),
        ]),
        View(
          {
            class: combine(
              {
                connected: vm$.state.execution_channel_connected,
                status: vm$.state.execution_run_status,
              },
              ({ connected, status }) =>
                `automation-execution-connection${
                  connected ? " is-connected" : ""
                }${status ? ` is-${automation_execution_status_class(status)}` : ""}`,
            ),
            attributes: { role: "status", "aria-live": "polite" },
          },
          [
            View({ class: "automation-execution-connection__dot" }),
            computed(
              combine(
                {
                  connected: vm$.state.execution_channel_connected,
                  status: vm$.state.execution_run_status,
                },
                (value) => value,
              ),
              ({ connected, status }) =>
                status || (connected ? "实时连接" : "连接中"),
            ),
          ],
        ),
      ]),
      View(
        {
          class: "automation-execution-log-list",
          attributes: { "aria-live": "polite", "aria-relevant": "additions" },
        },
        [
          Show({
            when: computed(recent_logs_, (logs) => logs.length === 0),
            ok() {
              return View({ class: "automation-execution-log-empty" }, [
                "触发 Pipeline 后，这里会实时显示每个节点的入参、行为和输出。",
              ]);
            },
          }),
          For({
            each: recent_logs_,
            key: "_execution_key",
            render(entry_) {
              const entry =
                entry_ && entry_.value !== undefined ? entry_.value : entry_;
              return View(
                {
                  as: "details",
                  class: "automation-execution-log",
                  attributes: {
                    n: `automation-execution-log-${entry.node_id}`,
                  },
                },
                [
                  View(
                    {
                      as: "summary",
                      class: "automation-execution-log__summary dm-focus-ring",
                    },
                    [
                      View({ class: "automation-execution-log__node" }, [
                        entry.node_name || entry.node_id,
                      ]),
                      View(
                        {
                          class: `automation-execution-log__status is-${automation_execution_status_class(
                            entry.outcome,
                          )}`,
                        },
                        [vm$.methods.nodeExecutionStatusLabel(entry.outcome)],
                      ),
                      View({ class: "automation-execution-log__meta" }, [
                        `第 ${entry.attempt || 1} 次 · ${entry.duration_ms || 0} ms`,
                      ]),
                    ],
                  ),
                  View(
                    { class: "automation-execution-log__body" },
                    [
                      ...[
                        ["入参", entry.input],
                        ["节点行为", entry.behavior],
                        ["输出", entry.output],
                      ].map(([label, value]) =>
                        View({ class: "automation-execution-log__data" }, [
                          View({ class: "automation-execution-log__label" }, [
                            label,
                          ]),
                          View(
                            {
                              as: "pre",
                              class: "automation-execution-log__value",
                            },
                            [vm$.methods.formatExecutionValue(value)],
                          ),
                        ]),
                      ),
                      entry.error
                        ? View({ class: "automation-execution-log__error" }, [
                            entry.error,
                          ])
                        : null,
                    ].filter(Boolean),
                  ),
                ],
              );
            },
          }),
        ],
      ),
    ],
  );
}

function AutomationDeletePipelineConfirm(props) {
  const vm$ = props.store;
  return Confirm({
    store: vm$.ui.delete_dialog$,
    class: "dm-dialog--sm",
    name: "automation-delete-pipeline",
    title: "删除 Pipeline",
    description: computed(vm$.state.selected_pipeline, (flow) =>
      flow
        ? `确定删除「${flow.name || flow.id}」？关联的流程定义将停止使用。`
        : "确定删除当前 Pipeline？",
    ),
    cancelText: "取消",
    okText: "删除 Pipeline",
  });
}

function automation_service_text_field(props) {
  const { field$, field_schema, field_id, name, textarea } = props;
  const on_change = (event) => {
    const text = String((event && event.target && event.target.value) || "");
    const at = text.lastIndexOf("{{");
    if (at >= 0 && text.indexOf("}}", at) < 0) {
      field$.suggest_visible.as(true);
      field$.suggest_keyword.as(text.slice(at + 2));
    } else {
      field$.suggest_visible.as(false);
    }
  };
  const control = textarea
    ? Textarea({
        store: field$.input,
        attributes: {
          id: field_id,
          n: `automation-service-input-${name}`,
          rows: "4",
          "aria-label": field_schema.label || name,
          "aria-required": String(Boolean(field_schema.required)),
        },
        onChange: on_change,
      })
    : Input({
        store: field$.input,
        attributes: {
          id: field_id,
          n: `automation-service-input-${name}`,
          autocomplete: "off",
          "aria-label": field_schema.label || name,
          "aria-required": String(Boolean(field_schema.required)),
          inputmode:
            field_schema.type === "integer" || field_schema.type === "number"
              ? "decimal"
              : undefined,
        },
        onChange: on_change,
      });
  return View(
    {
      class: "automation-service-template-field",
      attributes: { n: `automation-service-template-field-${name}` },
    },
    [
      control,
      Show({
        when: field$.suggest_visible,
        ok() {
          return View(
            {
              class: "automation-context-suggest",
              attributes: {
                n: `automation-context-suggest-${name}`,
                role: "listbox",
              },
            },
            [
              For({
                each: computed(field$.suggest_keyword, (keyword) => {
                  const needle = String(keyword || "");
                  return (field$.context_keys || []).filter((entry) => {
                    const token = `${entry.namespace}.${entry.key}`;
                    return (
                      String(entry.key).includes(needle) ||
                      token.includes(needle)
                    );
                  });
                }),
                render(entry) {
                  const token = `${entry.namespace}.${entry.key}`;
                  return View(
                    {
                      as: "button",
                      class: "automation-context-suggest__item",
                      attributes: {
                        n: `automation-context-suggest-item-${name}-${token}`,
                        type: "button",
                        role: "option",
                      },
                      onClick() {
                        const current = String(field$.input.value || "");
                        const at = current.lastIndexOf("{{");
                        const next = current.slice(0, at) + "{{" + token + "}}";
                        field$.input.setValue(next);
                        field$.suggest_visible.as(false);
                      },
                    },
                    [`{{${token}}}`],
                  );
                },
              }),
            ],
          );
        },
      }),
    ],
  );
}

function AutomationToolFormRender(props) {
  const field_names = computed(props.store, (form) => {
    if (!form || !form.fields) return [];
    return Object.keys(form.fields);
  });
  return View(
    {
      class: "automation-service-form",
      attributes: { n: "automation-service-form" },
    },
    [
      For({
        each: field_names,
        render(name) {
          const form = props.store.value;
          if (!form || !form.fields) return null;
          const field$ = form.fields[name];
          if (!field$) return null;
          const field_schema = field$.form_schema || {};
          const field_id = `automation-service-field-${name}`;
          return View(
            {
              class: [
                "automation-form__field automation-service-form__field",
                field_schema.control === "textarea" ? "is-wide" : "",
              ]
                .filter(Boolean)
                .join(" "),
              attributes: { n: `automation-service-form-${name}` },
            },
            [
              View(
                {
                  as: "label",
                  class: "automation-form__label",
                  attributes: { for: field_id },
                },
                [field_schema.label || name, field_schema.required ? " *" : ""],
              ),
              Match({
                when: computed(field$, () => field_schema.control),
                cases: {
                  select() {
                    return Select({
                      store: field$.input,
                      attributes: {
                        id: field_id,
                        n: `automation-service-input-${name}`,
                        "aria-label": field_schema.label || name,
                        "aria-required": String(Boolean(field_schema.required)),
                      },
                    });
                  },
                  checkbox() {
                    return Checkbox({
                      store: field$.input,
                      attributes: {
                        id: field_id,
                        n: `automation-service-input-${name}`,
                        "aria-label": field_schema.label || name,
                        "aria-required": String(Boolean(field_schema.required)),
                      },
                    });
                  },
                  textarea() {
                    return automation_service_text_field({
                      field$,
                      field_schema,
                      field_id,
                      name,
                      textarea: true,
                    });
                  },
                  input() {
                    return automation_service_text_field({
                      field$,
                      field_schema,
                      field_id,
                      name,
                      textarea: false,
                    });
                  },
                },
              }),
              field_schema.description
                ? View(
                    {
                      class: "automation-service-form__help",
                      attributes: { title: field_schema.description },
                    },
                    [field_schema.description],
                  )
                : null,
            ].filter(Boolean),
          );
        },
      }),
    ],
  );
}

function AutomationAddNodeDialog(props) {
  const vm$ = props.store;
  return Dialog(
    {
      store: vm$.ui.add_dialog$,
      class: "automation-form-dialog",
      attributes: { n: "automation-add-dialog" },
    },
    [
      DialogHeader({}, [
        DialogTitle({}, ["添加节点"]),
        DialogDescription({}, [
          "选择节点类型并填入配置；新节点会连接到所选节点的后面。",
        ]),
      ]),
      DialogBody({}, [
        View({ class: "automation-form" }, [
          View({ class: "automation-form__row" }, [
            View({ class: "automation-form__field" }, [
              View({ class: "automation-form__label" }, ["节点类型"]),
              Select({
                store: vm$.ui.select_add_type$,
                attributes: {
                  n: "automation-add-type",
                  "aria-label": "选择节点类型",
                },
              }),
            ]),
            View({ class: "automation-form__field" }, [
              View({ class: "automation-form__label" }, ["连接自"]),
              Select({
                store: vm$.ui.select_add_from$,
                attributes: {
                  n: "automation-add-from",
                  "aria-label": "连接自哪个节点",
                },
              }),
            ]),
          ]),
          Show({
            when: computed(
              vm$.state.add_type,
              (node_type) => node_type === "ServiceNode",
            ),
            ok() {
              return View({ class: "automation-form__field" }, [
                View({ class: "automation-form__label" }, ["Service tool"]),
                Select({
                  store: vm$.ui.select_add_service_tool$,
                  attributes: {
                    n: "automation-add-service-tool",
                    "aria-label": "选择要调用的 Service tool",
                  },
                }),
              ]);
            },
          }),
          Show({
            when: vm$.state.add_service_tool,
            ok() {
              const tool = vm$.methods.catalogServiceTool(
                vm$.state.add_service_tool.value,
              );
              if (!tool) return View({});
              const input_schema = tool.input_schema || tool.inputSchema || {};
              const argument_names = Object.keys(input_schema.properties || {});
              return View({ class: "dm-alert automation-form__hint" }, [
                tool.description || tool.title || tool.name,
                "（参数：",
                argument_names.join("、") || "无",
                "）",
              ]);
            },
          }),
          Show({
            when: vm$.state.add_service_tool,
            ok() {
              return Show({
                when: computed(
                  vm$.state.add_service_form_schema,
                  (form_schema) =>
                    Array.isArray(form_schema) && form_schema.length > 0,
                ),
                ok() {
                  return View({ class: "automation-service-form-section" }, [
                    View({ class: "automation-form__label" }, ["Tool 参数"]),
                    AutomationToolFormRender({
                      store: vm$.state.add_service_form,
                    }),
                  ]);
                },
                else() {
                  return View({ class: "dm-alert automation-form__hint" }, [
                    "该 tool 无需填写参数。",
                  ]);
                },
              });
            },
          }),
          Show({
            when: vm$.state.add_service_form_error,
            ok() {
              return View(
                {
                  class: "dm-alert is-destructive",
                  attributes: { role: "alert" },
                },
                [vm$.state.add_service_form_error],
              );
            },
          }),
          Show({
            when: vm$.state.add_type,
            ok() {
              return View({ class: "dm-alert automation-form__hint" }, [
                vm$.methods.catalogDescription(vm$.state.add_type.value),
                "（配置键：",
                vm$.methods
                  .catalogConfigKeys(vm$.state.add_type.value)
                  .map((key) => `${key.key}${key.required ? "*" : ""}`)
                  .join("、") || "无",
                "）",
              ]);
            },
          }),
          View({ class: "automation-form__field" }, [
            View({ class: "automation-form__label" }, ["节点名称"]),
            Input({
              store: vm$.ui.input_add_name$,
              attributes: { n: "automation-add-name" },
            }),
          ]),
          View({ class: "automation-form__field" }, [
            View({ class: "automation-form__label" }, ["节点配置 JSON"]),
            Input({
              store: vm$.ui.input_add_config$,
              attributes: {
                n: "automation-add-config",
                autocomplete: "off",
              },
            }),
          ]),
        ]),
      ]),
      DialogFooter({}, [
        Button(
          {
            store: vm$.ui.btn_add_cancel$,
            attributes: { n: "automation-add-cancel", type: "button" },
          },
          ["取消"],
        ),
        Button(
          {
            store: vm$.ui.btn_add_submit$,
            attributes: { n: "automation-add-submit", type: "button" },
          },
          ["添加节点"],
        ),
      ]),
    ],
  );
}

function FlowEditPageView(props) {
  const vm$ = AutomationPageViewModel(props, { mode: "edit" });
  return View(
    {
      class: "content-page automation-page page",
      attributes: { n: "automation-page" },
      onMounted() {
        vm$.methods.ready();
      },
      beforeUnmounted() {
        vm$.methods.destroy();
      },
    },
    [
      View({ class: "content-toolbar-wrap automation-editor-toolbar-wrap" }, [
        AutomationWorkspaceToolbar({ store: vm$, mode: "edit" }),
      ]),
      Show({
        when: vm$.state.selected_pipeline,
        ok() {
          return View(
            {
              class: "automation-editor-workspace",
              attributes: { n: "automation-editor-workspace" },
            },
            [
              View({ class: "automation-editor-canvas" }, [
                View({ class: "automation-editor-canvas__header" }, [
                  View({}, [
                    View({ class: "automation-editor-panel__title" }, [
                      "工作流画布",
                    ]),
                    View({ class: "automation-editor-panel__hint" }, [
                      computed(
                        vm$.state.edit_nodes,
                        (nodes) => `${nodes.length} 个节点 · 点击节点编辑配置`,
                      ),
                    ]),
                  ]),
                ]),
                AutomationFlowGraph({ store: vm$, editable: true }),
                AutomationExecutionPanel({ store: vm$ }),
              ]),
              AutomationNodeInspector({ store: vm$ }),
            ],
          );
        },
      }),
      Show({
        when: computed(vm$.state.selected_pipeline, (flow) => !flow),
        ok() {
          return View({ class: "container" }, [
            AutomationEmptyState({
              detail: true,
              name: "automation-pipeline-editor-empty",
              title: computed(vm$.state.loading, (loading) =>
                loading ? "正在加载编辑器…" : "无法打开 Pipeline",
              ),
              description: computed(vm$.state.loading, (loading) =>
                loading
                  ? "正在准备节点库和流程定义。"
                  : "请返回列表确认 Pipeline 是否仍然存在。",
              ),
            }),
          ]);
        },
      }),
      AutomationAddNodeDialog({ store: vm$ }),
      AutomationRunPipelineDialog({ store: vm$ }),
      AutomationDeletePipelineConfirm({ store: vm$ }),
    ],
  );
}

export default FlowEditPageView;
