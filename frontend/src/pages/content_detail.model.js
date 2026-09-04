function first_non_empty(...values) {
  for (const value of values) {
    if (value !== undefined && value !== null && value !== "") {
      return value;
    }
  }
  return "";
}

function number_or_default(value, fallback) {
  const number = Number(value);
  return Number.isFinite(number) ? number : fallback;
}

function normalize_content_account(raw) {
  if (!raw || typeof raw !== "object") {
    return null;
  }
  return {
    ...raw,
    id: first_non_empty(raw.id, raw.ID),
    platform_id: first_non_empty(raw.platform_id, raw.PlatformID),
    external_id: first_non_empty(raw.external_id, raw.ExternalID),
    alias: first_non_empty(raw.alias, raw.Alias),
    nickname: first_non_empty(raw.nickname, raw.Nickname, raw.name),
    avatar_url: first_non_empty(raw.avatar_url, raw.AvatarURL),
    profile_url: first_non_empty(raw.profile_url, raw.ProfileURL),
  };
}

function normalize_embedded_content(raw) {
  const source = raw && typeof raw === "object" ? raw : {};
  const raw_content = first_non_empty(source.content, source.Content);
  const content =
    raw_content && typeof raw_content === "object" ? raw_content : {};
  return {
    ...source,
    relation_type: first_non_empty(
      source.relation_type,
      source.RelationType,
    ),
    sort_order: number_or_default(
      first_non_empty(source.sort_order, source.SortOrder),
      0,
    ),
    content: {
      ...content,
      id: first_non_empty(content.id, content.ID),
      platform_id: first_non_empty(content.platform_id, content.PlatformID),
      content_type: first_non_empty(
        content.content_type,
        content.ContentType,
        content.type,
        content.Type,
      ),
      title: first_non_empty(
        content.title,
        content.Title,
        content.description,
        "未命名媒体",
      ),
      cover_url: first_non_empty(
        content.cover_url,
        content.CoverURL,
        content.coverUrl,
      ),
    },
    detail_type: first_non_empty(source.detail_type, source.DetailType),
    detail: first_non_empty(source.detail, source.Detail) || null,
  };
}

function normalize_content_detail(raw) {
  const source = raw && typeof raw === "object" ? raw : {};
  const accounts_source = Array.isArray(source.accounts)
    ? source.accounts
    : Array.isArray(source.Accounts)
      ? source.Accounts
      : [];
  const tasks = Array.isArray(source.download_tasks)
    ? source.download_tasks
    : Array.isArray(source.DownloadTasks)
      ? source.DownloadTasks
      : [];
  const resources = Array.isArray(source.resources)
    ? source.resources
    : Array.isArray(source.Resources)
      ? source.Resources
      : [];
  const embedded_source = Array.isArray(source.embedded_contents)
    ? source.embedded_contents
    : Array.isArray(source.EmbeddedContents)
      ? source.EmbeddedContents
      : [];
  const embedded_contents = embedded_source.map(normalize_embedded_content);
  const embedded_content_by_id = new Map(
    embedded_contents.map((item) => [
      String((item.content && item.content.id) || ""),
      item.content,
    ]),
  );
  const relations_source =
    source.relations && typeof source.relations === "object"
      ? source.relations
      : source.Relations && typeof source.Relations === "object"
        ? source.Relations
        : {};
  const relations = Array.isArray(relations_source.list)
    ? relations_source.list
    : Array.isArray(relations_source.List)
      ? relations_source.List
      : [];
  return {
    ...source,
    id: first_non_empty(source.id, source.ID),
    platform_id: first_non_empty(source.platform_id, source.PlatformID),
    platform_name: first_non_empty(
      source.platform_name,
      source.PlatformName,
    ),
    content_type: first_non_empty(
      source.content_type,
      source.ContentType,
      source.type,
      source.Type,
    ),
    title: first_non_empty(
      source.title,
      source.Title,
      source.description,
      "未命名内容",
    ),
    description: first_non_empty(source.description, source.Description),
    url: first_non_empty(
      source.url,
      source.URL,
      source.content_url,
      source.ContentURL,
      source.source_url,
      source.SourceURL,
    ),
    source_url: first_non_empty(source.source_url, source.SourceURL),
    cover_url: first_non_empty(
      source.cover_url,
      source.CoverURL,
      source.coverUrl,
    ),
    file_size: number_or_default(
      first_non_empty(
        source.file_size,
        source.FileSize,
        source.size,
        source.detail && source.detail.size,
      ),
      0,
    ),
    publish_time: number_or_default(
      first_non_empty(source.publish_time, source.PublishTime),
      0,
    ),
    detail_type: first_non_empty(source.detail_type, source.DetailType),
    detail: first_non_empty(source.detail, source.Detail) || null,
    accounts: accounts_source.map(normalize_content_account).filter(Boolean),
    download_tasks: tasks,
    embedded_contents,
    resources: resources.map((resource) => {
      const download_task_status = resource_download_task_status(
        resource,
        tasks,
      );
      const resource_content_id = String(
        first_non_empty(
          resource && resource.content_id,
          resource && resource.ContentID,
          resource && resource.ContentId,
        ),
      );
      return {
        ...resource,
        owner_content: embedded_content_by_id.get(resource_content_id) || null,
        download_task_status,
        download_task_in_progress: ["running", "paused"].includes(
          download_task_status,
        ),
      };
    }),
    relations: {
      ...relations_source,
      list: relations,
      has_content: relations.length > 0,
      total: number_or_default(
        first_non_empty(relations_source.total, relations_source.Total),
        relations.length,
      ),
    },
  };
}

