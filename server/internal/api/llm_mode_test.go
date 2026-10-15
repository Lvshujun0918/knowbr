package api_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"knowbr/server/internal/api"
	"knowbr/server/internal/services"
	"knowbr/server/internal/store"
)

// 本文件把「大模型可用」这条路径也放到 HTTP 接口层验证（此前只有调用失败的回退路径有端到端测试）：
// 诊断成功时进入 llm 模式、摘要使用模型改写文本；模型编造数值时整段回退并记录原因；
// 同时核对每次请求都带上了当前学习会话编号。

// llmSummaryReply 是摘要请求的模型返回（纯文本，不含数字，满足长度与数值校验）。
const llmSummaryReply = "本次记录显示，学生在整体准确率与逐类指标之间尚未建立联系。" +
	"一级提示后能够指出少数类召回率为零，但迁移题作答仍缺少对错误代价的说明。" +
	"建议教师结合类别分布再讲解一次准确率的适用条件。"

// mockLLMServer 起一个 OpenAI 兼容的假服务端：诊断请求返回 diagJSON，摘要请求返回固定纯文本。
// 它同时记录每次请求携带的会话标识，供测试核对。
func mockLLMServer(t *testing.T, diagJSON string, seenSessions *[]string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*seenSessions = append(*seenSessions, r.Header.Get("x-opencode-session"))
		raw, _ := io.ReadAll(r.Body)
		content := diagJSON
		if strings.Contains(string(raw), "请改写为") {
			content = llmSummaryReply
		}
		reply, _ := json.Marshal(map[string]any{
			"choices": []map[string]any{{"message": map[string]string{"role": "assistant", "content": content}}},
		})
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(reply)
	}))
}

func newLLMHandler(t *testing.T, dbName string, diagJSON string, seen *[]string) http.Handler {
	t.Helper()
	srv := mockLLMServer(t, diagJSON, seen)
	t.Cleanup(srv.Close)
	st, err := store.Open(filepath.Join(t.TempDir(), dbName))
	if err != nil {
		t.Fatalf("打开数据库失败：%v", err)
	}
	llm := services.NewLLMClient(services.LLMConfig{
		Enabled:      true,
		BaseURL:      srv.URL,
		APIKey:       "test-key-not-real",
		Model:        "test-model",
		Timeout:      3 * time.Second,
		UserAgent:    "zhixingqiao-knowbr/1.0",
		ExtraHeaders: map[string]string{"x-opencode-session": "{{session}}"},
	})
	return api.NewServer(st, api.Config{Version: "test"}, llm).Router()
}

