import { AutomationPageViewModel } from "./automation.model.js";

function automation_trigger_badge(props) {
  const type = props.type || "Cron";
  return Tag(
    {
      class: `automation-trigger automation-trigger--${String(type).toLowerCase()}`,
      attributes: { n: `automation-trigger-${String(type).toLowerCase()}` },
    },
    [props.label],
  );
}

function automation_status_badge(value) {
  const normalized = String(value || "UNKNOWN").toUpperCase();
  let tone = "info";
  if (["COMPLETED", "ENABLED", "TRUE", "YES"].includes(normalized)) {
    tone = "success";
  } else if (["FAILED", "FALSE", "NO", "CANCELLED"].includes(normalized)) {
    tone = "danger";
  } else if (["RUNNING", "WAITING", "QUEUED"].includes(normalized)) {
    tone = "warning";
  }
  return Tag(
    {
      class: `automation-status automation-status--${tone}`,
      attributes: { n: `automation-status-${normalized.toLowerCase()}` },
    },
    [normalized],
  );
}

function AutomationPageView(props) {
  const vm$ = AutomationPageViewModel(props);
  return View(
    {
      class: "content-page automation-page page",
      attributes: { n: "automation-page" },
      onMounted() {
        vm$.methods.ready();
      },
    },
    [
      View({ class: "content-toolbar-wrap container" }, [
        AutomationPageToolbar({ store: vm$ }),
      ]),
      Show({
        when: vm$.state.error,
        ok() {
          return View({ class: "container" }, [
            Alert(
              {
                variant: "destructive",
                class: "automation-alert",
                attributes: { n: "automation-error-alert" },
              },
              [
                AlertTitle({ attributes: { n: "automation-error-title" } }, [
                  "请求失败",
                ]),
                AlertDescription({}, [vm$.state.error.value]),
              ],
            ),
          ]);
        },
      }),
      Show({
        when: vm$.state.notice,
        ok() {
          return View({ class: "container" }, [
            Alert(
              {
                class: "automation-alert",
                attributes: { n: "automation-notice-alert" },
              },
              [AlertDescription({}, [vm$.state.notice.value])],
            ),
          ]);
        },
      }),
      View({ class: "content-main container" }, [
        View(
          { class: "automation-body", attributes: { n: "automation-body" } },
          [
            // AutomationPageSide({ store: vm$ }),
            View(
              {
                class: "automation-side",
                attributes: { n: "automation-side" },
              },
              [
                Show({
                  when: computed(vm$.state.tab, (tab) => tab === "pipelines"),
                  ok() {
                    return AutomationPipelineList({ store: vm$ });
                  },
                }),
                Show({
                  when: computed(vm$.state.tab, (tab) => tab === "schedules"),
                  ok() {
                    return AutomationScheduleList({ store: vm$ });
                  },
                }),
              ],
            ),
            View(
              {
                class: "automation-main",
                attributes: { n: "automation-main" },
              },
              [
                Show({
                  when: computed(
                    {
                      tab: vm$.state.tab,
                      schedule: vm$.state.selected_schedule,
                    },
                    (state) => state.tab === "schedules" && state.schedule,
                  ),
                  ok() {
                    return AutomationScheduleSummary({ store: vm$ });
                  },
                }),
                Show({
                  when: vm$.state.selected_pipeline,
                  ok() {
                    return AutomationPipelineDetail({ store: vm$ });
                  },
                }),
                Show({
                  when: computed(vm$.state.selected_pipeline, (flow) => !flow),
                  ok() {
                    return View(
                      { class: "automation-empty automation-empty--detail" },
                      ["请选择或创建一个 Pipeline"],
                    );
                  },
                }),
              ],
            ),
          ],
        ),
      ]),
      AutomationCreatePipelineDialog({ store: vm$ }),
      AutomationAddNodeDialog({ store: vm$ }),
      AutomationCreateScheduleDialog({ store: vm$ }),
    ],
  );
}

