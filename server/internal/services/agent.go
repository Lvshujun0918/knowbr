package services

import (
	"encoding/json"
	"fmt"
	"strings"

	"knowbr/server/internal/content"
	"knowbr/server/internal/models"
)

// 智能体动作：每一步只选择一个动作，并说明为什么。
const (
	AgentAskEvidence     = "ask_evidence"     // 先补齐必需证据，暂不给提示
	AgentGiveHint        = "give_hint"        // 按建议等级给出下一级提示
	AgentVerifyRevision  = "verify_revision"  // 等学生提交修正后的解释
	AgentAdvanceTransfer = "advance_transfer" // 进入迁移测试或完成作答
	AgentEscalate        = "escalate"         // 建议教师介入
	AgentComplete        = "complete"         // 流程已完成
)

var agentActionLabels = map[string]string{
	AgentAskEvidence:     "先补齐必需证据",
	AgentGiveHint:        "给出下一级提示",
	AgentVerifyRevision:  "等待学生的修正解释",
	AgentAdvanceTransfer: "推进迁移测试",
	AgentEscalate:        "建议教师介入",
	AgentComplete:        "本轮学习已完成",
}

var agentActionTitles = map[string]string{
	AgentAskEvidence:     "材料还不足以支撑下一步辅导，先请你补充必需证据",
	AgentGiveHint:        "下一步给你一级更深的提示，而不是直接给答案",
	AgentVerifyRevision:  "请用自己的话重新解释，系统会判断是否有所推进",
	AgentAdvanceTransfer: "用一道新情境题检验能否迁移",
	AgentEscalate:        "规则模式的提示已难以推进，建议教师当面辅导",
	AgentComplete:        "学习闭环已完成",
}

// AgentToolCall 是一次真实执行的工具调用记录。
// Result 是工具的返回摘要，学生与教师可以据此核对“智能体的判断依据是什么”。
type AgentToolCall struct {
	Name    string `json:"name"`
	Purpose string `json:"purpose"`
	Result  string `json:"result"`
}

// AgentPlanItem 是智能体维护的分步计划。
type AgentPlanItem struct {
	Step   string `json:"step"`
	Title  string `json:"title"`
	Status string `json:"status"` // done | current | pending
	Note   string `json:"note,omitempty"`
}

// AgentRepeatedIssue 是跨会话记忆发现的重复问题。
type AgentRepeatedIssue struct {
	Code   string `json:"code"`
	Label  string `json:"label"`
	Count  int    `json:"count"`
	Advice string `json:"advice"`
}

// AgentMemory 是本机历史记录提供的长期记忆。
// 本项目没有账号体系，历史记录按“本机”归并，不能保证属于同一名学生，因此只在单机演示场景下作为参考。
type AgentMemory struct {
	LookedAt   int                  `json:"lookedAt"`
	Repeated   []AgentRepeatedIssue `json:"repeated"`
	Mastered   []string             `json:"mastered"`
	Milestones []string             `json:"milestones"`
	Note       string               `json:"note"`
}

// AgentStep 是智能体在当前状态下决定的下一步行动。
type AgentStep struct {
	Action           string          `json:"action"`
	ActionLabel      string          `json:"actionLabel"`
	Title            string          `json:"title"`
	Reason           string          `json:"reason"`
	Requires         []string        `json:"requires"`
	SuggestHintLevel int             `json:"suggestHintLevel"`
	Blocked          bool            `json:"blocked"`
	BlockedReason    string          `json:"blockedReason,omitempty"`
	CanAct           bool            `json:"canAct"`
	Plan             []AgentPlanItem `json:"plan"`
	Tools            []AgentToolCall `json:"tools"`
	Memory           *AgentMemory    `json:"memory,omitempty"`
	DecidedAt        string          `json:"decidedAt"`
	PolicyNote       string          `json:"policyNote"`
}

// AgentPolicyNote 说明这一步的性质，避免把确定性策略说成自由推理。
const AgentPolicyNote = "本步骤由服务端的确定性策略引擎计算得出：它先运行下方列出的工具，再依据材料完整性、提示与修正记录、以及本机历史记录选择动作；每一项依据都可以在工具轨迹与证据中核对，不是自由生成的判断。"

// AgentInput 是决策所需的全部状态。
type AgentInput struct {
	Status            string
	TaskType          string
	Diagnosis         Diagnosis
	Metrics           *MatrixMetrics
	Hints             []models.Hint
	Revisions         []models.Revision
	HintState         models.HintState
	HasTransfer       bool
	TransferAnswered  bool
	TransferCompleted bool
	History           []models.Session
}

