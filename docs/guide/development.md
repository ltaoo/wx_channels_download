---
title: 开发与检查
---

# 开发与检查

在项目根目录，以管理员身份启动终端，然后运行：

```sh
go run main.go
```

## 后端回归检查

在项目根目录运行：

```sh
go test ./...
go vet ./...
```

## 前端回归检查

在 `frontend` 目录运行：

```sh
npm test
npm run lint
```
