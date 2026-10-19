import { AutomationPageViewModel } from "./automation.model.js";
import {
  AutomationEmptyState,
  AutomationFeedback,
  AutomationFlowGraph,
  AutomationNodeExecutionWindows,
  AutomationRunPipelineDialog,
  AutomationWorkspaceToolbar,
  AutomationStartTriggers,
} from "./flow.components.js";

function AutomationNodeInspector(props) {
  const vm$ = props.store;
  return View(
    {
      class: "dm-panel fixed z-sticky top-120px max-520:top-160px right-4 bottom-4 w-[min(420px,_calc(100vw_-_32px))] min-w-0 min-h-0 flex flex-col overflow-hidden shadow-md transition-[transform_200ms_ease,_opacity_200ms_ease,_visibility_200ms] motion-reduce:transition-none",
      style: computed(vm$.state.node_inspector_open, (open) => ({
        transform: open ? "translateX(0)" : "translateX(calc(100% + 16px))",
        opacity: open ? "1" : "0",
        visibility: open ? "visible" : "hidden",
      })),
      attributes: {
        n: "automation-node-inspector",
        id: "automation-node-inspector",
        inert: computed(vm$.state.node_inspector_open, (open) => !open),
        "aria-hidden": computed(vm$.state.node_inspector_open, (open) => String(!open)),
        "aria-labelledby": "automation-node-inspector-title",
      },
      onKeyDown(event) {
        if (event.key !== "Escape") return;
        vm$.methods.closeNodeInspector();
        document.querySelector(".group-automation-flow-node.is-selected")?.focus();
      },
    },
    [
      View({ class: "flex items-center justify-between gap-2 p-3 bg-color-bg-surface border-bottom-[var(--dm-border-width)_solid_var(--dm-color-border)]", attributes: { n: "automation-node-inspector-header" } }, [
        View({ attributes: { n: "automation-node-inspector-heading" } }, [
          View({ class: "text-color-text-primary font-display text-base font-semibold", attributes: { n: "automation-node-inspector-title", id: "automation-node-inspector-title" } }, ["节点配置"]),
          View({ class: "mt-0-5 text-color-text-secondary text-caption leading-base", attributes: { n: "automation-node-inspector-description" } }, [
            "修改当前选中节点",
          ]),
        ]),
        Button({
          store: vm$.ui.btn_close_node_inspector$,
          attributes: { n: "automation-node-inspector-close", type: "button", "aria-label": "关闭节点配置" },
        }, ["关闭"]),
      ]),
      (() => {
        const node = combine(
          { selected_id: vm$.state.selected_edit_node_id, nodes: vm$.state.edit_nodes },
          ({ selected_id, nodes }) => {
            return (nodes || []).find(
              (item) => item.id === selected_id,
            );
          },
        );
        return View({ class: "flex-1 min-h-0 overflow-auto", attributes: { n: "automation-node-inspector-body" } }, [Show({
          when: computed(node, (t) => !t),
          ok() {
            return AutomationEmptyState({
              compact: true,
              name: "automation-node-inspector-empty",
              title: "请选择节点",
              description: "在画布中选择节点后编辑配置。",
            });
          },
        }), For({
          each: computed(node, (value) => value ? [value] : []),
          key: "id",
          render(value) {
            const current_node = value && value.value !== undefined ? value.value : value;
            const is_start = current_node.id === vm$.state.edit_start_node_id.value;
            return View(
              { class: "flex flex-col gap-3 p-3", attributes: { n: "automation-node-inspector-fields" } },
              [
                View({ class: "min-w-0 flex items-center gap-2 [&_.group-automation-code]:min-w-0", attributes: { n: "automation-inspector-node-summary" } }, [
                  Tag({ variant: is_start ? "success" : "info", attributes: { n: "automation-inspector-node-type" } }, [
                    vm$.methods.flowLabel(current_node.type),
                  ]),
                  View(
                    {
                      class: "group-automation-code overflow-hidden font-mono text-sm text-ellipsis whitespace-nowrap",
                      attributes: { n: "automation-inspector-node-id", title: current_node.id },
                    },
                    [current_node.id],
                  ),
                ]),
                View({ class: "min-w-0 flex flex-col gap-1 [&_.dm-select]:w-full [&_.dm-input]:w-full [&.dm-flex]:flex-row [&.dm-flex]:items-center [&.dm-flex]:gap-1-5", attributes: { n: "automation-inspector-node-name-field" } }, [
                  Label({ class: "text-color-text-secondary text-xs font-semibold", attributes: { n: "automation-inspector-node-name-label" } }, ["节点名称"]),
                  Input({
                    store: vm$.ui.input_edit_node_name$,
                    attributes: {
                      n: "automation-edit-node-name",
                      "aria-label": "节点名称",
                    },
                  }),
                ]),
                current_node.type !== "StartNode" ? View({ class: "min-w-0 flex flex-col gap-1 [&_.dm-select]:w-full [&_.dm-input]:w-full [&.dm-flex]:flex-row [&.dm-flex]:items-center [&.dm-flex]:gap-1-5", attributes: { n: "automation-inspector-node-config-field" } }, [
                  Label({ class: "text-color-text-secondary text-xs font-semibold", attributes: { n: "automation-inspector-node-config-label" } }, ["节点配置 JSON"]),
                  Textarea({
                    store: vm$.ui.input_edit_node_config$,
                    class: "min-h-220px font-mono text-xs",
                    attributes: {
                      n: "automation-edit-node-config",
                      rows: "12",
                      spellcheck: "false",
                      "aria-label": "节点配置 JSON",
                    },
                  }),
                ]) : null,
                current_node.type !== "StartNode" ? View({ class: "min-w-0 flex flex-col gap-1", attributes: { n: "automation-inspector-node-policy-field" } }, [
                  Label({ attributes: { n: "automation-inspector-node-policy-label", for: "automation-edit-node-policy" }, class: "text-color-text-secondary text-xs font-semibold" }, ["执行策略 JSON"]),
                  Textarea({ store: vm$.ui.input_edit_node_policy$, class: "font-mono text-xs", attributes: { id: "automation-edit-node-policy", n: "automation-edit-node-policy", rows: "8", spellcheck: "false", "aria-label": "执行策略 JSON" } }),
                  View({ class: "text-color-text-secondary text-caption", attributes: { n: "automation-inspector-node-policy-help" } }, ["retry.max_attempts 是总尝试次数（1～20）；error_next_node 是失败后的节点 ID。人工确认的 input_schema 用于校验回复。"]),
                ]) : null,
                current_node.type !== "StartNode" ? Button(
                  {
                    store: vm$.ui.btn_apply_node_config$,
                    attributes: {
                      n: "automation-apply-node-config",
                      type: "button",
                    },
                  },
                  ["应用节点配置"],
                ) : null,
                is_start ? AutomationStartTriggers({ store: vm$ }) : null,
                View({ class: "pt-3 border-top-[var(--dm-border-width)_solid_var(--dm-color-border)]", attributes: { n: "automation-inspector-next-nodes" } }, [
                  View({ class: "block mb-1 text-color-text-secondary text-xs font-medium", attributes: { n: "automation-inspector-next-nodes-label" } }, ["后续节点"]),
                  View({ class: "group-automation-code font-mono text-sm overflow-visible text-overflow-initial whitespace-pre-wrap word-break-break-all", attributes: { n: "automation-inspector-next-nodes-value" } }, [
                    computed(node, (value) => (value && value.next_ids || []).join("、") || "无"),
                  ]),
                ]),
                !is_start
                  ? Button(
                      {
                        store: vm$.ui.btn_remove_selected_node$,
                        class: "mt-auto",
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
        })]);
      })(),
    ],
  );
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
      class: "relative",
      attributes: { n: `automation-service-template-field-${name}` },
    },
    [
      control,
      Show({
        when: field$.suggest_visible,
        ok() {
          return View(
            {
              class: "absolute z-sticky top-[calc(100%_+_var(--dm-space-1))] left-0 right-0 max-h-200px p-1 overflow-auto bg-color-bg-surface border-[var(--dm-border-width)_solid_var(--dm-color-border-strong)] rounded-content shadow-md",
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
                      class: "w-full block p-[var(--dm-space-1-5)_var(--dm-space-2)] text-color-text-primary bg-transparent border-0 rounded-xs text-sm text-left cursor-pointer hover:bg-color-bg-interactive",
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
      class: "grid grid-cols-[repeat(2,_minmax(0,_1fr))] gap-3 max-900:grid-cols-1fr",
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
                "min-w-0 flex flex-col gap-1 [&_.dm-select]:w-full [&_.dm-input]:w-full [&.dm-flex]:flex-row [&.dm-flex]:items-center [&.dm-flex]:gap-1-5 [&.is-wide]:grid-column-[1_/_-1] [&_.dm-textarea-root]:w-full [&_.dm-textarea]:w-full [&.dm-flex_.group-automation-service-form__help]:ml-auto",
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
                  class: "text-color-text-secondary text-xs font-semibold",
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
                      class: "group-automation-service-form__help -webkit-box overflow-hidden text-color-text-secondary text-xs leading-base -webkit-box-orient-vertical -webkit-line-clamp-2",
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
  // 绿点走「新增」（接在节点后面），黄点走「插入」（插在 A → B 这条连线中间）
  const insert_to$ = vm$.state.add_insert_to;
  const insert_route$ = combine(
    {
      from: vm$.state.add_from,
      to: insert_to$,
      nodes: vm$.state.edit_nodes,
    },
    (state) => {
      const nodes = state.nodes || [];
      const label = (node_id) => {
        const node = nodes.find((item) => item.id === node_id);
        return (node && (node.name || node.id)) || node_id;
      };
      return `${label(state.from)} → ${label(state.to)}`;
    },
  );
  return Dialog(
    {
      store: vm$.ui.add_dialog$,
      class: "automation-form-dialog",
      attributes: { n: "automation-add-dialog" },
    },
    [
      DialogHeader({}, [
        Show({
          when: insert_to$,
          ok() {
            return [
              DialogTitle({}, ["插入节点"]),
              DialogDescription({}, [
                "选择节点类型并填入配置；新节点会插到这条连线中间。",
              ]),
            ];
          },
          else() {
            return [
              DialogTitle({}, ["添加节点"]),
              DialogDescription({}, [
                "选择节点类型并填入配置；新节点会连接到所选节点的后面。",
              ]),
            ];
          },
        }),
      ]),
      DialogBody({}, [
        View({ class: "flex flex-col gap-3" }, [
          View({ class: "grid grid-cols-[repeat(2,_minmax(0,_1fr))] gap-3 [&_.dm-button]:align-self-flex-end max-900:grid-cols-1fr" }, [
            View({ class: "min-w-0 flex flex-col gap-1 [&_.dm-select]:w-full [&_.dm-input]:w-full [&.dm-flex]:flex-row [&.dm-flex]:items-center [&.dm-flex]:gap-1-5" }, [
              View({ class: "text-color-text-secondary text-xs font-semibold" }, ["节点类型"]),
              Select({
                store: vm$.ui.select_add_type$,
                attributes: {
                  n: "automation-add-type",
                  "aria-label": "选择节点类型",
                },
              }),
            ]),
            Show({
              when: computed(insert_to$, (to) => !to),
              ok() {
                return View({ class: "min-w-0 flex flex-col gap-1 [&_.dm-select]:w-full [&_.dm-input]:w-full [&.dm-flex]:flex-row [&.dm-flex]:items-center [&.dm-flex]:gap-1-5" }, [
                  View({ class: "text-color-text-secondary text-xs font-semibold" }, ["连接自"]),
                  Select({
                    store: vm$.ui.select_add_from$,
                    attributes: {
                      n: "automation-add-from",
                      "aria-label": "连接自哪个节点",
                    },
                  }),
                ]);
              },
              else() {
                return View({ class: "min-w-0 flex flex-col gap-1 [&_.dm-select]:w-full [&_.dm-input]:w-full [&.dm-flex]:flex-row [&.dm-flex]:items-center [&.dm-flex]:gap-1-5" }, [
                  View({ class: "text-color-text-secondary text-xs font-semibold" }, ["插入位置"]),
                  View(
                    {
                      class: "dm-alert p-[var(--dm-space-2)_var(--dm-space-2-5)] text-color-warning bg-color-warning-soft border-[var(--dm-border-width)_solid_color-mix(in_srgb,_var(--dm-color-warning)_30%,_transparent)] rounded-content text-xs",
                      attributes: { n: "automation-insert-route" },
                    },
                    [insert_route$],
                  ),
                ]);
              },
            }),
          ]),
          Show({
            when: computed(
              vm$.state.add_type,
              (node_type) => node_type === "ServiceNode",
            ),
            ok() {
              return View({ class: "min-w-0 flex flex-col gap-1 [&_.dm-select]:w-full [&_.dm-input]:w-full [&.dm-flex]:flex-row [&.dm-flex]:items-center [&.dm-flex]:gap-1-5" }, [
                View({ class: "text-color-text-secondary text-xs font-semibold" }, ["Service tool"]),
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
              return View({ class: "dm-alert p-[var(--dm-space-2)_var(--dm-space-2-5)] text-color-warning bg-color-warning-soft border-[var(--dm-border-width)_solid_color-mix(in_srgb,_var(--dm-color-warning)_30%,_transparent)] rounded-content text-xs" }, [
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
                  return View({ class: "flex flex-col gap-2 p-3 bg-color-bg-subtle border-[var(--dm-border-width)_solid_var(--dm-color-border)] rounded-content" }, [
                    View({ class: "text-color-text-secondary text-xs font-semibold" }, ["Tool 参数"]),
                    AutomationToolFormRender({
                      store: vm$.state.add_service_form,
                    }),
                  ]);
                },
                else() {
                  return View({ class: "dm-alert p-[var(--dm-space-2)_var(--dm-space-2-5)] text-color-warning bg-color-warning-soft border-[var(--dm-border-width)_solid_color-mix(in_srgb,_var(--dm-color-warning)_30%,_transparent)] rounded-content text-xs" }, [
                    "该 tool 无需填写参数。",
                  ]);
                },
              });
            },
          }),

          Show({
            when: vm$.state.add_type,
            ok() {
              return View({ class: "dm-alert p-[var(--dm-space-2)_var(--dm-space-2-5)] text-color-warning bg-color-warning-soft border-[var(--dm-border-width)_solid_color-mix(in_srgb,_var(--dm-color-warning)_30%,_transparent)] rounded-content text-xs" }, [
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
          View({ class: "min-w-0 flex flex-col gap-1 [&_.dm-select]:w-full [&_.dm-input]:w-full [&.dm-flex]:flex-row [&.dm-flex]:items-center [&.dm-flex]:gap-1-5" }, [
            View({ class: "text-color-text-secondary text-xs font-semibold" }, ["节点名称"]),
            Input({
              store: vm$.ui.input_add_name$,
              attributes: { n: "automation-add-name" },
            }),
          ]),
          View({ class: "min-w-0 flex flex-col gap-1 [&_.dm-select]:w-full [&_.dm-input]:w-full [&.dm-flex]:flex-row [&.dm-flex]:items-center [&.dm-flex]:gap-1-5" }, [
            View({ class: "text-color-text-secondary text-xs font-semibold" }, ["节点配置 JSON"]),
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
          [
            Show({
              when: insert_to$,
              ok() {
                return ["插入节点"];
              },
              else() {
                return ["添加节点"];
              },
            }),
          ],
        ),
      ]),
    ],
  );
}

function FlowEditPageView(props) {
  const vm$ = AutomationPageViewModel(props, { mode: "edit" });
  return View(
    {
      class: "group-content-page relative h-full min-h-0 overflow-hidden bg-content-bg text-content-text min-w-0 font-sans",
      attributes: { n: "automation-page" },
      onMounted() {
        vm$.methods.ready();
      },
      beforeUnmounted() {
        vm$.methods.destroy();
      },
    },
    [
      Show({
        when: vm$.state.selected_pipeline,
        ok() {
          return View(
            {
              class: "absolute inset-0 [&_.group-automation-flow-scroll]:h-full [&_.group-automation-flow-scroll]:min-h-0 [&_.group-automation-flow-scroll]:border-0 [&_.group-automation-flow-scroll]:rounded-0",
              attributes: { n: "automation-editor-workspace" },
            },
            [
              AutomationFlowGraph({ store: vm$, editable: true }),
              AutomationNodeInspector({ store: vm$ }),
            ],
          );
        },
      }),
      AutomationWorkspaceToolbar({ store: vm$, mode: "edit" }),
      View({ class: "fixed z-sticky bottom-4 left-4 max-w-[calc(100vw_-_32px)] pointer-events-none", attributes: { n: "automation-editor-feedback" } }, [
        AutomationFeedback({ store: vm$ }),
      ]),
      Show({
        when: computed(vm$.state.selected_pipeline, (flow) => !flow),
        ok() {
          return View({ class: "" }, [
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
      AutomationNodeExecutionWindows({ store: vm$ }),
    ],
  );
}

export default FlowEditPageView;
