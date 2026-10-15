package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"knowbr/server/internal/api"
	"knowbr/server/internal/content"
	"knowbr/server/internal/models"
	"knowbr/server/internal/services"
	"knowbr/server/internal/store"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	m.Run()
}

func newTestHandler(t *testing.T, dbPath string) http.Handler {
	t.Helper()
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("打开数据库失败：%v", err)
	}
	llm := services.NewLLMClient(services.LLMConfig{})
	srv := api.NewServer(st, api.Config{Version: "test"}, llm)
	return srv.Router()
}

func do(t *testing.T, h http.Handler, method, path string, payload any) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	var body *bytes.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("序列化请求失败：%v", err)
		}
		body = bytes.NewReader(raw)
	} else {
		body = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	out := map[string]any{}
	if rec.Body.Len() > 0 {
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("响应不是 JSON（%s）：%s", path, rec.Body.String())
		}
	}
	return rec, out
}

// dig 按路径读取响应字段，缺失时返回 nil。
func dig(m map[string]any, path ...string) any {
	var cur any = m
	for _, key := range path {
		obj, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur, ok = obj[key]
		if !ok {
			return nil
		}
	}
	return cur
}

func digStr(t *testing.T, m map[string]any, path ...string) string {
	t.Helper()
	v, ok := dig(m, path...).(string)
	if !ok {
		t.Fatalf("字段 %s 不是字符串：%v", strings.Join(path, "."), dig(m, path...))
	}
	return v
}

func digNum(t *testing.T, m map[string]any, path ...string) float64 {
	t.Helper()
	v, ok := dig(m, path...).(float64)
	if !ok {
		t.Fatalf("字段 %s 不是数字：%v", strings.Join(path, "."), dig(m, path...))
	}
	return v
}

func digList(t *testing.T, m map[string]any, path ...string) []any {
	t.Helper()
	v, ok := dig(m, path...).([]any)
	if !ok {
		t.Fatalf("字段 %s 不是数组：%v", strings.Join(path, "."), dig(m, path...))
	}
	return v
}

// demoSubmission 把预置演示案例转换为提交结构。
func demoSubmission(t *testing.T, id string) models.Submission {
	t.Helper()
	c, ok := content.DemoCaseByID(id)
	if !ok {
		t.Fatalf("演示案例 %s 不存在", id)
	}
	raw, err := json.Marshal(c.Submission)
	if err != nil {
		t.Fatal(err)
	}
	var sub models.Submission
	if err := json.Unmarshal(raw, &sub); err != nil {
		t.Fatal(err)
	}
	sub.DemoCaseID = id
	return sub
}

// fullTransferAnswer 覆盖分类类迁移题的三个共同考查方向：数据分布与准确率局限、指标选择、错误代价与阈值。
// 具体题目由规则模板按当前任务类型生成，同一任务下的不同情境共享这组考查方向。
const fullTransferAnswer = "正样本只占很小比例，属于明显的类别不平衡，全部判成多数类也能得到很高的准确率，" +
	"所以准确率具有误导性、不能反映模型是否可用。主指标应看召回率，同时用精确率监控反向错误带来的成本，" +
	"必要时结合 PR 曲线而不是只盯一个数字。本任务中漏掉真正的目标样本代价更高，" +
	"所以要按可接受的错误代价调整阈值，并在混淆矩阵中核对两类错误的具体数量。"

