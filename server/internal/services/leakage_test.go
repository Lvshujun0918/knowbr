package services

import (
	"strings"
	"testing"

	"knowbr/server/internal/content"
	"knowbr/server/internal/models"
)

// leakageInput 由预置数据泄漏案例构造诊断输入（与接口层提交路径保持一致）。
func leakageInput(t *testing.T) DiagnoseInput {
	t.Helper()
	c, ok := content.DemoCaseByID("case-leakage")
	if !ok {
		t.Fatal("缺少数据预处理与数据泄漏演示案例")
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

// 新增场景：预处理任务必须落到 data_preparation，并命中“先在全量数据上做预处理”的规则。
func TestLeakageCaseDiagnosis(t *testing.T) {
	in := leakageInput(t)
	diag := Diagnose(in)
	if diag.TaskType != content.TaskDataPipeline {
		t.Fatalf("任务类型应为 data_preparation，实际 %s", diag.TaskType)
	}
	if diag.PrimaryArchetype != "E15" {
		t.Errorf("主误区应为 E15（先全量预处理再划分），实际 %s", diag.PrimaryArchetype)
	}
	hasError := false
	for _, c := range diag.Claims {
		if c.Status == ClaimError && strings.Contains(c.KnowledgePoint, "顺序") {
			hasError = true
		}
	}
	if !hasError {
		t.Errorf("应把“先用全部数据预处理再划分”判为错误判断：%+v", diag.Claims)
	}
	// 必需证据（含预处理顺序与特征清单）都必须被识别为已提供。
	for _, n := range diag.EvidenceNeeds {
		if n.Required && !n.Present {
			t.Errorf("预置案例的必需证据应齐备，缺失：%s", n.Name)
		}
	}
	// 不越界：不引入其它任务类型的知识点。
	for _, k := range diag.Knowledge {
		for _, bad := range []string{"召回率", "混淆矩阵", "轮廓系数", "误差量纲", "梯度"} {
			if strings.Contains(k.Name, bad) {
				t.Errorf("预处理任务的知识点不应包含「%s」：%s", bad, k.Name)
			}
		}
	}
}

// 预处理任务的三级提示不得转向分类指标、聚类内部指标或回归误差指标。
func TestLeakageHintsStayOnTopic(t *testing.T) {
	in := leakageInput(t)
	diag := Diagnose(in)
	state := models.HintState{NextLevel: 1, MaxLevel: 3, UsedLevels: []int{}}
	forbidden := []string{"混淆矩阵", "召回率", "精确率", "类别不平衡", "轮廓系数", "惯性", "MAE", "RMSE", "梯度"}
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
		state = newState
	}
}

// 修正反馈在预处理任务下按流程证据判断，不要求其它任务类型的指标。
func TestLeakageRevisionUsesPipelineEvidence(t *testing.T) {
	in := leakageInput(t)
	diag := Diagnose(in)
	res := AnalyzeRevision(RevisionRequest{
		Content:            "我会先划分训练集与测试集，只在训练集上拟合标准化与填充参数，再转换测试集；并逐列检查特征在预测时刻是否已知，删除事后才能观测的列。",
		InitialExplanation: in.Explanation,
		Diagnosis:          diag,
		State:              models.HintState{NextLevel: 1, MaxLevel: 3},
		TaskType:           diag.TaskType,
	})
	if res.Revision.Progress == "none" {
		t.Errorf("补上正确顺序与特征复查应判为有推进：%s", res.Revision.Feedback)
	}
	joined := strings.Join(append(res.Revision.MissingKeywords, res.Revision.AddressedKeywords...), "；")
	for _, bad := range []string{"召回率", "精确率", "混淆矩阵", "轮廓系数", "验证集"} {
		if strings.Contains(joined, bad) {
			t.Errorf("预处理任务的证据要点不应出现「%s」：%s", bad, joined)
		}
	}
}

// 预处理任务必须有对应的迁移题族，且题目限定在数据流程知识点内。
func TestLeakageTransferStaysInTaskType(t *testing.T) {
	if len(content.FamiliesByTaskType(content.TaskDataPipeline)) == 0 {
		t.Fatal("预处理任务缺少迁移题族")
	}
	item, info := GenerateTransfer(GenerateInput{
		TaskType:   content.TaskDataPipeline,
		Archetypes: []string{"E15"},
		Seed:       "seed-leakage",
	})
	if item.TaskType != content.TaskDataPipeline {
		t.Fatalf("迁移题应属于预处理任务，实际 %s", item.TaskType)
	}
	text := item.Topic + item.Context + item.Question
	for _, bad := range []string{"混淆矩阵", "召回率", "精确率", "轮廓系数", "MAE", "RMSE"} {
		if strings.Contains(text, bad) {
			t.Errorf("预处理任务迁移题不得出现「%s」：%s", bad, text)
		}
	}
	if info.SelectionReason == "" || info.DedupNote == "" {
		t.Error("迁移题必须标注选题依据与重复检查结果")
	}
}