function AutomationPageToolbar(props) {
  const vm$ = props.store;
  return View({ class: "content-toolbar automation-toolbar" }, [
    View({ class: "dm-flex dm-items-center dm-gap-2" }, [
      View({ class: "automation-toolbar-title" }, ["自动化"]),
      Tab(
        {
          selected: computed(vm$.state.tab, (tab) => tab === "pipelines"),
          onClick() {
            vm$.methods.switchTab("pipelines");
          },
          attributes: { n: "automation-tab-pipelines" },
        },
        ["Pipeline"],
      ),
      Tab(
        {
          selected: computed(vm$.state.tab, (tab) => tab === "schedules"),
          onClick() {
            vm$.methods.switchTab("schedules");
          },
          attributes: { n: "automation-tab-schedules" },
        },
        ["自动化"],
      ),
    ]),
    View({ class: "dm-flex dm-items-center dm-gap-2" }, [
      Button(
        {
          store: vm$.ui.btn_refresh$,
          attributes: { n: "automation-refresh-button", type: "button" },
        },
        ["刷新"],
      ),
      Button(
        {
          store: vm$.ui.btn_create_pipeline$,
          attributes: { n: "automation-create-pipeline", type: "button" },
        },
        ["创建 Pipeline"],
      ),
    ]),
  ]);
}

function AutomationPageSide(props) {}

function AutomationPipelineList(props) {
  const vm$ = props.store;
  return View({ class: "automation-list" }, [
    For({
      each: vm$.state.pipelines,
      key: "id",
      render(pipeline_) {
        const pipeline =
          pipeline_ && pipeline_.value !== undefined
            ? pipeline_.value
            : pipeline_;
        const selected = computed(
          vm$.state.selected_flow_id,
          (flow_id) => flow_id === pipeline.id,
        );
        return View(
          {
            class: computed(
              selected,
              (is_selected) =>
                `automation-card${is_selected ? " is-selected" : ""}`,
            ),
            attributes: {
              n: `automation-pipeline-card-${pipeline.id}`,
              role: "button",
              tabindex: "0",
            },
            onClick() {
              vm$.methods.selectPipeline(pipeline.id);
            },
          },
          [
            View({ class: "automation-card__header" }, [
              View(
                {
                  class: "automation-card__title",
                  attributes: { title: pipeline.name || pipeline.id },
                },
                [pipeline.name || pipeline.id],
              ),
            ]),
            View({ class: "automation-card__subtitle" }, [pipeline.id]),
            View({ class: "automation-card__meta" }, [
              View({}, [`${(pipeline.nodes || []).length} 节点`]),
              View({}, [`${(pipeline.edges || []).length} 连线`]),
            ]),
          ],
        );
      },
    }),
    Show({
      when: computed(
        vm$.state.pipelines,
        (pipelines) => pipelines.length === 0,
      ),
      ok() {
        return View({ class: "automation-empty" }, [
          vm$.state.loading.value
            ? "正在加载 Pipeline..."
            : "暂无 Pipeline，点击「创建 Pipeline」新建",
        ]);
      },
    }),
  ]);
}

