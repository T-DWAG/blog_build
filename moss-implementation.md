# Moss 驻站桌宠 · 实现说明书（沙盒版）

> 位置：`E:\data\code\blog_build-ui-sandbox`（**沙盒**，原项目 `blog_build-main` 未动）
> 状态：实现已可运行（后端 `:8080` + 前端 `:8000` 均在跑）；**git 未提交**，工作区持有全部改动
> 约束：简洁 / 不破坏现有架构 / 纯前端零后端改动；台词全部 canned（零 LLM 成本）

---

## 0. 一句话概览

在 8 个页面注入 `moss.js`，生成一只**右下角常驻、可拖拽、有状态、能开复古终端弹窗问答**的"之眼"桌宠。它不碰任何现有业务代码，只读公开 API + 以钩子方式增强 chat 页。

- 视觉：白环=本体（永远在），绿瞳=状态（呼吸/巡环/熄灭）
- 问答：单击 Moss → 复古 CRT 磷光终端弹窗 → 走既有 `POST /api/ai/chat` SSE 真实问答
- 状态：online(呼吸眨眼) / think(瞳孔巡环) / sleep(429 熄瞳) / nokey(503) / lost(网络)

---

## 1. 文件改动总览

| 文件 | 类型 | 内容 |
|---|---|---|
| `frontend/moss.js` | 🆕 748 行 | Moss 全部运行时逻辑（下详） |
| `frontend/style.css` | 追加 | 文件**尾部**追加 3 段 `.moss-*`（起始行见 §4），前部原版规则零改动 |
| 8 个 HTML（index/about/posts/projects/guestbook/search/chat/article） | +1~2 行 | `<head>` 引 `moss.js`（about 另补引 `api-base.js`） |
| `frontend/chat.html` | +150/-4 | chat 页内联的 MOSS 层（问候/彩蛋/SSE 钩子，见 §3） |
| `frontend/404.html` | +1 行 | Moss 台词彩蛋一句 |

合计约 `+519 / -4` 行；`-4` 均为原版两行问候文案/两行旧错误分支的替换，无逻辑删除。

---

## 2. `frontend/moss.js` 代码地图（748 行）

> 结构：单个 IIFE，模块私有；对外只暴露 `window.Moss`（行 400）。

### 2.1 常量与台词库（§）
- 行 21–27：常量 — `mossSnooze`(429 补丁 key)、`mossPetPos`(拖放位置 key)、探活超时 4s、打盹阈值 75s
- 行 30–36：`STATE_TXT` — 各状态对外文案（休眠/未通电/失联…）
- 行 38–74：`ROOM` — 按页面"房间"的台词表（index/about/posts/projects/guestbook/search/chat/article）
- 行 75–80：`GENERIC` — 兜底台词

### 2.2 之眼与基本表达
- 行 94–110：`orbSVG(size)` — 11×11 像素网格生成白环(半径2.7–4.7)+绿瞳(≤1.6)，尺寸=11 的倍数
- 行 111–133：`say()` 气泡 / `wink()` 长眨眼 / `talk()` 说话（当前房间台词）

### 2.3 桌宠载体（DOM/拖拽/眼神/打盹/深夜）
- 行 134：`SIZE` 桌面 44px / 窄屏(<700) 33px
- 行 140–170：`updateRect/applyPos/home` — 位置缓存、sessionStorage 恢复、回岗
- 行 172–199：`setIris/clearIris/onPointerMove` — **眼神跟随鼠标**（限 online，向量限幅 1.8 格，60ms 节流；指针凑近 <150px 偶尔眨眼）
- 行 200–257：`bindDrag` — pointer 拖拽：`moss-press` 下压瞪眼 → 慢放留下(记 `mossPetPos`+小回弹) / 快甩(`moss-toss` 转圈) `home()` 飞回；拖动后抑制 click
- 行 258–275：`bindClickTalk` — **单击 → `openMossDialog()` 复古弹窗**；双击 → 回岗；Enter/Space 同单击
- 行 276–304：`bindDoze` — 空置 75s → 瞳孔半闭打盹；pointer/key/scroll 活动即醒，醒时先眨眼
- 行 305–309：`isNight()` 0–6 点判定
- 行 310–341：`buildPet()` — 惰性挂载 `#mossPet` DOM（`moss-pet` 容器 + 板 `.moss-plate` + 眼 `.moss-orb` + z + 气泡 `#mossBubble`）；注册全部交互；`role=button` 可键盘

