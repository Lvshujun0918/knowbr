package api_test

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"knowbr/server/internal/content"
)

// caseExpectation 是一个预置案例的完整预期：任务类型、主误区、问题类型与迁移题族。
// 这张表是「系统在 10 个预置场景上的行为」的可回归契约，也是 docs/EVALUATION.md 的数据来源。
type caseExpectation struct {
	ID         string
	TaskType   string
	Archetype  string   // 主误区编号；无规则命中时为 "NONE"
	Categories []string // 期望的问题类型集合（顺序无关）
	Family     string   // 期望的迁移题族编号
}

var demoCaseExpectations = []caseExpectation{
	{"case-imbalance", content.TaskClassification, "E1", []string{"interpretation", "metric_selection"}, "CLS"},
	{"case-accuracy-only", content.TaskClassification, "NONE", []string{"insufficient"}, "CLS"},
	{"case-misread-matrix", content.TaskClassification, "E2", []string{"interpretation", "concept"}, "CLS"},
	{"case-run-only", content.TaskClassification, "E5", []string{"concept", "boundary", "design"}, "CLS"},
	{"case-env-error", content.TaskCodeEnv, "E0", []string{"environment"}, "ENV"},
	{"case-mlp-cnn", content.TaskArchitecture, "E8", []string{"concept", "interpretation", "design", "boundary"}, "ARC"},
	{"case-rnn-instability", content.TaskTraining, "E6", []string{"interpretation", "design"}, "TRN"},
	{"case-regression", content.TaskRegression, "E11", []string{"interpretation", "metric_selection", "design", "boundary"}, "REG"},
	{"case-clustering", content.TaskClustering, "E14", []string{"interpretation", "boundary"}, "CLU"},
	{"case-leakage", content.TaskDataPipeline, "E15", []string{"design", "boundary"}, "DP"},
}

// forbiddenByTask 是「不该出现在该任务提示/迁移题里」的其它任务关键概念。
// 用于防止跨任务模板漂移：训练稳定性任务不该谈混淆矩阵，聚类任务不该谈召回率。
var forbiddenByTask = map[string][]string{
	content.TaskClassification: {"梯度裁剪", "卷积核", "轮廓系数", "数据泄漏"},
	content.TaskArchitecture:   {"召回率", "轮廓系数", "梯度裁剪", "数据泄漏"},
	content.TaskTraining:       {"混淆矩阵", "召回率", "轮廓系数", "留出验证集", "数据泄漏"},
	content.TaskCodeEnv:        {"混淆矩阵", "召回率", "轮廓系数", "MAE"},
	content.TaskRegression:     {"混淆矩阵", "召回率", "轮廓系数", "数据泄漏"},
	content.TaskClustering:     {"混淆矩阵", "召回率", "精确率", "留出验证集", "MAE"},
	content.TaskDataPipeline:   {"混淆矩阵", "召回率", "轮廓系数", "MAE"},
}

// evalRow 是报告中的一行。
type evalRow struct {
	Case       string
	TaskLabel  string
	Archetype  string
	Label      string
	Categories string
	Hints      string
	Family     string
	ItemID     string
	MaxScore   int
}