function detail_id_from_query(query) {
  return String((query && query.id) || "");
}

function prop_value(value) {
  if (value && typeof value === "object" && "value" in value) {
    return value.value;
  }
  return value;
}

function content_source_url(content) {
  return String(
    first_non_empty(
      content && content.source_url,
      content && content.SourceURL,
      content && content.url,
      content && content.URL,
    ),
  ).trim();
}

function resource_file_path(resource) {
  return String(
    first_non_empty(
      resource && resource.local_path,
      resource && resource.LocalPath,
      resource && resource.file_path,
      resource && resource.FilePath,
    ),
  ).trim();
}

function resource_file_url(resource) {
  const local_path = resource_file_path(resource);
  return local_path ? `/api/file?path=${encodeURIComponent(local_path)}` : "";
}

function object_value(source, ...keys) {
  if (!source || typeof source !== "object") return undefined;
  for (const key of keys) {
    if (source[key] !== undefined && source[key] !== null) {
      return source[key];
    }
  }
  return undefined;
}

function content_cover_url(content) {
  const source = content && typeof content === "object" ? content : {};
  const content_record = object_value(source, "content", "Content");
  const assets = object_value(content_record, "assets", "Assets");
  const resources = Array.isArray(source.resources) ? source.resources : [];
  const resources_by_id = new Map(
    resources.map((resource) => [
      String(object_value(resource, "id", "ID") || ""),
      resource,
    ]),
  );

  for (const asset of Array.isArray(assets) ? assets : []) {
    const role = String(object_value(asset, "role", "Role") || "")
      .trim()
      .toLowerCase();
    if (role !== "cover") continue;

    const linked_resources = object_value(
      asset,
      "download_resources",
      "DownloadResources",
    );
    for (const linked_resource of Array.isArray(linked_resources)
      ? linked_resources
      : []) {
      const resource_id = String(
        object_value(linked_resource, "id", "ID") || "",
      );
      const resource = resources_by_id.get(resource_id) || linked_resource;
      const asset_url = resource_file_url(resource);
      if (resource_file_available(resource) && asset_url) {
        return asset_url;
      }
    }
  }

  return String(
    first_non_empty(source.cover_url, source.CoverURL, source.coverUrl),
  ).trim();
}

function platform_name(content) {
  if (content && content.platform_name) {
    return content.platform_name;
  }
  const platform_id = String((content && content.platform_id) || "").trim();
  return window.PLATFORM_NAMES[platform_id] || platform_id || "未知平台";
}

function content_type_label(value) {
  const type = String(value || "").trim().toLowerCase();
  return window.CONTENT_TYPE_NAMES[type] || type || "内容";
}

function normalize_task_status(status) {
  const value = String(status ?? "").trim().toLowerCase();
  if (
    [
      "1",
      "2",
      "4",
      "preparing",
      "downloading",
      "merging",
      "running",
    ].includes(value)
  ) {
    return "running";
  }
  if (["3", "paused"].includes(value)) {
    return "paused";
  }
  if (["5", "finished", "completed", "success", "done"].includes(value)) {
    return "finished";
  }
  if (
    ["6", "7", "failed", "fail", "failure", "error", "cancelled", "canceled"].includes(
      value,
    )
  ) {
    return "failed";
  }
  return "waiting";
}

