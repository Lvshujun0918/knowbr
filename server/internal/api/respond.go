package api

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"

	"knowbr/server/internal/services"
)

// 运行模式标签。
const (
	ModeDemo = "demo"
	ModeRule = "rule"
	ModeLLM  = "llm"
)

// fail 返回统一结构的错误响应，前端据此展示友好提示并保留用户输入。
func fail(c *gin.Context, status int, code, message, detail string) {
	c.JSON(status, gin.H{
		"ok": false,
		"error": gin.H{
			"code":    code,
			"message": message,
			"detail":  detail,
		},
	})
}

func failValidation(c *gin.Context, err error) {
	if ve, ok := err.(*services.ValidationError); ok {
		c.JSON(http.StatusBadRequest, gin.H{
			"ok": false,
			"error": gin.H{
				"code":    ve.Code,
				"field":   ve.Field,
				"message": ve.Message,
				"detail":  ve.Detail,
			},
		})
		return
	}
	fail(c, http.StatusBadRequest, "BAD_REQUEST", err.Error(), "")
}

func notFound(c *gin.Context) {
	fail(c, http.StatusNotFound, "NOT_FOUND", "学习记录不存在或已被删除。", "")
}

// modeInfo 返回模式标签与诚实说明文本。
// configured 表示服务端是否配置了大模型 API Key，fallback 非空表示已尝试调用但失败了。
func modeInfo(mode string, llmUsed bool, fallback, model string, configured bool) (label, notice string) {
	suffix := ""
	switch {
	case fallback != "":
		suffix = "检测到已配置大模型，但本次调用未成功（" + fallback + "），已自动回退到规则引擎，你提交的内容不会因此丢失。"
	case !configured:
		suffix = "本次未配置大模型 API Key，全部结论由内置规则与程序计算生成。"
	case !llmUsed:
		suffix = "本次按要求跳过了大模型调用，诊断与评分由内置规则与程序计算生成。"
	}
	switch mode {
	case ModeLLM:
		return "大模型模式", "本次诊断的自然语言表达由大模型（" + model + "）组织；诊断类别、指标数值与评分仍来自规则引擎与程序计算，未由模型改动。"
	case ModeDemo:
		return "演示模式（规则引擎）", "当前使用预置模拟案例与内置规则引擎运行。" + suffix
	default:
		return "规则模板模式", "当前运行在规则模板模式：材料检查、诊断、分层提示与迁移评分全部由内置规则生成。" + suffix
	}
}

// appModeInfo 返回首页展示的应用级运行模式标签与说明。
func appModeInfo(llm interface{ Available() bool }) (label, notice string) {
	if llm != nil && llm.Available() {
		return "大模型模式（已配置）", "检测到服务端已配置大模型 API Key：诊断与反馈的自然语言表达会尝试由大模型组织；指标数值、诊断类别与迁移评分仍由程序与规则引擎计算。任何调用失败都会自动回退到规则模板，已填写的内容不会丢失。"
	}
	return "规则模板模式", "未检测到大模型 API Key：材料检查、诊断、分层提示与迁移评分全部由内置规则完成，功能完整可用，不依赖外部服务。"
}

func rawOrNil(s string) json.RawMessage {
	if s == "" {
		return nil
	}
	return json.RawMessage(s)
}

func decodeJSON[T any](s string) (T, bool) {
	var out T
	if s == "" {
		return out, false
	}
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return out, false
	}
	return out, true
}

func encodeJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
}
