package services

import (
	"testing"

	"knowbr/server/internal/content"
)

// 每个预置案例都必须落到正确的任务类型上：任务类型决定后续诊断、提示与迁移题的范围。
func TestDetectTaskTypeForDemoCases(t *testing.T) {
	want := map[string]string{
		"case-imbalance":       content.TaskClassification,
		"case-accuracy-only":   content.TaskClassification,
		"case-misread-matrix":  content.TaskClassification,
		"case-run-only":        content.TaskClassification,
		"case-env-error":       content.TaskCodeEnv,
		"case-mlp-cnn":         content.TaskArchitecture,
		"case-rnn-instability": content.TaskTraining,
		"case-regression":      content.TaskRegression,
		"case-clustering":      content.TaskClustering,
	}
	for _, c := range content.DemoCases {
		t.Run(c.ID, func(t *testing.T) {
			d := c.Submission
			got := DetectTaskType(DiagnoseInput{
				ExperimentTitle:    d.ExperimentTitle,
				ExperimentGoal:     d.ExperimentGoal,
				ModelName:          d.ModelName,
				Explanation:        d.StudentInitialExplanation,
				Question:           d.Question,
				DatasetDescription: d.DatasetDescription,
				MetricsText:        d.MetricsText,
				ExecutionLog:       d.ExecutionLog,
				CodeSnippet:        d.CodeSnippet,
			})
			if got != want[c.ID] {
				t.Errorf("案例 %s 的任务类型应为 %s，实际 %s", c.ID, want[c.ID], got)
			}
		})
	}
}

// 任务类型必须限制规则命中范围：分类指标类规则不得在训练稳定性任务上触发。
func TestArchetypeTaskTypeGating(t *testing.T) {
	m, err := ComputeMetrics(coreDemoMatrix(), []string{"类别A", "类别B", "类别C"})
	if err != nil {
		t.Fatal(err)
	}
	text := "准确率达到 90%，模型效果很好，可以直接使用。"
	classification := matchArchetypes(text, m, "类别 A 900 个、B 90 个、C 10 个。", content.TaskClassification)
	if len(classification) == 0 {
		t.Fatal("分类任务下应命中分类指标类规则")
	}
	training := matchArchetypes(text, m, "类别 A 900 个、B 90 个、C 10 个。", content.TaskTraining)
	for _, a := range training {
		if a.Code == "E1" || a.Code == "E2" || a.Code == "E3" || a.Code == "E4" {
			t.Fatalf("训练稳定性任务不应命中分类指标类规则：%v", training)
		}
	}
}

// 每个任务类型都必须有可用的必需证据定义，且训练类任务不需要混淆矩阵。
func TestTrainingTaskDoesNotRequireMatrix(t *testing.T) {
	in := DiagnoseInput{
		ExperimentTitle:    "RNN 训练损失异常",
		ExperimentGoal:     "判断训练是否稳定并分析原因。",
		Explanation:        "可能发生了梯度爆炸。",
		ExecutionLog:       "epoch 4 train_loss=6.84 grad_norm=95.3",
		DatasetDescription: "字符级语料 5000 行。",
	}
	taskType := DetectTaskType(in)
	if taskType != content.TaskTraining {
		t.Fatalf("应识别为训练稳定性任务，实际 %s", taskType)
	}
	needs := BuildEvidenceNeeds(in, taskType)
	for _, n := range needs {
		if n.Name == "混淆矩阵或按类别的指标" {
			t.Fatal("训练稳定性任务不应把混淆矩阵列为必需材料")
		}
	}
	found := false
	for _, n := range needs {
		if n.Name == "训练日志（含 loss 与梯度范数等数值）" {
			found = true
			if !n.Required || !n.Present {
				t.Errorf("训练日志应为必需且已提供：%+v", n)
			}
		}
	}
	if !found {
		t.Fatalf("训练稳定性任务应要求训练日志：%+v", needs)
	}
}