// 端到端：演示案例 → 诊断 → 分层辅导 → 迁移测试 → 学习反馈 → 教师视图。
func TestFullDemoFlowThroughHTTP(t *testing.T) {
	h := newTestHandler(t, filepath.Join(t.TempDir(), "flow.db"))

	rec, created := do(t, h, http.MethodPost, "/api/sessions", demoSubmission(t, "case-imbalance"))
	if rec.Code != http.StatusOK {
		t.Fatalf("创建会话失败：%d %s", rec.Code, rec.Body.String())
	}
	id := digStr(t, created, "session", "id")
	if digStr(t, created, "session", "mode") != "demo" {
		t.Errorf("演示案例应标记为演示模式，实际 %s", digStr(t, created, "session", "mode"))
	}
	if digStr(t, created, "session", "dataNotice") == "" {
		t.Error("演示记录必须携带模拟数据声明")
	}
	if digStr(t, created, "session", "diagnosis", "primaryArchetype") != "E1" {
		t.Errorf("核心案例应命中 E1，实际 %v", dig(created, "session", "diagnosis", "matchedArchetypes"))
	}
	if got := digNum(t, created, "session", "metrics", "accuracy"); got != 0.9 {
		t.Errorf("准确率应为 0.9，实际 %v", got)
	}
	classes := digList(t, created, "session", "metrics", "classes")
	second := classes[1].(map[string]any)
	if second["recall"].(float64) != 0 || second["recallUndefined"].(bool) {
		t.Errorf("类别B 召回率应为 0 且不为未定义，实际 %v", second)
	}
	if llmUsed, ok := dig(created, "session", "llmUsed").(bool); !ok || llmUsed {
		t.Error("未配置 API Key 时不应声称使用了大模型")
	}
	if notice := digStr(t, created, "session", "modeNotice"); !strings.Contains(notice, "未配置大模型") {
		t.Errorf("规则模式必须说明未调用大模型的真实原因：%s", notice)
	}

	// 分层辅导：一级提示不泄露完整答案，重复请求不重复记录。
	rec, hint1 := do(t, h, http.MethodPost, "/api/sessions/"+id+"/hints", map[string]any{"level": 1, "requestedByStudent": true})
	if rec.Code != http.StatusOK {
		t.Fatalf("生成一级提示失败：%d %s", rec.Code, rec.Body.String())
	}
	if !dig(hint1, "hint", "socratic").(bool) {
		t.Error("概念类问题的一级提示应标记为苏格拉底式提问")
	}
	if !strings.Contains(digStr(t, hint1, "hint", "content"), "类别B") {
		t.Errorf("提示应引用本次矩阵证据：%s", digStr(t, hint1, "hint", "content"))
	}
	rec, again := do(t, h, http.MethodPost, "/api/sessions/"+id+"/hints", map[string]any{"level": 1})
	if rec.Code != http.StatusOK || !dig(again, "alreadyProvided").(bool) {
		t.Errorf("重复请求同一等级应返回已存在的提示：%d %v", rec.Code, again)
	}
	if len(digList(t, again, "hints")) != 1 {
		t.Error("重复请求不应产生新的提示记录")
	}
	// 三级提示给出局部计算示例。
	_, hint3 := do(t, h, http.MethodPost, "/api/sessions/"+id+"/hints", map[string]any{"level": 3, "requestedByStudent": true})
	if !strings.Contains(digStr(t, hint3, "hint", "content"), "0 / 90") {
		t.Errorf("三级提示应包含可核查的局部计算：%s", digStr(t, hint3, "hint", "content"))
	}
	if got := digNum(t, hint3, "hintState", "nextLevel"); got != 3 {
		t.Errorf("推进到第 3 级后 nextLevel 应为 3，实际 %v", got)
	}

	// 学生修正解释。
	rec, rev := do(t, h, http.MethodPost, "/api/sessions/"+id+"/revisions", map[string]any{
		"content": "类别B 的召回率是 0%，因为它的 90 个样本全部被预测成 A；类别分布不平衡，只看准确率会误导。",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("提交修正失败：%d %s", rec.Code, rec.Body.String())
	}
	if digStr(t, rev, "revision", "progress") != "improved" {
		t.Errorf("应判定为 improved，实际 %v（%s）", dig(rev, "revision", "progress"), digStr(t, rev, "revision", "feedback"))
	}
	if digStr(t, rev, "revision", "feedback") == "" {
		t.Error("修正后必须给出反馈")
	}

	// 迁移测试：作答前不得出现参考答案。
	rec, tq := do(t, h, http.MethodPost, "/api/sessions/"+id+"/transfer/question", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("生成迁移题失败：%d %s", rec.Code, rec.Body.String())
	}
	// 迁移题必须限定在当前实验的任务类型内，并说明来源、选择理由与去重检查结果。
	if got := digStr(t, tq, "transfer", "itemId"); got == "T1" {
		t.Error("不得把演示与提示中已经出现过的疾病筛查示例原样当作迁移题")
	}
	if got := digStr(t, tq, "transfer", "taskType"); got != "classification_metrics" {
		t.Errorf("迁移题必须限定在本次实验的任务类型内，实际 %q", got)
	}
	if got := digStr(t, tq, "transfer", "source"); got == "" {
		t.Error("必须标注题目来源类型（规则模板 / 固定题库）")
	}
	if got := digStr(t, tq, "transfer", "generationNote"); !strings.Contains(got, "规则模板") {
		t.Errorf("必须说明题目由规则模板组合生成，而不是模型动态生成：%s", got)
	}
	if got := digStr(t, tq, "transfer", "selectionReason"); !strings.Contains(got, "任务类型") {
		t.Errorf("必须说明选这道题的依据：%s", got)
	}
	if got := digStr(t, tq, "transfer", "dedupNote"); !strings.Contains(got, "相似度") {
		t.Errorf("必须给出与示例情境/历史题目的重复检查说明：%s", got)
	}
	if got := digNum(t, tq, "transfer", "similarityMax"); got >= 0.45 {
		t.Errorf("与示例情境的相似度 %.2f 达到了重复阈值，不应被选为迁移题", got)
	}
	if len(digList(t, tq, "transfer", "rubric")) != 3 {
		t.Error("迁移题应展示 3 个评分维度")
	}
	if s, _ := dig(tq, "transfer", "referenceAnswer").(string); s != "" {
		t.Error("作答前不得返回参考答案")
	}
	if digStr(t, tq, "transfer", "disclaimer") == "" {
		t.Error("必须展示评分口径声明")
	}
	rec, _ = do(t, h, http.MethodPost, "/api/sessions/"+id+"/transfer/reference", nil)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("作答前查看参考答案应被拒绝，实际 %d", rec.Code)
	}
	rec, scored := do(t, h, http.MethodPost, "/api/sessions/"+id+"/transfer/answer", map[string]any{"answer": fullTransferAnswer})
	if rec.Code != http.StatusOK {
		t.Fatalf("提交答案失败：%d %s", rec.Code, rec.Body.String())
	}
	if got := digNum(t, scored, "transfer", "score"); got != 6 {
		t.Errorf("覆盖全部要点的回答应得 6 分，实际 %v", got)
	}
	if s, _ := dig(scored, "transfer", "referenceAnswer").(string); s != "" {
		t.Error("提交答案的响应里仍不应包含参考答案")
	}
	for _, f := range digList(t, scored, "transfer", "rubricFeedback") {
		fb := f.(map[string]any)
		if fb["reason"] == "" {
			t.Errorf("评分维度必须给出理由：%v", fb)
		}
	}
	rec, revealed := do(t, h, http.MethodPost, "/api/sessions/"+id+"/transfer/reference", nil)
	if rec.Code != http.StatusOK || digStr(t, revealed, "transfer", "referenceAnswer") == "" {
		t.Errorf("作答后应能查看参考答案：%d", rec.Code)
	}

	// 完成学习并生成反馈。
	rec, done := do(t, h, http.MethodPost, "/api/sessions/"+id+"/transfer/complete", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("完成学习失败：%d %s", rec.Code, rec.Body.String())
	}
	if digStr(t, done, "session", "status") != "completed" {
		t.Errorf("完成后状态应为 completed，实际 %s", digStr(t, done, "session", "status"))
	}
	summary := dig(done, "summary").(map[string]any)
	if summary["mainIssue"] == "" || summary["teacherSummary"] == "" {
		t.Error("学习摘要必须包含主要误区与教师反馈")
	}
	if len(summary["teacherNotes"].([]any)) == 0 {
		t.Error("学习摘要应包含建议教师关注的问题")
	}
	if len(summary["hintLevels"].([]any)) == 0 {
		t.Error("学习摘要应记录已使用的提示等级")
	}
	if md := digStr(t, done, "markdown"); !strings.Contains(md, "迁移测试") || !strings.Contains(md, "掌握情况") {
		t.Errorf("Markdown 导出内容不完整：%s", md)
	}

	// 记录与教师视图。
	_, list := do(t, h, http.MethodGet, "/api/sessions", nil)
	records := digList(t, list, "records")
	if len(records) != 1 {
		t.Fatalf("记录数量应为 1，实际 %d", len(records))
	}
	row := records[0].(map[string]any)
	if row["statusLabel"] != "已完成学习" || row["transferScore"].(float64) != 6 {
		t.Errorf("记录行字段错误：%v", row)
	}
	_, teacher := do(t, h, http.MethodGet, "/api/teacher/overview", nil)
	if digNum(t, teacher, "counts", "total") != 1 {
		t.Errorf("教师视图统计错误：%v", dig(teacher, "counts"))
	}
	if len(digList(t, teacher, "commonIssues")) == 0 {
		t.Error("教师视图应给出共性问题（基于真实记录）")
	}
	if !strings.Contains(digStr(t, teacher, "note"), "不是真实班级") {
		t.Errorf("教师视图必须声明统计口径：%s", digStr(t, teacher, "note"))
	}

	// 导出接口（返回文本，不能用 JSON 助手解析）。
	exportReq := httptest.NewRequest(http.MethodGet, "/api/sessions/"+id+"/export?format=md", nil)
	exportRec := httptest.NewRecorder()
	h.ServeHTTP(exportRec, exportReq)
	if exportRec.Code != http.StatusOK || !strings.Contains(exportRec.Header().Get("Content-Type"), "text/markdown") {
		t.Errorf("Markdown 导出异常：%d %s", exportRec.Code, exportRec.Header().Get("Content-Type"))
	}
	if !strings.Contains(exportRec.Body.String(), "知行桥") {
		t.Error("导出内容应包含产品标题")
	}
	// TXT 导出应带下载头并携带教师摘要与免责声明。
	txtReq := httptest.NewRequest(http.MethodGet, "/api/sessions/"+id+"/export?format=txt&download=1", nil)
	txtRec := httptest.NewRecorder()
	h.ServeHTTP(txtRec, txtReq)
	if txtRec.Code != http.StatusOK || !strings.Contains(txtRec.Header().Get("Content-Type"), "text/plain") {
		t.Errorf("TXT 导出异常：%d %s", txtRec.Code, txtRec.Header().Get("Content-Type"))
	}
	if !strings.Contains(txtRec.Header().Get("Content-Disposition"), "attachment") {
		t.Errorf("download=1 应触发附件下载：%s", txtRec.Header().Get("Content-Disposition"))
	}
	if !strings.Contains(txtRec.Body.String(), "规则化反馈") {
		t.Error("TXT 导出应包含评分口径声明")
	}

	// 删除记录。
	rec, _ = do(t, h, http.MethodDelete, "/api/sessions/"+id, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("删除失败：%d", rec.Code)
	}
	rec, _ = do(t, h, http.MethodGet, "/api/sessions/"+id, nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("删除后应返回 404，实际 %d", rec.Code)
	}
}

