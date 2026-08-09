/// <reference path="model.js" />
/**
 * @file Download manager UI views
 */
function map_download_task_icon_name(filename) {
  if (!filename) return "file";
  const ext = String(filename).split(".").pop().toLowerCase();
  if (ext === "mp3") return "file-volume";
  if (ext === "mp4") return "file-play";
  if (["jpg", "jpeg", "png", "gif", "webp"].includes(ext)) {
    return "file-image";
  }
  return "file";
}
function format_download_progress_text(percent) {
  const p = Number(percent);
  if (!Number.isFinite(p)) return "0";
  return String(Math.round(Math.max(0, Math.min(100, p))));
}
function download_task_preview_url(task) {
  const id = task && task.id;
  if (id === undefined || id === null || id === "") return "";
  const origin = DownloaderOrigin;
  const base = String(origin || "").replace(/\/$/, "");
  return `${base}/preview?id=${encodeURIComponent(id)}`;
}
function open_download_task_preview(task) {
  const url = download_task_preview_url(task);
  if (!url) {
    WXU.error({
      msg: "task id is empty",
      source: "view.js:33",
    });
    return;
  }
  window.open(url, "_blank", "noopener");
}
function NumberView(props = {}) {
  const value = Object.prototype.hasOwnProperty.call(props, "value")
    ? props.value
    : props.number;
  const characterWidth = props.characterWidth || "0.65em";
  const decimalWidth = props.decimalWidth || "0.3em";
  const toCharacters = (currentValue) => {
    return Array.from(currentValue == null ? "" : String(currentValue)).map(
      (character, index) => ({
        key: `${index}:${character}`,
        character,
      }),
    );
  };
  const characters_ =
    value && value.__is_ref
      ? computed(value, toCharacters)
      : toCharacters(value);
  return View(
    {
      class: ["wx-number-view", props.class].filter(Boolean).join(" "),
      style: {
        display: "inline-flex",
        "align-items": "center",
        "white-space": "nowrap",
        ...(props.style || {}),
      },
      attributes: props.attributes || {},
    },
    [
      For({
        key: "key",
        each: characters_,
        render(item) {
          const width = item.character === "." ? decimalWidth : characterWidth;
          return View(
            {
              class: "wx-number-view-character",
              style: {
                display: "inline-flex",
                width,
                "min-width": width,
                "flex-basis": width,
                "flex-shrink": "0",
                "align-items": "center",
                "justify-content": "center",
                "text-align": "center",
                ...(props.characterStyle || {}),
              },
            },
            [item.character],
          );
        },
      }),
    ],
  );
}
function DownloadInfinityIcon(props = {}) {
  const size = props.size || 14;
  return SVG.SVG(
    {
      style: {
        display: "block",
        "flex-shrink": "0",
      },
      attributes: {
        width: String(size),
        height: String(size),
        xmlns: "http://www.w3.org/2000/svg",
        viewBox: "0 0 24 24",
        fill: "none",
        stroke: "currentColor",
        "stroke-width": "2",
        "stroke-linecap": "round",
        "stroke-linejoin": "round",
        class: "lucide lucide-infinity-icon lucide-infinity",
        "aria-hidden": "true",
      },
    },
    [
      SVG.Path({
        attributes: {
          d: "M6 16c5 0 7-8 12-8a4 4 0 0 1 0 8c-5 0-7-8-12-8a4 4 0 1 0 0 8",
        },
      }),
    ],
  );
}
function is_live_stream_download_task(task) {
  if (!task || !Array.isArray(task.files)) return false;
  return task.files.some((file) => {
    if (!file) return false;
    return [file.type, file.resource_type].some(
      (type) => String(type || "").toUpperCase() === "STREAM",
    );
  });
}
function DownloadTaskFileIcon(props) {
  const size = props.size || 32;
  const iconName_ = computed(props.task, (task) => {
    return map_download_task_icon_name(task.name);
  });
  return Match({
    when: iconName_,
    cases: {
      "file-volume"() {
        return Timeless.Icon({ name: "file-volume", size });
      },
      "file-play"() {
        return Timeless.Icon({ name: "file-play", size });
      },
      "file-image"() {
        return Timeless.Icon({ name: "file-image", size });
      },
      file() {
        return Timeless.Icon({ name: "file", size });
      },
    },
  });
}
function Skeleton(props = {}) {
  return View({
    class: ["wx-skeleton", props.class].filter(Boolean).join(" "),
    style: props.style || {},
  });
}
function DownloadTaskSkeletonCard(props = {}) {
  return View(
    {
      class: ["weui-cell wx-dl-item wx-dl-item-skeleton", props.class]
        .filter(Boolean)
        .join(" "),
      style: {
        "box-sizing": "border-box",
      },
    },
    [
      View(
        {
          class: "weui-cell__hd",
          style: {
            "margin-right": "16px",
            width: "50px",
            height: "50px",
            display: "flex",
            "align-items": "center",
            "justify-content": "center",
          },
        },
        [
          Skeleton({
            class: "wx-dl-skeleton-icon",
            style: {
              width: "50px",
              height: "50px",
              "border-radius": "12px",
            },
          }),
        ],
      ),
      View(
        {
          class: "weui-cell__bd",
          style: { "min-width": "0" },
        },
        [
          Skeleton({
            class: "wx-dl-skeleton-line",
            style: {
              width: "74%",
              height: "14px",
              "border-radius": "4px",
            },
          }),
          Skeleton({
            class: "wx-dl-skeleton-line",
            style: {
              width: "52%",
              height: "14px",
              "border-radius": "4px",
              "margin-top": "6px",
            },
          }),
          Skeleton({
            class: "wx-dl-skeleton-line",
            style: {
              width: "34%",
              height: "12px",
              "border-radius": "4px",
              "margin-top": "10px",
            },
          }),
        ],
      ),
      View(
        {
          class: "weui-cell__ft",
          style: {
            display: "flex",
            "align-items": "center",
            gap: "12px",
            "margin-left": "12px",
          },
        },
        [
          Skeleton({
            style: {
              width: "20px",
              height: "20px",
              "border-radius": "50%",
            },
          }),
          Skeleton({
            style: {
              width: "20px",
              height: "20px",
              "border-radius": "50%",
            },
          }),
        ],
      ),
    ],
  );
}

function CreateTaskDialogView(props) {
  const title = "创建下载任务";
  const urlPlaceholder = "请输入下载地址，例如 https://example.com/file.mp4";
  const vm$ = props.store;
  const inputStyle = {
    width: "100%",
    "box-sizing": "border-box",
    padding: "10px 12px",
    "font-size": "14px",
    "line-height": "20px",
    border: "1px solid var(--weui-FG-3)",
    "border-radius": "6px",
    background: "var(--weui-BG-1)",
    color: "var(--weui-FG-0)",
    outline: "none",
  };

  return Timeless.weui.Dialog(
    {
      store: vm$.ui.createTaskDialog$,
      style: { "z-index": "10000" },
    },
    [
      View({ style: { padding: "20px 20px 16px" } }, [
        View(
          {
            style: {
              "font-size": "17px",
              "font-weight": "600",
              "line-height": "24px",
              "margin-bottom": "12px",
            },
          },
          [title],
        ),
        Input({
          placeholder: urlPlaceholder,
          style: { ...inputStyle, "margin-bottom": "12px" },
          onInput(e) {
            vm$.methods.setCreateTaskText(e && e.target ? e.target.value : "");
          },
        }),
        View(
          {
            style: {
              "font-size": "13px",
              color: "var(--weui-FG-1)",
              "margin-bottom": "6px",
            },
          },
          ["文件名"],
        ),
        Input({
          placeholder: "自动识别，可手动修改",
          value: computed(vm$.state.create_task_filename, (f) => f),
          style: inputStyle,
          onInput(e) {
            vm$.state.create_task_filename.as(
              e && e.target ? e.target.value : "",
            );
          },
        }),
      ]),
    ],
  );
}

