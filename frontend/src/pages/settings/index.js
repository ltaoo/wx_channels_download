import { fetchAppConfig, updateAppConfig } from "@/biz/request.js";

function FormRender(props, children) {
  const fieldNames = Object.keys(props.store?.fields || {});

  return View({ class: Timeless.classNames([props.class, "space-y-5"]) }, [
    For({
      each: fieldNames,
      render(name) {
        const field$ = props.store.fields?.[name];
        if (!field$) return null;
        const fid = `config-field-${name.replace(/[^a-zA-Z0-9_-]/g, "-")}`;
        const inline = field$.input.shape === "checkbox";
        const description = field$.description || "";

        return View(
          {
            class: Timeless.classNames([
              "gap-2",
              inline ? "flex items-start" : "flex flex-col",
            ]),
          },
          [
            Show({
              when: !inline,
              ok() {
                return FieldLabel({ for: fid, store: field$ });
              },
            }),
            Match({
              when: field$.input.shape,
              cases: {
                select() {
                  return Select({ id: fid, store: field$.input });
                },
                input() {
                  if (field$.inputComponent === "textarea") {
                    return Textarea({ id: fid, store: field$.input });
                  }
                  return Input({ id: fid, store: field$.input });
                },
                "number-input"() {
                  return NumberInput({ id: fid, store: field$.input });
                },
                "file-input"() {
                  return FileInput({ id: fid, store: field$.input });
                },
                checkbox() {
                  return Checkbox({ id: fid, store: field$.input });
                },
              },
            }),
            Show({
              when: inline,
              ok() {
                return View({ class: "min-w-0 space-y-1 pt-0.5" }, [
                  FieldInlineLabel({ for: fid, store: field$ }),
                  description
                    ? View(
                        {
                          class:
                            "text-xs leading-5 text-zinc-500 dark:text-zinc-400",
                        },
                        [description],
                      )
                    : null,
                ]);
              },
            }),
            Show({
              when: !inline && !!description,
              ok() {
                return View(
                  {
                    class:
                      "text-xs leading-5 text-zinc-500 dark:text-zinc-400",
                  },
                  [description],
                );
              },
            }),
          ],
        );
      },
    }),
    Fragment({}, children),
  ]);
}

const GROUP_ORDER = [
  "Proxy",
  "Download",
  "Channels",
  "API",
  "Database",
  "OfficialAccount",
  "FileHelper",
  "Pagespy",
  "Debug",
  "Inject",
  "Cloudflare",
  "Update",
];

function normalizeValue(item, values) {
  const value = values?.[item.key] ?? item.default ?? "";
  if (value === null || value === undefined) return "";
  return value;
}

function displayFilename(value) {
  const text = String(value || "").trim();
  if (!text) return "";
  const normalized = text.replace(/\\/g, "/");
  return normalized.split("/").filter(Boolean).pop() || text;
}

function filePickerValue(value) {
  const text = String(value || "").trim();
  if (!text) return null;
  return [
    {
      name: displayFilename(text),
      path: text,
      kind: "file",
      type: "",
    },
  ];
}

function createInput(item, value) {
  const disabled = !!item.readonly;

  if (item.type === "boolean" || typeof item.default === "boolean") {
    return new Timeless.ui.CheckboxCore({
      defaultValue: !!value,
      disabled,
    });
  }

  if (item.type === "select" && Array.isArray(item.options) && item.options.length) {
    return new Timeless.ui.SelectCore({
      defaultValue: String(value ?? ""),
      disabled,
      options: item.options.map(
        (option) =>
          new Timeless.ui.SelectItemCore({
            label: option,
            value: option,
          }),
      ),
    });
  }

  if (item.type === "file") {
    return new Timeless.ui.FilePickerCore({
      defaultValue: filePickerValue(value),
      accept: item.accept || undefined,
      disabled,
    });
  }

  if (item.type === "number") {
    const numberValue =
      value === "" || value === null || value === undefined ? null : Number(value);
    return new Timeless.ui.NumberInputCore({
      defaultValue: Number.isNaN(numberValue) ? null : numberValue,
      placeholder: item.key,
      disabled,
      step: 1,
      precision: 0,
    });
  }

  if (item.type === "textarea") {
    return new Timeless.ui.InputCore({
      defaultValue: String(value ?? ""),
      disabled,
      placeholder: item.key,
    });
  }

  return new Timeless.ui.InputCore({
    defaultValue: String(value ?? ""),
    disabled,
    placeholder: item.key,
  });
}