// 每个预置案例都必须能给出可用诊断与迁移题（回归：防止预置案例被误判为“证据不足”）。
func TestAllDemoCasesProduceUsableDiagnosis(t *testing.T) {
	h := newTestHandler(t, filepath.Join(t.TempDir(), "democases.db"))
	wantArchetype := map[string]string{
		"case-imbalance":       "E1",
		"case-misread-matrix":  "E2",
		"case-run-only":        "E5",
		"case-env-error":       "E0",
		"case-accuracy-only":   "NONE",
		"case-mlp-cnn":         "E8",
		"case-rnn-instability": "E6",
		"case-regression":      "E11",
		"case-clustering":      "E14",
	}
	for _, c := range content.DemoCases {
		t.Run(c.ID, func(t *testing.T) {
			rec, body := do(t, h, http.MethodPost, "/api/sessions", demoSubmission(t, c.ID))
			if rec.Code != http.StatusOK {
				t.Fatalf("预置案例应可直接提交：%d %s", rec.Code, rec.Body.String())
			}
			if got := digStr(t, body, "session", "diagnosis", "primaryArchetype"); got != wantArchetype[c.ID] {
				t.Errorf("主误区应为 %s，实际 %s", wantArchetype[c.ID], got)
			}
			for _, path := range [][]string{
				{"session", "diagnosis", "mainIssue"},
				{"session", "diagnosis", "reasoning"},
				{"session", "diagnosis", "nextStep"},
				{"session", "diagnosis", "masteryLabel"},
			} {
				if digStr(t, body, path...) == "" {
					t.Errorf("预置案例的 %s 不能为空", strings.Join(path, "."))
				}
			}
			if len(digList(t, body, "session", "diagnosis", "categories")) == 0 {
				t.Error("诊断必须至少给出一个问题类型")
			}
			if len(digList(t, body, "session", "diagnosis", "evidence")) == 0 {
				t.Error("诊断必须至少给出一条证据")
			}
			id := digStr(t, body, "session", "id")
			rec, tq := do(t, h, http.MethodPost, "/api/sessions/"+id+"/transfer/question", nil)
			if rec.Code != http.StatusOK || digStr(t, tq, "transfer", "itemId") == "" {
				t.Errorf("预置案例应能生成迁移题：%d", rec.Code)
			}
			// 清理，避免影响其它断言
			do(t, h, http.MethodDelete, "/api/sessions/"+id, nil)
		})
	}
}

