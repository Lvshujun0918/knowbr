package services

import (
	"strings"
	"testing"

	"knowbr/server/internal/content"
	"knowbr/server/internal/models"
)

// 需求文档测试 1：同一份回答中的不同错误必须被拆成独立的诊断对象。
func TestClaimsSplitDistinctIssues(t *testing.T) {
	in := DiagnoseInput{
		ExperimentTitle:    "MLP 与 CNN 图像分类模型对比实验",
		ExperimentGoal:     "在同一图像数据集上训练 MLP 与 CNN 并解释差异来源。",
		DatasetDescription: "图像分类数据集共 10000 张图，10 个类别。",
		ExecutionLog:       "MLP test_accuracy = 0.72\nCNN test_accuracy = 0.92",
		Explanation: "CNN 的准确率比 MLP 高 20 个百分点，说明 CNN 的网络结构更复杂，所以准确率一定更高。" +
			"如果继续增加 MLP 的隐藏层和神经元数量，它的准确率就一定能超过 CNN。" +
			"只要测试集准确率提高，就说明模型的泛化能力更强，不需要再分析其他指标。",
	}
	diag := Diagnose(in)
	if diag.TaskType != content.TaskArchitecture {
		t.Fatalf("应识别为模型结构任务，实际 %s", diag.TaskType)
	}
	if len(diag.Claims) < 3 {
		t.Fatalf("三个不同主张应被分别诊断，实际只有 %d 条：%+v", len(diag.Claims), diag.Claims)
	}
	points := map[string]string{}
	for _, c := range diag.Claims {
		points[c.KnowledgePoint] = c.Status
	}
	for _, want := range []string{"模型复杂度与性能的因果关系", "模型容量与数据规模的匹配", "训练集与测试集表现差异（过拟合）"} {
		if _, ok := points[want]; !ok {
			t.Errorf("缺少对「%s」的独立诊断：%v", want, points)
		}
	}
	for _, c := range diag.Claims {
		if c.Status == ClaimError {
			if c.Evidence == "" || c.Reason == "" || c.EvidenceBoundary == "" || c.NextQuestion == "" {
				t.Errorf("主张 %s 必须带证据、原因、证据边界与追问：%+v", c.KnowledgePoint, c)
			}
		}
	}
	if diag.MasteryStatus != MasteryNotShown && diag.MasteryStatus != MasteryPartial {
		t.Errorf("存在错误主张时掌握情况不应为 %s", diag.MasteryStatus)
	}
}

// 需求文档测试 6 / 2.4：未提及不等于未掌握，不相关的知识点不得列为缺陷。
func TestNotMentionedIsNotDeficiency(t *testing.T) {
	diag := Diagnose(DiagnoseInput{
		ExperimentTitle:    "MLP 与 CNN 图像分类模型对比实验",
		ExperimentGoal:     "比较两种结构在图像分类上的表现。",
		DatasetDescription: "训练集 8000 张、测试集 2000 张。",
		Explanation:        "MLP 需要把图像展平成一维向量，空间关系被打散；CNN 通过局部连接与权重共享提取局部特征，因此通常在图像任务上更有优势。",
	})
	if diag.TaskType != content.TaskArchitecture {
		t.Fatalf("任务类型错误：%s", diag.TaskType)
	}
	text := diag.MainIssue + diag.Reasoning + diag.NextStep
	for _, forbidden := range []string{"F1", "精确率", "召回率", "类别不平衡", "混淆矩阵"} {
		if strings.Contains(text, forbidden) {
			t.Errorf("结构比较任务不应要求学生回答「%s」：%s", forbidden, text)
		}
	}
	for _, k := range diag.Knowledge {
		if k.Status == ClaimError {
			t.Errorf("正确的结构解释不应被判为错误：%+v", k)
		}
	}
	for _, c := range diag.Claims {
		if c.Status == ClaimError {
			t.Errorf("正确的结构解释不应被判为错误主张：%+v", c)
		}
	}
}

// 需求文档测试 9 的核心：训练稳定性任务的诊断必须围绕日志数值，且不转向分类指标。
func TestTrainingClaimsStayOnTopic(t *testing.T) {
	diag := Diagnose(DiagnoseInput{
		ExperimentTitle:    "RNN 训练损失异常与梯度裁剪实验",
		ExperimentGoal:     "判断训练是否稳定并分析损失异常的原因。",
		DatasetDescription: "字符级语料 5000 行，词表大小 128。",
		ModelName:          "单层 RNN，学习率 0.5，未使用梯度裁剪",
		ExecutionLog:       "epoch 3 train_loss=2.37 grad_norm=18.9\nepoch 4 train_loss=6.84 grad_norm=95.3\nepoch 5 train_loss=nan grad_norm=nan",
		CodeSnippet:        "loss = train_one_epoch(model, loader, optimizer)",
		Explanation:        "可能发生了梯度爆炸，学习率 0.5 也许让问题更严重。但我不确定能不能仅凭这份日志确定根本原因。",
		MetricsText:        "训练损失与梯度范数见运行日志（模拟数据）。",
	})
	if diag.TaskType != content.TaskTraining {
		t.Fatalf("应识别为训练稳定性任务，实际 %s", diag.TaskType)
	}
	if diag.PrimaryArchetype != "E6" {
		t.Errorf("应命中训练异常归因规则 E6，实际 %v", diag.MatchedArchetypes)
	}
	joined := strings.Join(diag.Evidence, "\n")
	if !strings.Contains(joined, "95.3") {
		t.Errorf("诊断证据必须引用学生日志中的具体数值：%s", joined)
	}
	for _, forbidden := range []string{"混淆矩阵", "召回率", "精确率", "F1", "疾病筛查", "漏诊", "类别分布"} {
		if strings.Contains(diag.MainIssue+diag.Reasoning+diag.NextStep, forbidden) {
			t.Errorf("训练稳定性诊断不得出现「%s」：%s", forbidden, diag.MainIssue+diag.Reasoning+diag.NextStep)
		}
	}
	if !strings.Contains(diag.Reasoning, "推断") {
		t.Errorf("诊断理由应说明哪些是推断、哪些尚未证实：%s", diag.Reasoning)
	}
	if diag.MasteryStatus != MasteryPartial {
		t.Errorf("“假设合理但未证实”应判为部分掌握，实际 %s", diag.MasteryStatus)
	}
}