function CreatePlatformTaskDialogView(props) {
  const title = "平台创建下载任务";
  const vm$ = props.store;
  const inputStyle = {
    width: "100%",
    "box-sizing": "border-box",
    padding: "10px 12px",
    "font-size": "14px",
    "line-height": "20px",
    border: "1px solid var(--weui-FG-3)",
    "border-radius": "6px",
    background: "var(--weui-BG-1)",
    color: "var(--weui-FG-0)",
    outline: "none",
    "margin-bottom": "12px",
  };
  const labelStyle = {
    "font-size": "13px",
    color: "var(--weui-FG-1)",
    "margin-bottom": "6px",
  };

  return Timeless.weui.Dialog(
    {
      store: vm$.ui.createPlatformTaskDialog$,
      style: { "z-index": "10000" },
    },
    [
      View({ style: { padding: "20px 20px 16px" } }, [
        View(
          {
            style: {
              "font-size": "17px",
              "font-weight": "600",
              "line-height": "24px",
              "margin-bottom": "12px",
            },
          },
          [title],
        ),
        View({ style: labelStyle }, ["平台名称"]),
        Input({
          placeholder: "如 wx_channels、bilibili",
          style: inputStyle,
          onInput(e) {
            vm$.state.create_platform_text.as(
              e && e.target ? e.target.value : "",
            );
          },
        }),
        View({ style: labelStyle }, ["内容 JSON"]),
        Input({
          type: "textarea",
          placeholder: '平台内容原始 JSON，如 {"feed_id":"xxx"}',
          style: { ...inputStyle, "min-height": "80px", resize: "vertical" },
          onInput(e) {
            vm$.state.create_platform_json.as(
              e && e.target ? e.target.value : "",
            );
          },
        }),
        View({ style: labelStyle }, ["保存路径（可选）"]),
        Input({
          placeholder: "留空则使用默认下载目录",
          style: inputStyle,
          onInput(e) {
            vm$.state.create_platform_save_path.as(
              e && e.target ? e.target.value : "",
            );
          },
        }),
        View({ style: labelStyle }, ["文件名（可选）"]),
        Input({
          placeholder: "留空则自动命名",
          style: inputStyle,
          onInput(e) {
            vm$.state.create_platform_filename.as(
              e && e.target ? e.target.value : "",
            );
          },
        }),
        View(
          {
            style: {
              display: "flex",
              "align-items": "center",
              gap: "8px",
              "font-size": "13px",
              color: "var(--weui-FG-0)",
            },
          },
          [
            DownloadTaskSelectionCheckbox({
              checked: vm$.state.create_platform_download_cover,
              ariaLabel: "同时下载封面",
              onToggle() {
                vm$.state.create_platform_download_cover.toggle();
              },
            }),
            View({}, ["同时下载封面"]),
          ],
        ),
      ]),
    ],
  );
}

function resourceFileEmoji(name) {
  var ext = String(name || "")
    .split(".")
    .pop()
    .toLowerCase();
  if (/^(jpe?g|png|gif|webp|svg|bmp|ico)$/.test(ext)) return "🖼️";
  if (/^(mp4|avi|mkv|mov|webm|flv|wmv|m4v)$/.test(ext)) return "🎬";
  if (/^(mp3|wav|aac|flac|ogg|wma|m4a)$/.test(ext)) return "🎵";
  if (/^(html?|css|js|json|xml)$/.test(ext)) return "🌐";
  return "📄";
}

function buildPreviewTree(preview) {
  if (preview && preview.tree) {
    return preview.tree;
  }
  var resources = (preview && preview.resources) || [];
  var root = { type: "directory", name: "", children: [] };
  for (var i = 0; i < resources.length; i++) {
    var res = resources[i];
    var name = res.name || "";
    var parts = name.split("/").filter(Boolean);
    if (parts.length === 0) {
      root.children.push({
        type: "file",
        name: name,
        kind: res.kind,
        endpoints: res.endpoints,
      });
      continue;
    }
    var node = root;
    for (var j = 0; j < parts.length; j++) {
      var part = parts[j];
      if (j === parts.length - 1) {
        node.children.push({
          type: "file",
          name: part,
          kind: res.kind,
          endpoints: res.endpoints,
        });
      } else {
        var dir = null;
        for (var k = 0; k < node.children.length; k++) {
          if (
            node.children[k].type === "directory" &&
            node.children[k].name === part
          ) {
            dir = node.children[k];
            break;
          }
        }
        if (!dir) {
          dir = { type: "directory", name: part, children: [] };
          node.children.push(dir);
        }
        node = dir;
      }
    }
  }
  return root;
}

function PreviewResourceNode(props) {
  const { node, level } = props;
  var indent = Math.min(level * 18, 90) + "px";
  return Show({
    when: node.type === "directory",
    ok() {
      return View(
        {
          key: node.name + "-dir-" + level,
          style: { "margin-left": indent, "margin-bottom": "2px" },
        },
        [
          View(
            {
              style: {
                display: "flex",
                "align-items": "center",
                gap: "4px",
                "font-size": "13px",
                color: "var(--weui-FG-1)",
                "font-weight": "500",
                cursor: "default",
                "border-radius": "4px",
                padding: "3px 6px",
              },
            },
            [
              View(
                {
                  style: {
                    "flex-shrink": "0",
                    width: "16px",
                    "text-align": "center",
                  },
                },
                ["📁"],
              ),
              View(
                {
                  style: {
                    flex: "1",
                    overflow: "hidden",
                    "text-overflow": "ellipsis",
                    "white-space": "nowrap",
                    "text-align": "left",
                  },
                },
                [node.name || "根目录"],
              ),
            ],
          ),
          View({ style: { "margin-left": "8px" } }, [
            For({
              each: node.children,
              render(child) {
                return PreviewResourceNode({ node: child, level: level + 1 });
              },
            }),
          ]),
        ],
      );
    },
    else() {
      return View(
        {
          key: node.name + "-file-" + level,
          style: { "margin-left": indent, "margin-bottom": "2px" },
        },
        [
          View(
            {
              style: {
                display: "flex",
                "align-items": "center",
                gap: "6px",
                "font-size": "13px",
                color: "var(--weui-FG-0)",
                padding: "3px 6px",
                "border-radius": "4px",
              },
            },
            [
              View(
                {
                  style: {
                    "flex-shrink": "0",
                    width: "16px",
                    "text-align": "center",
                  },
                },
                [resourceFileEmoji(node.name)],
              ),
              View(
                {
                  style: {
                    flex: "1",
                    overflow: "hidden",
                    "text-overflow": "ellipsis",
                    "white-space": "nowrap",
                    "text-align": "left",
                  },
                },
                [node.name || "文件"],
              ),
            ],
          ),
        ],
      );
    },
  });
}

function PreviewResourceTree(props) {
  const { preview } = props;
  return View({ style: { "padding-top": "4px" } }, [
    For({
      each: computed(preview, (t) => {
        var tree = buildPreviewTree(t);
        var children = tree && tree.children ? tree.children : [];
        WXU.log
          .Info()
          .Int("task_id", t.task_id || 0)
          .Int("children_count", children.length)
          .Msg("build tree children");
        return children;
      }),
      render(node) {
        return PreviewResourceNode({ node, level: 0 });
      },
    }),
  ]);
}

