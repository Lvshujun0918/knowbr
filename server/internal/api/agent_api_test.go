package api_test

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

// 接口层：智能体先要证据、再自动给出提示，并把决策依据留在轨迹里。
func TestAgentStepAsksEvidenceThenActOnItsOwn(t *testing.T) {
	h := newTestHandler(t, filepath.Join(t.TempDir(), "agent.db"))

	// 只提交准确率：必需证据缺失，智能体应拒绝直接给提示。
	recThin, thinBody := do(t, h, http.MethodPost, "/api/sessions", map[string]any{
		"experimentTitle":           "分类实验",
		"studentInitialExplanation": "我的模型准确率是 90%，效果怎么样？",
	})
	if recThin.Code != http.StatusOK {
		t.Fatalf("创建会话失败：%d", recThin.Code)
	}
	thinID := digStr(t, thinBody, "session", "id")

	rec, got := do(t, h, http.MethodGet, "/api/sessions/"+thinID+"/agent", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("读取智能体决策失败：%d", rec.Code)
	}
	if action := digStr(t, got, "agent", "action"); action != "ask_evidence" {
		t.Fatalf("证据不足时应先要材料，实际 %s", action)
	}
	if !dig(got, "agent", "blocked").(bool) {
		t.Error("应标记为阻塞")
	}
	if len(digList(t, got, "agent", "requires")) == 0 {
		t.Error("应列出需要补充的材料")
	}

	rec, stepped := do(t, h, http.MethodPost, "/api/sessions/"+thinID+"/agent/step", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("智能体步失败：%d %s", rec.Code, rec.Body.String())
	}
	if dig(stepped, "executed").(bool) {
		t.Error("证据不足时不应执行任何动作")
	}
	if len(digList(t, stepped, "session", "hints")) != 0 {
		t.Error("证据不足时不应产生提示")
	}

	// 材料完整：智能体应自行给出第一级提示。
	_, fullBody := do(t, h, http.MethodPost, "/api/sessions", demoSubmission(t, "case-imbalance"))
	fullID := digStr(t, fullBody, "session", "id")
	rec, acted := do(t, h, http.MethodPost, "/api/sessions/"+fullID+"/agent/step", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("智能体步失败：%d %s", rec.Code, rec.Body.String())
	}
	if !dig(acted, "executed").(bool) {
		t.Fatalf("材料完整时应自动执行提示动作：%v", dig(acted, "executedNote"))
	}
	if lvl := digNum(t, acted, "hint", "level"); lvl != 1 {
		t.Errorf("首次提示应为一级，实际 %v", lvl)
	}
	if digStr(t, acted, "session", "status") != "tutoring" {
		t.Error("给出提示后状态应进入分层辅导")
	}
	if note := digStr(t, acted, "executedNote"); !strings.Contains(note, "非学生点击") {
		t.Errorf("应说明这是系统自主决定：%s", note)
	}

	// 轨迹里应能读到刚才那次提示。
	_, after := do(t, h, http.MethodGet, "/api/sessions/"+fullID+"/agent", nil)
	tools := digList(t, after, "agent", "tools")
	found := false
	for _, tool := range tools {
		m := tool.(map[string]any)
		if m["name"] == "progress_check" && strings.Contains(m["result"].(string), "已提供提示 1 条") {
			found = true
		}
	}
	if !found {
		t.Errorf("工具轨迹应反映已提供的提示：%v", tools)
	}
}

// 接口层：连续无进展时，智能体自己把会话标记为需要教师介入。
func TestAgentEscalatesThroughAPI(t *testing.T) {
	h := newTestHandler(t, filepath.Join(t.TempDir(), "agent-esc.db"))
	_, createdBody := do(t, h, http.MethodPost, "/api/sessions", demoSubmission(t, "case-imbalance"))
	id := digStr(t, createdBody, "session", "id")

	for i := 0; i < 2; i++ {
		rec, _ := do(t, h, http.MethodPost, "/api/sessions/"+id+"/revisions", map[string]any{"content": "我觉得模型没问题。"})
		if rec.Code != http.StatusOK {
			t.Fatalf("提交修正失败：%d", rec.Code)
		}
	}
	rec, stepped := do(t, h, http.MethodPost, "/api/sessions/"+id+"/agent/step", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("智能体步失败：%d %s", rec.Code, rec.Body.String())
	}
	if action := digStr(t, stepped, "agent", "action"); action != "escalate" {
		t.Fatalf("连续无进展时应建议教师介入，实际 %s", action)
	}
	if !dig(stepped, "session", "hintState", "teacherIntervention").(bool) {
		t.Error("应为会话写入教师介入标记")
	}
	if note := digStr(t, stepped, "executedNote"); !strings.Contains(note, "教师介入") {
		t.Errorf("应说明教师介入的处理结果：%s", note)
	}
	// 重复执行不应重复记录：第二跳必须明确说明“不重复”。
	_, again := do(t, h, http.MethodPost, "/api/sessions/"+id+"/agent/step", nil)
	if dig(again, "executed").(bool) {
		t.Error("已标记过的介入不应被重复执行")
	}
	if note := digStr(t, again, "executedNote"); !strings.Contains(note, "不重复") {
		t.Errorf("应说明为何不再重复记录：%s", note)
	}
}

// 接口层：跨会话记忆——同一误区在本机记录中重复出现时应被识别出来。
func TestAgentMemorySpansSavedRecords(t *testing.T) {
	h := newTestHandler(t, filepath.Join(t.TempDir(), "agent-mem.db"))
	for i := 0; i < 2; i++ {
		if rec, _ := do(t, h, http.MethodPost, "/api/sessions", demoSubmission(t, "case-imbalance")); rec.Code != http.StatusOK {
			t.Fatalf("第 %d 次创建会话失败：%d", i+1, rec.Code)
		}
	}
	_, memBody := do(t, h, http.MethodPost, "/api/sessions", demoSubmission(t, "case-imbalance"))
	id := digStr(t, memBody, "session", "id")

	_, got := do(t, h, http.MethodGet, "/api/sessions/"+id+"/agent", nil)
	if lookedAt := digNum(t, got, "agent", "memory", "lookedAt"); lookedAt < 2 {
		t.Errorf("应读取到 2 条历史记录，实际 %v", lookedAt)
	}
	repeated := digList(t, got, "agent", "memory", "repeated")
	found := false
	for _, r := range repeated {
		m := r.(map[string]any)
		if m["code"] == "E1" && m["count"].(float64) >= 2 {
			found = true
		}
	}
	if !found {
		t.Errorf("应识别出重复出现的误区 E1：%v", repeated)
	}
	if note := digStr(t, got, "agent", "memory", "note"); !strings.Contains(note, "无法确认") {
		t.Errorf("记忆来源的局限必须如实说明：%s", note)
	}
}
