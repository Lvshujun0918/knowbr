package services

import (
	"strings"
	"testing"

	"knowbr/server/internal/content"
	"knowbr/server/internal/models"
)

// regressionInput 由预置回归案例构造诊断输入（与接口层提交路径保持一致）。
func regressionInput(t *testing.T) DiagnoseInput {
	t.Helper()
	c, ok := content.DemoCaseByID("case-regression")
	if !ok {
		t.Fatal("缺少回归任务演示案例")
	}
	d := c.Submission
	return DiagnoseInput{
		ExperimentTitle:    d.ExperimentTitle,
		ExperimentGoal:     d.ExperimentGoal,
		ModelName:          d.ModelName,
		Explanation:        d.StudentInitialExplanation,
		Question:           d.Question,
		DatasetDescription: d.DatasetDescription,
		MetricsText:        d.MetricsText,
		ExecutionLog:       d.ExecutionLog,
		CodeSnippet:        d.CodeSnippet,
	}
}

// 新增场景：回归任务必须落到 regression_metrics，并命中“误用 R² / 训练集误差”的规则。
func TestRegressionCaseDiagnosis(t *testing.T) {
	in := regressionInput(t)
	diag := Diagnose(in)
	if diag.TaskType != content.TaskRegression {
		t.Fatalf("任务类型应为 regression_metrics，实际 %s", diag.TaskType)
	}
	if diag.TaskTypeLabel == "" || len(diag.TaskFocus) == 0 {
		t.Error("回归任务必须给出中文标签与关注知识点")
	}
	matched := map[string]bool{}
	for _, a := range diag.MatchedArchetypes {
		matched[a.Code] = true
	}
	if !matched["E11"] {
		t.Errorf("应命中“用 R² 代替误差量级”规则 E11，实际 %v", matched)
	}
	if diag.PrimaryArchetype != "E11" {
		t.Errorf("主误区应为 E11，实际 %s", diag.PrimaryArchetype)
	}
	if !strings.Contains(diag.Reasoning, "R²") && !strings.Contains(diag.Reasoning, "量纲") {
		t.Errorf("诊断理由应说明 R² 与误差量纲的关系：%s", diag.Reasoning)
	}
	// 主张必须指出这组指标是在同一批数据上算出来的。
	hasTrainOnly := false
	for _, c := range diag.Claims {
		if c.Status == ClaimError {
			hasTrainOnly = true
		}
	}
	if len(diag.Claims) == 0 || !hasTrainOnly {
		t.Errorf("应把“训练集误差很小所以可用”这类主张判为错误：%+v", diag.Claims)
	}
	// 回归任务的知识点清单来自回归任务定义，不应混入分类指标。
	for _, k := range diag.Knowledge {
		if strings.Contains(k.Name, "召回率") || strings.Contains(k.Name, "混淆矩阵") {
			t.Errorf("回归任务的知识点不应包含分类指标：%s", k.Name)
		}
	}
}

// 回归任务的三级提示不得转向混淆矩阵、逐类召回率等分类指标。
func TestRegressionHintsStayOnTopic(t *testing.T) {
	in := regressionInput(t)
	diag := Diagnose(in)
	state := models.HintState{NextLevel: 1, MaxLevel: 3, UsedLevels: []int{}}
	forbidden := []string{"混淆矩阵", "召回率", "精确率", "F1", "类别不平衡", "少数类", "疾病筛查"}
	for level := 1; level <= 3; level++ {
		hint, newState, err := BuildHint(HintRequest{
			Level:     level,
			Diagnosis: diag,
			Existing:  []models.Hint{},
			State:     state,
			TaskType:  diag.TaskType,
		})
		if err != nil {
			t.Fatalf("生成第 %d 级提示失败：%v", level, err)
		}
		for _, bad := range forbidden {
			if strings.Contains(hint.Content, bad) {
				t.Errorf("第 %d 级提示不得出现「%s」：%s", level, bad, hint.Content)
			}
		}
		if hint.Why == "" {
			t.Errorf("第 %d 级提示必须说明为什么选择它", level)
		}
		state = newState
	}
}

// 修正反馈在回归任务下按回归证据要点判断，而不是按分类指标要点判断。
func TestRegressionRevisionUsesRegressionEvidence(t *testing.T) {
	in := regressionInput(t)
	diag := Diagnose(in)
	res := AnalyzeRevision(RevisionRequest{
		Content:            "只在训练集上算误差所以偏乐观，我会按时间顺序留出验证集，在验证集上报告 RMSE 与 R²。",
		InitialExplanation: in.Explanation,
		Diagnosis:          diag,
		State:              models.HintState{NextLevel: 1, MaxLevel: 3},
		TaskType:           diag.TaskType,
	})
	if res.Revision.Progress == "none" {
		t.Errorf("明确补上验证集与指标应判为有推进：%s", res.Revision.Feedback)
	}
	joined := strings.Join(res.Revision.MissingKeywords, "；")
	if strings.Contains(joined, "召回率") || strings.Contains(joined, "精确率") {
		t.Errorf("回归任务的缺失要点不应是分类指标：%s", joined)
	}
	for _, k := range res.Revision.AddressedKeywords {
		if !strings.Contains(k, "验证") && !strings.Contains(k, "训练集") && !strings.Contains(k, "基线") &&
			!strings.Contains(k, "误差") && !strings.Contains(k, "适用") {
			t.Errorf("新增要点应属于回归任务的证据要点：%s", k)
		}
	}
}

// 回归任务必须有对应的迁移题族，且题目限定在回归知识点内。
func TestRegressionTransferStaysInTaskType(t *testing.T) {
	families := content.FamiliesByTaskType(content.TaskRegression)
	if len(families) == 0 {
		t.Fatal("回归任务缺少迁移题族")
	}
	item, info := GenerateTransfer(GenerateInput{
		TaskType:   content.TaskRegression,
		Archetypes: []string{"E11", "E12"},
		Seed:       "seed-regression",
	})
	if item.TaskType != content.TaskRegression {
		t.Fatalf("迁移题应属于回归任务，实际 %s", item.TaskType)
	}
	text := item.Topic + item.Context + item.Question
	for _, bad := range []string{"混淆矩阵", "召回率", "精确率", "类别不平衡", "疾病筛查"} {
		if strings.Contains(text, bad) {
			t.Errorf("回归任务迁移题不得出现「%s」：%s", bad, text)
		}
	}
	if info.SelectionReason == "" || info.DedupNote == "" {
		t.Error("迁移题必须标注选题依据与重复检查结果")
	}
}