function CreateTaskPreviewDialogView(props) {
  const title = "下载任务预览";
  const vm$ = props.store;
  const preview_ = vm$.state.create_task_preview;
  const labelStyle = {
    "font-size": "13px",
    color: "var(--weui-FG-1)",
    "margin-bottom": "4px",
  };
  const valueStyle = {
    "font-size": "14px",
    color: "var(--weui-FG-0)",
    "line-height": "20px",
    "word-break": "break-all",
  };
  const sectionStyle = { "margin-bottom": "12px" };
  return Timeless.weui.Dialog(
    {
      store: vm$.ui.createTaskPreviewDialog$,
      style: { "z-index": "10000", width: "560px" },
    },
    [
      View(
        {
          style: {
            padding: "20px 20px 16px",
            "max-height": "60vh",
            overflow: "auto",
          },
        },
        [
          View(
            {
              style: {
                "font-size": "17px",
                "font-weight": "600",
                "line-height": "24px",
                "margin-bottom": "16px",
              },
            },
            [title],
          ),
          Show({
            when: preview_,
            ok() {
              return [
                View({ style: sectionStyle }, [
                  View({ style: labelStyle }, ["任务名称"]),
                  View({ style: valueStyle }, [
                    computed(preview_, function (p) {
                      return p && (p.task_name || "-");
                    }),
                  ]),
                ]),
                View({ style: sectionStyle }, [
                  View({ style: labelStyle }, ["协议"]),
                  View({ style: valueStyle }, [
                    computed(preview_, function (p) {
                      return p && (p.protocol || "-");
                    }),
                  ]),
                ]),
                View({ style: sectionStyle }, [
                  View({ style: labelStyle }, ["资源类型"]),
                  View({ style: valueStyle }, [
                    computed(preview_, function (p) {
                      return p && (p.resource_type || "-");
                    }),
                  ]),
                ]),
                View({ style: sectionStyle }, [
                  View({ style: labelStyle }, ["保存路径"]),
                  View({ style: valueStyle }, [
                    computed(preview_, function (p) {
                      return p && (p.save_path || "-");
                    }),
                  ]),
                ]),
                Show({
                  when: computed(preview_, function (p) {
                    return p && p.resources && p.resources.length > 0;
                  }),
                  ok() {
                    return View({ style: sectionStyle }, [
                      View(
                        { style: { ...labelStyle, "margin-bottom": "8px" } },
                        [
                          "资源列表（共 ",
                          computed(preview_, function (p) {
                            return p && p.resources ? p.resources.length : 0;
                          }),
                          " 项）",
                        ],
                      ),
                      PreviewResourceTree({ preview: preview_ }),
                    ]);
                  },
                }),
              ];
            },
          }),
        ],
      ),
    ],
  );
}

function CreatePlatformTaskPreviewDialogView(props) {
  const title = "平台任务预览";
  const vm$ = props.store;
  const preview_ = vm$.state.create_platform_preview;
  const labelStyle = {
    "font-size": "13px",
    color: "var(--weui-FG-1)",
    "margin-bottom": "4px",
  };
  const valueStyle = {
    "font-size": "14px",
    color: "var(--weui-FG-0)",
    "line-height": "20px",
    "word-break": "break-all",
  };
  const sectionStyle = { "margin-bottom": "12px" };
  return Timeless.weui.Dialog(
    {
      store: vm$.ui.createPlatformTaskPreviewDialog$,
      style: { "z-index": "10000", width: "560px" },
    },
    [
      View(
        {
          style: {
            padding: "20px 20px 16px",
            "max-height": "60vh",
            overflow: "auto",
          },
        },
        [
          View(
            {
              style: {
                "font-size": "17px",
                "font-weight": "600",
                "line-height": "24px",
                "margin-bottom": "16px",
              },
            },
            [title],
          ),
          Show({
            when: preview_,
            ok() {
              return [
                View({ style: sectionStyle }, [
                  View({ style: labelStyle }, ["平台"]),
                  View({ style: valueStyle }, [
                    computed(preview_, function (p) {
                      return p && (p.platform || "-");
                    }),
                  ]),
                ]),
                View({ style: sectionStyle }, [
                  View({ style: labelStyle }, ["任务名称"]),
                  View({ style: valueStyle }, [
                    computed(preview_, function (p) {
                      return p && (p.task_name || "-");
                    }),
                  ]),
                ]),
                View({ style: sectionStyle }, [
                  View({ style: labelStyle }, ["资源类型"]),
                  View({ style: valueStyle }, [
                    computed(preview_, function (p) {
                      return p && (p.resource_type || "-");
                    }),
                  ]),
                ]),
                View({ style: sectionStyle }, [
                  View({ style: labelStyle }, ["保存路径"]),
                  View({ style: valueStyle }, [
                    computed(preview_, function (p) {
                      return p && (p.save_path || "-");
                    }),
                  ]),
                ]),
                View(
                  {
                    style: {
                      display: "flex",
                      gap: "24px",
                      "margin-bottom": "12px",
                    },
                  },
                  [
                    View({ style: sectionStyle }, [
                      View({ style: labelStyle }, ["资源数量"]),
                      View({ style: valueStyle }, [
                        computed(preview_, function (p) {
                          return String(
                            p && p.resource_count != null
                              ? p.resource_count
                              : "-",
                          );
                        }),
                      ]),
                    ]),
                    View({ style: sectionStyle }, [
                      View({ style: labelStyle }, ["端点数量"]),
                      View({ style: valueStyle }, [
                        computed(preview_, function (p) {
                          return String(
                            p && p.endpoint_count != null
                              ? p.endpoint_count
                              : "-",
                          );
                        }),
                      ]),
                    ]),
                  ],
                ),
                Show({
                  when: computed(preview_, function (p) {
                    return p && p.resources && p.resources.length > 0;
                  }),
                  ok() {
                    return View({ style: sectionStyle }, [
                      View(
                        { style: { ...labelStyle, "margin-bottom": "8px" } },
                        [
                          "资源列表（共 ",
                          computed(preview_, function (p) {
                            return p && p.resources ? p.resources.length : 0;
                          }),
                          " 项）",
                        ],
                      ),
                      PreviewResourceTree({ preview: preview_ }),
                    ]);
                  },
                }),
              ];
            },
          }),
        ],
      ),
    ],
  );
}

function ClearTasksConfirmDialog(props) {
  const checkboxStyle = computed(
    props.store.state.delete_delete_files,
    (checked) => {
      return {
        width: "18px",
        height: "18px",
        "box-sizing": "border-box",
        "border-radius": "4px",
        border: "1px solid " + (checked ? "#07C160" : "var(--weui-FG-3)"),
        background: checked ? "#07C160" : "transparent",
        color: "#fff",
        display: "inline-flex",
        "align-items": "center",
        "justify-content": "center",
        flex: "0 0 auto",
      };
    },
  );

  return Timeless.weui.Dialog(
    {
      store: props.store.ui.clearConfirmDialog$,
      style: {
        "z-index": "10000",
      },
    },
    [
      View({ style: { padding: "20px 20px 16px" } }, [
        View(
          {
            style: {
              "font-size": "17px",
              "font-weight": "600",
              "line-height": "24px",
              "margin-bottom": "8px",
            },
          },
          ["删除下载记录"],
        ),
        View(
          {
            style: {
              "font-size": "14px",
              "line-height": "20px",
              color: "var(--weui-FG-1)",
              "margin-bottom": "16px",
            },
          },
          ["确认删除下载记录？"],
        ),
        View(
          {
            role: "checkbox",
            tabIndex: "0",
            attributes: {
              "aria-checked": computed(
                props.store.state.delete_delete_files,
                (checked) => (checked ? "true" : "false"),
              ),
            },
            style: {
              display: "flex",
              "align-items": "center",
              gap: "10px",
              padding: "10px 0",
              cursor: "pointer",
              "user-select": "none",
              "font-size": "14px",
              "line-height": "20px",
            },
            onClick() {
              // if (loading_.value) {
              //   return;
              // }
              // deleteFiles_.as((prev) => !prev);
              props.store.methods.handleClickCheckboxConfirmDeleteFiles();
            },
            // onKeyDown(e) {
            //   if (loading_.value) {
            //     return;
            //   }
            //   if (e.key === " " || e.key === "Enter") {
            //     e.preventDefault();
            //     deleteFiles_.as((prev) => !prev);
            //   }
            // },
          },
          [
            View({ style: checkboxStyle }, [
              Show({
                when: props.store.state.delete_delete_files,
                ok() {
                  return Timeless.Icon({ name: "check", size: 14 });
                },
              }),
            ]),
            View({}, ["同时删除已下载的文件"]),
          ],
        ),
      ]),
    ],
  );
}

