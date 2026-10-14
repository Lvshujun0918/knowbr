package services

import (
	"strings"
	"testing"

	"knowbr/server/internal/models"
)

func demoInput(t *testing.T) DiagnoseInput {
	t.Helper()
	m, err := ComputeMetrics(coreDemoMatrix(), []string{"类别A", "类别B", "类别C"})
	if err != nil {
		t.Fatalf("计算矩阵失败：%v", err)
	}
	return DiagnoseInput{
		Explanation:        "我的模型准确率达到 90%，所以我认为模型效果很好，可以直接使用了。",
		Question:           "为什么准确率这么高，我还是不太确定这个模型能不能用？",
		DatasetDescription: "类别 A 900 个、类别 B 90 个、类别 C 10 个，共 1000 个样本。",
		ExperimentGoal:     "完成三分类任务并判断模型是否可用。",
		MetricsText:        "准确率 0.90",
		ExecutionLog:       "accuracy: 0.90",
		HaveMatrix:         true,
		Labels:             m.Labels,
		Metrics:            m,
	}
}

// 测试 1 / 需求文档 E1：完整演示案例应指出只看准确率的局限，并引用 B、C 两类的数据。
func TestDiagnoseCoreDemoCase(t *testing.T) {
	diag := Diagnose(demoInput(t))
	if !diag.Completeness.Sufficient {
		t.Fatalf("完整演示案例应判定材料充分：%+v", diag.Completeness)
	}
	if diag.PrimaryArchetype != "E1" {
		t.Errorf("主误区应为 E1，实际 %s（命中：%v）", diag.PrimaryArchetype, diag.MatchedArchetypes)
	}
	if !strings.Contains(diag.MainIssue, "90") || !strings.Contains(diag.MainIssue, "类别B") {
		t.Errorf("主要问题应引用准确率与零召回类别，实际：%s", diag.MainIssue)
	}
	joined := strings.Join(diag.Evidence, "\n")
	if !strings.Contains(joined, "学生原话") {
		t.Errorf("证据应包含学生原话，实际：%s", joined)
	}
	if !strings.Contains(joined, "全部被预测为类别A") {
		t.Errorf("证据应包含由矩阵计算的错误去向，实际：%s", joined)
	}
	if diag.MasteryStatus != MasteryNotShown {
		t.Errorf("掌握情况应为尚未体现，实际 %s", diag.MasteryStatus)
	}
	if diag.MasteryLabel == "" || diag.NextStep == "" || diag.Reasoning == "" {
		t.Error("诊断结果必须包含掌握标签、诊断理由与下一步建议")
	}
	codes := map[string]bool{}
	for _, c := range diag.Categories {
		codes[c.Code] = true
	}
	if !codes["interpretation"] || !codes["metric_selection"] {
		t.Errorf("E1 应对应结果解释与指标选择两类问题，实际 %v", codes)
	}
}

// 测试 2：只提交准确率时应说明证据不足并追问，而不能武断下结论。
func TestDiagnoseInsufficientEvidence(t *testing.T) {
	diag := Diagnose(DiagnoseInput{
		Explanation: "模型准确率为 90%，这个效果怎么样？",
		MetricsText: "准确率 90%",
		HaveMatrix:  false,
	})
	if diag.Completeness.Sufficient {
		t.Fatal("只有准确率时不应判定为材料充分")
	}
	if diag.PrimaryCategory != "insufficient" || diag.MasteryStatus != MasteryInsufficient {
		t.Errorf("应给出“证据不足”诊断，实际 category=%s mastery=%s", diag.PrimaryCategory, diag.MasteryStatus)
	}
	if strings.Contains(diag.MainIssue, "很好") || strings.Contains(diag.MainIssue, "很差") {
		t.Errorf("不得武断判断模型好坏，实际：%s", diag.MainIssue)
	}
	if !strings.Contains(diag.NextStep, "类别") || !strings.Contains(diag.NextStep, "任务目标") {
		t.Errorf("下一步应要求补充类别分布与任务目标，实际：%s", diag.NextStep)
	}
	if len(diag.Completeness.Missing) == 0 {
		t.Error("应列出缺失材料")
	}
}

