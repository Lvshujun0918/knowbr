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
// LatestClaims 是学生最新一次回答的主张分析结果，用于避免重复追问已经掌握的内容。
type HintRequest struct {
	Level              int
	Diagnosis          Diagnosis
	Metrics            *MatrixMetrics
	Existing           []models.Hint
	State              models.HintState
	RequestedByStudent bool
	TaskType           string
	LatestAnswer       string
	LatestClaims       []Claim
	LogTail            string
}

// BuildHint 生成指定等级的提示，并推进提示状态。
// 环境类问题（E0）不使用苏格拉底式提问，直接给出可执行的排查步骤。
// 每一级提示都会说明它针对哪个知识点、为什么这样选择，并与学生最新回答绑定。
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
	if req.TaskType == "" {
		req.TaskType = req.Diagnosis.TaskType
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
		body = fallbackHint(req.Level, req.Metrics, req.TaskType)
	}

	// 依据学生回答调整：已掌握的部分不再重复追问，只处理仍未解决的一点。
	// 有修正记录时以最新回答为准，否则回到诊断阶段的主张分析。
	lead, target, claimID, targetQuestion := latestAnswerContext(req)
	if lead != "" {
		body = lead + "\n" + body
	}
	// 没有命中内置规则时，一级提示直接抛出针对该知识点的问题，而不是通用引导。
	if !ok && req.Level == 1 && targetQuestion != "" {
		body = lead + "\n" + targetQuestion + "\n" + fallbackHint(req.Level, req.Metrics, req.TaskType)
	}
	if note := personalNote(req); note != "" {
		body = body + "\n" + note
	}

	hint := NewHint(req.Level, body, req.Diagnosis.PrimaryArchetype, socratic, req.RequestedByStudent)
	hint.TargetKnowledge = target
	hint.AddressesClaim = claimID
	hint.Why = hintReason(req, target)
	state.UsedLevels = appendUniqueInt(state.UsedLevels, req.Level)
	state.NextLevel = req.Level + 1
	if state.NextLevel > state.MaxLevel {
		state.NextLevel = state.MaxLevel
	}
	return hint, state, nil
}

// latestAnswerContext 依据学生回答说明“已经解决了什么、本轮只处理什么”，
// 并返回本轮需要处理的知识点、对应主张与针对该知识点的追问。
func latestAnswerContext(req HintRequest) (lead, target, claimID, targetQuestion string) {
	claims := req.LatestClaims
	if len(claims) == 0 {
		claims = req.Diagnosis.Claims
	}
	if len(claims) == 0 {
		return "", "", "", ""
	}
	var mastered, unresolved []Claim
	for _, c := range claims {
		switch c.Status {
		case ClaimMastered:
			mastered = append(mastered, c)
		case ClaimError, ClaimPartial, ClaimInsufficient:
			unresolved = append(unresolved, c)
		}
	}
	if len(mastered) > 0 {
		names := make([]string, 0, len(mastered))
		for _, c := range mastered {
			names = append(names, c.KnowledgePoint)
		}
		lead = fmt.Sprintf("先确认：你已经解决了「%s」，这部分不再重复追问。", strings.Join(dedupe(names), "、"))
	}
	if len(unresolved) == 0 {
		if lead != "" {
			lead += " 当前学习目标已经达到，可以直接进入修正验证或迁移测试。"
		}
		return lead, "", "", ""
	}
	target = unresolved[0].KnowledgePoint
	if lead != "" {
		lead += fmt.Sprintf(" 本轮只处理剩下的一点：「%s」。", target)
	} else {
		lead = fmt.Sprintf("本轮只处理一点：「%s」。", target)
	}
	return lead, target, unresolved[0].ID, unresolved[0].NextQuestion
}

