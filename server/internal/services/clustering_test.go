package services

import (
	"strings"
	"testing"

	"knowbr/server/internal/content"
	"knowbr/server/internal/models"
)

// clusteringInput 由预置聚类案例构造诊断输入（与接口层提交路径保持一致）。
func clusteringInput(t *testing.T) DiagnoseInput {
	t.Helper()
	c, ok := content.DemoCaseByID("case-clustering")
	if !ok {
		t.Fatal("缺少聚类任务演示案例")
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

// 新增场景：聚类任务必须落到 clustering_metrics，并命中“只看内部指标数值”的规则。
func TestClusteringCaseDiagnosis(t *testing.T) {
	in := clusteringInput(t)
	diag := Diagnose(in)
	if diag.TaskType != content.TaskClustering {
		t.Fatalf("任务类型应为 clustering_metrics，实际 %s", diag.TaskType)
	}
	if diag.TaskTypeLabel == "" || len(diag.TaskFocus) == 0 {
		t.Error("聚类任务必须给出中文标签与关注知识点")
	}
	if diag.PrimaryArchetype != "E14" {
		t.Errorf("主误区应为 E14（只看内部指标数值就下结论），实际 %s", diag.PrimaryArchetype)
	}
	hasError := false
	for _, c := range diag.Claims {
		if c.Status == ClaimError {
			hasError = true
		}
	}
	if !hasError {
		t.Errorf("应把“轮廓系数很好所以可以直接使用”判为错误判断：%+v", diag.Claims)
	}
	// 无监督任务不得混入有监督或回归的知识点。
	for _, k := range diag.Knowledge {
		for _, bad := range []string{"召回率", "混淆矩阵", "误差量纲", "基线"} {
			if strings.Contains(k.Name, bad) {
				t.Errorf("聚类任务的知识点不应包含「%s」：%s", bad, k.Name)
			}
		}
	}
	// 必需证据：簇数、内部指标都应在材料里被识别为已提供。
	needs := map[string]bool{}
	for _, n := range diag.EvidenceNeeds {
		if n.Required && !n.Present {
			needs[n.Name] = false
		}
	}
	if len(needs) > 0 {
		t.Errorf("预置案例的必需证据应齐备，实际缺失：%v", needs)
	}
}

// 聚类任务的三级提示不得转向有监督指标或回归误差指标。
func TestClusteringHintsStayOnTopic(t *testing.T) {
	in := clusteringInput(t)
	diag := Diagnose(in)
	state := models.HintState{NextLevel: 1, MaxLevel: 3, UsedLevels: []int{}}
	forbidden := []string{"混淆矩阵", "召回率", "精确率", "F1", "类别不平衡", "疾病筛查", "MAE", "RMSE", "留出验证集"}
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

// 修正反馈在聚类任务下按聚类证据要点判断。
func TestClusteringRevisionUsesClusteringEvidence(t *testing.T) {
	in := clusteringInput(t)
	diag := Diagnose(in)
	res := AnalyzeRevision(RevisionRequest{
		Content:            "轮廓系数 0.62 只说明几何上分得开，不代表业务含义；我会先标准化再聚类，对多个簇数比较轮廓系数并固定随机种子重复运行。",
		InitialExplanation: in.Explanation,
		Diagnosis:          diag,
		State:              models.HintState{NextLevel: 1, MaxLevel: 3},
		TaskType:           diag.TaskType,
	})
	if res.Revision.Progress == "none" {
		t.Errorf("补上标准化与稳定性检查应判为有推进：%s", res.Revision.Feedback)
	}
	joined := strings.Join(append(res.Revision.MissingKeywords, res.Revision.AddressedKeywords...), "；")
	for _, bad := range []string{"召回率", "精确率", "混淆矩阵", "验证集"} {
		if strings.Contains(joined, bad) {
			t.Errorf("聚类任务的证据要点不应出现「%s」：%s", bad, joined)
		}
	}
}

// 聚类任务必须有对应的迁移题族，且题目限定在聚类知识点内。
func TestClusteringTransferStaysInTaskType(t *testing.T) {
	families := content.FamiliesByTaskType(content.TaskClustering)
	if len(families) == 0 {
		t.Fatal("聚类任务缺少迁移题族")
	}
	item, info := GenerateTransfer(GenerateInput{
		TaskType:   content.TaskClustering,
		Archetypes: []string{"E14"},
		Seed:       "seed-clustering",
	})
	if item.TaskType != content.TaskClustering {
		t.Fatalf("迁移题应属于聚类任务，实际 %s", item.TaskType)
	}
	text := item.Topic + item.Context + item.Question
	for _, bad := range []string{"混淆矩阵", "召回率", "精确率", "类别不平衡", "MAE", "RMSE"} {
		if strings.Contains(text, bad) {
			t.Errorf("聚类任务迁移题不得出现「%s」：%s", bad, text)
		}
	}
	if info.SelectionReason == "" || info.DedupNote == "" {
		t.Error("迁移题必须标注选题依据与重复检查结果")
	}
}
