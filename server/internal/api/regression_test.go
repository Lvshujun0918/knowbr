package api_test

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"knowbr/server/internal/models"
)

// 需求文档测试 9（P0）：Lab5-02 这类训练稳定性案例的诊断、三级提示、迁移题与评分
// 必须全部围绕当前实验主题，不得转向混淆矩阵、类别分布或分类指标。
func TestLab5RNNCaseEndToEnd(t *testing.T) {
	h := newTestHandler(t, filepath.Join(t.TempDir(), "rnn.db"))

	rec, created := do(t, h, http.MethodPost, "/api/sessions", demoSubmission(t, "case-rnn-instability"))
	if rec.Code != http.StatusOK {
		t.Fatalf("提交 RNN 案例失败：%d %s", rec.Code, rec.Body.String())
	}
	id := digStr(t, created, "session", "id")
	if got := digStr(t, created, "session", "diagnosis", "taskType"); got != "training_stability" {
		t.Fatalf("任务类型应为训练稳定性，实际 %s", got)
	}
	if got := digStr(t, created, "session", "diagnosis", "primaryArchetype"); got != "E6" {
		t.Errorf("应命中训练异常归因规则 E6，实际 %s", got)
	}
	// 初始诊断必须引用日志中的具体数值。
	evidence := strings.Join(toStrList(digList(t, created, "session", "diagnosis", "evidence")), "\n")
	if !strings.Contains(evidence, "95.3") {
		t.Errorf("诊断证据必须引用日志中的具体数值：%s", evidence)
	}
	reasoning := digStr(t, created, "session", "diagnosis", "reasoning")
	if !strings.Contains(reasoning, "推断") {
		t.Errorf("诊断理由必须区分事实与推断：%s", reasoning)
	}
	// 逐条主张必须覆盖“假设尚未证实”这一点。
	claims := digList(t, created, "session", "diagnosis", "claims")
	if len(claims) == 0 {
		t.Fatal("必须把学生的主张拆成独立的诊断对象")
	}

	// 三级提示：内容与文档禁止项。
	forbiddenHint := []string{"混淆矩阵", "召回率", "精确率", "F1", "疾病筛查", "漏诊", "类别分布", "1000"}
	for level := 1; level <= 3; level++ {
		rec, hint := do(t, h, http.MethodPost, "/api/sessions/"+id+"/hints", map[string]any{"level": level, "requestedByStudent": true})
		if rec.Code != http.StatusOK {
			t.Fatalf("生成第 %d 级提示失败：%d %s", level, rec.Code, rec.Body.String())
		}
		content := digStr(t, hint, "hint", "content")
		for _, bad := range forbiddenHint {
			if strings.Contains(content, bad) {
				t.Errorf("第 %d 级提示不得出现「%s」：%s", level, bad, content)
			}
		}
		if digStr(t, hint, "hint", "why") == "" {
			t.Errorf("第 %d 级提示必须说明为什么选择它", level)
		}
	}

	// 迁移题：限定在训练稳定性任务内，且标注来源与去重检查。
	rec, tq := do(t, h, http.MethodPost, "/api/sessions/"+id+"/transfer/question", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("生成迁移题失败：%d %s", rec.Code, rec.Body.String())
	}
	if got := digStr(t, tq, "transfer", "taskType"); got != "training_stability" {
		t.Fatalf("迁移题任务类型应为训练稳定性，实际 %s", got)
	}
	question := digStr(t, tq, "transfer", "topic") + digStr(t, tq, "transfer", "context") + digStr(t, tq, "transfer", "question")
	for _, bad := range []string{"混淆矩阵", "召回率", "精确率", "F1", "疾病筛查", "漏诊", "类别不平衡", "1000"} {
		if strings.Contains(question, bad) {
			t.Errorf("训练稳定性迁移题不得出现「%s」：%s", bad, question)
		}
	}
	if digStr(t, tq, "transfer", "generationNote") == "" || digStr(t, tq, "transfer", "dedupNote") == "" {
		t.Error("迁移题必须标注生成方式与重复检查结果")
	}
	if got := digNum(t, tq, "transfer", "similarityMax"); got >= 0.45 {
		t.Errorf("与示例情境的相似度 %.2f 达到重复阈值", got)
	}

	// 评分必须给出可核查的理由，而不是只看关键词命中数量。
	weak := "训练不收敛，加梯度裁剪就好了。"
	rec, scored := do(t, h, http.MethodPost, "/api/sessions/"+id+"/transfer/answer", map[string]any{"answer": weak})
	if rec.Code != http.StatusOK {
		t.Fatalf("提交答案失败：%d %s", rec.Code, rec.Body.String())
	}
	if got := digNum(t, scored, "transfer", "score"); got >= 6 {
		t.Errorf("笼统回答不应直接得满分，实际 %v", got)
	}
	for _, f := range digList(t, scored, "transfer", "rubricFeedback") {
		fb := f.(map[string]any)
		if fb["reason"] == "" {
			t.Errorf("评分维度必须给出理由：%v", fb)
		}
	}
	_, revealed := do(t, h, http.MethodPost, "/api/sessions/"+id+"/transfer/reference", nil)
	reference := digStr(t, revealed, "transfer", "referenceAnswer")
	if reference == "" {
		t.Fatal("作答后应能查看参考答案")
	}
	if !strings.Contains(reference, "对照") && !strings.Contains(reference, "学习率") {
		t.Errorf("训练稳定性参考答案应围绕对照与学习率展开：%s", reference)
	}

	// 学习反馈必须记录真实发生的过程：仍未解决的问题、任务类型与题目来源。
	rec, done := do(t, h, http.MethodPost, "/api/sessions/"+id+"/transfer/complete", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("完成学习失败：%d %s", rec.Code, rec.Body.String())
	}
	summary := dig(done, "summary").(map[string]any)
	if len(summary["stillOpen"].([]any)) == 0 {
		t.Error("学习反馈必须列出仍未解决的问题")
	}
	if len(summary["claims"].([]any)) == 0 || len(summary["knowledge"].([]any)) == 0 {
		t.Error("学习反馈必须保留逐条主张与知识点状态")
	}
	teacher := digStr(t, done, "summary", "teacherSummary")
	if !strings.Contains(teacher, "训练稳定性") {
		t.Errorf("教师摘要应写明任务类型：%s", teacher)
	}
	if !strings.Contains(teacher, "题目来源") {
		t.Errorf("教师摘要应写明迁移题来源：%s", teacher)
	}
}