// 测试 3：学生误读混淆矩阵时应指出矛盾并给出具体数据，且不贬低学生。
func TestDiagnoseContradiction(t *testing.T) {
	in := demoInput(t)
	in.Explanation = "从混淆矩阵看，我把所有样本都预测成了类别A，所以三个类别都能被正确分类，模型对每一类都识别得很好。"
	diag := Diagnose(in)
	if !diag.Contradiction.Detected {
		t.Fatal("应识别出陈述与矩阵证据矛盾")
	}
	if !strings.Contains(diag.Contradiction.Detail, "100") {
		t.Errorf("矛盾说明应给出非对角线样本数量，实际：%s", diag.Contradiction.Detail)
	}
	if diag.MasteryStatus != MasteryNotShown {
		t.Errorf("矛盾时掌握情况应为尚未体现，实际 %s", diag.MasteryStatus)
	}
	if !diag.TeacherIntervention {
		t.Error("陈述与证据矛盾时应建议教师介入")
	}
	for _, bad := range []string{"错误", "太差", "不应该", "居然"} {
		if strings.Contains(diag.MainIssue, bad) {
			t.Errorf("诊断语气应中性、不贬低学生，实际：%s", diag.MainIssue)
		}
	}
	if !strings.Contains(diag.NextStep, "第 i 行第 j 列") {
		t.Errorf("下一步应引导学生重新理解矩阵行列含义，实际：%s", diag.NextStep)
	}
}

// 测试 4 / 需求文档 E2—E5：内置典型误区应能被稳定命中。
func TestArchetypeDetection(t *testing.T) {
	m, err := ComputeMetrics(coreDemoMatrix(), []string{"类别A", "类别B", "类别C"})
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		text string
		want string
	}{
		{"E1 忽视类别不平衡", "准确率达到 90%，模型效果很好，可以直接使用。", "E1"},
		{"E2 只看对角线", "混淆矩阵只要看对角线就够了，对角线上的数字都很大。", "E2"},
		{"E3 认为召回率都差不多", "各类别的召回率应该差不多，都很高。", "E3"},
		{"E4 只想提高准确率", "下一步我打算继续提高准确率，让准确率更高一些。", "E4"},
		{"E5 代码跑通即学会", "代码跑通了、运行成功，说明我已经学会这个实验了。", "E5"},
		{"E0 环境报错", "运行时报错：ModuleNotFoundError: No module named 'sklearn'，请问怎么办？", "E0"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			diag := Diagnose(DiagnoseInput{
				Explanation:        tc.text,
				DatasetDescription: "类别 A 900 个、B 90 个、C 10 个。",
				ExperimentGoal:     "完成三分类实验。",
				HaveMatrix:         true,
				Labels:             m.Labels,
				Metrics:            m,
			})
			found := false
			for _, a := range diag.MatchedArchetypes {
				if a.Code == tc.want {
					found = true
				}
			}
			if !found {
				t.Fatalf("应命中 %s，实际命中 %v", tc.want, diag.MatchedArchetypes)
			}
			if diag.Reasoning == "" || diag.NextStep == "" || diag.MainIssue == "" {
				t.Error("诊断必须包含主要问题、理由与下一步")
			}
		})
	}
}

// “代码跑通就等于学会”的场景：有数据分布与运行证据但没有混淆矩阵，
// 也应能开展诊断并命中 E5，同时在限制中说明缺少逐类指标。
func TestDiagnoseRunOnlyCaseIsE5(t *testing.T) {
	diag := Diagnose(DiagnoseInput{
		Explanation:        "代码跑通了而且没有报错，说明我已经学会了这个实验，可以做下一个了。",
		DatasetDescription: "训练集 2000 条评论，测试集 500 条，正负样本比例约 1:1。",
		ExperimentGoal:     "运行情感分类实验并解释模型表现。",
		MetricsText:        "准确率 0.873",
		ExecutionLog:       "accuracy: 0.873",
		CodeSnippet:        "model = MultinomialNB().fit(X_train_vec, y_train)",
	})
	if !diag.Completeness.Sufficient {
		t.Fatalf("有数据分布与运行证据时应能开展诊断：%+v", diag.Completeness)
	}
	if diag.PrimaryArchetype != "E5" {
		t.Fatalf("“代码跑通就等于学会”应命中 E5，实际 %v", diag.MatchedArchetypes)
	}
	if diag.MasteryStatus != MasteryNotShown {
		t.Errorf("掌握情况应为尚未体现，实际 %s", diag.MasteryStatus)
	}
	if !strings.Contains(diag.MainIssue, "运行成功") && !strings.Contains(diag.MainIssue, "学会") {
		t.Errorf("主要问题应指向“程序成功≠概念掌握”，实际：%s", diag.MainIssue)
	}
	if len(diag.Completeness.Limitations) == 0 {
		t.Error("未提供矩阵时应说明诊断限制")
	}
}

