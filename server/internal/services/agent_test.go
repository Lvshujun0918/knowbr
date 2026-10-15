package services

import (
	"encoding/json"
	"strings"
	"testing"

	"knowbr/server/internal/models"
)

func agentInputFromDemo(t *testing.T) AgentInput {
	t.Helper()
	diag := Diagnose(demoInput(t))
	metrics, err := ComputeMetrics(coreDemoMatrix(), []string{"类别A", "类别B", "类别C"})
	if err != nil {
		t.Fatal(err)
	}
	return AgentInput{
		Status:    models.StatusDiagnosed,
		TaskType:  diag.TaskType,
		Diagnosis: diag,
		Metrics:   metrics,
		Hints:     []models.Hint{},
		Revisions: []models.Revision{},
		HintState: models.HintState{NextLevel: 1, MaxLevel: 3, UsedLevels: []int{}},
	}
}

// 证据不足时必须先要材料，并明确拒绝直接给出具体计算示例。
func TestAgentAsksForEvidenceBeforeHinting(t *testing.T) {
	in := agentInputFromDemo(t)
	in.Diagnosis = Diagnose(DiagnoseInput{ExperimentTitle: "分类实验", Explanation: "我的模型准确率是 90%，效果怎么样？"})
	in.Metrics = nil

	step := DecideNextAction(in)
	if step.Action != AgentAskEvidence {
		t.Fatalf("证据不足时应先要材料，实际 %s（%s）", step.Action, step.Reason)
	}
	if !step.Blocked || step.BlockedReason == "" {
		t.Error("应当标记为阻塞，并说明为什么不能给示例")
	}
	if step.CanAct {
		t.Error("阻塞状态下不应有可执行的自动动作")
	}
	if len(step.Requires) == 0 {
		t.Error("必须列出需要补充的具体材料")
	}
	if step.SuggestHintLevel != 0 {
		t.Errorf("不该同时给出提示等级，实际 %d", step.SuggestHintLevel)
	}
	if len(step.Plan) == 0 || step.Plan[1].Status != "done" {
		t.Error("计划应记录已完成诊断")
	}
}

// 决策必须附带真实执行的工具轨迹与可核对的结果。
func TestAgentDecisionCarriesToolsAndPlan(t *testing.T) {
	step := DecideNextAction(agentInputFromDemo(t))
	if step.Action != AgentGiveHint {
		t.Fatalf("核心案例在未使用提示时应给出提示，实际 %s（%s）", step.Action, step.Reason)
	}
	if step.SuggestHintLevel != 1 {
		t.Errorf("首次提示应为一级，实际 %d", step.SuggestHintLevel)
	}
	if !step.CanAct {
		t.Error("给出提示属于可执行动作")
	}
	names := []string{}
	for _, tool := range step.Tools {
		names = append(names, tool.Name)
		if tool.Purpose == "" || tool.Result == "" {
			t.Errorf("工具 %s 缺少用途或结果", tool.Name)
		}
	}
	for _, want := range []string{"evidence_check", "metrics_compute", "history_lookup", "progress_check"} {
		if !containsAny(strings.Join(names, ","), []string{want}) {
			t.Errorf("缺少工具 %s，实际 %v", want, names)
		}
	}
	if !strings.Contains(strings.Join(names, ","), "metrics_compute") {
		t.Error("提供了矩阵却没有运行指标工具")
	}
	if step.PolicyNote == "" {
		t.Error("必须说明这是确定性策略而不是自由推理")
	}
	if step.ActionLabel == "" || step.Title == "" || step.DecidedAt == "" {
		t.Error("动作标签、标题与时间必须齐全")
	}
}

// 未提供矩阵时不能假装算过指标。
func TestAgentDoesNotComputeMetricsWithoutMatrix(t *testing.T) {
	in := agentInputFromDemo(t)
	in.Metrics = nil
	step := DecideNextAction(in)
	for _, tool := range step.Tools {
		if tool.Name == "metrics_compute" && !strings.Contains(tool.Result, "未提交混淆矩阵") {
			t.Errorf("没有矩阵时不应给出指标结果：%s", tool.Result)
		}
	}
}

func TestAgentEscalatesAfterRepeatedNoProgress(t *testing.T) {
	in := agentInputFromDemo(t)
	in.Status = models.StatusTutoring
	in.Hints = []models.Hint{{Level: 1}, {Level: 2}}
	in.HintState = models.HintState{NextLevel: 3, MaxLevel: 3, UsedLevels: []int{1, 2}, NoProgressRounds: 2}
	in.Revisions = []models.Revision{
		{Content: "我觉得模型没问题。", Progress: "none"},
		{Content: "我觉得模型还是没问题。", Progress: "none"},
	}
	step := DecideNextAction(in)
	if step.Action != AgentEscalate {
		t.Fatalf("连续无进展时应建议教师介入，实际 %s（%s）", step.Action, step.Reason)
	}
	if !strings.Contains(step.Reason, "2") {
		t.Errorf("理由应引用无进展轮次：%s", step.Reason)
	}
}