function AutomationScheduleList(props) {
  const vm$ = props.store;
  return View({ class: "automation-list" }, [
    For({
      each: vm$.state.schedules,
      key: "id",
      render(schedule_) {
        const schedule =
          schedule_ && schedule_.value !== undefined
            ? schedule_.value
            : schedule_;
        const selected = computed(
          vm$.state.selected_schedule_id,
          (id) => id === schedule.id,
        );
        const metadata = vm$.methods.scheduleMetadata(schedule);
        return View(
          {
            class: computed(
              selected,
              (is_selected) =>
                `automation-card automation-card--schedule${
                  is_selected ? " is-selected" : ""
                }`,
            ),
            attributes: {
              n: `automation-schedule-card-${schedule.id}`,
              role: "button",
              tabindex: "0",
            },
            onClick() {
              vm$.methods.selectSchedule(schedule.id);
            },
          },
          [
            View({ class: "automation-card__header" }, [
              View(
                {
                  class: "automation-card__title",
                  attributes: { title: schedule.name },
                },
                [schedule.name],
              ),
              View({ class: "automation-card__badges" }, [
                automation_trigger_badge({
                  type: metadata.type,
                  label: vm$.methods.triggerLabel(metadata.type),
                }),
                metadata.type === "Cron"
                  ? automation_status_badge(
                      schedule.enabled ? "ENABLED" : "DISABLED",
                    )
                  : null,
              ]),
            ]),
            View({ class: "automation-card__subtitle" }, [
              vm$.methods.pipelineName(schedule.flow_id),
            ]),
            View({ class: "automation-card__meta" }, [
              View({ class: "automation-card__code" }, [
                metadata.type === "Cron"
                  ? schedule.cron_expr
                  : metadata.event_key || metadata.start_node,
              ]),
              View({}, [
                metadata.type === "Cron"
                  ? `下次 ${vm$.methods.formatTime(schedule.next_run_at)}`
                  : `开始 ${metadata.start_node || "-"}`,
              ]),
            ]),
            View(
              {
                class: "automation-card__actions",
                onClick(event) {
                  event.stopPropagation();
                },
              },
              [
                metadata.type === "Cron"
                  ? Button(
                      {
                        store: vm$.ui.btn_schedule_toggle$.bind(schedule),
                        attributes: {
                          n: `automation-schedule-toggle-${schedule.id}`,
                          type: "button",
                        },
                      },
                      [schedule.enabled ? "暂停" : "启用"],
                    )
                  : null,
                Button(
                  {
                    store: vm$.ui.btn_schedule_trigger$.bind(schedule),
                    attributes: {
                      n: `automation-schedule-trigger-${schedule.id}`,
                      type: "button",
                    },
                  },
                  [metadata.type === "Event" ? "触发事件" : "手动执行"],
                ),
              ],
            ),
          ],
        );
      },
    }),
    Show({
      when: computed(
        vm$.state.schedules,
        (schedules) => schedules.length === 0,
      ),
      ok() {
        return View({ class: "automation-empty" }, [
          vm$.state.loading.value ? "正在加载自动化..." : "暂无自动化流程",
        ]);
      },
    }),
  ]);
}

// function AutomationPageMain(props) {
//   const vm$ = props.store;
//   return ;
// }