// 测试 6：异常输入必须返回可理解的中文提示，且不产生记录、不崩溃。
func TestInvalidInputHandling(t *testing.T) {
	h := newTestHandler(t, filepath.Join(t.TempDir(), "invalid.db"))

	cases := []struct {
		name     string
		payload  models.Submission
		wantCode string
	}{
		{
			name:     "缺少学生解释",
			payload:  models.Submission{ExperimentTitle: "分类模型评价实验"},
			wantCode: "EXPLANATION_REQUIRED",
		},
		{
			name: "矩阵含负数",
			payload: models.Submission{
				StudentInitialExplanation: "模型看起来不错。",
				ConfusionMatrix:           [][]float64{{3, -1}, {0, 2}},
			},
			wantCode: "MATRIX_NEGATIVE",
		},
		{
			name: "矩阵行列不一致",
			payload: models.Submission{
				StudentInitialExplanation: "模型看起来不错。",
				ConfusionMatrix:           [][]float64{{3, 1}, {0, 2, 1}},
			},
			wantCode: "MATRIX_NOT_SQUARE",
		},
		{
			name: "粘贴矩阵含非数字",
			payload: models.Submission{
				StudentInitialExplanation: "模型看起来不错。",
				ConfusionMatrixText:       "3 1\nx 2",
			},
			wantCode: "MATRIX_PARSE_ERROR",
		},
		{
			name: "矩阵全为 0",
			payload: models.Submission{
				StudentInitialExplanation: "模型看起来不错。",
				ConfusionMatrix:           [][]float64{{0, 0}, {0, 0}},
			},
			wantCode: "MATRIX_ALL_ZERO",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec, body := do(t, h, http.MethodPost, "/api/sessions", tc.payload)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("应返回 400，实际 %d：%s", rec.Code, rec.Body.String())
			}
			if digStr(t, body, "error", "code") != tc.wantCode {
				t.Errorf("错误码应为 %s，实际 %s（%s）", tc.wantCode, digStr(t, body, "error", "code"), digStr(t, body, "error", "message"))
			}
			if digStr(t, body, "error", "message") == "" {
				t.Error("错误响应必须包含面向用户的中文提示")
			}
		})
	}

	_, list := do(t, h, http.MethodGet, "/api/sessions", nil)
	if len(digList(t, list, "records")) != 0 {
		t.Error("非法提交不应产生学习记录")
	}

	// 只有准确率：允许提交，但诊断必须是“证据不足”，不得武断判断。
	rec, created := do(t, h, http.MethodPost, "/api/sessions", models.Submission{
		ExperimentTitle:           "分类模型准确率实验",
		StudentInitialExplanation: "模型准确率为 90%，这个效果怎么样？",
		MetricsText:               "准确率 90%",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("应允许提交并给出诊断：%d %s", rec.Code, rec.Body.String())
	}
	id := digStr(t, created, "session", "id")
	if digStr(t, created, "session", "diagnosis", "primaryCategory") != "insufficient" {
		t.Errorf("应诊断为证据不足，实际 %s", digStr(t, created, "session", "diagnosis", "primaryCategory"))
	}
	if sufficient, ok := dig(created, "session", "diagnosis", "completeness", "sufficient").(bool); !ok || sufficient {
		t.Error("材料不完整时 sufficient 应为 false")
	}
	if got := dig(created, "session", "metrics"); got != nil {
		t.Errorf("未提供矩阵时不应返回指标：%v", got)
	}
	// 回归：列表字段必须始终是数组，不能是 null（前端会直接渲染/遍历）。
	for _, path := range [][]string{
		{"session", "diagnosis", "matchedArchetypes"},
		{"session", "diagnosis", "evidence"},
		{"session", "diagnosis", "evidenceSource"},
		{"session", "diagnosis", "matrixFacts"},
		{"session", "diagnosis", "categories"},
		{"session", "diagnosis", "completeness", "missing"},
		{"session", "diagnosis", "completeness", "limitations"},
		{"session", "hints"},
		{"session", "revisions"},
		{"session", "hintState", "usedLevels"},
		{"session", "categories"},
	} {
		if _, ok := dig(created, path...).([]any); !ok {
			t.Errorf("字段 %s 必须是数组，实际 %v", strings.Join(path, "."), dig(created, path...))
		}
	}

	// 生成迁移题后，列表字段同样不能为 null。
	rec, tq := do(t, h, http.MethodPost, "/api/sessions/"+id+"/transfer/question", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("生成迁移题失败：%d", rec.Code)
	}
	for _, path := range [][]string{
		{"transfer", "rubric"},
		{"transfer", "rubricFeedback"},
		{"transfer", "matchedKeywords"},
		{"transfer", "missingPoints"},
	} {
		if _, ok := dig(tq, path...).([]any); !ok {
			t.Errorf("字段 %s 必须是数组，实际 %v", strings.Join(path, "."), dig(tq, path...))
		}
	}

	_, summary := do(t, h, http.MethodGet, "/api/sessions/"+id+"/summary", nil)
	for _, path := range [][]string{
		{"summary", "keyEvidence"},
		{"summary", "hintTrail"},
		{"summary", "hintLevels"},
		{"summary", "studentNextSteps"},
		{"summary", "teacherNotes"},
	} {
		if _, ok := dig(summary, path...).([]any); !ok {
			t.Errorf("字段 %s 必须是数组，实际 %v", strings.Join(path, "."), dig(summary, path...))
		}
	}

	// 空修正内容与空答案应被拒绝。
	rec, _ = do(t, h, http.MethodPost, "/api/sessions/"+id+"/revisions", map[string]any{"content": "   "})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("空修正内容应返回 400，实际 %d", rec.Code)
	}
	rec, _ = do(t, h, http.MethodPost, "/api/sessions/"+id+"/transfer/answer", map[string]any{"answer": ""})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("空答案应返回 400，实际 %d", rec.Code)
	}
	// 非法提示等级。
	rec, _ = do(t, h, http.MethodPost, "/api/sessions/"+id+"/hints", map[string]any{"level": 9})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("非法提示等级应返回 400，实际 %d", rec.Code)
	}
	// 不存在的会话。
	rec, _ = do(t, h, http.MethodGet, "/api/sessions/not-exist", nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("不存在的会话应返回 404，实际 %d", rec.Code)
	}
}

