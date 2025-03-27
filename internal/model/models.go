/**
  @author: Hanhai
  @since: 2025/3/27 00:11:34
  @desc:
**/

package model

import (
	"time"
)

// Model 模型信息
type Model struct {
	ID         string     `json:"id"`          // 模型ID
	IsFree     bool       `json:"is_free"`     // 是否免费
	IsGiftable bool       `json:"is_giftable"` // 是否可用赠费
	StrategyID int        `json:"strategy_id"` // 模型使用的策略ID
	Type       int        `json:"type"`        // 模型类型：1-对话，2-生图，3-视频，4-语音，5-嵌入，6-重排序，7-推理
	CreatedAt  time.Time  `json:"created_at"`  // 创建时间
	UpdatedAt  time.Time  `json:"updated_at"`  // 更新时间
	DeletedAt  *time.Time `json:"deleted_at"`  // 删除时间（软删除）
}

// TableName 指定表名
func (Model) TableName() string {
	return "models"
}

// FreeModels 免费模型列表
var FreeModels = []string{
	"deepseek-ai/DeepSeek-R1-Distill-Qwen-7B",
	"deepseek-ai/DeepSeek-R1-Distill-Qwen-1.5B",
	"Qwen/Qwen2.5-7B-Instruct",
	"Qwen/Qwen2.5-Coder-7B-Instruct",
	"Qwen/Qwen2-7B-Instruct",
	"Qwen/Qwen2-1.5B-Instruct",
	"THUDM/chatglm3-6b",
	"BAAI/bge-m3",
	"BAAI/bge-reranker-v2-m3",
	"BAAI/bge-large-zh-v1.5",
	"BAAI/bge-large-en-v1.5",
	"netease-youdao/bce-embedding-base_v1",
	"netease-youdao/bce-reranker-base_v1",
	"internlm/internlm2_5-7b-chat",
}

// GiftableModels 可用赠费模型列表
var GiftableModels = []string{
	"deepseek-ai/DeepSeek-R1",
	"deepseek-ai/DeepSeek-V3",
	"deepseek-ai/DeepSeek-V2.5",
	"deepseek-ai/deepseek-vl2",
	"deepseek-ai/DeepSeek-R1-Distill-Qwen-32B",
	"deepseek-ai/DeepSeek-R1-Distill-Qwen-14B",
	"Pro/deepseek-ai/DeepSeek-R1-Distill-Qwen-7B",
	"Pro/deepseek-ai/DeepSeek-R1-Distill-Qwen-1.5B",
	"Qwen/QwQ-32B",
	"Qwen/QwQ-32B-Preview",
	"Qwen/Qwen2.5-VL-72B-Instruct",
	"Qwen/Qwen2.5-72B-Instruct-128K",
	"Qwen/Qwen2.5-72B-Instruct",
	"Qwen/Qwen2.5-32B-Instruct",
	"Qwen/Qwen2.5-14B-Instruct",
	"Qwen/Qwen2.5-Coder-32B-Instruct",
	"Qwen/QVQ-72B-Preview",
	"Qwen/Qwen2.5-VL-7B-Instruct",
	"Pro/Qwen/Qwen2.5-Coder-7B-Instruct",
	"Pro/Qwen/Qwen2-VL-7B-Instruct",
	"Pro/Qwen/Qwen2.5-7B-Instruct",
	"Pro/Qwen/Qwen2-7B-Instruct",
	"Pro/Qwen/Qwen2-1.5B-Instruct",
	"internlm/internlm2_5-20b-chat",
	"TeleAI/TeleChat2",
	"Pro/THUDM/glm-4-9b-chat",
	"Pro/BAAI/bge-m3",
	"Pro/BAAI/bge-reranker-v2-m3",
}

// 推理模型列表
var ReasonModels = []string{
	"Qwen/QwQ-32B-Preview",
	"Qwen/QwQ-32B",
	"deepseek-ai/DeepSeek-R1",
	"deepseek-ai/DeepSeek-R1-Distill-Qwen-14B",
	"deepseek-ai/DeepSeek-R1-Distill-Qwen-32B",
	"deepseek-ai/DeepSeek-R1-Distill-Llama-70B",
	"deepseek-ai/DeepSeek-R1-Distill-Qwen-1.5B",
	"deepseek-ai/DeepSeek-R1-Distill-Qwen-7B",
	"deepseek-ai/DeepSeek-R1-Distill-Llama-8B",
	"Pro/deepseek-ai/DeepSeek-R1-Distill-Qwen-1.5B",
	"Pro/deepseek-ai/DeepSeek-R1-Distill-Qwen-7B",
	"Pro/deepseek-ai/DeepSeek-R1-Distill-Llama-8B",
	"Pro/deepseek-ai/DeepSeek-R1",
}