function AutomationScheduleSummary(props) {
  const vm$ = props.store;
  const schedule_ = vm$.state.selected_schedule;
  return View(
    {
      class: "automation-summary",
      attributes: { n: "automation-schedule-summary" },
    },
    [
      computed(schedule_, (schedule) => {
        if (!schedule) return null;
        const metadata = vm$.methods.scheduleMetadata(schedule);
        const pipeline = (vm$.state.pipelines.value || []).find(
          (item) => item.id === schedule.flow_id,
        );
        const rows = [
          ["流程名称", View({}, [schedule.name])],
          [
            "触发方式",
            automation_trigger_badge({
              type: metadata.type,
              label: vm$.methods.triggerLabel(metadata.type),
            }),
          ],
          [
            "开始节点",
            View({ class: "automation-code" }, [
              vm$.methods.effectiveStartNode(pipeline, schedule),
            ]),
          ],
          [
            "触发配置",
            View({ class: "automation-code" }, [
              metadata.type === "Cron"
                ? schedule.cron_expr
                : metadata.type === "Event"
                  ? metadata.event_key
                  : "manual",
            ]),
          ],
          [
            "下次执行",
            metadata.type === "Cron"
              ? vm$.methods.formatTime(schedule.next_run_at)
              : "由触发动作执行",
          ],
          [
            "状态",
            metadata.type === "Cron"
              ? automation_status_badge(
                  schedule.enabled ? "ENABLED" : "DISABLED",
                )
              : automation_trigger_badge({
                  type: metadata.type,
                  label: vm$.methods.triggerLabel(metadata.type),
                }),
          ],
        ];
        const children = rows.map(([label, value]) =>
          View({ class: "automation-summary__item" }, [
            View({ class: "automation-property__label" }, [label]),
            value,
          ]),
        );
        children.push(
          View({ class: "automation-summary__item is-wide" }, [
            View({ class: "automation-property__label" }, ["初始数据"]),
            View({ class: "automation-code automation-code--wrap" }, [
              schedule.initial_data || "{}",
            ]),
          ]),
        );
        return View({}, children);
      }),
      View({ class: "automation-runs" }, [
        View({ class: "automation-section-title" }, ["执行记录"]),
        For({
          each: vm$.state.runs,
          key: "id",
          render(run_) {
            const run = run_ && run_.value !== undefined ? run_.value : run_;
            return View(
              {
                class: "automation-run",
                attributes: { n: `automation-run-${run.id}` },
              },
              [
                automation_status_badge(run.status),
                View({ class: "automation-run__main" }, [
                  View({ class: "automation-run__trigger" }, [
                    run.trigger_type || "Manual",
                  ]),
                  View({ class: "automation-run__time" }, [
                    vm$.methods.formatTime(run.started_at || run.created_at),
                  ]),
                ]),
                View({ class: "automation-run__node" }, [
                  run.current_node || "-",
                ]),
                View({ class: "automation-run__error" }, [run.error || ""]),
              ],
            );
          },
        }),
        Show({
          when: computed(vm$.state.runs, (runs) => runs.length === 0),
          ok() {
            return View({ class: "automation-empty" }, ["暂无执行记录"]);
          },
        }),
      ]),
    ],
  );
}

function AutomationPipelineDetail(props) {
  const vm$ = props.store;
  return View({ class: "automation-detail" }, [
    computed(vm$.state.selected_pipeline, (flow) => {
      if (!flow) return null;
      return [
        View({ class: "automation-detail__header" }, [
          View({}, [
            View({ class: "automation-detail__title" }, [flow.name || flow.id]),
            View({ class: "automation-detail__subtitle" }, [flow.id]),
          ]),
          View({ class: "dm-flex dm-items-center dm-gap-2 dm-flex-wrap" }, [
            Button(
              {
                store: vm$.ui.btn_schedule_create$,
                attributes: {
                  n: "automation-create-schedule",
                  type: "button",
                },
              },
              ["创建自动化"],
            ),
            Button(
              {
                store: vm$.ui.btn_run_flow$,
                attributes: { n: "automation-run-flow", type: "button" },
              },
              ["立即执行"],
            ),
            Button(
              {
                store: vm$.ui.btn_delete_flow$,
                attributes: { n: "automation-delete-flow", type: "button" },
              },
              ["删除"],
            ),
          ]),
        ]),
        View({ class: "automation-properties" }, [
          View({ class: "automation-summary__item" }, [
            View({ class: "automation-property__label" }, ["上下文 Schema"]),
            View({ class: "automation-code" }, [
              vm$.methods.schemaText(flow.context_schema),
            ]),
          ]),
          View({ class: "automation-summary__item" }, [
            View({ class: "automation-property__label" }, ["开始节点"]),
            View({ class: "automation-code" }, [flow.start_node_id]),
          ]),
          View({ class: "automation-summary__item" }, [
            View({ class: "automation-property__label" }, ["未保存变更"]),
            View({}, [
              computed(vm$.state.dirty, (dirty) => (dirty ? "有" : "无")),
            ]),
          ]),
        ]),
        View({ class: "automation-editor-bar" }, [
          Button(
            {
              store: vm$.ui.btn_add_node$,
              attributes: { n: "automation-add-node", type: "button" },
            },
            ["添加节点"],
          ),
          Button(
            {
              store: vm$.ui.btn_save_flow$,
              attributes: { n: "automation-save-flow", type: "button" },
            },
            ["保存 Pipeline"],
          ),
        ]),
        AutomationFlowGraph({ store: vm$ }),
      ];
    }),
  ]);
}