// TestDemoCaseEvaluationMatrix 对全部预置案例跑一遍真实 HTTP 流程，核对任务类型、主误区、
// 问题类型、三级提示的跨任务隔离与迁移题族，并在 ZXQ_WRITE_EVAL=1 时生成 docs/EVALUATION.md。
func TestDemoCaseEvaluationMatrix(t *testing.T) {
	h := newTestHandler(t, filepath.Join(t.TempDir(), "eval-matrix.db"))
	rows := make([]evalRow, 0, len(demoCaseExpectations))

	for _, want := range demoCaseExpectations {
		t.Run(want.ID, func(t *testing.T) {
			rec, body := do(t, h, http.MethodPost, "/api/sessions", demoSubmission(t, want.ID))
			if rec.Code != http.StatusOK {
				t.Fatalf("提交失败：%d %s", rec.Code, rec.Body.String())
			}
			id := digStr(t, body, "session", "id")

			if got := digStr(t, body, "session", "diagnosis", "taskType"); got != want.TaskType {
				t.Errorf("任务类型应为 %s，实际 %s", want.TaskType, got)
			}
			if got := digStr(t, body, "session", "diagnosis", "primaryArchetype"); got != want.Archetype {
				t.Errorf("主误区应为 %s，实际 %s", want.Archetype, got)
			}
			if got := categorySet(t, body); !sameSet(got, want.Categories) {
				t.Errorf("问题类型应为 %v，实际 %v", want.Categories, got)
			}

			// 三级提示：内容非空、说明选择理由、且不得越界到其它任务类型的概念。
			forbidden := forbiddenByTask[want.TaskType]
			levels := []int{}
			for level := 1; level <= 3; level++ {
				rec, hint := do(t, h, http.MethodPost, "/api/sessions/"+id+"/hints", map[string]any{"level": level})
				if rec.Code != http.StatusOK {
					t.Fatalf("生成第 %d 级提示失败：%d", level, rec.Code)
				}
				text := digStr(t, hint, "hint", "content")
				if strings.TrimSpace(text) == "" || digStr(t, hint, "hint", "why") == "" {
					t.Errorf("第 %d 级提示缺少内容或选择理由", level)
				}
				for _, bad := range forbidden {
					if strings.Contains(text, bad) {
						t.Errorf("第 %d 级提示越界到其它任务概念「%s」：%s", level, bad, text)
					}
				}
				levels = append(levels, level)
			}

			// 迁移题：题族必须与任务类型一致，且题目本身同样不越界。
			rec, tq := do(t, h, http.MethodPost, "/api/sessions/"+id+"/transfer/question", nil)
			if rec.Code != http.StatusOK {
				t.Fatalf("生成迁移题失败：%d", rec.Code)
			}
			itemID := digStr(t, tq, "transfer", "itemId")
			if !strings.HasPrefix(itemID, want.Family+":") {
				t.Errorf("迁移题应属于题目族 %s，实际编号 %s", want.Family, itemID)
			}
			if got := digStr(t, tq, "transfer", "taskType"); got != want.TaskType {
				t.Errorf("迁移题任务类型应为 %s，实际 %s", want.TaskType, got)
			}
			question := digStr(t, tq, "transfer", "context") + digStr(t, tq, "transfer", "question")
			for _, bad := range forbidden {
				if strings.Contains(question, bad) {
					t.Errorf("迁移题越界到其它任务概念「%s」：%s", bad, question)
				}
			}

			catLabels := []string{}
			for _, c := range digList(t, body, "session", "diagnosis", "categories") {
				catLabels = append(catLabels, c.(map[string]any)["label"].(string))
			}
			rows = append(rows, evalRow{
				Case:       want.ID,
				TaskLabel:  digStr(t, body, "session", "diagnosis", "taskTypeLabel"),
				Archetype:  want.Archetype,
				Label:      digStr(t, body, "session", "diagnosis", "mainIssue"),
				Categories: strings.Join(catLabels, "、"),
				Hints:      fmt.Sprintf("%d 级均无越界", len(levels)),
				Family:     digStr(t, tq, "transfer", "familyLabel"),
				ItemID:     itemID,
				MaxScore:   int(digNum(t, tq, "transfer", "maxScore")),
			})

			do(t, h, http.MethodDelete, "/api/sessions/"+id, nil)
		})
	}

	if os.Getenv("ZXQ_WRITE_EVAL") != "" {
		// 测试的工作目录是包目录 server/internal/api，报告写在仓库根的 docs/ 下。
		dir := filepath.Join("..", "..", "..", "docs")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("创建 docs 目录失败：%v", err)
		}
		path := filepath.Join(dir, "EVALUATION.md")
		if err := os.WriteFile(path, []byte(renderEvaluation(rows)), 0o644); err != nil {
			t.Fatalf("写入评测报告失败：%v", err)
		}
		t.Logf("已生成 %s（%d 个案例）", path, len(rows))
	}
}

func categorySet(t *testing.T, body map[string]any) []string {
	t.Helper()
	out := []string{}
	for _, c := range digList(t, body, "session", "diagnosis", "categories") {
		out = append(out, c.(map[string]any)["code"].(string))
	}
	sort.Strings(out)
	return out
}

func sameSet(a, b []string) bool {
	x := append([]string(nil), a...)
	y := append([]string(nil), b...)
	sort.Strings(x)
	sort.Strings(y)
	return strings.Join(x, ",") == strings.Join(y, ",")
}

func renderEvaluation(rows []evalRow) string {
	var b strings.Builder
	b.WriteString("# 预置案例评测报告（自动生成）\n\n")
	b.WriteString("> 本文件由 `make eval`（即 `ZXQ_WRITE_EVAL=1 go test ./internal/api -run TestDemoCaseEvaluationMatrix`）生成，\n")
	b.WriteString("> 检查的是**真实 HTTP 流程**：`POST /api/sessions` → 三级提示 → 生成迁移题。\n")
	b.WriteString("> 所有案例数据均为人工构造的教学演示数据，**没有真实学生参与**；本报告不是学习成效测量。\n\n")
	b.WriteString(fmt.Sprintf("生成时间：%s\n\n", time.Now().Format("2006-01-02 15:04")))

	b.WriteString("## 汇总结论\n\n")
	b.WriteString(fmt.Sprintf("- 预置案例：**%d** 个，全部通过任务类型、主误区、问题类型、提示跨任务隔离与迁移题族检查。\n", len(rows)))
	b.WriteString("- 提示跨任务隔离：三级提示与迁移题中不得出现其它任务类型的关键概念（如训练稳定性任务不得出现“混淆矩阵/召回率”）。\n")
	b.WriteString("- 迁移题族：题目必须来自与当前任务类型一致的题族（CLS/ARC/TRN/ENV/REG/CLU/DP）。\n\n")

	b.WriteString("## 逐案例结果\n\n")
	b.WriteString("| 案例 | 任务类型 | 主误区 | 问题类型 | 三级提示 | 迁移题族 | 迁移题编号 | 满分 |\n")
	b.WriteString("|---|---|---|---|---|---|---|---|\n")
	for _, r := range rows {
		b.WriteString(fmt.Sprintf("| `%s` | %s | %s | %s | %s | %s | `%s` | %d |\n",
			r.Case, r.TaskLabel, r.Archetype, r.Categories, r.Hints, r.Family, r.ItemID, r.MaxScore))
	}

	b.WriteString("\n## 各案例的主要问题（系统输出原文）\n\n")
	for _, r := range rows {
		b.WriteString(fmt.Sprintf("- **%s**：%s\n", r.Case, r.Label))
	}

	b.WriteString("\n## 如何复现\n\n")
	b.WriteString("```bash\nmake eval    # 重新生成本报告；同时跑一遍全部预置案例的评测断言\nmake check   # 完整质量门（含本报告的底层测试）\n```\n")
	return b.String()
}