### 2.4 状态机与后端语义
- 行 342–357：`setState(st)` — 切 `html[data-moss=...]`（动画全在 CSS），更新 aria/title，`dispatchEvent('moss:state')`
- 行 358–383：`nextMonthFirst/snoozeActive/doSnooze/wake` — **429 跨页补丁**：写 `localStorage.mossSnooze=次月1日0点`，未到期任何页都休眠；到期自动清除回在线
- 行 384–399：`probe()` — 页面加载探活 `GET /api/ai/suggestions`：200→online / 503→nokey / 网络失败→lost

### 2.5 对外 API（行 400–421）
```js
window.Moss = {
  setState,          // chat 页 SSE 钩子驱动
  getState, orb, wake, doSnooze, snoozeActive,
  say, wink, talk    // 供 chat 彩蛋/气泡复用
};
```

### 2.6 复古直连弹窗（行 423–727）
- 行 423–435：默认建议问题 + `DLG_EGGS` 本地彩蛋（whoami/ls/你是真人吗/能毁灭人类吗/天气）
- 行 437–480：`dlgLine/dlgPending/dlgFinish/dlgScroll` — 消息渲染（textContent 防注入）
- 行 482–495：`openMossDialog/closeMossDialog` — 打开/关闭（关时 abort 进行中的流）
- 行 496–556：`buildDlg()` — **惰性构建弹窗 DOM**（首次单击才生成）；开场白按深夜/白天各一句；关闭方式：`[x]`/ESC/点暗处
- 行 557–580：`dlgChipsInit()` — 快捷提问 chips（读 `/api/ai/suggestions`，空则默认四条）
- 行 581–621：`dlgSend/dlgCancel` — 彩蛋先拦截（不走 LLM）；否则进入真问答流；`■ 停止`
- 行 622–727：`dlgStream/dlgReadSSE/dlgFrame` — **复用现有后端协议**的 SSE 迷你客户端：`tool` 帧→`setState('think')`（宠物同步巡环）；`message` 首字→online；`done`→收尾入历史；`error code 429/503`→ 台词化+状态切换
- 行 728+：`init()` — DOMContentLoaded 挂载宠物；URL `?moss=online|think|sleep|nokey|lost` 强制预览状态

---

## 3. `frontend/chat.html` 的 MOSS 层（内联，标注见行 108 注释）

| 位置(行) | 代码/业务 |
|---|---|
| 13 | `<script src="moss.js">` |
| 50 | 窗头文案 `MOSS · ~/chat`（原 `t-dwag@blog: ~/chat` 换装） |
| 52 | 首条问候占位 `<span id="mossFirstLine">` |
| 108–120 | `moss = window.Moss` 引用 + idle 计时变量 |
| 118–128 | `mossGreet()` — 深夜(00–06) / 第 1、3 次复访 / 常规，四款问候 |
| 131–157 | `EGGS` 彩蛋表 + `eggReply()` 精确匹配 |
| 150–205 | `mossWink/mossBubble/mossWinkSay/mossRest/armIdle` — 复用宠物气泡/长眨眼；`mossRest` 只在非休眠补丁期才回 online |
| 206–209 | 填充问候 + 启动 40s idle（每会话 1 次「去逛资料库了吗？《Eino 那篇》值得看」） |
| 300–310 | `send()` 顶部**彩蛋拦截**（命中即气泡回复，不发 LLM） |
| 352–366 | 流结束/`error`：**429 → `moss.doSnooze()` 台词化「能源耗尽…」**；503 → `moss.setState('nokey')`「尚未通电…」 |
| 420–451 | SSE 帧钩子：`message` 首字→online、`done`→online、`tool`→`setState('think')` 思考态 |
| 515–521 | mascot 点击说话（随机金句）+ 输入时重置 idle |

---

## 4. `frontend/style.css` 追加段（文件尾部，行 851 起）

| 起始行 | 段落 | 内容 |
|---|---|---|
| 851 | **MOSS 住客 · 桌宠** | `:root` 锁定动画参数（眨眼 3.4s/呼吸 5.5s/巡环 1.6s）；`.moss-pet` 容器(固定右下)、`.moss-plate` 托盘(黑底白边硬阴影)、`.moss-z` 深夜 z、气泡、之眼 SVG 结构与三态 CSS、打盹、窄屏适配 |
| 996 | **MOSS 住客 · 交互打磨** | 柔和缓动(transition)、press 下压/瞪眼、settle 回弹、toss 转圈回岗、眼神平滑、hover 轻抬、气泡淡入 |
| 1034 | **MOSS 直连对话框 · CRT 磷光** | 复古弹窗：绿磷光玻璃壳、扫描线(`::after`)、DOS 标题+闪烁光标 `▌`、`[x]` 关钮、消息/chips/输入行全套磷光样式 |

