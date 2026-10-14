package services

import (
	"errors"
	"fmt"
	"strings"

	"knowbr/server/internal/content"
	"knowbr/server/internal/models"
)

// ErrHintLevelOutOfRange 表示请求的提示等级不在 1—3 之间。
var ErrHintLevelOutOfRange = errors.New("提示等级必须是 1、2 或 3")

// ErrUnknownArchetype 表示诊断结果没有可用的提示模板。
var ErrUnknownArchetype = errors.New("未找到对应的提示模板")

// HintRequest 描述一次提示生成请求。
type HintRequest struct {
	Level              int
	Diagnosis          Diagnosis
	Metrics            *MatrixMetrics
	Existing           []models.Hint
	State              models.HintState
	RequestedByStudent bool
}

// BuildHint 生成指定等级的提示，并推进提示状态。
// 环境类问题（E0）不使用苏格拉底式提问，直接给出可执行的排查步骤。
func BuildHint(req HintRequest) (models.Hint, models.HintState, error) {
	if req.Level < 1 || req.Level > 3 {
		return models.Hint{}, req.State, ErrHintLevelOutOfRange
	}
	state := req.State
	if state.MaxLevel == 0 {
		state.MaxLevel = 3
	}
	// 同一等级已提供过则直接返回原提示，避免重复记录。
	for _, h := range req.Existing {
		if h.Level == req.Level {
			return h, state, nil
		}
	}

	archetype, ok := archetypeByCode(req.Diagnosis.PrimaryArchetype)
	socratic := true
	var body string
	switch {
	case ok && archetype.DirectTroubleshooting:
		socratic = false
		body = pickHint(archetype.Hints, req.Level)
	case ok:
		body = pickHint(archetype.Hints, req.Level)
	default:
		body = fallbackHint(req.Level, req.Metrics)
	}
	if note := personalNote(req.Level, req.Metrics, req.Diagnosis); note != "" {
		body = body + "\n" + note
	}

	hint := NewHint(req.Level, body, req.Diagnosis.PrimaryArchetype, socratic, req.RequestedByStudent)
	state.UsedLevels = appendUniqueInt(state.UsedLevels, req.Level)
	state.NextLevel = req.Level + 1
	if state.NextLevel > state.MaxLevel {
		state.NextLevel = state.MaxLevel
	}
	return hint, state, nil
}

func pickHint(h content.HintSet, level int) string {
	switch level {
	case 1:
		return h.Level1
	case 2:
		return h.Level2
	default:
		return h.Level3
	}
}

// fallbackHint 用于诊断结论为“证据不足”或未命中内置规则时的提示。
func fallbackHint(level int, m *MatrixMetrics) string {
	switch level {
	case 1:
		return "先把模型给出的结论拆开：这个结论依据的是哪一条证据？请指出你提交材料中最能支持或推翻它的一项数据。"
	case 2:
		return "判断分类模型是否可用，需要三样东西：类别分布（各类别样本数）、按类别分解的指标（各类别召回率/精确率）、以及任务目标（哪种错误代价更高）。请对照你提交的材料，看看这三样里缺了哪一样。"
	default:
		if m != nil {
			return fmt.Sprintf("局部示例：你提交的矩阵总样本 %.0f 个、正确 %.0f 个，整体准确率 %.1f%%。请任选一个类别，用它所在行的正确数除以该行总数得到召回率，写出计算过程，并说明这个数值是否支持你的结论。", m.Total, m.Correct, m.Accuracy*100)
		}
		return "局部示例：设总样本 1000 个，其中某类 10 个，模型把这类样本全部预测成了另一类，那么该类的召回率为 0/10 = 0%，而整体准确率仍可能高达 90%。请用同样的方法计算你实验中的一个类别。"
	}
}

// personalNote 追加一条引用本次矩阵计算结果的说明，使提示与学生证据绑定。
func personalNote(level int, m *MatrixMetrics, diag Diagnosis) string {
	if m == nil {
		return ""
	}
	parts := []string{}
	if len(m.ZeroRecallClass) > 0 {
		parts = append(parts, fmt.Sprintf("本次矩阵中召回率为 0 的类别是：%s", strings.Join(m.ZeroRecallClass, "、")))
	}
	parts = append(parts, fmt.Sprintf("整体准确率 %.1f%%，宏平均 F1 %.3f（均由你提交的矩阵计算得到）", m.Accuracy*100, m.MacroF1))
	if diag.Contradiction.Detected {
		parts = append(parts, "提示：你之前的结论与矩阵数据存在矛盾，请先核对行列含义。")
	}
	return "（引用你提交的数据：" + strings.Join(parts, "；") + "）"
}

func archetypeByCode(code string) (content.Archetype, bool) {
	for _, a := range content.Archetypes {
		if a.Code == code {
			return a, true
		}
	}
	return content.Archetype{}, false
}

// ArchetypeByCode 供接口层展示规则库信息。
func ArchetypeByCode(code string) (content.Archetype, bool) { return archetypeByCode(code) }

func appendUniqueInt(list []int, v int) []int {
	for _, x := range list {
		if x == v {
			return list
		}
	}
	return append(list, v)
}
