package services

import (
	"strings"
	"testing"

	"knowbr/server/internal/content"
)

func demoExampleTexts() []string { return content.ExampleScenarioTexts }

// 迁移题必须与当前任务类型一致，且不得越出该任务类型的知识范围。
func TestGenerateTransferStaysInTaskType(t *testing.T) {
	forbiddens := map[string][]string{
		content.TaskTraining:     {"混淆矩阵", "召回率", "精确率", "F1", "疾病筛查", "漏诊", "类别不平衡", "1000"},
		content.TaskArchitecture: {"混淆矩阵", "召回率", "精确率", "F1", "疾病筛查", "漏诊", "类别不平衡"},
	}
	for _, taskType := range []string{content.TaskClassification, content.TaskTraining, content.TaskArchitecture} {
		t.Run(taskType, func(t *testing.T) {
			found := 0
			for seed := 0; seed < 12; seed++ {
				item, info := GenerateTransfer(GenerateInput{
					TaskType:     taskType,
					Archetypes:   []string{"E1", "E8", "E6"},
					FocusPoints:  TransferFocusPoints(taskType),
					ExampleTexts: demoExampleTexts(),
					Seed:         string(rune('a'+seed)) + "-seed",
				})
				if item.TaskType != taskType {
					t.Fatalf("题目任务类型应为 %s，实际 %s", taskType, item.TaskType)
				}
				found++
				text := item.Topic + item.Context + item.Question + item.ReferenceAnswer
				if strings.Contains(text, "{{") {
					t.Errorf("题目存在未替换的参数槽位：%s", text)
				}
				for _, forbidden := range forbiddens[taskType] {
					if strings.Contains(text, forbidden) {
						t.Errorf("任务类型 %s 的题目不得出现「%s」：%s", taskType, forbidden, text)
					}
				}
				if item.MaxScore != 6 || len(item.Rubric) != 3 {
					t.Errorf("题目结构异常：maxScore=%d rubric=%d", item.MaxScore, len(item.Rubric))
				}
				if info.SimilarityMax >= content.SimilarityThreshold {
					t.Errorf("题目的相似度 %.2f 达到重复阈值", info.SimilarityMax)
				}
				if !strings.Contains(info.DedupNote, "相似度") {
					t.Errorf("必须给出去重检查说明：%s", info.DedupNote)
				}
			}
			if found == 0 {
				t.Fatalf("任务类型 %s 下没有可用题目", taskType)
			}
		})
	}
}

// 需求文档测试 4：迁移题不得原样复刻演示案例或提示中已经出现过的示例情境。
func TestGenerateTransferAvoidsExampleScenario(t *testing.T) {
	item, info := GenerateTransfer(GenerateInput{
		TaskType:     content.TaskClassification,
		Archetypes:   []string{"E1", "E3", "E4"},
		FocusPoints:  TransferFocusPoints(content.TaskClassification),
		ExampleTexts: demoExampleTexts(),
		Seed:         "case-imbalance",
	})
	if item.ID == "T1" {
		t.Fatal("不得直接使用与示例情境相同的疾病筛查固定题")
	}
	if item.ExampleScenario != "" {
		t.Errorf("生成优先应选择新情境，实际沿用了示例情境：%s", item.ExampleScenario)
	}
	if info.Source != content.SourceRuleTemplate {
		t.Errorf("来源应标注为规则模板组合生成，实际 %s", info.Source)
	}
	question := item.Context + item.Question
	for _, example := range demoExampleTexts() {
		if s := content.Similarity(question, example); s >= content.SimilarityThreshold {
			t.Errorf("与示例情境相似度过高（%.2f）：%s", s, example)
		}
	}
	if item.Scenario == "" || item.KnowledgePoint == "" {
		t.Error("题目必须给出情境标签与考查知识点")
	}
	if len(item.RequiredData) == 0 || item.AnswerRequirement == "" {
		t.Error("题目必须给出必要数据与作答要求")
	}
}