// DecideNextAction 依据当前状态选择下一步动作，并给出可核查的工具轨迹与分步计划。
func DecideNextAction(in AgentInput) AgentStep {
	taskType := in.TaskType
	if taskType == "" {
		taskType = in.Diagnosis.TaskType
	}

	tools := []AgentToolCall{}
	missingRequired := requiredMissingEvidence(in.Diagnosis)
	tools = append(tools, evidenceTool(in, missingRequired))
	if in.Metrics != nil {
		tools = append(tools, metricsTool(in.Metrics))
	} else {
		tools = append(tools, AgentToolCall{
			Name:    "metrics_compute",
			Purpose: "用学生提交的混淆矩阵计算指标",
			Result:  "未提交混淆矩阵，本次不计算任何指标；智能体不会替学生假设类别分布。",
		})
	}
	memory := BuildAgentMemory(in.History)
	tools = append(tools, historyTool(memory))
	tools = append(tools, progressTool(in))

	unresolved := unresolvedClaims(in.Diagnosis)
	repeated := repeatedIssueFor(memory, in.Diagnosis.PrimaryArchetype)

	step := AgentStep{
		Requires:   []string{},
		Plan:       buildAgentPlan(in),
		Tools:      tools,
		Memory:     memory,
		DecidedAt:  nowISO(),
		PolicyNote: AgentPolicyNote,
	}

	switch {
	case in.TransferCompleted || in.Status == models.StatusCompleted:
		step.Action = AgentComplete
		step.Reason = "迁移题已作答并完成本次学习，本轮闭环结束；后续可对比历史记录中的共性问题。"

	case in.HasTransfer && !in.TransferAnswered:
		step.Action = AgentAdvanceTransfer
		step.Reason = "迁移题已经生成，但还没有你的作答。系统不会在你作答前给出参考答案与评分。"

	case in.Status == models.StatusTransfer && !in.HasTransfer:
		step.Action = AgentAdvanceTransfer
		step.Reason = "当前已进入迁移测试阶段，需要生成一道与本次任务类型一致的新情境题。"

	case len(missingRequired) > 0 && len(in.Hints) == 0:
		// 关键约束：证据不足时拒绝直接给出三级示例，先要材料。
		step.Action = AgentAskEvidence
		step.Requires = missingRequired
		step.Blocked = true
		step.BlockedReason = fmt.Sprintf("缺少 %d 项必需证据（%s）。在补齐之前给出具体计算示例，会把示例里的数值误当成你实验的结果。",
			len(missingRequired), strings.Join(firstN(missingRequired, 2), "、"))
		step.Reason = fmt.Sprintf("材料完整性工具判定当前任务（%s）缺少必需证据；智能体决定先补齐证据，再决定提示等级。", content.TaskTypeLabel(taskType))

	case in.HintState.NoProgressRounds >= 2:
		step.Action = AgentEscalate
		step.Reason = fmt.Sprintf("修正记录工具显示连续 %d 轮没有新增可核查的证据点，规则模式的提示已难以继续推进。", in.HintState.NoProgressRounds)

	case len(unresolved) > 0:
		level := in.HintState.NextLevel
		if level < 1 {
			level = 1
		}
		if level > 3 {
			level = 3
		}
		if repeated != nil {
			// 记忆介入：同一误区此前出现过时，从最基础的观察方向重新开始，不再重复概念解释。
			level = 1
			step.Reason = fmt.Sprintf("历史记忆显示该误区此前出现过 %d 次（%s）；本轮不再重复概念解释，改为从一级提示引导你直接对照证据。", repeated.Count, repeated.Label)
		} else if len(in.Hints) == 0 {
			step.Reason = fmt.Sprintf("诊断识别出尚未解决的知识点「%s」，且你还没有使用过提示；先给一级观察方向，由你自己完成关键判断。", unresolved[0].KnowledgePoint)
		} else {
			step.Reason = fmt.Sprintf("你已使用过 %d 级提示且尚未解决「%s」，按逐级加深的约定给出下一级提示。",
				len(in.Hints), unresolved[0].KnowledgePoint)
		}
		step.Action = AgentGiveHint
		step.SuggestHintLevel = level
		step.CanAct = true

	case len(in.Revisions) == 0:
		step.Action = AgentVerifyRevision
		step.Reason = "还没有你的修正解释，系统无法判断提示是否起效；请先用你自己的话重新解释一次。"

	default:
		step.Action = AgentAdvanceTransfer
		step.Reason = "最近一次修正已经提交，且没有未解决的知识点留在辅导环节；用新情境题检验迁移。"
	}

	if step.Reason == "" {
		step.Action = AgentVerifyRevision
		step.Reason = "当前证据状态下没有可执行的推进动作，等待学生继续提交内容。"
	}
	step.ActionLabel = agentActionLabels[step.Action]
	step.Title = agentActionTitles[step.Action]
	return step
}