func hintReason(req HintRequest, target string) string {
	var parts []string
	switch req.Level {
	case 1:
		parts = append(parts, "一级提示用启发性问题定位你推理中的缺口")
	case 2:
		parts = append(parts, "二级提示解释你还没接上的概念联系")
	default:
		parts = append(parts, "三级提示给出与当前实验直接相关的局部示例")
	}
	if target != "" {
		parts = append(parts, "本轮针对的知识点："+target)
	}
	if len(req.LatestClaims) > 0 {
		parts = append(parts, "本提示依据你最新一次回答的内容生成")
	}
	if req.TaskType != "" {
		parts = append(parts, "任务类型："+content.TaskTypeLabel(req.TaskType))
	}
	return strings.Join(parts, "；") + "。"
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
// 提示内容按当前任务类型生成，不会把其它任务的知识点搬进来。
func fallbackHint(level int, m *MatrixMetrics, taskType string) string {
	switch taskType {
	case content.TaskTraining:
		switch level {
		case 1:
			return "先从日志里挑出异常出现的那一轮：这一轮相比上一轮发生了什么变化？请分别写出三行——日志记录的事实、你的推断、以及日志里查不到的信息。"
		case 2:
			return "梯度裁剪限制单步更新的幅度上限，学习率决定每次更新的基准尺度：两者都可能抑制发散，但改变的不是同一件事。要判断是哪一个在起作用，需要单变量对照：固定其它设置，分别只调学习率、只加裁剪，再比较损失与梯度范数的曲线；同时确认损失与输入中是否出现非有限值。"
		default:
			return "局部示例（训练稳定性）：若某轮梯度范数从 18.9 涨到 95.3、下一轮损失变成 NaN，能确定的事实是“该轮出现了数值异常”，不能确定的是它由梯度爆炸、损失或数据中的非有限值、还是实现错误引起。最小验证：把学习率降为原来的十分之一再跑一次——若仍然出现 NaN，说明问题不只是学习率偏大；在训练循环里加一行断言检查数值是否为有限值，可以排除数据或实现问题。"
		}
	case content.TaskArchitecture:
		switch level {
		case 1:
			return "先把“谁更准”和“为什么更准”分开：你观察到的是两个模型在测试集上的差距，还是它们结构上的差异？请分别写出一句。"
		case 2:
			return "MLP 需要把图像展平成一维向量，相邻像素的空间关系不再被结构显式利用；CNN 通过局部连接与权重共享提取局部特征，参数随通道数增长。容量更大只说明拟合能力上限更高，是否真的更好要看训练集与测试集上的表现和参数量。"
		default:
			return "局部示例（结构比较）：若 CNN 用 3 万个参数达到 92%，而 MLP 用 61 万个参数只有 72%，那么“更复杂所以更准”无法解释这个结果。请用你实验里的参数量与准确率填入同样的对照，再说明结构差异在其中起什么作用。"
		}
	}
	switch level {
	case 1:
		return "先把模型给出的结论拆开：这个结论依据的是哪一条证据？请指出你提交材料中最能支持或推翻它的一项数据。"
	case 2:
		return "判断分类模型是否可用，需要三样东西：类别分布（各类别样本数）、按类别分解的指标（各类别召回率/精确率）、以及任务目标（哪种错误代价更高）。请对照你提交的材料，看看这三样里缺了哪一样。"
	default:
		if m != nil {
			return fmt.Sprintf("局部示例：你提交的矩阵总样本 %.0f 个、正确 %.0f 个，整体准确率 %.1f%%。请任选一个类别，用它所在行的正确数除以该行总数得到召回率，写出计算过程，并说明这个数值是否支持你的结论。", m.Total, m.Correct, m.Accuracy*100)
		}
		return "先不要引入任何你没有提供的数据。请回到你填写的实验结果：它是在哪个数据集上得到的、衡量的是哪一件事？再说明它无法回答哪个问题。"
	}
}

// personalNote 追加一条引用本次提交数据的说明，使提示与学生证据绑定。
// 没有矩阵时不会编造任何数值。
func personalNote(req HintRequest) string {
	parts := []string{}
	if req.Metrics != nil {
		m := req.Metrics
		if len(m.ZeroRecallClass) > 0 {
			parts = append(parts, fmt.Sprintf("本次矩阵中召回率为 0 的类别是：%s", strings.Join(m.ZeroRecallClass, "、")))
		}
		parts = append(parts, fmt.Sprintf("整体准确率 %.1f%%，宏平均 F1 %.3f（均由你提交的矩阵计算得到）", m.Accuracy*100, m.MacroF1))
	}
	if req.TaskType == content.TaskTraining && strings.TrimSpace(req.LogTail) != "" {
		parts = append(parts, fmt.Sprintf("你提交的日志末行：%s（仅引用你提供的内容，未做任何补全）", req.LogTail))
	}
	if req.Diagnosis.Contradiction.Detected {
		parts = append(parts, "提示：你之前的结论与矩阵数据存在矛盾，请先核对行列含义。")
	}
	if len(parts) == 0 {
		return ""
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
