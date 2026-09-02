# blog_build

站主个人博客的完整实现仓库：公开站（文章 / 项目 / 留言 / 搜索 / **AI 分身**）+ Go 后端 + PostgreSQL 数据层，按「Obsidian 契约文档」分步实现（S01–S12，当前完成到 S12 AI 分身）。

```
frontend/   公开站（纯静态 HTML/CSS/JS，无构建步骤，可直接放 GitHub Pages）
server/     Go 后端：公开 API + 管理台（HTML embed 进二进制）+ Eino AI 分身
```

## 架构

```text
浏览器（公开站 :8000）──api-base.js──► 后端 API + 管理台（:8080）
                                            │
                                      PostgreSQL 16 + pgvector（:5432）
                                            │
                                    articles / projects / messages /
                                    settings（persona、AI 预算）…
```

- 后端 Gin；管理台页面、静态资源、`internal/agent/knowledge.md`、schema 全部 `go:embed` 进二进制。
- 公开站纯静态，页面里的 `frontend/api-base.js` 决定后端地址：`localhost` 访问 → `http://127.0.0.1:8080`；其它域名（GitHub Pages 等）→ 部署时把它改成你的后端公网地址。
- AI 分身：Eino `react` agent + **OpenAI 兼容** ChatModel（`BaseURL`/`Model`/`APIKey` 三项可配，默认 DeepSeek）+ 3 只内嵌只读工具（`search_articles` / `get_article` / `list_projects`，直接查库，**不经 MCP**）。

## 功能域

| 域 | 说明 |
|---|---|
| 管理台 | 登录（JWT + 失败锁定）、文章/项目管理、留言审核、设置 |
| 文章 / 项目 | Markdown 正文，标签、置顶、发布/草稿状态 |
| 留言板 | 提交后待审核，站主通过才对外可见（防垃圾） |
| 搜索 | 文章标题/正文 + 项目名，标签聚合 |
| AI 分身 | 访客问答：预置知识（knowledge.md embed + 管理台 persona）→ 需要时调工具查库 → 附来源回答；月度用量预算 |

## 快速启动（本机联调）

### 0. 前置

- Go（`server/go.mod` 声明 `go 1.25.9`）
- Docker（起数据库用）

### 1. 数据库

```bash
cd server
docker compose up -d db      # pgvector/pgvector:pg16，库 blog，用户 postgres/postgres
```

### 2. 后端（API + 管理台）

```bash
cd server
export DATABASE_URL='postgres://postgres:postgres@localhost:5432/blog?sslmode=disable'
export JWT_SECRET='换成长随机串'
export ADMIN_USERNAME='admin'
export ADMIN_PASSWORD='换成强密码'
# AI 三项可留空：服务照常启动，仅 AI 对话返回 503
export AI_API_KEY=''                       # OpenAI 兼容服务的密钥
export AI_BASE_URL='https://api.deepseek.com'   # 默认 DeepSeek；可换任意兼容端点
export AI_MODEL='deepseek-v4-flash'        # 默认 deepseek-chat

go run ./cmd/blog
```

- 首次启动自动建表并种子管理员与设置（含 AI 预算 `settings.ai.budget_per_month=10`）。
- 验证：`curl http://localhost:8080/healthz` → `{"code":0,...}`。
- 管理台：`http://localhost:8080/admin`（页面在二进制里，无需单独起）。

### 3. 前端

```bash
cd frontend
python3 -m http.server 8000    # 或 python3 serve.py [port] / nginx / GitHub Pages
```

浏览器打开 `http://localhost:8000/chat.html`（AI 分身）、`guestbook.html`（留言板）等。

## 配置（环境变量）

| 变量 | 必填 | 说明 |
|---|---|---|
| `DATABASE_URL` | 是 | Postgres 连接串 |
| `JWT_SECRET` | 是 | 签名密钥，生产务必随机长串 |
| `ADMIN_USERNAME` / `ADMIN_PASSWORD` | 是 | 管理台账号；启动时幂等种子 |
| `ADDR` | 否 | 监听地址，默认 `:8080` |
| `AI_API_KEY` | 否 | 留空：服务可起，`/api/ai/chat` 返回 503 |
| `AI_BASE_URL` | 否 | OpenAI 兼容端点，默认 `https://api.deepseek.com` |
| `AI_MODEL` | 否 | 默认 `deepseek-chat` |

> 换 AI 供应商只改 `AI_BASE_URL` + `AI_MODEL`，不改代码。密钥一律走环境变量/`.env`，**不落库、不进 git**（`PutSetting` 会拒绝含 `api_key`/`secret` 的设置）。

## AI 分身设计要点（S12）

- 材料只来自两块：**预置知识**（`server/internal/agent/knowledge.md`，`go:embed`，站主身份/技术栈/回答规则）+ **管理台 persona**（`settings.persona`，可热改）+ 查库结果。材料没有的明确说「不知道」，回答附来源。
- 3 只工具是 Eino `utils.InferTool` 包出的普通 Go 函数直查 `articles`/`projects`，只暴露已发布内容；无 MCP、无端口、无独立进程。
- 额度：`settings.ai.budget_per_month`（默认 10），按月计数 `settings.ai_usage`，跨月自动重置，事务 + 行锁保证并发原子；用尽返回 429 且不调模型。
- 修改 `knowledge.md` / 管理台 HTML 后需**重新构建后端**才生效（embed），前端刷新无效。
- 未配 Key 不阻塞启动：`agent.New` 返回 `ErrNoAPIKey`，main 记录日志并以 `ag=nil` 继续，`/api/ai/chat` → 503。

## 测试

数据库需在运行：

```bash
cd server
make test-s1 ... make test-s12   # 每步验收对应测试集；当前进度 test-s12
```

- 大部分测试连真实 Postgres（`DATABASE_URL` 缺省 `localhost:5432/blog`），部分会清理业务表，**不要在含真实数据的库上跑**。
- `internal/compose` 的端到端测试会真实 `docker compose up`，要求 8080/5432 空闲。

## 生产部署

- 后端：`cd server && docker compose up -d --build`（多阶段镜像，静态链接，只跑二进制）。
- 公开站：`frontend/` 推 GitHub Pages 或任意静态托管；把 `api-base.js` 的非 localhost 分支改成你的后端公网地址（`https`）。
- 后端 CORS 白名单在 `server/internal/api/server.go` 的 `allowedOrigins`，线上域名需加进去。
- 上线前在管理台把 persona、AI 预算、suggestions 配好。

## 常见问题

- **留言不显示**：提交后是 `pending`，需管理台 `/admin/messages` 审核通过才公开（设计如此）。
- **WSL2 联调**：Windows 浏览器访问 WSL 内服务时若 localhost 转发失效，用 WSL IP 访问，并把 `api-base.js` 临时指向 `http://<WSL-IP>:8080`。
- **DeepSeek 模型名**：可用 `curl https://api.deepseek.com/models -H "Authorization: Bearer $AI_API_KEY"` 查询；当前有 `deepseek-v4-flash` / `deepseek-v4-pro` 等。