function OverwriteDownloadConfirmDialog(props) {
  var selectedAction_ = props.store.state.overwrite;

  function select(actionValue) {
    WXU.log.Info().Str("action", actionValue).Msg("select overwrite type");
    selectedAction_.as({ value: actionValue });
  }

  return Timeless.weui.Dialog(
    {
      store: props.store.ui.overwriteConfirmDialog$,
      style: {
        "z-index": "10000",
      },
    },
    [
      View(
        {
          style: {
            "max-width": "calc(100vw - 32px)",
            "padding-top": "20px",
            "text-align": "center",
          },
        },
        [
          View(
            {
              style: {
                "font-size": "17px",
                "font-weight": "600",
                "line-height": "24px",
                "margin-bottom": "8px",
              },
            },
            ["文件已存在"],
          ),
          View(
            {
              style: {
                "font-size": "14px",
                "line-height": "20px",
                color: "var(--weui-FG-1)",
                "margin-bottom": "16px",
              },
            },
            ["已存在该下载内容，请选择操作"],
          ),
          View(
            {
              role: "radio",
              tabIndex: "0",
              attributes: {
                "aria-checked": computed(selectedAction_, function (s) {
                  return s.value === "overwrite" ? "true" : "false";
                }),
              },
              style: {
                display: "flex",
                "align-items": "center",
                gap: "10px",
                cursor: "pointer",
                "user-select": "none",
                "font-size": "14px",
                "line-height": "20px",
              },
              onClick: function () {
                select("overwrite");
              },
            },
            [
              View(
                {
                  style: computed(selectedAction_, function (selected) {
                    var checked = selected.value === "overwrite";
                    return {
                      width: "18px",
                      height: "18px",
                      "box-sizing": "border-box",
                      "border-radius": "50%",
                      border:
                        "2px solid " +
                        (checked ? "#07C160" : "var(--weui-FG-3)"),
                      background: checked ? "#07C160" : "transparent",
                      display: "inline-flex",
                      "align-items": "center",
                      "justify-content": "center",
                      "flex-shrink": "0",
                    };
                  }),
                },
                [
                  Show({
                    when: computed(selectedAction_, function (s) {
                      return s.value === "overwrite";
                    }),
                    ok: function () {
                      return View({
                        style: {
                          width: "8px",
                          height: "8px",
                          "border-radius": "50%",
                          background: "#fff",
                        },
                      });
                    },
                  }),
                ],
              ),
              View({}, ["覆盖"]),
            ],
          ),
          View(
            {
              role: "radio",
              tabIndex: "0",
              attributes: {
                "aria-checked": computed(selectedAction_, function (s) {
                  return s.value === "duplicate" ? "true" : "false";
                }),
              },
              style: {
                display: "flex",
                "align-items": "center",
                gap: "10px",
                "padding-top": "10px",
                cursor: "pointer",
                "user-select": "none",
                "font-size": "14px",
                "line-height": "20px",
              },
              onClick: function () {
                select("duplicate");
              },
            },
            [
              View(
                {
                  style: computed(selectedAction_, function (selected) {
                    var checked = selected.value === "duplicate";
                    return {
                      width: "18px",
                      height: "18px",
                      "box-sizing": "border-box",
                      "border-radius": "50%",
                      border:
                        "2px solid " +
                        (checked ? "#07C160" : "var(--weui-FG-3)"),
                      background: checked ? "#07C160" : "transparent",
                      display: "inline-flex",
                      "align-items": "center",
                      "justify-content": "center",
                      "flex-shrink": "0",
                    };
                  }),
                },
                [
                  Show({
                    when: computed(selectedAction_, function (s) {
                      return s.value === "duplicate";
                    }),
                    ok: function () {
                      return View({
                        style: {
                          width: "8px",
                          height: "8px",
                          "border-radius": "50%",
                          background: "#fff",
                        },
                      });
                    },
                  }),
                ],
              ),
              View({}, ["重复下载"]),
            ],
          ),
        ],
      ),
    ],
  );
}

const OVERWRITE_DOWNLOAD_ACTION_ITEMS = [
  {
    value: "overwrite",
    label: "覆盖",
    description: "删除已有任务和文件后重新创建",
    icon: "refresh-cw",
  },
  {
    value: "skip",
    label: "跳过",
    description: "保留已有任务，不创建当前任务",
    icon: "corner-down-right",
  },
  {
    value: "duplicate",
    label: "重复",
    description: "保留已有任务，再创建一份",
    icon: "copy",
  },
];

function OverwriteDownloadDialogTitle(props) {
  return View(
    {
      style: {
        "font-size": "17px",
        "font-weight": "600",
        "line-height": "24px",
        "margin-bottom": "8px",
      },
    },
    [props.text],
  );
}

function OverwriteDownloadCurrentDuplicateTask(props) {
  var conflict_ = props.store.state.overwrite_conflict;
  return View(
    {
      style: {
        "min-width": "0",
        "margin-bottom": "12px",
        "text-align": "left",
        "font-size": "14px",
        "font-weight": "600",
        "line-height": "20px",
        color: "var(--weui-FG-0)",
        overflow: "hidden",
        "text-overflow": "ellipsis",
        "white-space": "nowrap",
      },
    },
    [
      computed(conflict_, (t) => {
        return t.name;
      }),
      "(",
      computed(conflict_, (t) => {
        return t.index;
      }),
      "/",
      computed(conflict_, (t) => {
        return t.total;
      }),
      ")",
    ],
  );
}

function OverwriteDownloadConflictCard(props) {
  var conflict_ = props.store.state.overwrite_conflict;
  return View(
    {
      style: {
        display: "flex",
        "align-items": "flex-start",
        gap: "10px",
        padding: "10px 12px",
        "border-radius": "8px",
        background: "var(--weui-BG-1)",
        "text-align": "left",
        "margin-bottom": "12px",
      },
    },
    [],
  );
}