// requiredMissingEvidence 返回当前任务中必需但尚未提供的材料名称。
func requiredMissingEvidence(diag Diagnosis) []string {
	out := []string{}
	for _, need := range diag.EvidenceNeeds {
		if need.Required && !need.Present {
			out = append(out, need.Name)
		}
	}
	return out
}

func unresolvedClaims(diag Diagnosis) []Claim {
	out := []Claim{}
	for _, c := range diag.Claims {
		switch c.Status {
		case ClaimError, ClaimPartial, ClaimInsufficient:
			out = append(out, c)
		}
	}
	return out
}

func evidenceTool(in AgentInput, missing []string) AgentToolCall {
	present, required := 0, 0
	for _, need := range in.Diagnosis.EvidenceNeeds {
		if need.Required {
			required++
			if need.Present {
				present++
			}
		}
	}
	result := fmt.Sprintf("必需材料 %d/%d 项已提供；", present, required)
	if len(missing) == 0 {
		result += "没有缺失的必需材料，可以据此判定下一步。"
	} else {
		result += "缺失：" + strings.Join(missing, "、") + "。"
	}
	if len(in.Diagnosis.Completeness.Limitations) > 0 {
		result += "限制：" + strings.Join(firstN(in.Diagnosis.Completeness.Limitations, 2), "；") + "。"
	}
	return AgentToolCall{
		Name:    "evidence_check",
		Purpose: "按当前任务类型核对必需证据是否齐全",
		Result:  result,
	}
}

func metricsTool(m *MatrixMetrics) AgentToolCall {
	result := fmt.Sprintf("总样本 %.0f，正确 %.0f，准确率 %.1f%%，宏平均 F1 %.3f，加权平均 F1 %.3f。",
		m.Total, m.Correct, m.Accuracy*100, m.MacroF1, m.WeightedF1)
	if len(m.ZeroRecallClass) > 0 {
		result += fmt.Sprintf("召回率为 0 的类别：%s。", strings.Join(m.ZeroRecallClass, "、"))
	}
	if m.Imbalanced {
		result += fmt.Sprintf("类别不平衡（最大类占比 %.0f%%，不平衡比 %.1f）。", m.MaxClassRatio*100, m.ImbalanceRatio)
	}
	return AgentToolCall{
		Name:    "metrics_compute",
		Purpose: "由程序计算混淆矩阵指标（不经过任何模型）",
		Result:  result,
	}
}

func progressTool(in AgentInput) AgentToolCall {
	improved, none := 0, 0
	for _, r := range in.Revisions {
		switch r.Progress {
		case "improved":
			improved++
		case "none":
			none++
		}
	}
	result := fmt.Sprintf("已提供提示 %d 条（等级 %v）；学生修正 %d 次，其中明确推进 %d 次、无进展 %d 次；连续无进展 %d 轮。",
		len(in.Hints), in.HintState.UsedLevels, len(in.Revisions), improved, none, in.HintState.NoProgressRounds)
	return AgentToolCall{
		Name:    "progress_check",
		Purpose: "检查提示与修正记录，判断是否仍在推进",
		Result:  result,
	}
}

func historyTool(m *AgentMemory) AgentToolCall {
	result := fmt.Sprintf("读取本机历史记录 %d 条；发现重复出现的典型误区 %d 类，学生已体现掌握的知识点 %d 个。",
		m.LookedAt, len(m.Repeated), len(m.Mastered))
	if len(m.Repeated) > 0 {
		parts := make([]string, 0, len(m.Repeated))
		for _, r := range firstRepeated(m.Repeated, 3) {
			parts = append(parts, fmt.Sprintf("%s（%d 次）", r.Label, r.Count))
		}
		result += "重复误区：" + strings.Join(parts, "、") + "。"
	}
	return AgentToolCall{
		Name:    "history_lookup",
		Purpose: "读取本机历史记录，识别重复误区与已掌握内容",
		Result:  result,
	}
}

func firstRepeated(list []AgentRepeatedIssue, n int) []AgentRepeatedIssue {
	if len(list) <= n {
		return list
	}
	return list[:n]
}

func repeatedIssueFor(m *AgentMemory, archetype string) *AgentRepeatedIssue {
	if m == nil || archetype == "" {
		return nil
	}
	for i := range m.Repeated {
		if m.Repeated[i].Code == archetype {
			return &m.Repeated[i]
		}
	}
	return nil
}