// 测试 7：记录写入后重新打开数据库仍然存在（服务端持久化）。
func TestRecordsPersistAcrossReopen(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "persist.db")
	h := newTestHandler(t, dbPath)

	_, created := do(t, h, http.MethodPost, "/api/sessions", demoSubmission(t, "case-misread-matrix"))
	id := digStr(t, created, "session", "id")
	if !dig(created, "session", "diagnosis", "contradiction", "detected").(bool) {
		t.Error("误读矩阵的案例应识别出陈述与证据矛盾")
	}
	if !dig(created, "session", "diagnosis", "teacherIntervention").(bool) {
		t.Error("矛盾场景应建议教师介入")
	}

	// 重新打开同一数据库文件（模拟重启）。
	h2 := newTestHandler(t, dbPath)
	rec, got := do(t, h2, http.MethodGet, "/api/sessions/"+id, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("重启后应仍能读到记录：%d", rec.Code)
	}
	if digStr(t, got, "session", "mainIssue") == "" {
		t.Error("重启后记录内容不完整")
	}

	// 清空后不再存在。
	rec, cleared := do(t, h2, http.MethodDelete, "/api/sessions", nil)
	if rec.Code != http.StatusOK || digNum(t, cleared, "deleted") != 1 {
		t.Fatalf("清空记录失败：%d %v", rec.Code, cleared)
	}
	_, list := do(t, h2, http.MethodGet, "/api/sessions", nil)
	if len(digList(t, list, "records")) != 0 {
		t.Error("清空后不应还有记录")
	}
}