// 需求文档测试 5：学生回答改变，下一轮内容也必须改变。
func TestAnswerChangeChangesNextRound(t *testing.T) {
	h := newTestHandler(t, filepath.Join(t.TempDir(), "change.db"))

	makeSession := func(explanation string) map[string]any {
		sub := demoSubmission(t, "case-mlp-cnn")
		sub.StudentInitialExplanation = explanation
		rec, body := do(t, h, http.MethodPost, "/api/sessions", sub)
		if rec.Code != http.StatusOK {
			t.Fatalf("创建会话失败：%d %s", rec.Code, rec.Body.String())
		}
		return body
	}

	good := makeSession("MLP 需要把图像展平成一维向量，空间关系被打散；CNN 通过局部连接和权重共享提取局部特征，所以在图像任务上通常更有结构优势。")
	bad := makeSession("因为 CNN 更复杂。")

	goodStep := digStr(t, good, "session", "diagnosis", "nextStep")
	badStep := digStr(t, bad, "session", "diagnosis", "nextStep")
	if goodStep == badStep {
		t.Errorf("两种回答应得到不同的下一步：%s", goodStep)
	}
	if got := digStr(t, good, "session", "diagnosis", "masteryStatus"); got != "demonstrated" {
		t.Errorf("正确的结构解释应判为已体现理解，实际 %s", got)
	}
	if got := digStr(t, bad, "session", "diagnosis", "masteryStatus"); got == "demonstrated" {
		t.Errorf("笼统回答不应判为已体现理解")
	}
	if strings.Contains(goodStep, "展平") && strings.Contains(goodStep, "再说明一遍") {
		t.Errorf("已经掌握的概念不应被重复追问：%s", goodStep)
	}

	goodID := digStr(t, good, "session", "id")
	badID := digStr(t, bad, "session", "id")
	_, goodHint := do(t, h, http.MethodPost, "/api/sessions/"+goodID+"/hints", map[string]any{"level": 1})
	_, badHint := do(t, h, http.MethodPost, "/api/sessions/"+badID+"/hints", map[string]any{"level": 1})
	if digStr(t, goodHint, "hint", "content") == digStr(t, badHint, "hint", "content") {
		t.Error("不同回答应得到不同的提示内容")
	}
}

