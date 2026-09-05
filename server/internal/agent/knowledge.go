// Package agent 承载 AI 分身（chat 页）相关代码。
//
// S12 阶段落地三块能力：
//  1. 内置知识种子（knowledge.go）：只读访问入口，knowledge.md 经 go:embed
//     进二进制，由 prompt.go 拼进 system prompt（不建知识库、不向量化）；
//  2. 直接 Eino 工具（tools.go）：search_articles / get_article / list_projects，
//     用 utils.InferTool 从类型化参数推断 JSON Schema，不经过 MCP，
//     只暴露已发布内容，供对话编排（agent.go）挂载；
//  3. 对话服务（agent.go / prompt.go）：agent.Service 稳定接口 + ReAct 智能体
//     （eino-ext openai ChatModel + react.NewAgent，MaxStep=8、流式、历史取
//     最近 4 条），系统提示由人设 + 内置知识 + 硬性规则组成、不含用户问题；
//     agenttest.Fake 提供 API 层测试替身，HTTP 路由见 internal/api/chat.go。
package agent

import _ "embed"

//go:embed knowledge.md
var seedKnowledgeMD string

// SeedKnowledge 返回内置知识种子（knowledge.md 原文，UTF-8 Markdown）。
// 由 prompt.go 直接拼进 system prompt；S12 不做切块/向量化/入库。
func SeedKnowledge() string {
	return seedKnowledgeMD
}