// 误读混淆矩阵但未命中其它规则时，应归类到矩阵读数（E2）并给出相应提示与迁移题。
func TestDiagnoseContradictionWithoutOtherArchetype(t *testing.T) {
	in := demoInput(t)
	in.Explanation = "从混淆矩阵看，我把所有样本都预测成了类别A，所以三个类别都能被正确分类，模型对每一类都识别得很好。"
	diag := Diagnose(in)
	if !diag.Contradiction.Detected {
		t.Fatal("应识别出陈述与证据矛盾")
	}
	if diag.PrimaryArchetype != "E2" {
		t.Errorf("矛盾场景应归类到矩阵读数 E2，实际 %s（%v）", diag.PrimaryArchetype, diag.MatchedArchetypes)
	}
	codes := map[string]bool{}
	for _, c := range diag.Categories {
		codes[c.Code] = true
	}
	if !codes["concept"] || !codes["interpretation"] {
		t.Errorf("应同时归入概念理解与结果解释两类问题，实际 %v", codes)
	}
	hint, _, err := BuildHint(HintRequest{Level: 1, Diagnosis: diag, Metrics: in.Metrics, State: models.HintState{MaxLevel: 3}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(hint.Content, "对角线") {
		t.Errorf("一级提示应引导重新理解矩阵读数：%s", hint.Content)
	}
}

// E1 需要类别不平衡证据：矩阵平衡且学生未提到不平衡时不应误报。
func TestE1RequiresImbalanceEvidence(t *testing.T) {
	m, err := ComputeMetrics([][]float64{{45, 5}, {5, 45}}, []string{"正类", "负类"})
	if err != nil {
		t.Fatal(err)
	}
	diag := Diagnose(DiagnoseInput{
		Explanation: "准确率 90%，模型效果很好。",
		HaveMatrix:  true,
		Labels:      m.Labels,
		Metrics:     m,
	})
	for _, a := range diag.MatchedArchetypes {
		if a.Code == "E1" {
			t.Fatalf("平衡数据且未提及不平衡时不应命中 E1：%v", diag.MatchedArchetypes)
		}
	}
}

// 功能 E：三级提示必须逐级展开，环境问题直接给排障步骤。
func TestHintLevels(t *testing.T) {
	diag := Diagnose(demoInput(t))
	metrics, err := ComputeMetrics(coreDemoMatrix(), []string{"类别A", "类别B", "类别C"})
	if err != nil {
		t.Fatal(err)
	}
	state := models.HintState{NextLevel: 1, MaxLevel: 3}
	var existing []models.Hint
	for level := 1; level <= 3; level++ {
		hint, newState, err := BuildHint(HintRequest{Level: level, Diagnosis: diag, Metrics: metrics, Existing: existing, State: state, RequestedByStudent: true})
		if err != nil {
			t.Fatalf("生成第 %d 级提示失败：%v", level, err)
		}
		if !hint.RequestedByStudent || hint.Level != level || hint.Content == "" {
			t.Fatalf("提示记录不完整：%+v", hint)
		}
		if !strings.Contains(hint.Content, "类别B") {
			t.Errorf("第 %d 级提示应引用本次矩阵中的零召回类别：%s", level, hint.Content)
		}
		existing = append(existing, hint)
		state = newState
	}
	if !strings.Contains(existing[2].Content, "0 / 90") {
		t.Errorf("三级提示应给出局部计算示例，实际：%s", existing[2].Content)
	}
	if existing[1].Level != 2 || !strings.Contains(existing[1].Content, "召回率") {
		t.Errorf("二级提示应解释概念并回到证据：%s", existing[1].Content)
	}
	// 同一等级重复请求应返回原提示，不重复记录。
	again, _, err := BuildHint(HintRequest{Level: 1, Diagnosis: diag, Metrics: metrics, Existing: existing, State: state})
	if err != nil {
		t.Fatal(err)
	}
	if again.CreatedAt != existing[0].CreatedAt {
		t.Error("重复请求同一等级应返回已提供的提示")
	}
	if _, _, err := BuildHint(HintRequest{Level: 4, Diagnosis: diag, Metrics: metrics, State: state}); err == nil {
		t.Error("非法提示等级应返回错误")
	}
}

func TestEnvHintIsDirectNotSocratic(t *testing.T) {
	diag := Diagnose(DiagnoseInput{
		Explanation:  "运行时报错：ModuleNotFoundError: No module named 'sklearn'。",
		HaveMatrix:   false,
		CodeSnippet:  "from sklearn.datasets import load_iris",
		ExecutionLog: "ModuleNotFoundError: No module named 'sklearn'",
	})
	if diag.PrimaryArchetype != "E0" {
		t.Fatalf("应命中 E0，实际 %v", diag.MatchedArchetypes)
	}
	hint, _, err := BuildHint(HintRequest{Level: 1, Diagnosis: diag, State: models.HintState{MaxLevel: 3}})
	if err != nil {
		t.Fatal(err)
	}
	if hint.Socratic {
		t.Error("环境类问题不应标记为苏格拉底式提问")
	}
	if !strings.Contains(hint.Content, "报错") {
		t.Errorf("环境提示应直接给出排障指引：%s", hint.Content)
	}
}

func TestRevisionProgressAndTeacherIntervention(t *testing.T) {
	diag := Diagnose(demoInput(t))
	metrics, _ := ComputeMetrics(coreDemoMatrix(), []string{"类别A", "类别B", "类别C"})
	state := models.HintState{MaxLevel: 3}

	// 第一次修正：补充了按类别召回率与不平衡，应判定为 improved。
	res := AnalyzeRevision(RevisionRequest{
		Content:            "类别B 的召回率是 0%，因为 90 个样本全部被预测成 A；类别分布也不平衡。",
		InitialExplanation: "我的模型准确率达到 90%，所以我认为模型效果很好。",
		Diagnosis:          diag,
		Metrics:            metrics,
		State:              state,
	})
	if res.Revision.Progress != "improved" || !res.Improved {
		t.Fatalf("应判定为 improved，实际 %s（%s）", res.Revision.Progress, res.Revision.Feedback)
	}
	if len(res.Revision.AddressedKeywords) == 0 {
		t.Error("应记录新增的证据要点")
	}
	state = res.State

	// 连续两轮无进展 → 建议教师介入。
	prev := []models.Revision{res.Revision}
	for i := 0; i < 2; i++ {
		r := AnalyzeRevision(RevisionRequest{
			Content:   "我还是觉得模型没问题。",
			Previous:  prev,
			Diagnosis: diag,
			Metrics:   metrics,
			State:     state,
		})
		state = r.State
		prev = append(prev, r.Revision)
		if r.Revision.Progress != "none" {
			t.Fatalf("第 %d 轮应判定为无进展，实际 %s", i+1, r.Revision.Progress)
		}
	}
	if !state.TeacherIntervention || state.TeacherReason == "" {
		t.Error("连续两轮无进展后应建议教师介入并说明原因")
	}
	if !strings.Contains(prev[len(prev)-1].Feedback, "教师介入") {
		t.Errorf("反馈中应提示教师介入，实际：%s", prev[len(prev)-1].Feedback)
	}
}

// 证据不足时给出的提示也必须可执行，且不得引用未提供的矩阵。
func TestHintWithoutMatrixDoesNotInventNumbers(t *testing.T) {
	diag := Diagnose(DiagnoseInput{Explanation: "模型准确率为 90%，这个效果怎么样？", MetricsText: "准确率 90%"})
	hint, _, err := BuildHint(HintRequest{Level: 2, Diagnosis: diag, State: models.HintState{MaxLevel: 3}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(hint.Content, "本次矩阵") {
		t.Errorf("未提供矩阵时不得引用矩阵数据：%s", hint.Content)
	}
	if hint.Content == "" {
		t.Error("提示内容不能为空")
	}
}