function OverwriteDownloadActionList(props) {
  var selectedAction_ = props.store.state.overwrite;
  var processing_ = props.store.state.overwrite_processing;

  function select(actionValue) {
    if (processing_ && processing_.value) {
      return;
    }
    props.store.methods.setOverwriteAction(actionValue);
  }

  function actionRow(item) {
    return View(
      {
        role: "radio",
        tabIndex: "0",
        attributes: {
          "aria-checked": computed(selectedAction_, function (selected) {
            return selected.value === item.value ? "true" : "false";
          }),
        },
        style: computed(selectedAction_, function (selected) {
          var checked = selected.value === item.value;
          return {
            display: "flex",
            "align-items": "center",
            gap: "12px",
            padding: "10px 12px",
            "border-radius": "8px",
            border: "1px solid " + (checked ? "#07C160" : "var(--weui-FG-3)"),
            background: checked ? "rgba(7, 193, 96, 0.08)" : "transparent",
            cursor: processing_ && processing_.value ? "default" : "pointer",
            "user-select": "none",
          };
        }),
        onClick: function () {
          select(item.value);
        },
        onKeyDown: function (e) {
          if (e.key === " " || e.key === "Enter") {
            e.preventDefault();
            select(item.value);
          }
        },
      },
      [
        View(
          {
            style: computed(selectedAction_, function (selected) {
              var checked = selected.value === item.value;
              return {
                width: "28px",
                height: "28px",
                "border-radius": "50%",
                background: checked ? "#07C160" : "var(--weui-BG-1)",
                color: checked ? "#fff" : "var(--weui-FG-1)",
                display: "inline-flex",
                "align-items": "center",
                "justify-content": "center",
                "flex-shrink": "0",
              };
            }),
          },
          [Timeless.Icon({ name: item.icon, size: 16 })],
        ),
        View({ style: { "min-width": "0", flex: "1 1 auto" } }, [
          View(
            {
              style: {
                "font-size": "14px",
                "font-weight": "600",
                "line-height": "20px",
                color: "var(--weui-FG-0)",
              },
            },
            [item.label],
          ),
          View(
            {
              style: {
                "font-size": "12px",
                "line-height": "17px",
                color: "var(--weui-FG-1)",
                "margin-top": "2px",
              },
            },
            [item.description],
          ),
        ]),
        Show({
          when: computed(selectedAction_, function (selected) {
            return selected.value === item.value;
          }),
          ok: function () {
            return Timeless.Icon({ name: "check", size: 18 });
          },
        }),
      ],
    );
  }

  return View(
    {
      role: "radiogroup",
      style: {
        display: "grid",
        gap: "8px",
        "margin-bottom": "12px",
        "text-align": "left",
      },
    },
    [
      For({
        each: OVERWRITE_DOWNLOAD_ACTION_ITEMS,
        render: actionRow,
      }),
    ],
  );
}

function OverwriteDownloadApplyAllControl(props) {
  var applyAll_ = props.store.state.overwrite_apply_all;
  var processing_ = props.store.state.overwrite_processing;
  function toggle() {
    if (!processing_ || !processing_.value) {
      props.store.methods.toggleOverwriteApplyAll();
    }
  }
  return View(
    {
      role: "checkbox",
      tabIndex: "0",
      attributes: {
        "aria-checked": computed(applyAll_, function (checked) {
          return checked ? "true" : "false";
        }),
      },
      style: {
        display: "flex",
        "align-items": "center",
        gap: "10px",
        padding: "8px 0 0",
        cursor: "pointer",
        "user-select": "none",
        "font-size": "14px",
        "line-height": "20px",
        "text-align": "left",
      },
      onClick: toggle,
      onKeyDown: function (e) {
        if (e.key === " " || e.key === "Enter") {
          e.preventDefault();
          toggle();
        }
      },
    },
    [
      View(
        {
          style: computed(applyAll_, function (checked) {
            return {
              width: "18px",
              height: "18px",
              "box-sizing": "border-box",
              "border-radius": "4px",
              border: "1px solid " + (checked ? "#07C160" : "var(--weui-FG-3)"),
              background: checked ? "#07C160" : "transparent",
              color: "#fff",
              display: "inline-flex",
              "align-items": "center",
              "justify-content": "center",
              "flex-shrink": "0",
            };
          }),
        },
        [
          Show({
            when: applyAll_,
            ok: function () {
              return Timeless.Icon({ name: "check", size: 14 });
            },
          }),
        ],
      ),
      View({}, ["应用给所有"]),
    ],
  );
}

function OverwriteDownloadProcessingHint(props) {
  return Show({
    when: props.store.state.overwrite_processing,
    ok: function () {
      return View(
        {
          style: {
            "font-size": "12px",
            "line-height": "17px",
            color: "var(--weui-FG-1)",
            "margin-top": "10px",
          },
        },
        ["正在处理..."],
      );
    },
  });
}

function OverwriteDownloadDialogContent(props, children) {
  return View(
    {
      style: {
        "max-width": "calc(100vw - 32px)",
        "padding-top": "20px",
        "text-align": "center",
      },
    },
    children,
  );
}

function SingleOverwriteDownloadConfirmDialog(props) {
  return Timeless.weui.Dialog(
    {
      store: props.store.ui.singleOverwriteConfirmDialog$,
      style: {
        "z-index": "10000",
      },
    },
    [
      OverwriteDownloadDialogContent(props, [
        OverwriteDownloadDialogTitle({ text: "已存在确认" }),
        OverwriteDownloadActionList(props),
        OverwriteDownloadProcessingHint(props),
      ]),
    ],
  );
}

const OVERWRITE_DOWNLOAD_BATCH_DIALOG_Z_INDEX = "10001";

function BatchOverwriteDownloadConfirmDialog(props) {
  return Timeless.weui.Dialog(
    {
      store: props.store.ui.batchOverwriteConfirmDialog$,
      style: {
        "z-index": OVERWRITE_DOWNLOAD_BATCH_DIALOG_Z_INDEX,
      },
    },
    [
      OverwriteDownloadDialogContent(props, [
        OverwriteDownloadDialogTitle({ text: "批量已存在确认" }),
        OverwriteDownloadCurrentDuplicateTask(props),
        OverwriteDownloadActionList(props),
        OverwriteDownloadApplyAllControl(props),
        OverwriteDownloadProcessingHint(props),
      ]),
    ],
  );
}

function TaskDeleteConfirmDialog(props) {
  const checkboxStyle = computed(
    props.store.state.delete_delete_files,
    (checked) => {
      return {
        width: "18px",
        height: "18px",
        "box-sizing": "border-box",
        "border-radius": "4px",
        border: "1px solid " + (checked ? "#07C160" : "var(--weui-FG-3)"),
        background: checked ? "#07C160" : "transparent",
        color: "#fff",
        display: "inline-flex",
        "align-items": "center",
        "justify-content": "center",
        flex: "0 0 auto",
      };
    },
  );

  return Timeless.weui.Dialog(
    {
      store: props.store.ui.deleteConfirmDialog$,
      style: {
        "z-index": "10000",
      },
    },
    [
      View({ style: { padding: "20px 20px 16px" } }, [
        View(
          {
            style: {
              "font-size": "17px",
              "font-weight": "600",
              "line-height": "24px",
              "margin-bottom": "8px",
            },
          },
          ["删除下载任务"],
        ),
        View(
          {
            style: {
              "font-size": "14px",
              "line-height": "20px",
              color: "var(--weui-FG-1)",
              "margin-bottom": "16px",
            },
          },
          ["确定删除下载任务记录？", "此操作不可恢复。"],
        ),
        View(
          {
            role: "checkbox",
            tabIndex: "0",
            attributes: {
              "aria-checked": computed(
                props.store.state.delete_delete_files,
                (checked) => (checked ? "true" : "false"),
              ),
            },
            style: {
              display: "flex",
              "align-items": "center",
              gap: "10px",
              padding: "10px 0",
              cursor: "pointer",
              "user-select": "none",
              "font-size": "14px",
              "line-height": "20px",
            },
            onClick() {
              props.store.methods.handleClickCheckboxConfirmDeleteFiles();
            },
          },
          [
            View({ style: checkboxStyle }, [
              Show({
                when: props.store.state.delete_delete_files,
                ok() {
                  return Timeless.Icon({ name: "check", size: 14 });
                },
              }),
            ]),
            View({}, ["同时删除已下载的文件"]),
          ],
        ),
      ]),
    ],
  );
}

function CreateDownloadTaskDialog(props) {
  const text_ = props.text;
  const loading_ = props.loading;

  return Timeless.weui.Dialog({ store: props.store }, [
    View({ style: { width: "520px", padding: "20px 20px 16px" } }, [
      View(
        {
          style: {
            "font-size": "17px",
            "font-weight": "600",
            "line-height": "24px",
            "margin-bottom": "14px",
          },
        },
        ["创建下载任务"],
      ),
      Timeless.Textarea({
        value: text_,
        disabled: loading_,
        placeholder: "https://example.com/video.mp4",
        attributes: {
          rows: "10",
          spellcheck: "false",
        },
        class: "wx-dl-create-task-textarea wx-dl-dark-scroll",
        onInput(e) {
          const target =
            e && e.target && typeof e.target.get$elm === "function"
              ? e.target.get$elm()
              : e && e.target;
          props.onInput(
            target && typeof target.value === "string" ? target.value : "",
          );
        },
      }),
    ]),
  ]);
}