// buildAgentPlan 把流程映射为分步计划，并标注每一步的状态。
func buildAgentPlan(in AgentInput) []AgentPlanItem {
	status := in.Status
	hasDiagnosis := in.Diagnosis.MainIssue != ""
	hintDone := len(in.Hints) > 0
	revisionDone := len(in.Revisions) > 0

	items := []AgentPlanItem{
		{Step: "submit", Title: "提交实验材料", Status: "done"},
		{Step: "diagnosis", Title: "诊断问题类型与证据缺口", Status: "done"},
		{Step: "tutoring", Title: "分层辅导与修正验证", Status: "pending"},
		{Step: "transfer", Title: "迁移测试", Status: "pending"},
		{Step: "report", Title: "学习反馈", Status: "pending"},
	}
	if !hasDiagnosis {
		items[1].Status = "current"
		items[1].Note = "诊断尚未生成"
		return items
	}
	switch status {
	case models.StatusDiagnosed:
		items[2].Status = "current"
		items[2].Note = "尚未使用提示或提交修正"
	case models.StatusTutoring:
		items[2].Status = "done"
		items[3].Status = "current"
		if hintDone {
			items[2].Note = fmt.Sprintf("已提供 %d 条提示", len(in.Hints))
		}
		if revisionDone {
			items[2].Note += fmt.Sprintf("，修正 %d 次", len(in.Revisions))
		}
	case models.StatusTransfer:
		items[2].Status = "done"
		items[3].Status = "current"
		if in.TransferAnswered {
			items[3].Note = "已作答，待完成本次学习"
		} else {
			items[3].Note = "已生成题目，待作答"
		}
	case models.StatusCompleted:
		items[2].Status = "done"
		items[3].Status = "done"
		items[4].Status = "done"
	}
	return items
}

// BuildAgentMemory 从本机历史记录中提取长期记忆：重复出现的误区与已体现掌握的知识点。
func BuildAgentMemory(history []models.Session) *AgentMemory {
	mem := &AgentMemory{
		LookedAt:   len(history),
		Repeated:   []AgentRepeatedIssue{},
		Mastered:   []string{},
		Milestones: []string{},
		Note:       "记忆来自本机保存的历史学习记录；本项目不收集姓名与学号，无法确认这些记录是否属于同一名学生，因此记忆只在单机演示场景下作为参考。",
	}
	counts := map[string]int{}
	for _, sess := range history {
		diag, ok := decodeHistoryDiagnosis(sess.DiagnosisJSON)
		if !ok {
			continue
		}
		if code := sess.PrimaryArchetype; code != "" && code != "NONE" {
			counts[code]++
		}
		for _, k := range diag.Knowledge {
			if k.Status == ClaimMastered {
				mem.Mastered = append(mem.Mastered, k.Name)
			}
		}
		if len(mem.Milestones) < 3 && sess.TransferScore >= 0 {
			mem.Milestones = append(mem.Milestones, fmt.Sprintf("《%s》迁移题 %d/%d 分", sess.ExperimentTitle, sess.TransferScore, transferMaxScore(sess)))
		}
	}
	mem.Mastered = dedupe(mem.Mastered)
	codes := make([]string, 0, len(counts))
	for code := range counts {
		codes = append(codes, code)
	}
	sortStrings(codes)
	for _, code := range codes {
		a, ok := archetypeByCode(code)
		label := code
		if ok {
			label = a.Name
		}
		mem.Repeated = append(mem.Repeated, AgentRepeatedIssue{
			Code:  code,
			Label: label,
			Count: counts[code],
			Advice: fmt.Sprintf("这一误区在本机历史记录中共出现 %d 次；本轮从最基础的观察方向重新引导，并优先在迁移题中考查同一知识点。", counts[code]),
		})
	}
	return mem
}

func transferMaxScore(sess models.Session) int {
	t, ok := decodeHistoryTransfer(sess.TransferJSON)
	if !ok || t.MaxScore == 0 {
		if item, found := content.ResolveTransferItem(t.ItemID); found {
			return item.MaxScore
		}
		return 6
	}
	return t.MaxScore
}

func decodeHistoryDiagnosis(raw string) (Diagnosis, bool) {
	if strings.TrimSpace(raw) == "" {
		return Diagnosis{}, false
	}
	var d Diagnosis
	if err := json.Unmarshal([]byte(raw), &d); err != nil {
		return Diagnosis{}, false
	}
	return d, true
}

func decodeHistoryTransfer(raw string) (TransferState, bool) {
	if strings.TrimSpace(raw) == "" {
		return TransferState{}, false
	}
	var t TransferState
	if err := json.Unmarshal([]byte(raw), &t); err != nil {
		return TransferState{}, false
	}
	return t, true
}

func sortStrings(list []string) {
	for i := 1; i < len(list); i++ {
		for j := i; j > 0 && list[j] < list[j-1]; j-- {
			list[j], list[j-1] = list[j-1], list[j]
		}
	}
}
