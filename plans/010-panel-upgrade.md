# 010-面板在线升级功能

## 概述

在管理面板中添加在线升级功能，通过 Web UI 直接升级面板（backend + frontend + postgres 容器）。

## 核心交互流程

1. 用户访问任意页面 → VersionFooter 显示升级按钮（常显）
2. 如有新版本：显示 3.0.0 → v3.1.0  [升级]
3. 如无新版本：显示 3.0.0  [升级]
4. 点击升级 → 弹出确认对话框（显示版本信息 + 升级说明）
5. 确认 → 后台执行升级 → 提示升级已提交，面板将自动重启

## VersionFooter 显示效果

**无更新时：**
`
v3.0.0  [升级]
Powered by Flvx
`

**有新版本时：**
`
v3.0.0 → v3.1.0  [升级]
Powered by Flvx
`

## 任务清单

### Phase 1: 后端基础设施

- [x] 1.1 修改 go-backend/Dockerfile - 安装 docker CLI
- [x] 1.2 修改 docker-compose-v4.yml 和 v6.yml - 挂载 docker.sock 和 /opt/flux_panel
- [x] 1.3 在 config.go 中添加 FLUX_VERSION 环境变量读取

### Phase 2: 后端 API 实现

- [x] 2.1 panelUpgradeCheck - POST /api/v1/panel/upgrade/check
- [x] 2.2 panelReleases - POST /api/v1/panel/upgrade/releases  
- [x] 2.3 panelUpgrade - POST /api/v1/panel/upgrade（核心，异步执行）
- [x] 2.4 在 handler.go Register 中注册新路由

### Phase 3: 前端 UI

- [x] 3.1 新增 API 调用函数（vite-frontend/src/api/index.ts）
- [x] 3.2 修改 VersionFooter 组件 - 添加升级按钮（常显）
- [x] 3.3 升级确认弹窗组件
- [x] 3.4 升级进度提示（Toast）

### Phase 4: 测试与优化

- [ ] 4.1 测试 SQLite 模式升级
- [ ] 4.2 测试 PostgreSQL 模式升级
- [ ] 4.3 测试升级失败回滚
- [ ] 4.4 优化错误处理和用户提示

## 升级流程（后端异步执行）

1. 确定目标版本
2. 记录当前版本号（用于回滚）
3. 下载新的 docker-compose.yml
4. 更新 .env 中的 FLUX_VERSION
5. docker compose pull（backend + frontend + postgres）
6. docker compose down（优雅停止，SIGTERM + 30s 超时）
7. docker compose up -d
   - 如使用 postgres，先启动 postgres 并等待 healthy
   - 再启动 backend 和 frontend
8. 等待 backend healthy check 通过
9. 失败则恢复 .env 中的旧版本号 + docker compose up -d

## 文件变更清单

| 文件 | 变更 |
|------|------|
| go-backend/Dockerfile | 安装 docker CLI |
| docker-compose-v4.yml | 挂载 docker.sock |
| docker-compose-v6.yml | 挂载 docker.sock |
| go-backend/internal/config/config.go | 添加 FLUXVersion 字段 |
| go-backend/internal/http/handler/upgrade.go | 新增 3 个 handler + 辅助函数 |
| go-backend/internal/http/handler/handler.go | 添加 fluxVersion 字段 + 注册新路由 |
| go-backend/internal/app/app.go | 传递 fluxVersion 到 Handler |
| vite-frontend/src/api/index.ts | 新增面板升级 API 调用 |
| vite-frontend/src/components/version-footer.tsx | 添加升级按钮（常显）+ 弹窗 |

## 设计说明

**升级按钮常显的原因：**
- 支持删除指定版本重新打包发布的场景
- 用户可以主动触发重新拉取镜像
- 即使没有新版本提示，也能强制升级到最新镜像

**版本选择逻辑：**
- 弹窗中提供版本选择器，可选择指定版本升级
- 留空则默认升级到当前通道的最新版本
- 版本列表从 GitHub Releases API 获取，支持稳定版/开发版切换