function DownloadTaskSelectionCheckbox(props) {
  const checked_ = props.checked;
  const size = props.size || 18;
  const boxStyle = computed(checked_, (checked) => {
    return {
      width: `${size}px`,
      height: `${size}px`,
      "box-sizing": "border-box",
      "border-radius": "4px",
      border: "1px solid " + (checked ? "#07C160" : "var(--weui-FG-3)"),
      background: checked ? "#07C160" : "transparent",
      color: "#fff",
      display: "inline-flex",
      "align-items": "center",
      "justify-content": "center",
      flex: "0 0 auto",
    };
  });

  return View(
    {
      role: "checkbox",
      tabIndex: "0",
      attributes: {
        "aria-label": props.ariaLabel || "选择下载任务",
        "aria-checked": computed(checked_, (checked) =>
          checked ? "true" : "false",
        ),
      },
      class: props.class || "",
      style: {
        width: `${size + 4}px`,
        height: `${size + 4}px`,
        display: "inline-flex",
        "align-items": "center",
        "justify-content": "center",
        cursor: "pointer",
        "user-select": "none",
        flex: "0 0 auto",
        ...(props.style || {}),
      },
      onClick(e) {
        if (e && typeof e.stopPropagation === "function") {
          e.stopPropagation();
        }
        if (typeof props.onToggle === "function") {
          props.onToggle(e);
        }
      },
      onKeyDown(e) {
        if (e.key === " " || e.key === "Enter") {
          e.preventDefault();
          if (typeof props.onToggle === "function") {
            props.onToggle(e);
          }
        }
      },
    },
    [
      View({ style: boxStyle }, [
        Show({
          when: checked_,
          ok() {
            return [
              Timeless.Icon({ name: "check", size: Math.max(12, size - 4) }),
            ];
          },
        }),
      ]),
    ],
  );
}

