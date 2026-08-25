---
title: 工作目录
---

# 工作目录

通过 `workdir` 可以把数据库和用户脚本集中放在单独目录中，避免在程序所在目录产生运行时文件。

默认配置为：

```yaml
workdir: ""
```

默认值为 `""`。未配置或留空时，使用下载器可执行文件所在目录。

如需使用单独的工作目录，可以配置为：

```yaml
workdir: "./workdir"
```

- 可以使用绝对路径，也可以使用相对路径。
- 相对路径以 `config.yaml` 所在目录为基准。
- 目录不存在时，程序会在启动时自动创建。

配置后，与数据库和用户脚本相关的主要文件如下：

```text
workdir/
├── data.db
├── global.js
└── hooks.js
```

- SQLite 数据库默认生成在 `workdir/data.db`。
- 全局用户脚本放在 `workdir/global.js`。
- 下载 Hook 脚本放在 `workdir/hooks.js`。

脚本文件不是必需的；只有需要自定义脚本功能时才需要创建。脚本的具体用法请参考[用户脚本](./script.md)、[下载任务 Hook](../feature/download-hooks.md)和[指定下载的文件名](../feature/filename.md)。

## 自定义文件名或路径

下面这些路径如果使用相对路径，也会以 `workdir` 为基准：

```yaml
workdir: "./runtime"

inject:
  globalScript: "scripts/global.js"

download:
  hooksScript: "scripts/hooks.js"

db:
  filepath: "data-custom.db"
```

对应文件分别位于 `runtime/scripts/global.js`、`runtime/scripts/hooks.js` 和 `runtime/data-custom.db`。

## 迁移已有文件

设置 `workdir` 不会自动移动已有的数据库或脚本。如果项目目录中已有 `data.db`、`global.js` 或 `hooks.js`，请在程序停止后将它们移动到新的工作目录，再重新启动程序。数据库旁如果还有 `data.db-wal` 或 `data.db-shm`，也需要一起移动。
