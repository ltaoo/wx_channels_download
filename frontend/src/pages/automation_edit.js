import { AutomationPageViewModel } from "./automation.model.js";
import { TemplateEditor } from "../components/template_editor.js";
import {
  AutomationEmptyState,
  AutomationFeedback,
  AutomationFlowGraph,
  AutomationNodeExecutionWindows,
  AutomationRunPipelineDialog,
  AutomationWorkspaceToolbar,
  AutomationStartTriggers,
} from "./automation.components.js";

function AutomationNodeInspector(props) {
  const vm$ = props.store;
  return View(
    {
      class: "flex fixed z-sticky top-120px max-520:top-160px right-4 bottom-4 w-105 max-w-screen-inset min-w-0 min-h-0 flex-col overflow-hidden dm-panel shadow-md motion-reduce:transition-none transition-slide-panel",
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
        document.querySelector(".graph-node-target.is-selected")?.focus();
      },
    },
    [
      View({ class: "flex items-center justify-between p-3 gap-2 border-b-default border-b-solid border-b-color-default bg-color-bg-surface", attributes: { n: "automation-node-inspector-header" } }, [
        View({ attributes: { n: "automation-node-inspector-heading" } }, [
          View({ class: "font-display text-base font-semibold text-color-text-primary", attributes: { n: "automation-node-inspector-title", id: "automation-node-inspector-title" } }, ["节点配置"]),
          View({ class: "mt-0.5 text-caption leading-base text-color-text-secondary", attributes: { n: "automation-node-inspector-description" } }, [
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
        return View({ class: "min-h-0 flex-1 overflow-auto", attributes: { n: "automation-node-inspector-body" } }, [Show({
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
              { class: "flex flex-col p-3 gap-3", attributes: { n: "automation-node-inspector-fields" } },
              [
                View({ class: "flex min-w-0 items-center gap-2", attributes: { n: "automation-inspector-node-summary" } }, [
                  Tag({ variant: is_start ? "success" : "info", attributes: { n: "automation-inspector-node-type" } }, [
                    vm$.methods.flowLabel(current_node.type),
                  ]),
                  View(
                    {
                      class: "min-w-0 overflow-hidden font-mono text-sm whitespace-nowrap text-ellipsis code-label",
                      attributes: { n: "automation-inspector-node-id", title: current_node.id },
                    },
                    [current_node.id],
                  ),
                ]),
                View({ class: "flex min-w-0 flex-col gap-1 form-field ", attributes: { n: "automation-inspector-node-name-field" } }, [
                  Label({ class: "text-xs font-semibold text-color-text-secondary", attributes: { n: "automation-inspector-node-name-label" } }, ["节点名称"]),
                  Input({
                    store: vm$.ui.input_edit_node_name$,
                    attributes: {
                      n: "automation-edit-node-name",
                      "aria-label": "节点名称",
                    },
                  }),
                ]),
                current_node.type !== "StartNode" ? View({ class: "flex min-w-0 flex-col gap-1 form-field ", attributes: { n: "automation-inspector-node-config-field" } }, [
                  Label({ class: "text-xs font-semibold text-color-text-secondary", attributes: { n: "automation-inspector-node-config-label" } }, ["节点配置 JSON"]),
                  TemplateEditor({
                    store: vm$.ui.input_edit_node_config$,
                    value: vm$.state.edit_node_config,
                    get_variables: vm$.methods.availableContextKeys,
                    min_height: "220px",
                    attributes: {
                      n: "automation-edit-node-config",
                      spellcheck: "false",
                      "aria-label": "节点配置 JSON",
                    },
                  }),
                  View({ class: "text-caption text-color-text-secondary", attributes: { n: "automation-node-template-help" } }, ["输入 {{ 选择变量；方向键选择，Tab / Enter 补全，Esc 关闭建议。"]),
                ]) : null,
                current_node.type !== "StartNode" ? View({ class: "flex min-w-0 flex-col gap-1", attributes: { n: "automation-inspector-node-policy-field" } }, [
                  Label({ attributes: { n: "automation-inspector-node-policy-label", for: "automation-edit-node-policy" }, class: "text-xs font-semibold text-color-text-secondary" }, ["执行策略 JSON"]),
                  Textarea({ store: vm$.ui.input_edit_node_policy$, class: "font-mono text-xs", attributes: { id: "automation-edit-node-policy", n: "automation-edit-node-policy", rows: "8", spellcheck: "false", "aria-label": "执行策略 JSON" } }),
                  View({ class: "text-caption text-color-text-secondary", attributes: { n: "automation-inspector-node-policy-help" } }, ["retry.max_attempts 是总尝试次数（1～20）；error_next_node 是失败后的节点 ID。人工确认的 input_schema 用于校验回复。"]),
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
                View({ class: "pt-3 border-t-default border-t-solid border-t-color-default", attributes: { n: "automation-inspector-next-nodes" } }, [
                  View({ class: "block mb-1 text-xs font-medium text-color-text-secondary", attributes: { n: "automation-inspector-next-nodes-label" } }, ["后续节点"]),
                  View({ class: "overflow-visible font-mono text-sm whitespace-pre-wrap word-break-break-all text-overflow-initial code-label", attributes: { n: "automation-inspector-next-nodes-value" } }, [
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
  return TemplateEditor({
    store: field$.input,
    get_variables: field$.get_variables,
    multiline: textarea,
    attributes: {
      id: field_id,
      n: `automation-service-input-${name}`,
      "aria-label": field_schema.label || name,
      "aria-required": String(Boolean(field_schema.required)),
      inputmode: field_schema.type === "integer" || field_schema.type === "number" ? "decimal" : undefined,
    },
  });
}

function AutomationToolFormRender(props) {
  const field_names = computed(props.store, (form) => {
    if (!form || !form.fields) return [];
    return Object.keys(form.fields);
  });
  return View(
    {
      class: "grid grid-cols-2 max-900:grid-cols-1fr gap-3",
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
                "flex min-w-0 flex-col gap-1 form-field-expanded ",
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
                  class: "text-xs font-semibold text-color-text-secondary",
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
                      class: "-webkit-box overflow-hidden text-xs leading-base -webkit-box-orient-vertical -webkit-line-clamp-2 text-color-text-secondary form-help",
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
          View({ class: "grid grid-cols-2 max-900:grid-cols-1fr gap-3 form-grid" }, [
            View({ class: "flex min-w-0 flex-col gap-1 form-field " }, [
              View({ class: "text-xs font-semibold text-color-text-secondary" }, ["节点类型"]),
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
                return View({ class: "flex min-w-0 flex-col gap-1 form-field " }, [
                  View({ class: "text-xs font-semibold text-color-text-secondary" }, ["连接自"]),
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
                return View({ class: "flex min-w-0 flex-col gap-1 form-field " }, [
                  View({ class: "text-xs font-semibold text-color-text-secondary" }, ["插入位置"]),
                  View(
                    {
                      class: "py-2 px-2.5 text-xs border-default border-solid border-color-warning-soft rounded-content bg-color-warning-soft text-color-warning dm-alert",
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
              return View({ class: "flex min-w-0 flex-col gap-1 form-field " }, [
                View({ class: "text-xs font-semibold text-color-text-secondary" }, ["Service tool"]),
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
              return View({ class: "py-2 px-2.5 text-xs border-default border-solid border-color-warning-soft rounded-content bg-color-warning-soft text-color-warning dm-alert" }, [
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
                  return View({ class: "flex flex-col p-3 gap-2 border-default border-solid border-color-default rounded-content bg-color-bg-subtle" }, [
                    View({ class: "text-xs font-semibold text-color-text-secondary" }, ["Tool 参数"]),
                    AutomationToolFormRender({
                      store: vm$.state.add_service_form,
                    }),
                  ]);
                },
                else() {
                  return View({ class: "py-2 px-2.5 text-xs border-default border-solid border-color-warning-soft rounded-content bg-color-warning-soft text-color-warning dm-alert" }, [
                    "该 tool 无需填写参数。",
                  ]);
                },
              });
            },
          }),

          Show({
            when: vm$.state.add_type,
            ok() {
              return View({ class: "py-2 px-2.5 text-xs border-default border-solid border-color-warning-soft rounded-content bg-color-warning-soft text-color-warning dm-alert" }, [
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
          View({ class: "flex min-w-0 flex-col gap-1 form-field " }, [
            View({ class: "text-xs font-semibold text-color-text-secondary" }, ["节点名称"]),
            Input({
              store: vm$.ui.input_add_name$,
              attributes: { n: "automation-add-name" },
            }),
          ]),
          View({ class: "flex min-w-0 flex-col gap-1 form-field " }, [
            View({ class: "text-xs font-semibold text-color-text-secondary" }, ["节点配置 JSON"]),
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

function AutomationEditPageView(props) {
  const vm$ = AutomationPageViewModel(props, { mode: "edit" });
  return View(
    {
      class: "relative h-full min-h-0 min-w-0 overflow-hidden font-sans bg-content-bg text-content-text group-content-page",
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
              class: "absolute inset-0",
              attributes: { n: "automation-editor-workspace" },
            },
            [
              AutomationFlowGraph({ store: vm$, editable: true, class: "h-full min-h-0 border-0 rounded-0" }),
              AutomationNodeInspector({ store: vm$ }),
            ],
          );
        },
      }),
      AutomationWorkspaceToolbar({ store: vm$, mode: "edit" }),
      View({ class: "fixed z-sticky bottom-4 left-4 max-w-screen-inset pointer-events-none", attributes: { n: "automation-editor-feedback" } }, [
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

export default AutomationEditPageView;