> 全部 `.moss-*` / `.moss-dlg-*` 前缀，无任何全局选择器触碰原版规则。

---

## 5. 业务操作 / 事件 → 状态迁移

| 事件（真实来源） | 动作 | 落点 |
|---|---|---|
| 页面加载，探活 200 | online：呼吸+眨眼 | `moss.js` probe |
| 探活失败/超时 | lost：熄瞳整体转暗 | 同上 |
| chat/弹窗收到 SSE `tool` 帧 | think：瞳孔巡环 + 「正在检索…（调用 X）」 | `chat.html:451` / `moss.js dlgFrame` |
| `message` 首字 | online（正文边收边渲染） | `chat.html:422` / `moss.js dlgFrame` |
| `done` | online 复位 | 同上 |
| `error 429`（预算用尽） | sleep：熄瞳 + 写 `localStorage.mossSnooze` 至次月1日（**全站一致休眠**） | `chat.html:362` / `moss.js doSnooze` |
| `error 503`（未配 Key） | nokey：熄瞳 | `chat.html:366` |
| 单击 Moss | 开 CRT 弹窗（首次惰性构建），问答走真 AI | `bindClickTalk → openMossDialog` |
| 拖动/双击/快甩 | 移动留位（sessionStorage）/回岗/转圈飞回 | `bindDrag` |
| 鼠标靠近 | 眼神跟随；很近偶尔眨眼 | `onPointerMove` |
| 空置 75s / 恢复活动 | 打盹（瞳孔半闭）/惊醒眨眼 | `bindDoze` |
| 深夜 0–6 点 | 趴睡 + z | `isNight → .moss-night` |
| chat 输入彩蛋/40s idle | 本地回复/搭话（零 LLM） | `chat.html MOSS 层` |
| URL `?moss=think` 等 | 强制预览任意状态 | `init()` |

### 存储键
- `localStorage.mossSnooze`：429 补丁截止时间戳（次月 1 日 0 点）
- `sessionStorage.mossPetPos`：桌宠拖放位置 `{x,y}`
- `localStorage.mossVisits`：复访匿名计数（chat 页问候用）

---

## 6. 本地运行与验证

```bash
# 前端（沙盒目录，已有 python 静态服务）
cd E:/data/code/blog_build-ui-sandbox/frontend && python serve.py 8000

# 后端（需 Docker Postgres + DeepSeek Key；已跑）
cd E:/data/code/blog_build-ui-sandbox/server
export DATABASE_URL='postgres://postgres:postgres@localhost:5432/blog?sslmode=disable'
export AI_API_KEY='sk-…'  AI_BASE_URL='https://api.deepseek.com/v1'  AI_MODEL='deepseek-v4-flash'
go run ./cmd/blog
```

演示地址：
- 任意页 → `http://localhost:8000/`，右下角桌宠：单击弹窗 / 拖 / 双击回岗
- 状态预览：`?moss=online|think|sleep|nokey|lost`
- 真问答：弹窗里问任意问题（DeepSeek 预算内）；chat 页同链路

> 注意：本地库 projects/articles 是早期乱码 demo 数据，属数据问题，与 Moss 无关。

---

## 7. 合回原项目（待用户确认后执行）

1. 沙盒 git 已含 baseline（=原版）与全部工作区改动
2. 需迁移文件：`moss.js`（新）+ `style.css` 尾部 3 段（851–文件尾）+ 8 页各 1 行 script + `chat.html` 的 MOSS 层 + `404.html` 1 行
3. 建议：确认满意后从沙盒导出 `moss.patch` → 在原项目 `git apply --check` 后应用；或直接复制文件并逐页补引

---

## 8. 可调点

| 参数 | 位置 | 当前值 |
|---|---|---|
| 桌宠尺寸 | `moss.js:134` | 44（窄屏 33） |
| 打盹阈值 | `moss.js:25` | 75s |
| 探活超时 | `moss.js:24` | 4s |
| 眨眼/呼吸/巡环 | `style.css` 桌宠段 `:root` | 3.4s / 5.5s / 1.6s |
| 房间台词 | `moss.js ROOM` 行 38–74 | 可随意改文案 |
| 弹窗彩蛋 | `moss.js DLG_EGGS` 行 424 | 可增删 |
