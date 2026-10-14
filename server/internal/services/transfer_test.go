package services

import (
	"strings"
	"testing"

	"knowbr/server/internal/content"
)

func TestSelectTransferItemPrefersMatchedArchetype(t *testing.T) {
	item := SelectTransferItem([]string{"E1"}, nil)
	if item.ID != "T1" {
		t.Errorf("命中 E1 时应优先选 T1（疾病筛查），实际 %s", item.ID)
	}
	used := SelectTransferItem([]string{"E1"}, []string{"T1"})
	if used.ID == "T1" {
		t.Error("已使用的题目不应被重复选择")
	}
	none := SelectTransferItem(nil, nil)
	if none.ID == "" {
		t.Error("没有命中误区时也应能选出题目，保证流程不中断")
	}
}

func TestScoreTransferEmptyAnswer(t *testing.T) {
	item, ok := content.TransferItemByID("T1")
	if !ok {
		t.Fatal("题库缺少 T1")
	}
	state := ScoreTransfer(item, "不知道。")
	if state.Score != 0 {
		t.Errorf("无关回答应得 0 分，实际 %d", state.Score)
	}
	if !state.Answered || state.ScoredAt == "" {
		t.Error("作答后应标记为已作答并记录时间")
	}
	if len(state.RubricFeedback) != len(item.Rubric) {
		t.Fatalf("每个评分维度都应给出理由，实际 %d 条", len(state.RubricFeedback))
	}
	for _, f := range state.RubricFeedback {
		if f.Reason == "" {
			t.Errorf("维度 %s 缺少评分理由", f.Title)
		}
		if f.Score != 0 {
			t.Errorf("维度 %s 期望 0 分，实际 %d", f.Title, f.Score)
		}
	}
	if len(state.MissingPoints) != 3 {
		t.Errorf("应列出 3 个未覆盖维度，实际 %v", state.MissingPoints)
	}
	if state.Disclaimer == "" {
		t.Error("必须携带评分口径声明")
	}
}

func TestScoreTransferFullAnswer(t *testing.T) {
	item, _ := content.TransferItemByID("T1")
	answer := "阳性样本极少属于极端类别不平衡；本任务最怕漏诊（假阴性），所以主指标选阳性类的召回率，" +
		"同时看精确率以避免大量误报，并查看混淆矩阵中假阳性和假阴性的具体数量，再按可接受的漏诊率与误报代价调整阈值。"
	state := ScoreTransfer(item, answer)
	if state.Score != item.MaxScore {
		t.Fatalf("覆盖全部要点的回答应得满分 %d，实际 %d（理由：%+v）", item.MaxScore, state.Score, state.RubricFeedback)
	}
	for _, f := range state.RubricFeedback {
		if f.Score != f.MaxScore {
			t.Errorf("维度 %s 应得满分，实际 %d/%d：%s", f.Title, f.Score, f.MaxScore, f.Reason)
		}
		if f.Reason == "" {
			t.Errorf("维度 %s 缺少评分理由", f.Title)
		}
	}
	if len(state.MatchedKeywords) == 0 {
		t.Error("应记录命中的关键词作为评分依据")
	}
	if strings.Contains(state.ScoreLabel, "未") {
		t.Errorf("分数标签异常：%s", state.ScoreLabel)
	}
}

func TestScoreTransferPartialCredit(t *testing.T) {
	item, _ := content.TransferItemByID("T1")
	// 只提到“阳性样本很少”，仅覆盖第一个维度的一组关键词。
	state := ScoreTransfer(item, "阳性样本很少，所以准确率没意义。")
	if state.Score != 1 {
		t.Fatalf("部分命中应得 1 分，实际 %d（%+v）", state.Score, state.RubricFeedback)
	}
	// MissingPoints 表示“尚未得满分的维度”，部分得分的维度也应计入，供学生继续补充。
	if len(state.MissingPoints) != 3 {
		t.Errorf("应指出三个未满分维度，实际 %v", state.MissingPoints)
	}
	if state.RubricFeedback[0].Score != 1 {
		t.Errorf("第一个维度应得 1 分，实际 %d", state.RubricFeedback[0].Score)
	}
	if state.RubricFeedback[0].Reason == "" {
		t.Error("部分得分也必须给出理由")
	}
}

// 参考答案只能在学生作答之后出现。
func TestReferenceHiddenUntilAnswered(t *testing.T) {
	item, _ := content.TransferItemByID("T1")
	fresh := NewTransferState(item)
	if fresh.ReferenceAnswer != "" || len(fresh.ExpectedPoints) != 0 {
		t.Error("未作答时不得包含参考答案或期望要点")
	}
	if fresh.Answered {
		t.Error("未作答时 answered 应为 false")
	}
	revealed := RevealReference(fresh)
	if revealed.ReferenceAnswer != "" {
		t.Error("未作答时调用 RevealReference 不应泄露答案")
	}
	scored := ScoreTransfer(item, "关注阳性类召回率，因为漏诊代价高。")
	after := RevealReference(scored)
	if after.ReferenceAnswer == "" || len(after.ExpectedPoints) == 0 {
		t.Error("作答后应能查看参考答案与期望要点")
	}
}

func TestScoreIsDeterministic(t *testing.T) {
	item, _ := content.TransferItemByID("T2")
	answer := "有害垃圾被误判为其他类，召回率只有 35%，应优先处理该类并查看宏平均 F1。"
	first := ScoreTransfer(item, answer)
	second := ScoreTransfer(item, answer)
	if first.Score != second.Score {
		t.Fatalf("同一答案两次评分应一致：%d vs %d", first.Score, second.Score)
	}
	for i := range first.RubricFeedback {
		if first.RubricFeedback[i].Reason != second.RubricFeedback[i].Reason {
			t.Errorf("维度 %d 的理由应可复现", i)
		}
	}
}