// 三级提示必须与当前实验绑定：训练稳定性任务的提示不得出现类别不平衡或分类指标。
func TestTrainingHintsStayOnTopic(t *testing.T) {
	diag := Diagnose(DiagnoseInput{
		ExperimentTitle: "RNN 训练损失异常与梯度裁剪实验",
		ExperimentGoal:  "判断训练是否稳定并分析损失异常的原因。",
		ExecutionLog:    "epoch 4 train_loss=6.84 grad_norm=95.3\nepoch 5 train_loss=nan grad_norm=nan",
		Explanation:     "可能发生了梯度爆炸，但我不确定能否仅凭日志确定根因。",
	})
	state := models.HintState{NextLevel: 1, MaxLevel: 3}
	var existing []models.Hint
	for level := 1; level <= 3; level++ {
		hint, newState, err := BuildHint(HintRequest{
			Level: level, Diagnosis: diag, State: state, Existing: existing,
			TaskType: diag.TaskType, LogTail: "epoch 5 train_loss=nan grad_norm=nan",
		})
		if err != nil {
			t.Fatalf("生成第 %d 级提示失败：%v", level, err)
		}
		for _, forbidden := range []string{"混淆矩阵", "召回率", "精确率", "F1", "疾病筛查", "漏诊", "1000", "类别分布"} {
			if strings.Contains(hint.Content, forbidden) {
				t.Errorf("第 %d 级提示不得出现「%s」：%s", level, forbidden, hint.Content)
			}
		}
		if hint.Why == "" {
			t.Errorf("第 %d 级提示必须说明为什么选择它", level)
		}
		existing = append(existing, hint)
		state = newState
	}
	if !strings.Contains(existing[2].Content, "18.9") {
		t.Errorf("三级提示应给出与训练稳定性直接相关的局部示例：%s", existing[2].Content)
	}
}

// 已掌握的内容不应被反复追问：最新回答解决知识点后，提示必须说明这一点。
func TestHintAcknowledgesMasteredContent(t *testing.T) {
	diag := Diagnose(DiagnoseInput{
		ExperimentTitle: "RNN 训练损失异常与梯度裁剪实验",
		ExperimentGoal:  "判断训练是否稳定并分析损失异常的原因。",
		ExecutionLog:    "epoch 4 train_loss=6.84 grad_norm=95.3",
		Explanation:     "可能发生了梯度爆炸，但我不确定能否仅凭日志确定根因。",
	})
	latest := "从日志看，事实是第 4 轮梯度范数升到 95.3、第 5 轮出现 NaN；我的推断是梯度爆炸；未知的是损失或数据里有没有非有限值。"
	claims := AnalyzeClaims(DiagnoseInput{Explanation: latest}, diag.TaskType, diag.Contradiction)
	hint, _, err := BuildHint(HintRequest{
		Level: 2, Diagnosis: diag, State: models.HintState{NextLevel: 1, MaxLevel: 3}, TaskType: diag.TaskType,
		LatestAnswer: latest, LatestClaims: claims, LogTail: "epoch 4 train_loss=6.84 grad_norm=95.3",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(hint.Content, "已经解决") {
		t.Errorf("提示应先确认学生已经掌握的内容：%s", hint.Content)
	}
	if hint.Why == "" {
		t.Error("提示必须说明为什么选择它")
	}
	unresolved := 0
	for _, c := range claims {
		switch c.Status {
		case ClaimError, ClaimPartial, ClaimInsufficient:
			unresolved++
		}
	}
	if unresolved > 0 && hint.TargetKnowledge == "" {
		t.Error("存在未解决主张时，提示必须说明本轮针对的知识点")
	}
	if unresolved == 0 {
		if hint.TargetKnowledge != "" {
			t.Errorf("全部解决后不应再指定新的知识点：%s", hint.TargetKnowledge)
		}
		if !strings.Contains(hint.Content, "已经达到") {
			t.Errorf("全部解决后提示应说明当前目标已达到：%s", hint.Content)
		}
	}
}