function createField(item, values) {
  const value = normalizeValue(item, values);
  const field = new Timeless.ui.SingleFieldCore({
    name: item.key,
    label: item.title || item.key,
    input: createInput(item, value),
  });
  field.description = item.description || "";
  field.configItem = item;
  if (item.type === "textarea") {
    field.inputComponent = "textarea";
  }
  return field;
}

function groupSchema(schema) {
  const groups = new Map();
  for (const item of schema || []) {
    const group = item.group || "Other";
    if (!groups.has(group)) groups.set(group, []);
    groups.get(group).push(item);
  }

  return Array.from(groups.entries())
    .map(([name, items]) => ({ name, items }))
    .sort((a, b) => {
      const ai = GROUP_ORDER.indexOf(a.name);
      const bi = GROUP_ORDER.indexOf(b.name);
      if (ai === -1 && bi === -1) return a.name.localeCompare(b.name);
      if (ai === -1) return 1;
      if (bi === -1) return -1;
      return ai - bi;
    });
}

function buildForms(schema, values) {
  return groupSchema(schema).map((group) => {
    const fields = {};
    for (const item of group.items) {
      fields[item.key] = createField(item, values);
    }
    return {
      name: group.name,
      count: group.items.length,
      form: new Timeless.ui.ObjectFieldCore({ fields }),
    };
  });
}

function normalizeSubmitValue(field, value) {
  if (field?.configItem?.type !== "file") return value;
  if (!value) return "";
  const file = value[0];
  if (!file) return "";
  return file.path || file.webkitRelativePath || file.name || "";
}

function SettingsPageModel(props) {
  const reqs = {
    fetch: new Timeless.RequestCore(fetchAppConfig, {
      client: props.client,
    }),
    save: new Timeless.RequestCore(updateAppConfig, {
      client: props.client,
    }),
  };

  const loading_ = ref(false);
  const saving_ = ref(false);
  const error_ = ref("");
  const path_ = ref("");
  const forms_ = refarr([]);

  const saveBtn$ = new Timeless.ui.ButtonCore({
    async onClick() {
      await save();
    },
  });

  async function load() {
    loading_.as(true);
    error_.as("");
    const r = await reqs.fetch.run();
    loading_.as(false);
    if (r.error) {
      error_.as(r.error.message || String(r.error));
      return;
    }
    path_.as(r.data.path || "");
    forms_.as(buildForms(r.data.schema || [], r.data.values || {}));
  }

  async function save() {
    saving_.as(true);
    saveBtn$.setLoading(true);
    error_.as("");
    const values = {};

    for (const group of forms_.value) {
      const r = await group.form.validate();
      if (r.error) {
        const msg = r.error.message || r.error.messages?.join("，") || String(r.error);
        error_.as(msg);
        props.app.tip?.({ type: "error", text: [msg] });
        saving_.as(false);
        saveBtn$.setLoading(false);
        return;
      }
      for (const key of Object.keys(r.data)) {
        values[key] = normalizeSubmitValue(group.form.fields[key], r.data[key]);
      }
    }

    const r = await reqs.save.run({ values });
    saving_.as(false);
    saveBtn$.setLoading(false);
    if (r.error) {
      error_.as(r.error.message || String(r.error));
      props.app.tip?.({
        type: "error",
        text: [r.error.message || String(r.error)],
      });
      return;
    }

    path_.as(r.data.path || path_.value);
    props.app.tip?.({ type: "success", text: ["配置已保存"] });
  }

  return {
    state: {
      loading: loading_,
      saving: saving_,
      error: error_,
      path: path_,
      forms: forms_,
    },
    ui: {
      view: new Timeless.ui.ScrollViewCore({}),
      saveBtn: saveBtn$,
    },
    methods: {
      init: load,
      refresh: load,
      save,
    },
  };
}