function resource_download_task_status(resource, tasks) {
  const task_id = String(
    first_non_empty(
      resource && resource.task_id,
      resource && resource.TaskID,
      resource && resource.TaskId,
    ),
  ).trim();
  if (!task_id) return "waiting";

  const task = (Array.isArray(tasks) ? tasks : []).find(
    (item) =>
      String(first_non_empty(item && item.id, item && item.ID)).trim() ===
      task_id,
  );
  return normalize_task_status(
    first_non_empty(task && task.status, task && task.Status),
  );
}

function task_status(status) {
  const tone = normalize_task_status(status);
  const labels = {
    waiting: "等待中",
    running: "下载中",
    paused: "已暂停",
    finished: "已完成",
    failed: "失败",
  };
  return { label: labels[tone], tone };
}

function resource_download_finished(resource) {
  const status = String(
    first_non_empty(resource && resource.status, resource && resource.Status),
  )
    .trim()
    .toLowerCase();
  return ["2", "finished", "done"].includes(status);
}

function resource_file_available(resource) {
  if (!resource_download_finished(resource)) return false;
  const exists = first_non_empty(
    resource && resource.exists,
    resource && resource.Exists,
  );
  return exists === true && Boolean(resource_file_url(resource));
}

function resource_file_status(resource) {
  if (resource_file_available(resource)) return "已下载";
  if (resource_download_finished(resource)) return "文件不存在";
  const task_status = String(
    first_non_empty(
      resource && resource.download_task_status,
      resource && resource.DownloadTaskStatus,
    ),
  )
    .trim()
    .toLowerCase();
  if (task_status === "paused") return "已暂停";
  if (task_status === "failed") return "下载失败";
  const resource_status = String(
    first_non_empty(resource && resource.status, resource && resource.Status),
  )
    .trim()
    .toLowerCase();
  if (
    task_status === "running" ||
    ["1", "running", "downloading"].includes(resource_status)
  ) {
    return "下载中";
  }
  return "等待下载";
}

function file_type_icon(resource) {
  const type = String(
    first_non_empty(resource && resource.file_type, resource && resource.type),
  ).toLowerCase();
  return (
    {
      image: "image",
      video: "video",
      audio: "music",
      html: "file-code",
      zip: "archive",
      pdf: "file-text",
    }[type] || "file"
  );
}

function format_bytes(value) {
  const bytes = Number(value);
  if (!Number.isFinite(bytes) || bytes <= 0) {
    return "";
  }
  const units = ["B", "KB", "MB", "GB", "TB"];
  const index = Math.min(
    units.length - 1,
    Math.floor(Math.log(bytes) / Math.log(1024)),
  );
  const amount = bytes / Math.pow(1024, index);
  return `${amount >= 100 || index === 0 ? amount.toFixed(0) : amount.toFixed(1)} ${units[index]}`;
}

function sort_content_assets(assets) {
  return [...(Array.isArray(assets) ? assets : [])].sort((left, right) => {
    const left_created_at = number_or_default(
      first_non_empty(
        left && left.created_at,
        left && left.createdAt,
        left && left.CreatedAt,
      ),
      0,
    );
    const right_created_at = number_or_default(
      first_non_empty(
        right && right.created_at,
        right && right.createdAt,
        right && right.CreatedAt,
      ),
      0,
    );
    return right_created_at - left_created_at;
  });
}

function content_media_assets(assets) {
  return sort_content_assets(assets).filter((asset) => {
    const resources = first_non_empty(
      asset && asset.download_resources,
      asset && asset.DownloadResources,
    );
    return Array.isArray(resources) && resources.length > 0;
  });
}