function automation_edit_layout(nodes) {
  // Layered layout over the working copy: depth from the start node via
  // next_ids, x by layer, y by order within the layer.
  const by_id = {};
  (nodes || []).forEach((node) => {
    by_id[node.id] = node;
  });
  const layers = {};
  function walk(node_id, depth) {
    if (!by_id[node_id]) return;
    if (layers[node_id] !== undefined && layers[node_id] >= depth) {
      return;
    }
    layers[node_id] = depth;
    (by_id[node_id].next_ids || []).forEach((next) => walk(next, depth + 1));
  }
  const start = (nodes || []).length > 0 ? nodes[0].id : "";
  (nodes || []).forEach((node) => {
    if (node.type === "StartNode") walk(node.id, 0);
  });
  if (Object.keys(layers).length === 0 && start) walk(start, 0);
  (nodes || []).forEach((node) => {
    if (layers[node.id] === undefined) layers[node.id] = 0;
  });
  const per_layer = {};
  const positions = {};
  let max_depth = 0;
  (nodes || []).forEach((node) => {
    const depth = layers[node.id] || 0;
    max_depth = Math.max(max_depth, depth);
    per_layer[depth] = per_layer[depth] || [];
    per_layer[depth].push(node.id);
  });
  Object.entries(per_layer).forEach(([depth, ids]) => {
    ids.forEach((node_id, index) => {
      positions[node_id] = {
        left: 40 + Number(depth) * 230,
        top: 40 + index * 130,
      };
    });
  });
  const width = 40 + (max_depth + 1) * 230 + 40;
  const max_bottom = Object.values(positions).reduce(
    (max, position) => Math.max(max, position.top + 110),
    160,
  );
  return { positions, width, height: Math.max(max_bottom + 40, 320) };
}

function automation_edit_edges(nodes) {
  const edges = [];
  (nodes || []).forEach((node) => {
    (node.next_ids || []).forEach((target) => {
      edges.push({ from: node.id, to: target });
    });
  });
  return edges;
}

function automation_edge_path(edge, positions) {
  const from = positions[edge.from];
  const to = positions[edge.to];
  if (!from || !to) return "";
  const x1 = from.left + 90;
  const y1 = from.top + 58;
  const x2 = to.left + 90;
  const y2 = to.top + 6;
  const middle = Math.max(36, Math.abs(y2 - y1) / 2);
  return `M ${x1} ${y1} C ${x1} ${y1 + middle}, ${x2} ${y2 - middle}, ${x2} ${y2}`;
}