// 同一场会话重复生成得到同一道题；不同会话得到不同的情境或参数组合。
func TestGenerateTransferIsDeterministicAndVaried(t *testing.T) {
	in := func(seed string) GenerateInput {
		return GenerateInput{
			TaskType:     content.TaskTraining,
			Archetypes:   []string{"E6"},
			FocusPoints:  TransferFocusPoints(content.TaskTraining),
			ExampleTexts: demoExampleTexts(),
			Seed:         seed,
		}
	}
	a1, _ := GenerateTransfer(in("session-a"))
	a2, _ := GenerateTransfer(in("session-a"))
	if a1.ID != a2.ID || a1.Question != a2.Question {
		t.Fatalf("同一会话的迁移题必须可复现：%s vs %s", a1.ID, a2.ID)
	}
	seen := map[string]bool{a1.Question: true}
	varied := 0
	for _, seed := range []string{"session-b", "session-c", "session-d", "session-e", "session-f"} {
		item, _ := GenerateTransfer(in(seed))
		if !seen[item.Question] {
			varied++
			seen[item.Question] = true
		}
	}
	if varied == 0 {
		t.Error("不同会话应得到不同情境或不同数据条件的题目，而不是同一道题")
	}
	if len(seen) < 2 {
		t.Errorf("迁移题的变化过少，只有 %d 种", len(seen))
	}
}

// 与本机已保存的记录重复时，应改选其它题目。
func TestGenerateTransferAvoidsUsedItems(t *testing.T) {
	base := GenerateInput{
		TaskType:     content.TaskTraining,
		Archetypes:   []string{"E6"},
		FocusPoints:  TransferFocusPoints(content.TaskTraining),
		ExampleTexts: demoExampleTexts(),
		Seed:         "session-used",
	}
	first, _ := GenerateTransfer(base)
	second := base
	second.UsedItemIDs = []string{first.ID}
	second.UsedTopics = []string{first.Topic}
	again, info := GenerateTransfer(second)
	if again.ID == first.ID {
		t.Errorf("已使用过的题目不应被再次选中：%s", first.ID)
	}
	if len(info.ComparedItems) == 0 {
		t.Error("应记录比对过的历史题目")
	}
}

// 生成出来的题目必须能按编号重建，否则无法评分。
func TestGeneratedItemRoundTrip(t *testing.T) {
	item, _ := GenerateTransfer(GenerateInput{
		TaskType:     content.TaskClassification,
		Archetypes:   []string{"E1"},
		FocusPoints:  TransferFocusPoints(content.TaskClassification),
		ExampleTexts: demoExampleTexts(),
		Seed:         "round-trip",
	})
	rebuilt, ok := content.ResolveTransferItem(item.ID)
	if !ok {
		t.Fatalf("无法按编号重建题目：%s", item.ID)
	}
	if rebuilt.Question != item.Question || rebuilt.MaxScore != item.MaxScore || len(rebuilt.Rubric) != len(item.Rubric) {
		t.Fatal("重建出的题目与生成结果不一致")
	}
	state := NewTransferState(item)
	scored, ok := ScoreTransferState(state, "正样本比例很低，准确率会误导，应优先看召回率与精确率，并按漏报代价调整阈值。")
	if !ok {
		t.Fatal("应按编号重建题目并完成评分")
	}
	if scored.Score <= 0 || scored.Score > item.MaxScore {
		t.Errorf("评分超出范围：%d", scored.Score)
	}
	if len(scored.RubricFeedback) != len(item.Rubric) {
		t.Error("每个评分维度都应给出理由")
	}
	if scored.ItemID != item.ID || scored.TaskType != item.TaskType {
		t.Error("评分结果必须保留题目编号与任务类型")
	}
}

// 任务类型必须有题目可用，且固定题库只作为回退。
func TestEveryTaskTypeHasTransferItems(t *testing.T) {
	for _, tt := range content.TaskTypes {
		t.Run(tt.Code, func(t *testing.T) {
			item, info := GenerateTransfer(GenerateInput{
				TaskType:     tt.Code,
				FocusPoints:  tt.FocusKnowledge,
				ExampleTexts: demoExampleTexts(),
				Seed:         "coverage-" + tt.Code,
			})
			if item.ID == "" || item.Question == "" {
				t.Fatalf("任务类型 %s 没有可用题目", tt.Code)
			}
			if info.Source == content.SourceFixedBank && tt.Code != content.TaskGeneric {
				t.Errorf("任务类型 %s 不应只能回退到固定题库", tt.Code)
			}
		})
	}
}