function ContentDetailViewModel(props) {
  const detail_id_ = ref(
    String(
      prop_value(props.contentId) ||
        detail_id_from_query(props.view && props.view.query),
    ).trim(),
  );
  const detail_ = ref(null);
  const loading_ = ref(false);
  const error_ = ref("");
  let request_sequence = 0;

  const request_ = new Timeless.kit.RequestCore(
    (params) => window.request.get("/api/content/detail", params),
    {
      client: props.client,
      process(response) {
        if (response.error) {
          return Timeless.Result.Err(response.error);
        }
        return Timeless.Result.Ok(normalize_content_detail(response.data));
      },
    },
  );

  const check_files_request_ = new Timeless.kit.RequestCore(
    (files) =>
      window.request.post("/api/v1/download_task/check_files", {
        files,
      }),
    { client: props.client },
  );

  async function verify_resource_files(detail, sequence) {
    const resources = Array.isArray(detail && detail.resources)
      ? detail.resources
      : [];
    const files = resources
      .filter((resource) => resource && resource.id && resource.task_id)
      .map((resource) => ({
        id: resource.id,
        task_id: resource.task_id,
        name: resource.name || "",
        output_path: resource.local_path || "",
      }));
    if (files.length === 0) return detail;

    let result;
    try {
      result = await check_files_request_.run(files);
    } catch {
      return detail;
    }
    if (sequence !== request_sequence || result.error) return detail;
    const checked_files = Array.isArray(result.data && result.data.files)
      ? result.data.files
      : [];
    const checked_by_resource = new Map();
    for (const checked of checked_files) {
      if (!checked || !checked.checked) continue;
      checked_by_resource.set(`${checked.task_id}:${checked.id}`, checked);
    }
    if (checked_by_resource.size === 0) return detail;

    return {
      ...detail,
      resources: resources.map((resource) => {
        const checked = checked_by_resource.get(
          `${resource.task_id}:${resource.id}`,
        );
        if (!checked) return resource;
        const download_finished = resource_download_finished(resource);
        const exists = checked.exists === true && download_finished;
        return {
          ...resource,
          exists,
          local_file_checked: true,
          local_file_exists: exists,
          local_file_deleted: !exists && download_finished,
        };
      }),
    };
  }

  async function load(content_id = detail_id_.value) {
    const id = String(content_id || "").trim();
    if (!id) {
      detail_.as(null);
      error_.as("缺少内容 ID");
      return Timeless.Result.Err(new Error("缺少内容 ID"));
    }
    if (id === detail_id_.value && loading_.value) {
      return null;
    }
    const detail_changed = id !== detail_id_.value;
    detail_id_.as(id);
    if (detail_changed) {
      detail_.as(null);
    }
    const sequence = ++request_sequence;
    loading_.as(true);
    error_.as("");
    const result = await request_.run({ id });
    if (sequence !== request_sequence) {
      return result;
    }
    if (result.error) {
      loading_.as(false);
      detail_.as(null);
      error_.as(result.error.message || String(result.error));
      return result;
    }
    const verified_detail = await verify_resource_files(result.data, sequence);
    if (sequence !== request_sequence) {
      return result;
    }
    loading_.as(false);
    detail_.as(verified_detail);
    return result;
  }

  if (props.contentId && typeof props.contentId.subscribe === "function") {
    props.contentId.subscribe({
      onChange(content_id) {
        const id = String(content_id || "").trim();
        if (!id || id === detail_id_.value) return;
        load(id);
      },
    });
  }

  const methods = {
    ready() {
      return load(prop_value(props.contentId) || detail_id_.value);
    },
    refresh() {
      return load(detail_id_.value);
    },
    backToList() {
      if (typeof props.onBack === "function") {
        props.onBack();
        return;
      }
      props.history.push("root.shell.content");
    },
    openDetail(content_id) {
      return load(content_id);
    },
    openSource(content) {
      const url = content_source_url(content);
      if (url) {
        props.app.openWindow(url);
      }
    },
    openResource(resource) {
      if (!resource_file_available(resource)) return;
      const url = resource_file_url(resource);
      if (url) {
        props.app.openWindow(url);
      }
    },
    showResource(resource) {
      if (!resource_file_available(resource)) return;
      return window.dl$.requests.file.show.run({
        path: resource_file_path(resource),
        name: first_non_empty(
          resource && resource.name,
          resource && resource.Name,
        ),
      });
    },
    sourceURL: content_source_url,
    coverURL: content_cover_url,
    resourceFileURL: resource_file_url,
    resourceFileAvailable: resource_file_available,
    resourceFileStatus: resource_file_status,
    platformName: platform_name,
    typeLabel: content_type_label,
    taskStatus: task_status,
    fileTypeIcon: file_type_icon,
    formatTime: window.format_time,
    formatBytes: format_bytes,
    contentMediaAssets: content_media_assets,
  };

  const state = {
    detail_id: detail_id_,
    detail: detail_,
    loading: loading_,
    error: error_,
  };
  const ui = {};

  return { state, ui, methods };
}

export { ContentDetailViewModel, task_status };