function AutomationFlowGraph(props) {
  const vm$ = props.store;
  return View(
    { class: "automation-flow-scroll", attributes: { n: "automation-flow" } },
    [
      View(
        {
          class: "automation-flow-canvas",
          style: computed(vm$.state.edit_nodes, (nodes) => {
            const layout = automation_edit_layout(nodes);
            return `width:${layout.width}px;height:${layout.height}px;`;
          }),
        },
        [
          computed(vm$.state.edit_nodes, (nodes) => {
            const layout = automation_edit_layout(nodes);
            const edges = automation_edit_edges(nodes)
              .map((edge) => ({
                edge,
                d: automation_edge_path(edge, layout.positions),
              }))
              .filter((item) => item.d);
            if (edges.length === 0) return null;
            return SVG.SVG(
              {
                class: "automation-flow-edges",
                attributes: {
                  width: "100%",
                  height: "100%",
                  viewBox: `0 0 ${layout.width} ${layout.height}`,
                  xmlns: "http://www.w3.org/2000/svg",
                  "aria-hidden": "true",
                },
              },
              edges.map(({ d }) =>
                SVG.Path({
                  class: "automation-flow-edge",
                  attributes: { d, "stroke-width": "1.6", fill: "none" },
                }),
              ),
            );
          }),
          For({
            each: vm$.state.edit_nodes,
            key: "id",
            render(node_) {
              const node =
                node_ && node_.value !== undefined ? node_.value : node_;
              const layout = automation_edit_layout(vm$.state.edit_nodes.value);
              const position = layout.positions[node.id] || {
                left: 20,
                top: 20,
              };
              const is_start =
                node.id === vm$.state.edit_start_node_id.value ||
                node.type === "StartNode";
              return View(
                {
                  class: `automation-flow-node${is_start ? " is-start" : ""}`,
                  style: `left:${position.left}px;top:${position.top}px;`,
                  attributes: {
                    n: `automation-flow-node-${node.id}`,
                    title: node.id,
                  },
                },
                [
                  View({ class: "automation-flow-node__header" }, [
                    View({ class: "automation-flow-node__name" }, [
                      node.name || node.id,
                    ]),
                    View({ class: "automation-flow-node__type" }, [
                      vm$.methods.flowLabel(node.type),
                    ]),
                  ]),
                  View({ class: "automation-flow-node__id" }, [node.id]),
                  Show({
                    when: node.config && Object.keys(node.config).length > 0,
                    ok() {
                      return View({ class: "automation-flow-node__schema" }, [
                        JSON.stringify(node.config),
                      ]);
                    },
                  }),
                  Show({
                    when: !is_start,
                    ok() {
                      return View(
                        {
                          class: "automation-flow-node__remove",
                          attributes: {
                            n: `automation-flow-node-remove-${node.id}`,
                            role: "button",
                            tabindex: "0",
                            title: "删除节点",
                          },
                          onClick(event) {
                            event.stopPropagation();
                            vm$.methods.removeNode(node.id);
                          },
                        },
                        ["×"],
                      );
                    },
                  }),
                ],
              );
            },
          }),
        ],
      ),
    ],
  );
}

function AutomationTriggerOption(props) {
  const value = props.value;
  const selected = computed(props.current, (type) => type === value);
  return View(
    {
      class: computed(
        selected,
        (is_selected) =>
          `automation-trigger-option${is_selected ? " is-selected" : ""}`,
      ),
      attributes: {
        n: `${props.namePrefix}-trigger-option-${value.toLowerCase()}`,
        role: "radio",
        tabindex: "0",
      },
      onClick() {
        props.onSelect(value);
      },
    },
    [
      View({ class: "automation-trigger-option__name" }, [props.label]),
      View({ class: "automation-trigger-option__hint" }, [props.hint]),
    ],
  );
}