// 需求文档 9.1：大模型调用失败时必须回退到规则模式、不丢失已填内容，且不得声称调用成功。
func TestLLMFailureFallsBackAndKeepsSubmission(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "llmfail.db")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("打开数据库失败：%v", err)
	}
	// 指向一个不可达的地址，强制调用失败。
	llm := services.NewLLMClient(services.LLMConfig{
		Enabled: true,
		APIKey:  "fake-key-not-real",
		BaseURL: "http://127.0.0.1:1",
		Model:   "test-model",
		Timeout: 2 * time.Second,
	})
	h := api.NewServer(st, api.Config{Version: "test"}, llm).Router()

	sub := demoSubmission(t, "case-imbalance")
	rec, created := do(t, h, http.MethodPost, "/api/sessions", sub)
	if rec.Code != http.StatusOK {
		t.Fatalf("大模型失败时仍应成功完成诊断：%d %s", rec.Code, rec.Body.String())
	}
	sess := dig(created, "session").(map[string]any)
	if sess["llmUsed"].(bool) {
		t.Error("调用失败时不得标记为已使用大模型")
	}
	if reason, _ := sess["llmFallbackReason"].(string); reason == "" {
		t.Error("回退原因必须被记录，供界面诚实展示")
	}
	if notice, _ := sess["modeNotice"].(string); !strings.Contains(notice, "未成功") {
		t.Errorf("模式说明应告知调用失败并已回退：%s", notice)
	}
	if digStr(t, created, "session", "experiment", "studentInitialExplanation") != sub.StudentInitialExplanation {
		t.Error("回退时不得丢失学生已提交的内容")
	}
	if digStr(t, created, "session", "diagnosis", "primaryArchetype") != "E1" {
		t.Error("回退后应仍然给出规则引擎的诊断结论")
	}
	// 后续流程仍然可用。
	rec, hint := do(t, h, http.MethodPost, "/api/sessions/"+digStr(t, created, "session", "id")+"/hints", map[string]any{"level": 1})
	if rec.Code != http.StatusOK || digStr(t, hint, "hint", "content") == "" {
		t.Errorf("回退后提示功能应仍可用：%d", rec.Code)
	}
}