function SettingsGroup(group) {
  return View(
    {
      class:
        "rounded-lg border border-zinc-200 bg-white p-5 shadow-sm dark:border-zinc-800 dark:bg-zinc-950",
    },
    [
      View(
        {
          class:
            "mb-5 flex flex-wrap items-center justify-between gap-3 border-b border-zinc-100 pb-4 dark:border-zinc-900",
        },
        [
          View({}, [
            View(
              {
                class:
                  "text-base font-semibold text-zinc-950 dark:text-zinc-50",
              },
              [group.name],
            ),
            View({ class: "mt-1 text-xs text-zinc-500 dark:text-zinc-400" }, [
              `${group.count} 项配置`,
            ]),
          ]),
        ],
      ),
      FormRender({ store: group.form }, []),
    ],
  );
}

export default function SettingsPageView(props) {
  const vm$ = SettingsPageModel(props);

  return View(
    {
      class: "flex h-full flex-col bg-zinc-50 dark:bg-zinc-900",
      onMounted() {
        vm$.methods.init();
      },
    },
    [
      View(
        {
          class:
            "border-b border-zinc-200 bg-white px-6 py-5 dark:border-zinc-800 dark:bg-zinc-950",
        },
        [
          View({ class: "flex flex-wrap items-center justify-between gap-3" }, [
            View({ class: "min-w-0" }, [
              View(
                {
                  class:
                    "text-2xl font-semibold text-zinc-950 dark:text-zinc-50",
                },
                ["设置"],
              ),
              View(
                {
                  class:
                    "mt-1 truncate text-sm text-zinc-500 dark:text-zinc-400",
                  title: vm$.state.path,
                },
                [computed(vm$.state.path, (path) => path || "config.yaml")],
              ),
            ]),
            View({ class: "flex items-center gap-2" }, [
              Button(
                {
                  store: new Timeless.ui.ButtonCore({
                    variant: "outline",
                    disabled: vm$.state.loading,
                    onClick() {
                      vm$.methods.refresh();
                    },
                  }),
                  prefix: [Icon({ name: "refresh-cw", size: 16 })],
                },
                ["刷新"],
              ),
              Button(
                {
                  store: vm$.ui.saveBtn,
                  disabled: vm$.state.loading,
                  prefix: [Icon({ name: "save", size: 16 })],
                },
                ["保存"],
              ),
            ]),
          ]),
        ],
      ),
      ScrollView({ store: vm$.ui.view, class: "flex-1" }, [
        View({ class: "p-6" }, [
          Show({
            when: vm$.state.error,
            ok() {
              return View(
                {
                  class:
                    "mb-4 rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-700 dark:border-red-900 dark:bg-red-950 dark:text-red-300",
                },
                [vm$.state.error],
              );
            },
          }),
          Show({
            when: vm$.state.loading,
            ok() {
              return View(
                {
                  class:
                    "flex h-56 flex-col items-center justify-center gap-3 text-zinc-500",
                },
                [Icon({ name: "loader", size: 28 }), "加载中..."],
              );
            },
            else() {
              return View({ class: "grid gap-5 xl:grid-cols-2" }, [
                For({
                  each: vm$.state.forms,
                  render(group) {
                    return SettingsGroup(group);
                  },
                }),
              ]);
            },
          }),
        ]),
      ]),
    ],
  );
}