function DownloadTaskCard(props) {
  const vm$ = props.store;
  const task_ = props.task;
  const running_count_ = vm$.state.running_count;
  const iconSize = "50px";
  const state_ = computed(task_, (t) => {
    const pr = format_download_percent(t);
    const isLiveStream = is_live_stream_download_task(t);
    const normalizedStatus = normalize_download_status(t.status);
    const isPaused = normalizedStatus === "pause";
    const isRunning = normalizedStatus === "running";
    const isFailed = normalizedStatus === "error";
    const isPending = is_download_waiting_status(normalizedStatus);
    const isCompleted =
      normalizedStatus === "done" ||
      (pr === 100 && !isRunning && !isFailed && !isPaused && !isPending);

    const files = Array.isArray(t.files) ? t.files : [];
    const deletedFileCount = files.filter((file) => {
      return String((file && file.status) || "").toLowerCase() === "deleted";
    }).length;
    const hasDeletedFiles = deletedFileCount > 0;
    const allFilesDeleted =
      files.length > 0 && deletedFileCount === files.length;
    const filesDownloadedSize = files.reduce(
      (sum, f) => sum + (Number(f.downloaded) || 0),
      0,
    );
    const filesTotalSize = files.reduce(
      (sum, f) => sum + (Number(f.size) || 0),
      0,
    );
    const downloadedSize = Math.max(
      filesDownloadedSize,
      Number(t.downloaded) || 0,
    );
    const totalFileSize = Math.max(filesTotalSize, Number(t.size) || 0);
    const downloadedSizeText = format_download_size(downloadedSize);
    const totalSizeText = format_download_size(totalFileSize);

    let statusText = t.status;
    let statusColor = "var(--weui-FG-1)";
    let errorText = "";
    let progressText = "";
    let speedText = "";
    if (isRunning) {
      speedText = format_download_speed(
        t.speed ||
          (t.progress && typeof t.progress === "object" ? t.progress.speed : 0),
      );
      statusText = "下载中";
      progressText = format_download_progress_text(pr);
    } else if (isCompleted) {
      statusText = hasDeletedFiles
        ? allFilesDeleted
          ? "文件已删除"
          : "部分文件已删除"
        : "已完成";
      statusColor = hasDeletedFiles ? "#FA5151" : "#07C160";
    } else if (isFailed) {
      statusText = "失败";
      errorText = t.error || t._errMsg || "下载失败";
      statusColor = "#FA5151";
    } else if (isPending) {
      statusText = "等待中...";
    } else if (isPaused) {
      statusText = "已暂停";
      statusColor = "#FBC02D";
      progressText = format_download_progress_text(pr);
    }
    return {
      pr,
      isLiveStream,
      isCompleted,
      hasDeletedFiles,
      allFilesDeleted,
      isPaused,
      isRunning,
      isFailed,
      canResume: isFailed || isPaused,
      statusText,
      statusColor,
      errorText,
      progressText,
      speedText,
      downloadedSizeText,
      totalSizeText,
      totalFileSize,
    };
  });
  const isOpenExternal = is_download_open_external();
  const radius = 22;
  const circumference = 2 * Math.PI * radius;
  const offset = computed(state_, (d) => {
    return circumference - (d.pr / 100) * circumference;
  });
  const strokeColor = computed(state_, (d) => {
    return d.isPaused ? "#FBC02D" : "#07C160";
  });
  const taskId = (task_ && task_.value !== undefined ? task_.value : task_).id;
  const selected_ = computed(vm$.state.selected_task_ids, (ids) => {
    return (ids || []).some((id) => id === taskId);
  });
  const btnStyle = {
    color: "var(--weui-FG-0)",
    opacity: "0.8",
    "margin-left": "12px",
    cursor: "pointer",
    display: "flex",
    "align-items": "center",
    "justify-content": "center",
  };

  return View(
    {
      class: ["weui-cell wx-dl-item", props.class].filter(Boolean).join(" "),
      style: {
        "box-sizing": "border-box",
      },
    },
    [
      Show({
        when: props.showCheckbox,
        ok() {
          return DownloadTaskSelectionCheckbox({
            checked: selected_,
            ariaLabel: "选择下载任务",
            style: {
              "margin-right": "10px",
            },
            onToggle(e) {
              vm$.methods.toggleTaskSelected(
                task_.value !== undefined ? task_.value : task_,
                {
                  shiftKey: !!(e && e.shiftKey),
                },
              );
            },
          });
        },
      }),
      View(
        {
          class: "weui-cell__hd",
          style: {
            position: "relative",
            "margin-right": "16px",
            width: iconSize,
            height: iconSize,
            display: "flex",
            "align-items": "center",
            "justify-content": "center",
            color: "var(--weui-FG-0)",
          },
        },
        [
          View(
            {
              style: {
                position: "relative",
                width: "32px",
                height: "32px",
                display: "inline-flex",
                "align-items": "center",
                "justify-content": "center",
                "z-index": "1",
                "pointer-events": "none",
                opacity: computed(state_, (t) => {
                  return !t.isLiveStream && (t.isRunning || t.isPaused)
                    ? "0.2"
                    : "1";
                }),
              },
            },
            [
              DownloadTaskFileIcon({
                task: task_,
                size: 32,
              }),
            ],
          ),
          Show({
            when: computed(state_, (t) => {
              return !t.isLiveStream && (t.isRunning || t.isPaused);
            }),
            ok() {
              return [
                SVG.SVG(
                  {
                    style: {
                      position: "absolute",
                      top: "0",
                      left: "0",
                      transform: "rotate(-90deg)",
                      "z-index": "3",
                    },
                    attributes: {
                      width: "50",
                      height: "50",
                      viewBox: "0 0 50 50",
                    },
                  },
                  [
                    SVG.Circle({
                      attributes: {
                        cx: "25",
                        cy: "25",
                        r: radius,
                        stroke: "var(--weui-FG-3)",
                        "stroke-width": "3",
                        fill: "none",
                      },
                    }),
                    SVG.Circle({
                      attributes: {
                        cx: "25",
                        cy: "25",
                        r: radius,
                        stroke: strokeColor,
                        "stroke-width": "3",
                        fill: "none",
                        "stroke-dasharray": circumference,
                        "stroke-dashoffset": offset,
                        "stroke-linecap": "round",
                      },
                    }),
                  ],
                ),
                View(
                  {
                    style: {
                      position: "absolute",
                      inset: "0",
                      display: "grid",
                      "place-items": "center",
                      "font-size": "24px",
                      "font-weight": "700",
                      color: "var(--weui-FG-0)",
                      "line-height": "1",
                      "pointer-events": "none",
                      "z-index": "4",
                      "text-shadow":
                        "0 0 2px var(--weui-BG-3), 0 2px 4px rgba(0, 0, 0, 0.55)",
                      "-webkit-text-stroke": "0.3px var(--weui-BG-3)",
                    },
                  },
                  [
                    NumberView({
                      value: computed(state_, (t) => t.progressText),
                    }),
                  ],
                ),
              ];
            },
          }),
        ],
      ),
      View(
        {
          class: "weui-cell__bd",
          style: { "min-width": "0" },
        },
        [
          View(
            {
              style: {
                display: "flex",
                "align-items": "center",
                gap: "6px",
                "flex-wrap": "wrap",
              },
            },
            [
              View(
                {
                  type: "a",
                  class: "wx-dl-item-title",
                  attributes: {
                    href: computed(task_, (t) => download_task_preview_url(t)),
                    title: computed(task_, (t) => (t && t.name) || ""),
                    target: "_blank",
                    rel: "noopener noreferrer",
                  },
                  style: {
                    color: "var(--weui-FG-0)",
                    "font-weight": "500",
                    "font-size": "14px",
                    "min-width": "0",
                    cursor: "pointer",
                    "text-decoration": "none",
                  },
                  onClick(e) {
                    if (e && typeof e.preventDefault === "function") {
                      e.preventDefault();
                    }
                    const task =
                      task_ && task_.value !== undefined ? task_.value : task_;
                    open_download_task_preview(task);
                  },
                },
                [computed(task_, (t) => (t && t.name) || "")],
              ),
              Show({
                when: computed(state_, (d) => d.isLiveStream),
                ok() {
                  return View(
                    {
                      style: {
                        display: "inline-flex",
                        "align-items": "center",
                        height: "18px",
                        padding: "0 6px",
                        color: "#FA5151",
                        "font-size": "11px",
                        "font-weight": "600",
                        "line-height": "18px",
                        "white-space": "nowrap",
                        "text-decoration": "none",
                        background: "rgba(250, 81, 81, 0.12)",
                        border: "1px solid rgba(250, 81, 81, 0.35)",
                        "border-radius": "4px",
                      },
                    },
                    ["直播"],
                  );
                },
              }),
            ],
          ),
          View(
            {
              class: "weui-cell__desc",
              style: {
                "margin-top": "4px",
                color: "var(--weui-FG-1)",
                "font-size": "12px",
                display: "flex",
                "align-items": "center",
                gap: "3px",
                "flex-wrap": "nowrap",
                overflow: "hidden",
                "white-space": "nowrap",
                "text-overflow": "ellipsis",
              },
            },
            [
              View(
                {
                  style: computed(state_, (d) => ({ color: d.statusColor })),
                },
                [
                  computed(state_, (d) =>
                    String(d.statusText).split("•")[0].trim(),
                  ),
                ],
              ),
              "·",
              Show({
                when: computed(state_, (d) => d.isCompleted),
                ok() {
                  return NumberView({
                    value: computed(state_, (d) => d.totalSizeText),
                  });
                },
                else() {
                  return [
                    NumberView({
                      value: computed(
                        state_,
                        (d) => `${d.downloadedSizeText} /`,
                      ),
                    }),
                    Show({
                      when: computed(state_, (d) => d.isLiveStream),
                      ok() {
                        return DownloadInfinityIcon({ size: 14 });
                      },
                      else() {
                        return NumberView({
                          value: computed(state_, (d) => d.totalSizeText),
                        });
                      },
                    }),
                  ];
                },
              }),
              Show({
                when: computed(state_, (d) => d.isRunning && !!d.speedText),
                ok() {
                  return [
                    "·",
                    NumberView({
                      value: computed(state_, (d) => d.speedText),
                    }),
                  ];
                },
              }),
            ],
          ),
          Show({
            when: computed(state_, (d) => d.isFailed && !!d.errorText),
            ok() {
              return View(
                {
                  class: "weui-cell__desc wx-dl-item-error",
                  style: {
                    "margin-top": "2px",
                    color: "#FA5151",
                    "font-size": "11px",
                    "line-height": "16px",
                    display: "-webkit-box",
                    overflow: "hidden",
                    "word-break": "break-word",
                    "-webkit-box-orient": "vertical",
                    "-webkit-line-clamp": "2",
                  },
                  attributes: {
                    title: computed(state_, (d) => d.errorText),
                  },
                },
                [computed(state_, (d) => d.errorText)],
              );
            },
          }),
        ],
      ),
      View(
        {
          class: "weui-cell__ft",
          style: {
            display: "flex",
            "align-items": "center",
          },
        },
        [
          Match({
            when: combine(
              {
                state: state_,
                running_count: running_count_,
              },
              (t) => {
                if (t.state.isCompleted) {
                  return 1;
                }
                if (t.state.isRunning) {
                  return 2;
                }
                if (t.state.isLiveStream && t.state.isPaused) {
                  return 5;
                }
                if (t.running_count >= MaxRunning) {
                  return 5;
                }
                if (t.state.isPaused) {
                  return 3;
                }
                if (t.state.isFailed) {
                  return 4;
                }
                return 0;
              },
            ),
            cases: {
              0() {
                return View(
                  {
                    type: "a",
                    class: "wx-download-item-start",
                    style: btnStyle,
                    onClick() {
                      vm$.methods.startTask(task_.value);
                    },
                  },
                  [
                    Timeless.Icon({
                      name: "play",
                      size: 20,
                    }),
                  ],
                );
              },
              1() {
                return View(
                  {
                    type: "a",
                    class: "wx-download-item-open",
                    style: btnStyle,
                    onClick() {
                      vm$.methods.openTask(task_.value);
                    },
                  },
                  [
                    Show({
                      when: !!isOpenExternal,
                      ok() {
                        return [
                          Timeless.Icon({
                            name: "file-symlink",
                            size: 20,
                          }),
                        ];
                      },
                      else() {
                        return [
                          Timeless.Icon({
                            name: "folder",
                            size: 20,
                          }),
                        ];
                      },
                    }),
                  ],
                );
              },
              2() {
                return View(
                  {
                    type: "a",
                    class: "wx-download-item-pause",
                    style: btnStyle,
                    attributes: {
                      title: computed(state_, (t) =>
                        t.isLiveStream ? "停止录制" : "暂停",
                      ),
                      "aria-label": computed(state_, (t) =>
                        t.isLiveStream ? "停止录制" : "暂停",
                      ),
                    },
                    onClick() {
                      vm$.methods.pauseTask(task_.value, {
                        liveStream: state_.value.isLiveStream,
                      });
                    },
                  },
                  [
                    Show({
                      when: computed(state_, (t) => t.isLiveStream),
                      ok() {
                        return [
                          Timeless.Icon({
                            name: "square",
                            size: 20,
                          }),
                        ];
                      },
                      else() {
                        return [
                          Timeless.Icon({
                            name: "pause",
                            size: 20,
                          }),
                        ];
                      },
                    }),
                  ],
                );
              },
              3() {
                return View(
                  {
                    type: "a",
                    class: "wx-download-item-resume",
                    style: btnStyle,
                    onClick() {
                      vm$.methods.resumeTask(task_.value);
                    },
                  },
                  [
                    Timeless.Icon({
                      name: "play",
                      size: 20,
                    }),
                  ],
                );
              },
              4() {
                return View(
                  {
                    type: "a",
                    class: "wx-download-item-resume",
                    style: btnStyle,
                    onClick() {
                      vm$.methods.retryTask(task_.value);
                    },
                  },
                  [
                    Timeless.Icon({
                      name: "refresh-ccw",
                      size: 20,
                    }),
                  ],
                );
              },
              5() {
                return View({});
              },
            },
          }),
          View(
            {
              class: "wx-download-item-delete",
              style: btnStyle,
              onClick() {
                vm$.methods.requestDeleteTask(task_.value);
              },
            },
            [
              Timeless.Icon({
                name: "trash2",
                size: 20,
              }),
            ],
          ),
        ],
      ),
    ],
  );
}

