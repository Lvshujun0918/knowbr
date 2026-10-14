package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func testLLMClient(t *testing.T, handler http.HandlerFunc) (*LLMClient, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	client := NewLLMClient(LLMConfig{
		Enabled: true,
		BaseURL: srv.URL,
		APIKey:  "test-key-not-real",
		Model:   "test-model",
		Timeout: 3 * time.Second,
	})
	return client, srv
}

func chatBody(content string) string {
	payload := map[string]any{
		"choices": []map[string]any{
			{"message": map[string]string{"role": "assistant", "content": content}},
		},
	}
	raw, _ := json.Marshal(payload)
	return string(raw)
}

func TestLLMAvailableRequiresAPIKey(t *testing.T) {
	off := NewLLMClient(LLMConfig{Enabled: true, BaseURL: "http://example.com", APIKey: ""})
	if off.Available() {
		t.Error("没有 API Key 时不应认为大模型可用")
	}
	disabled := NewLLMClient(LLMConfig{Enabled: false, APIKey: "x"})
	if disabled.Available() {
		t.Error("显式关闭时不应认为大模型可用")
	}
	on := NewLLMClient(LLMConfig{Enabled: true, APIKey: "x"})
	if !on.Available() {
		t.Error("配置完整时应认为大模型可用")
	}
}

func TestEnhanceDiagnosisUsesGroundedOutput(t *testing.T) {
	diag := Diagnose(demoInput(t))
	metrics, err := ComputeMetrics(coreDemoMatrix(), []string{"类别A", "类别B", "类别C"})
	if err != nil {
		t.Fatal(err)
	}
	in := DiagnoseInput{Explanation: "准确率 90%，效果很好。", Metrics: metrics, HaveMatrix: true}

	client, srv := testLLMClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-key-not-real" {
			t.Errorf("密钥应通过请求头传递，实际 %q", got)
		}
		if !strings.HasSuffix(r.URL.Path, "/chat/completions") {
			t.Errorf("请求路径错误：%s", r.URL.Path)
		}
		fmt.Fprint(w, chatBody(`{"mainIssue":"只用整体准确率（90%）评价模型，忽略了类别B、类别C的召回率为0%。","reasoning":"准确率对每个样本等权，被多数类主导。","nextStep":"请逐类计算召回率。"}`))
	})
	defer srv.Close()

	enhanced, err := client.EnhanceDiagnosis(context.Background(), diag, in)
	if err != nil {
		t.Fatalf("应成功增强诊断：%v", err)
	}
	if !enhanced.LLMUsed || enhanced.Source != "llm" {
		t.Error("成功调用后应标记为大模型参与")
	}
	if enhanced.MainIssue == diag.MainIssue {
		t.Error("主要问题应被大模型改写")
	}
	// 诊断类别与证据必须保留规则引擎结果，不允许模型改动。
	if len(enhanced.Categories) != len(diag.Categories) || enhanced.PrimaryArchetype != diag.PrimaryArchetype {
		t.Error("诊断类别与命中的典型误区必须保持规则引擎的结果")
	}
	if len(enhanced.Evidence) != len(diag.Evidence) {
		t.Error("证据必须保持程序计算结果")
	}
}

// 模型编造未提供的数字时，整段回退到规则模板。
func TestEnhanceDiagnosisRejectsInventedNumbers(t *testing.T) {
	diag := Diagnose(demoInput(t))
	metrics, _ := ComputeMetrics(coreDemoMatrix(), []string{"类别A", "类别B", "类别C"})
	in := DiagnoseInput{Explanation: "准确率 90%，效果很好。", Metrics: metrics, HaveMatrix: true}

	client, srv := testLLMClient(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, chatBody(`{"mainIssue":"模型在类别B上的召回率只有 37.5%（编造数据）。","reasoning":"这是一段足够长的理由，用来通过结构校验并触发数值校验。","nextStep":"请重新计算各类别召回率。"}`))
	})
	defer srv.Close()

	if _, err := client.EnhanceDiagnosis(context.Background(), diag, in); err == nil {
		t.Fatal("引入未提供数值的输出应被拒绝")
	} else if !strings.Contains(err.Error(), "未提供的数值") {
		t.Errorf("错误信息应说明原因，实际：%v", err)
	}
}

func TestEnhanceDiagnosisFallsBackOnBadResponses(t *testing.T) {
	diag := Diagnose(demoInput(t))
	cases := []struct {
		name    string
		handler http.HandlerFunc
	}{
		{"接口返回 500", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, `{"error":{"message":"internal"}}`)
		}},
		{"返回非 JSON 文本", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, chatBody("抱歉，我无法回答这个问题。"))
		}},
		{"JSON 结构不完整", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, chatBody(`{"mainIssue":"唯一字段"}`))
		}},
		{"返回空 choices", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"choices":[]}`)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client, srv := testLLMClient(t, tc.handler)
			defer srv.Close()
			enhanced, err := client.EnhanceDiagnosis(context.Background(), diag, DiagnoseInput{Explanation: "x"})
			if err == nil {
				t.Fatal("异常响应应返回错误，以便上层回退到规则模板")
			}
			if enhanced.MainIssue != diag.MainIssue || enhanced.LLMUsed {
				t.Error("回退时必须保留规则引擎的诊断结果")
			}
		})
	}
}

func TestLLMTimeoutIsBounded(t *testing.T) {
	release := make(chan struct{})
	client, srv := testLLMClient(t, func(w http.ResponseWriter, r *http.Request) {
		<-release
	})
	defer srv.Close()
	defer close(release)
	client.cfg.Timeout = 200 * time.Millisecond
	client.httpc.Timeout = 200 * time.Millisecond

	start := time.Now()
	_, err := client.Chat(context.Background(), "system", "user")
	if err == nil {
		t.Fatal("超时应返回错误")
	}
	if elapsed := time.Since(start); elapsed > 1500*time.Millisecond {
		t.Errorf("超时控制失效，耗时 %v", elapsed)
	}
}

func TestNumbersGrounded(t *testing.T) {
	allowed := map[string]bool{"0.9": true, "90": true, "0": true, "1000": true, "0.316": true}
	if !numbersGrounded("准确率 90%，宏平均 F1 0.316。", allowed) {
		t.Error("引用已有数值应通过校验")
	}
	if numbersGrounded("召回率 37.5%。", allowed) {
		t.Error("编造数值应被拒绝")
	}
	if !numbersGrounded("请查看混淆矩阵第 2 行。", allowed) {
		t.Error("短序号（如行号）不应被视为编造指标")
	}
}