function AutomationCreatePipelineDialog(props) {
  const vm$ = props.store;
  return Dialog(
    {
      store: vm$.ui.create_dialog$,
      class: "automation-form-dialog",
      attributes: { n: "automation-create-dialog" },
    },
    [
      DialogHeader({}, [
        DialogTitle({}, ["创建 Pipeline"]),
        DialogDescription({}, [
          "选择触发方式并为开始节点声明接收参数；创建后 Pipeline 只包含开始节点，可继续添加后续节点。",
        ]),
      ]),
      DialogBody({}, [
        View({ class: "automation-form" }, [
          View({ class: "automation-form__legend" }, ["1. 触发方式"]),
          View({ class: "automation-trigger-options" }, [
            AutomationTriggerOption({
              store: vm$,
              current: vm$.state.create_trigger_type,
              namePrefix: "create",
              value: "Cron",
              label: "定时触发",
              hint: "按 Cron 计划执行",
              onSelect: vm$.methods.setCreateTriggerType,
            }),
            AutomationTriggerOption({
              store: vm$,
              current: vm$.state.create_trigger_type,
              namePrefix: "create",
              value: "Event",
              label: "事件触发",
              hint: "通过事件 Key 触发",
              onSelect: vm$.methods.setCreateTriggerType,
            }),
            AutomationTriggerOption({
              store: vm$,
              current: vm$.state.create_trigger_type,
              namePrefix: "create",
              value: "Manual",
              label: "手动触发",
              hint: "在页面中手动执行",
              onSelect: vm$.methods.setCreateTriggerType,
            }),
          ]),
          Show({
            when: computed(
              vm$.state.create_trigger_type,
              (type) => type === "Event",
            ),
            ok() {
              return View({ class: "automation-form__field" }, [
                View({ class: "automation-form__label" }, ["事件 Key"]),
                Input({
                  store: vm$.ui.input_create_event_key$,
                  attributes: {
                    n: "automation-create-event-key",
                    autocomplete: "off",
                  },
                }),
              ]);
            },
          }),
          Show({
            when: computed(
              vm$.state.create_trigger_type,
              (type) => type === "Cron",
            ),
            ok() {
              return View({ class: "automation-form__field" }, [
                View({ class: "automation-form__label" }, ["Cron 表达式"]),
                Input({
                  store: vm$.ui.input_create_cron$,
                  attributes: { n: "automation-create-cron" },
                }),
              ]);
            },
          }),
          View({ class: "automation-form__row" }, [
            View({ class: "automation-form__field" }, [
              View({ class: "automation-form__label" }, ["2. Pipeline 名称"]),
              Input({
                store: vm$.ui.input_create_name$,
                attributes: { n: "automation-create-name" },
              }),
            ]),
            View({ class: "automation-form__field" }, [
              View({ class: "automation-form__label" }, ["描述"]),
              Input({
                store: vm$.ui.input_create_description$,
                attributes: { n: "automation-create-description" },
              }),
            ]),
          ]),
          View({ class: "automation-form__legend" }, ["3. 开始节点接收的参数"]),
          For({
            key: "uid",
            each: vm$.state.create_params,
            render(param_) {
              const param =
                param_ && param_.value !== undefined ? param_.value : param_;
              return View({ class: "automation-form__row" }, [
                View({ class: "automation-form__field" }, [
                  View({ class: "automation-form__label" }, ["参数名"]),
                  Input({
                    store: param.input_key$,
                    attributes: {
                      n: `automation-create-param-key-${param.uid}`,
                      placeholder: "例如 url",
                    },
                  }),
                ]),
                View({ class: "automation-form__field" }, [
                  View({ class: "automation-form__label" }, ["类型"]),
                  Select({
                    store: param.select_type$,
                    attributes: {
                      n: `automation-create-param-type-${param.uid}`,
                    },
                  }),
                ]),
                Button(
                  {
                    store: vm$.ui.btn_param_remove$.bind(param),
                    attributes: {
                      n: `automation-create-param-remove-${param.uid}`,
                      type: "button",
                      title: "移除参数",
                    },
                  },
                  ["移除"],
                ),
              ]);
            },
          }),
          Button(
            {
              store: vm$.ui.btn_param_add$,
              attributes: {
                n: "automation-create-param-add",
                type: "button",
              },
            },
            ["+ 添加参数"],
          ),
          Show({
            when: computed(
              vm$.state.create_trigger_type,
              (type) => type === "Cron",
            ),
            ok() {
              return View({ class: "automation-form__field dm-flex" }, [
                Checkbox({
                  store: vm$.ui.checkbox_create_enabled$,
                  attributes: { n: "automation-create-enabled" },
                }),
                View({ class: "automation-form__label" }, [
                  "创建后同时生成定时计划并启用",
                ]),
              ]);
            },
          }),
        ]),
      ]),
      DialogFooter({}, [
        Button(
          {
            store: vm$.ui.btn_create_cancel$,
            attributes: { n: "automation-create-cancel", type: "button" },
          },
          ["取消"],
        ),
        Button(
          {
            store: vm$.ui.btn_create_submit$,
            attributes: { n: "automation-create-submit", type: "button" },
          },
          ["创建 Pipeline"],
        ),
      ]),
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
            when: vm$.state.add_type,
            ok() {
              return View({ class: "automation-form__hint" }, [
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

function AutomationCreateScheduleDialog(props) {
  const vm$ = props.store;
  return Dialog(
    {
      store: vm$.ui.schedule_dialog$,
      class: "automation-form-dialog",
      attributes: { n: "automation-schedule-dialog" },
    },
    [
      DialogHeader({}, [
        DialogTitle({}, ["创建自动化流程"]),
        DialogDescription({}, ["把 Pipeline 接入定时 / 事件 / 手动触发。"]),
      ]),
      DialogBody({}, [
        View({ class: "automation-form" }, [
          View({ class: "automation-form__legend" }, ["触发方式"]),
          View({ class: "automation-trigger-options" }, [
            AutomationTriggerOption({
              store: vm$,
              current: vm$.state.schedule_trigger_type,
              namePrefix: "schedule",
              value: "Cron",
              label: "定时触发",
              hint: "按 Cron 计划执行",
              onSelect: vm$.methods.setScheduleTriggerType,
            }),
            AutomationTriggerOption({
              store: vm$,
              current: vm$.state.schedule_trigger_type,
              namePrefix: "schedule",
              value: "Event",
              label: "事件触发",
              hint: "通过事件 Key 触发",
              onSelect: vm$.methods.setScheduleTriggerType,
            }),
            AutomationTriggerOption({
              store: vm$,
              current: vm$.state.schedule_trigger_type,
              namePrefix: "schedule",
              value: "Manual",
              label: "手动触发",
              hint: "在列表中手动执行",
              onSelect: vm$.methods.setScheduleTriggerType,
            }),
          ]),
          Show({
            when: computed(
              vm$.state.schedule_trigger_type,
              (type) => type === "Event",
            ),
            ok() {
              return View({ class: "automation-form__field" }, [
                View({ class: "automation-form__label" }, ["事件 Key"]),
                Input({
                  store: vm$.ui.input_schedule_event_key$,
                  attributes: { n: "automation-schedule-event-key" },
                }),
              ]);
            },
          }),
          Show({
            when: computed(
              vm$.state.schedule_trigger_type,
              (type) => type === "Cron",
            ),
            ok() {
              return View({ class: "automation-form__field" }, [
                View({ class: "automation-form__label" }, ["Cron 表达式"]),
                Input({
                  store: vm$.ui.input_schedule_cron$,
                  attributes: { n: "automation-schedule-cron" },
                }),
              ]);
            },
          }),
          View({ class: "automation-form__row" }, [
            View({ class: "automation-form__field" }, [
              View({ class: "automation-form__label" }, ["Pipeline"]),
              Select({
                store: vm$.ui.select_schedule_flow$,
                attributes: { n: "automation-schedule-flow" },
              }),
            ]),
            View({ class: "automation-form__field" }, [
              View({ class: "automation-form__label" }, ["开始节点"]),
              Select({
                store: vm$.ui.select_schedule_start$,
                attributes: { n: "automation-schedule-start" },
              }),
            ]),
          ]),
          View({ class: "automation-form__field" }, [
            View({ class: "automation-form__label" }, ["流程名称"]),
            Input({
              store: vm$.ui.input_schedule_name$,
              attributes: { n: "automation-schedule-name" },
            }),
          ]),
        ]),
      ]),
      DialogFooter({}, [
        Button(
          {
            store: vm$.ui.btn_schedule_cancel$,
            attributes: { n: "automation-schedule-cancel", type: "button" },
          },
          ["取消"],
        ),
        Button(
          {
            store: vm$.ui.btn_schedule_submit$,
            attributes: { n: "automation-schedule-submit", type: "button" },
          },
          ["创建流程"],
        ),
      ]),
    ],
  );
}

export default AutomationPageView;