function DownloadTaskListView(props) {
  const vm$ = props.store;
  const tasks_ = vm$.state.tasks;
  const listPaddingBottom =
    typeof props.paddingBottom !== "undefined"
      ? props.paddingBottom
      : typeof props.listPaddingBottom !== "undefined"
        ? props.listPaddingBottom
        : typeof props.bottomPadding !== "undefined"
          ? props.bottomPadding
          : 0;
  const listGutter =
    typeof props.gutter !== "undefined"
      ? props.gutter
      : typeof props.listGutter !== "undefined"
        ? props.listGutter
        : vm$.state.list_gutter;

  return View(
    {
      class: props.class || "wx-dl-list wx-dl-dark-scroll",
      style: props.style || {},
    },
    [
      Show({
        when: computed(tasks_, (items) => items.length > 0),
        ok() {
          return Show({
            when: vm$.state.list_render_enabled,
            ok() {
              const listHeightStyle = vm$.state.fixed_list_height
                ? {
                    height: `${vm$.state.list_height}px`,
                    "max-height": `${vm$.state.list_height}px`,
                  }
                : {
                    "max-height": "100%",
                  };
              return [
                VirtualListView({
                  style: {
                    ...listHeightStyle,
                    overflow: "auto",
                    position: "relative",
                    padding: props.padding || "0 12px",
                    "box-sizing": "border-box",
                    "background-color": "transparent",
                    ...(props.listViewStyle || {}),
                  },
                  key: "id",
                  size: props.size || 10,
                  buffer: vm$.state.list_buffer,
                  gutter: listGutter,
                  itemHeight: vm$.state.list_item_height,
                  paddingBottom: listPaddingBottom,
                  each: tasks_,
                  onMounted(e) {
                    vm$.methods.setListViewElement(e);
                  },
                  onScroll(pos) {
                    vm$.methods.handleListViewScroll(pos);
                  },
                  render(task_) {
                    const task =
                      task_ && task_.value !== undefined ? task_.value : task_;
                    if (vm$.methods.isPlaceholderTask(task)) {
                      vm$.methods.ensureTaskPageForIndex(task.__index);
                      return DownloadTaskSkeletonCard({
                        class: props.skeletonClass,
                      });
                    }
                    return DownloadTaskCard({
                      store: vm$,
                      task: task_,
                      class: props.itemClass,
                      showCheckbox: props.showCheckbox,
                    });
                  },
                }),
              ];
            },
          });
        },
        else() {
          return [
            View(
              {
                class: props.emptyClass || "weui-loadmore weui-loadmore_line",
              },
              [
                View(
                  {
                    class: "weui-loadmore__tips",
                  },
                  ["暂无下载任务"],
                ),
              ],
            ),
          ];
        },
      }),
    ],
  );
}

function DownloaderPanelView(props) {
  const vm$ = props.store;
  const task_count_ = vm$.state.task_count;
  const status_counts_ = vm$.state.status_counts;
  const active_status_ = vm$.state.active_status;
  const selected_task_count_ = vm$.state.selected_task_count;
  const showStatusCounts = props.showStatusCounts === true;

  return View(
    {
      class: "wx-dl-panel-container",
      style: props.showViewAll ? { "padding-bottom": "0" } : {},
      onMounted() {
        vm$.ready();
      },
      onUnmounted() {
        // vm$.clean();
      },
    },
    [
      View({ class: "wx-dl-header" }, [
        View({ class: "wx-dl-heading" }, [
          View({ class: "wx-dl-title" }, [
            "Downloads",
            computed(task_count_, (d) => {
              return d > 0 ? `（${d}）` : "";
            }),
          ]),
          Show({
            when: computed(status_counts_, (counts) => {
              return (
                showStatusCounts &&
                normalize_download_status_counts(counts).total > 0
              );
            }),
            ok() {
              return [
                View({ class: "wx-dl-status-counts" }, [
                  For({
                    each: DOWNLOAD_STATUS_COUNT_ITEMS,
                    render(item) {
                      return View(
                        {
                          role: "button",
                          tabIndex: "0",
                          attributes: {
                            "aria-pressed": computed(
                              active_status_,
                              (status) =>
                                status === item.key ? "true" : "false",
                            ),
                          },
                          class: computed(active_status_, (status) =>
                            [
                              "wx-dl-status-count",
                              "wx-dl-status-count-filter",
                              status === item.key
                                ? "wx-dl-status-count-active"
                                : "",
                              item.key === "error"
                                ? "wx-dl-status-count-error"
                                : "",
                            ]
                              .filter(Boolean)
                              .join(" "),
                          ),
                          onClick() {
                            vm$.methods.setStatusFilter(item.key);
                          },
                          onKeyDown(e) {
                            if (e.key === " " || e.key === "Enter") {
                              e.preventDefault();
                              vm$.methods.setStatusFilter(item.key);
                            }
                          },
                        },
                        [
                          View({ class: "wx-dl-status-count-label" }, [
                            item.label,
                          ]),
                          View({ class: "wx-dl-status-count-value" }, [
                            computed(status_counts_, (counts) => {
                              return String(
                                get_download_status_count(counts, item),
                              );
                            }),
                          ]),
                        ],
                      );
                    },
                  }),
                ]),
              ];
            },
          }),
        ]),
        Show({
          when: computed(selected_task_count_, (count) => count > 0),
          ok() {
            return [
              View(
                {
                  type: "button",
                  style: {
                    height: "28px",
                    display: "inline-flex",
                    "align-items": "center",
                    "justify-content": "center",
                    gap: "4px",
                    padding: "0 8px",
                    border: "1px solid rgba(250,81,81,0.42)",
                    "border-radius": "4px",
                    background: "transparent",
                    color: "#FA5151",
                    cursor: "pointer",
                    "font-size": "12px",
                    "line-height": "18px",
                    "white-space": "nowrap",
                  },
                  onClick() {
                    vm$.methods.requestDeleteSelectedTasks(false);
                  },
                },
                [
                  Timeless.Icon({ name: "trash2", size: 14 }),
                  computed(
                    selected_task_count_,
                    (count) => `删除选中 ${count}`,
                  ),
                ],
              ),
            ];
          },
        }),
        DropdownMenu(
          {
            store: vm$.ui.dropdown$,
          },
          [
            View(
              {
                class: "wx-dl-more-btn",
              },
              [
                Timeless.Icon({
                  name: "ellipsis-vertical",
                  style: { "font-size": "18px" },
                }),
              ],
            ),
          ],
        ),
      ]),
      DownloadTaskListView({
        store: vm$,
        paddingBottom: 12,
      }),
    ],
  );
}