func TestAgentAdvancesToTransferAndCompletes(t *testing.T) {
	in := agentInputFromDemo(t)
	in.Status = models.StatusTutoring
	in.HasTransfer = true
	in.TransferAnswered = false
	if step := DecideNextAction(in); step.Action != AgentAdvanceTransfer {
		t.Fatalf("已生成题目未作答时应推进作答，实际 %s", step.Action)
	}
	in.TransferAnswered = true
	in.TransferCompleted = true
	in.Status = models.StatusCompleted
	if step := DecideNextAction(in); step.Action != AgentComplete {
		t.Fatalf("已完成学习时应收束，实际 %s", step.Action)
	}
}

// 没有未解决知识点且尚未提交修正时，应等待学生自己的解释。
func TestAgentWaitsForRevisionWhenNothingUnresolved(t *testing.T) {
	in := agentInputFromDemo(t)
	for i := range in.Diagnosis.Claims {
		in.Diagnosis.Claims[i].Status = ClaimMastered
	}
	step := DecideNextAction(in)
	if step.Action != AgentVerifyRevision {
		t.Fatalf("无可推进知识点时应等待修正解释，实际 %s（%s）", step.Action, step.Reason)
	}
}

// 跨会话记忆：识别重复误区，并据此从一级提示重新开始。
func TestAgentMemoryRepeatsMisconceptionAndRestartsAtLevelOne(t *testing.T) {
	history := agentHistoryWithArchetype(t, "E1", 2)
	mem := BuildAgentMemory(history)
	if mem.LookedAt != 2 {
		t.Fatalf("应读取 2 条历史记录，实际 %d", mem.LookedAt)
	}
	if len(mem.Repeated) == 0 || mem.Repeated[0].Code != "E1" {
		t.Fatalf("应识别出重复误区 E1，实际 %+v", mem.Repeated)
	}
	if mem.Repeated[0].Count != 2 {
		t.Errorf("重复次数应为 2，实际 %d", mem.Repeated[0].Count)
	}
	if !strings.Contains(mem.Note, "无法确认这些记录是否属于同一名学生") {
		t.Error("必须如实说明记忆来源的局限，不能假装是个性化画像")
	}

	in := agentInputFromDemo(t)
	in.Status = models.StatusTutoring
	in.Hints = []models.Hint{{Level: 1}, {Level: 2}}
	in.HintState = models.HintState{NextLevel: 3, MaxLevel: 3, UsedLevels: []int{1, 2}}
	in.History = history
	step := DecideNextAction(in)
	if step.Action != AgentGiveHint {
		t.Fatalf("应给出提示，实际 %s", step.Action)
	}
	if step.SuggestHintLevel != 1 {
		t.Errorf("记忆命中重复误区时应从一级重新引导，实际 %d", step.SuggestHintLevel)
	}
	if !strings.Contains(step.Reason, "历史记忆") {
		t.Errorf("理由应说明记忆介入的原因：%s", step.Reason)
	}
}

func agentHistoryWithArchetype(t *testing.T, code string, n int) []models.Session {
	t.Helper()
	diag := Diagnose(demoInput(t))
	raw, err := json.Marshal(diag)
	if err != nil {
		t.Fatal(err)
	}
	out := make([]models.Session, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, models.Session{
			ID:               "hist-" + code + "-" + string(rune('a'+i)),
			ExperimentTitle:  "历史实验",
			PrimaryArchetype: code,
			DiagnosisJSON:    string(raw),
			TransferScore:    4,
			TransferJSON:     `{"itemId":"CLS:CLS-1:0","maxScore":6}`,
		})
	}
	return out
}

// 记忆中的已掌握知识点用于避免在迁移题里重复考查。
func TestAgentMemoryCollectsMasteredKnowledge(t *testing.T) {
	diag := Diagnose(demoInput(t))
	for i := range diag.Knowledge {
		diag.Knowledge[i].Status = ClaimMastered
	}
	raw, _ := json.Marshal(diag)
	mem := BuildAgentMemory([]models.Session{{ID: "h1", DiagnosisJSON: string(raw)}})
	if len(mem.Mastered) == 0 {
		t.Fatal("应收集已体现掌握的知识点")
	}
	if len(mem.Milestones) == 0 {
		t.Error("应记录迁移题得分作为进展标记")
	}
}