// 元信息接口应暴露运行模式、规则库与诚实声明。
func TestMetaAndDemoCasesEndpoints(t *testing.T) {
	h := newTestHandler(t, filepath.Join(t.TempDir(), "meta.db"))

	_, meta := do(t, h, http.MethodGet, "/api/meta", nil)
	if digStr(t, meta, "productName") == "" || digStr(t, meta, "tagline") == "" {
		t.Error("元信息缺少产品名称或一句话介绍")
	}
	if digStr(t, meta, "mode", "code") != "rule" {
		t.Errorf("未配置 API Key 时应为规则模式，实际 %s", digStr(t, meta, "mode", "code"))
	}
	if len(digList(t, meta, "archetypes")) < 6 {
		t.Error("规则库应至少包含 E0—E5 六个典型误区")
	}
	if len(digList(t, meta, "honesty")) == 0 {
		t.Error("应提供诚实与安全声明")
	}
	if len(digList(t, meta, "steps")) != 5 {
		t.Error("应展示五个流程步骤")
	}

	_, cases := do(t, h, http.MethodGet, "/api/demo-cases", nil)
	list := digList(t, cases, "cases")
	if len(list) < 3 {
		t.Fatalf("预置演示案例过少：%d", len(list))
	}
	for _, c := range list {
		item := c.(map[string]any)
		if item["dataNotice"] == "" {
			t.Errorf("案例 %v 必须标注为模拟数据", item["id"])
		}
		sub := item["submission"].(map[string]any)
		if sub["studentInitialExplanation"] == "" {
			t.Errorf("案例 %v 缺少学生解释，无法直接演示", item["id"])
		}
	}

	_, health := do(t, h, http.MethodGet, "/api/health", nil)
	if dig(health, "ok") != true || dig(health, "llmEnabled") != false {
		t.Errorf("健康检查异常：%v", health)
	}

	// 指标计算接口：正确返回与校验错误。
	_, computed := do(t, h, http.MethodPost, "/api/metrics/compute", map[string]any{
		"confusionMatrixText": "900 0 0\n90 0 0\n10 0 0",
		"classLabels":         []string{"类别A", "类别B", "类别C"},
	})
	if digNum(t, computed, "metrics", "accuracy") != 0.9 {
		t.Errorf("指标计算接口结果错误：%v", dig(computed, "metrics", "accuracy"))
	}
	rec, bad := do(t, h, http.MethodPost, "/api/metrics/compute", map[string]any{"confusionMatrixText": "a b\nc d"})
	if rec.Code != http.StatusBadRequest || digStr(t, bad, "error", "code") != "MATRIX_PARSE_ERROR" {
		t.Errorf("非法矩阵应返回校验错误：%d %v", rec.Code, bad)
	}
}
