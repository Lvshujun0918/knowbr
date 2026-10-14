package services

import (
	"strings"
	"testing"

	"knowbr/server/internal/content"
	"knowbr/server/internal/models"
)

// 修正反馈必须按任务类型判定：训练稳定性任务的回答不得被分类指标要求判为“无进展”。
func TestRevisionFeedbackStaysInTaskType(t *testing.T) {
	diag := Diagnose(DiagnoseInput{
		ExperimentTitle: "RNN 训练损失异常与梯度裁剪实验",
		ExperimentGoal:  "判断训练是否稳定并分析损失异常的原因。",
		ExecutionLog:    "epoch 4 train_loss=6.84 grad_norm=95.3\nepoch 5 train_loss=nan grad_norm=nan",
		Explanation:     "可能发生了梯度爆炸，但我不确定能否仅凭日志确定根因。",
	})
	if diag.TaskType != content.TaskTraining {
		t.Fatalf("任务类型错误：%s", diag.TaskType)
	}
	res := AnalyzeRevision(RevisionRequest{
		Content: "日志中的事实是第 4 轮梯度范数升到 95.3、第 5 轮出现 NaN；我的推断是梯度爆炸；" +
			"未知的是损失或数据里有没有非有限值。为了把原因缩小到一个变量，我打算只改变学习率再跑一次，其它设置固定；" +
			"如果仍然出现 NaN，就说明问题不只是学习率，梯度裁剪也不是根因。",
		InitialExplanation: "可能发生了梯度爆炸，但我不确定能否仅凭日志确定根因。",
		Diagnosis:          diag,
		TaskType:           diag.TaskType,
		State:              models.HintState{MaxLevel: 3},
	})
	if res.Revision.Progress == "none" {
		t.Fatalf("正确的训练稳定性回答不应判为无进展：%s", res.Revision.Feedback)
	}
	for _, forbidden := range []string{"召回率", "精确率", "F1", "混淆矩阵", "类别分布", "非对角线"} {
		if strings.Contains(res.Revision.Feedback, forbidden) {
			t.Errorf("训练稳定性任务的修正反馈不得出现「%s」：%s", forbidden, res.Revision.Feedback)
		}
	}
	for _, m := range res.Revision.MissingKeywords {
		if strings.Contains(m, "召回率") || strings.Contains(m, "精确率") {
			t.Errorf("缺失要点不得引用分类指标：%s", m)
		}
	}
	if len(res.Revision.AddressedKeywords) == 0 {
		t.Error("应记录新增的可核查证据要点")
	}
}

// 分类任务的修正反馈仍按分类指标判定，保证原有行为不回退。
func TestRevisionFeedbackForClassification(t *testing.T) {
	m, err := ComputeMetrics(coreDemoMatrix(), []string{"类别A", "类别B", "类别C"})
	if err != nil {
		t.Fatal(err)
	}
	diag := Diagnose(demoInput(t))
	res := AnalyzeRevision(RevisionRequest{
		Content:            "类别B 的召回率是 0%，因为 90 个样本全部被预测成 A；类别分布也不平衡。",
		InitialExplanation: "我的模型准确率达到 90%，所以我认为模型效果很好。",
		Diagnosis:          diag,
		Metrics:            m,
		TaskType:           diag.TaskType,
		State:              models.HintState{MaxLevel: 3},
	})
	if res.Revision.Progress != "improved" {
		t.Fatalf("应判定为 improved，实际 %s（%s）", res.Revision.Progress, res.Revision.Feedback)
	}
	if !strings.Contains(strings.Join(res.Revision.MissingKeywords, "；"), "精确率") &&
		!strings.Contains(strings.Join(res.Revision.MissingKeywords, "；"), "召回率") {
		t.Errorf("分类任务的缺失要点应仍是分类指标：%v", res.Revision.MissingKeywords)
	}
}