// 大模型可用且输出合规时：进入 llm 模式、保留学生原文、摘要采用模型改写文本。
func TestLLMModeEndToEndUsesModelOutput(t *testing.T) {
	var seen []string
	diagJSON := `{"mainIssue":"只用整体准确率 90% 评价模型，忽略了少数类的召回率。",` +
		`"reasoning":"准确率对每个样本等权，1000 个样本中多数类占绝大多数，因此整体指标无法反映少数类是否被识别出来。",` +
		`"nextStep":"请逐类计算召回率，并说明在本次任务目标下少数类是否可用。"}`
	h := newLLMHandler(t, "llmok.db", diagJSON, &seen)

	sub := demoSubmission(t, "case-imbalance")
	rec, created := do(t, h, http.MethodPost, "/api/sessions", sub)
	if rec.Code != http.StatusOK {
		t.Fatalf("大模型可用时创建会话应成功：%d %s", rec.Code, rec.Body.String())
	}
	id := digStr(t, created, "session", "id")
	if got := digStr(t, created, "session", "mode"); got != "llm" {
		t.Errorf("应进入 llm 模式，实际 %s", got)
	}
	if !dig(created, "session", "llmUsed").(bool) {
		t.Error("成功调用后必须标记 llmUsed")
	}
	if reason, _ := dig(created, "session", "llmFallbackReason").(string); reason != "" {
		t.Errorf("成功调用不应记录回退原因，实际 %q", reason)
	}
	if got := digStr(t, created, "session", "diagnosis", "mainIssue"); !strings.Contains(got, "只用整体准确率 90%") {
		t.Errorf("诊断应使用大模型改写后的表达，实际 %s", got)
	}
	if got := digStr(t, created, "session", "diagnosis", "source"); got != "llm" {
		t.Errorf("诊断来源应标注为 llm，实际 %s", got)
	}
	if got := digStr(t, created, "session", "experiment", "studentInitialExplanation"); got != sub.StudentInitialExplanation {
		t.Error("进入 llm 模式时不得改动学生提交的原文")
	}
	if notice := digStr(t, created, "session", "modeNotice"); !strings.Contains(notice, "大模型") {
		t.Errorf("模式说明应说明大模型的实际参与范围：%s", notice)
	}

	// 每次请求都必须携带当前学习会话编号，便于服务端做路由与缓存。
	if len(seen) == 0 {
		t.Fatal("没有观察到任何大模型请求")
	}
	for i, s := range seen {
		if s != id {
			t.Errorf("第 %d 次请求的会话标识应为 %s，实际 %q", i+1, id, s)
		}
	}
	if _, ok := dig(created, "session", "diagnosis", "llmNote").(string); !ok {
		t.Error("应提供 llmNote 说明大模型参与范围")
	}

	// 摘要沿用模型改写文本。
	rec, sum := do(t, h, http.MethodGet, "/api/sessions/"+id+"/summary", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("生成摘要失败：%d %s", rec.Code, rec.Body.String())
	}
	if got := digStr(t, sum, "summary", "teacherSummary"); got != llmSummaryReply {
		t.Errorf("摘要应使用大模型改写文本，实际 %s", got)
	}
}

// 大模型编造未提供的数值时：整段回退规则引擎，记录回退原因，学生内容不丢失。
func TestLLMModeFallsBackWhenModelInventsNumbers(t *testing.T) {
	var seen []string
	invented := `{"mainIssue":"模型准确率高达 7777%，因此可以认为模型效果很好。",` +
		`"reasoning":"整体准确率 7777% 说明模型在绝大多数样本上都预测正确，无需再看逐类指标。",` +
		`"nextStep":"可以继续调参以把准确率提升到 8888%。"}`
	h := newLLMHandler(t, "llmbad.db", invented, &seen)

	sub := demoSubmission(t, "case-imbalance")
	rec, created := do(t, h, http.MethodPost, "/api/sessions", sub)
	if rec.Code != http.StatusOK {
		t.Fatalf("回退时仍应完成诊断：%d %s", rec.Code, rec.Body.String())
	}
	if dig(created, "session", "llmUsed").(bool) {
		t.Error("编造数值时不得标记为已使用大模型")
	}
	if got := digStr(t, created, "session", "mode"); got != "demo" {
		t.Errorf("回退后应回到演示/规则模式，实际 %s", got)
	}
	reason, _ := dig(created, "session", "llmFallbackReason").(string)
	if reason == "" {
		t.Error("回退原因必须被记录，供界面诚实展示")
	}
	if got := digStr(t, created, "session", "diagnosis", "mainIssue"); strings.Contains(got, "7777") {
		t.Errorf("回退后不得保留模型编造的数值：%s", got)
	}
	if got := digStr(t, created, "session", "diagnosis", "primaryArchetype"); got != "E1" {
		t.Errorf("回退后应使用规则引擎结论，实际 %s", got)
	}
	if got := digStr(t, created, "session", "experiment", "studentInitialExplanation"); got != sub.StudentInitialExplanation {
		t.Error("回退时不得丢失学生已提交的内容")
	}
	if notice := digStr(t, created, "session", "modeNotice"); !strings.Contains(notice, "未成功") {
		t.Errorf("模式说明应告知调用未成功并已回退：%s", notice)
	}
}