// 需求文档测试 3：缺少证据时不得编造具体类别之间的错误。
func TestMissingEvidenceIsNotFabricated(t *testing.T) {
	h := newTestHandler(t, filepath.Join(t.TempDir(), "missing.db"))

	rec, created := do(t, h, http.MethodPost, "/api/sessions", models.Submission{
		ExperimentTitle:           "分类模型评价实验",
		ExperimentGoal:            "判断模型是否可用。",
		StudentInitialExplanation: "整体测试集准确率是 93%，模型看起来不错。",
		MetricsText:               "准确率 0.93",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("应允许提交：%d %s", rec.Code, rec.Body.String())
	}
	text := digStr(t, created, "session", "diagnosis", "mainIssue") +
		digStr(t, created, "session", "diagnosis", "reasoning") +
		digStr(t, created, "session", "diagnosis", "nextStep")
	for _, fabricated := range []string{"被预测为类别", "混淆矩阵中", "某一类有", "0 个样本"} {
		if strings.Contains(text, fabricated) {
			t.Errorf("没有提供矩阵时不得声称观察到具体类别错误（「%s」）：%s", fabricated, text)
		}
	}
	needs := digList(t, created, "session", "diagnosis", "evidenceNeeds")
	foundMissing := false
	for _, n := range needs {
		item := n.(map[string]any)
		if item["present"] == false && item["required"] == true {
			foundMissing = true
			if item["impact"] == "" {
				t.Errorf("必需材料缺失时必须说明它限制了哪些结论：%v", item)
			}
		}
	}
	if !foundMissing {
		t.Error("缺少必需材料时应明确列出")
	}
	// 未涉及的知识点不得被判为错误或已掌握。
	for _, k := range digList(t, created, "session", "diagnosis", "knowledge") {
		item := k.(map[string]any)
		if item["status"] == "error" && item["name"] != nil {
			if s, _ := item["note"].(string); !strings.Contains(s, "冲突") && !strings.Contains(s, "需要先修正") {
				t.Errorf("知识点被判为错误时必须说明原因：%v", item)
			}
		}
	}
}

// 需求文档测试 8：规则模式必须可识别，不能把模板结果说成模型动态生成。
func TestRuleModeIsIdentifiable(t *testing.T) {
	h := newTestHandler(t, filepath.Join(t.TempDir(), "mode.db"))
	_, created := do(t, h, http.MethodPost, "/api/sessions", demoSubmission(t, "case-imbalance"))
	id := digStr(t, created, "session", "id")
	_, tq := do(t, h, http.MethodPost, "/api/sessions/"+id+"/transfer/question", nil)
	note := digStr(t, tq, "transfer", "generationNote")
	if !strings.Contains(note, "规则模板") || !strings.Contains(note, "未调用大模型") {
		t.Errorf("必须明确说明题目由规则模板生成：%s", note)
	}
	if got := digStr(t, tq, "transfer", "source"); got != "rule_template" {
		t.Errorf("来源标记应为 rule_template，实际 %s", got)
	}
	if notice := digStr(t, created, "session", "modeNotice"); !strings.Contains(notice, "未配置") {
		t.Errorf("运行模式必须说明真实原因：%s", notice)
	}
}

func toStrList(list []any) []string {
	out := make([]string, 0, len(list))
	for _, v := range list {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
