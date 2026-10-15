package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// 额外请求头必须随每次请求发送，且 {{session}} 占位符要替换为本轮会话编号；
// 客户端还要声明自己的 User-Agent，而不是通用的 Go SDK 默认值。
func TestChatSendsExtraHeadersAndUserAgent(t *testing.T) {
	var gotSession, gotCustom, gotUA, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSession = r.Header.Get("x-opencode-session")
		gotCustom = r.Header.Get("X-Course")
		gotUA = r.Header.Get("User-Agent")
		gotAuth = r.Header.Get("Authorization")
		body, _ := json.Marshal(map[string]any{
			"choices": []map[string]any{{"message": map[string]string{"role": "assistant", "content": "好"}}},
		})
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	client := NewLLMClient(LLMConfig{
		Enabled: true,
		BaseURL: srv.URL,
		APIKey:  "test-key-not-real",
		Model:   "test-model",
		Timeout: 3 * time.Second,
		ExtraHeaders: map[string]string{
			"x-opencode-session": sessionPlaceholder,
			"X-Course":           "ai-basics",
		},
	})
	if _, err := client.Chat(context.Background(), "s", "u", "sess-42"); err != nil {
		t.Fatalf("调用失败：%v", err)
	}
	if gotSession != "sess-42" {
		t.Errorf("会话头应替换为学习会话编号，实际 %q", gotSession)
	}
	if gotCustom != "ai-basics" {
		t.Errorf("自定义请求头未发送，实际 %q", gotCustom)
	}
	if gotUA != defaultUserAgent {
		t.Errorf("应声明客户端身份 %q，实际 %q", defaultUserAgent, gotUA)
	}
	if gotAuth != "Bearer test-key-not-real" {
		t.Errorf("缺少认证头，实际 %q", gotAuth)
	}
}

// 未配置会话编号时，含占位符的请求头不应被发送为空值（避免服务端收到空会话标识）。
func TestChatSkipsHeaderWithEmptySession(t *testing.T) {
	var present bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, present = r.Header["X-opencode-Session"]
		body, _ := json.Marshal(map[string]any{
			"choices": []map[string]any{{"message": map[string]string{"role": "assistant", "content": "好"}}},
		})
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	client := NewLLMClient(LLMConfig{
		Enabled: true, BaseURL: srv.URL, APIKey: "k", Model: "m", Timeout: 3 * time.Second,
		ExtraHeaders: map[string]string{"x-opencode-session": sessionPlaceholder},
	})
	if _, err := client.Chat(context.Background(), "s", "u", ""); err != nil {
		t.Fatalf("调用失败：%v", err)
	}
	if present {
		t.Error("会话编号为空时不应发送空会话头")
	}
}

func TestParseExtraHeaders(t *testing.T) {
	got := parseExtraHeaders("x-opencode-session={{session}}; X-Course = ai-basics\nX-Blank=\nno-equals, ")
	if len(got) != 2 {
		t.Fatalf("应只保留两条合法请求头，实际 %v", got)
	}
	if got["x-opencode-session"] != sessionPlaceholder {
		t.Errorf("占位符应原样保留待替换，实际 %q", got["x-opencode-session"])
	}
	if got["X-Course"] != "ai-basics" {
		t.Errorf("应去除名称与取值两侧空白，实际 %q", got["X-Course"])
	}
	if len(parseExtraHeaders("")) != 0 {
		t.Error("空配置应得到空映射")
	}
}

// 环境变量应能完整驱动一次真实形态的配置装载（不含真实密钥）。
func TestLoadLLMConfigReadsExtraHeaders(t *testing.T) {
	t.Setenv("ZXQ_LLM_API_KEY", "k")
	t.Setenv("ZXQ_LLM_BASE_URL", "https://opencode.ai/zen/go/v1/")
	t.Setenv("ZXQ_LLM_MODEL", "deepseek-v4.1-flash")
	t.Setenv("ZXQ_LLM_EXTRA_HEADERS", "x-opencode-session={{session}}")
	t.Setenv("ZXQ_LLM_USER_AGENT", "zhixingqiao-knowbr/1.0")
	cfg := LoadLLMConfig()
	if !cfg.Enabled {
		t.Fatal("配置了密钥时应启用")
	}
	if cfg.BaseURL != "https://opencode.ai/zen/go/v1" {
		t.Errorf("末尾斜杠应被去除，实际 %q", cfg.BaseURL)
	}
	if cfg.Model != "deepseek-v4.1-flash" {
		t.Errorf("模型名未生效：%q", cfg.Model)
	}
	if cfg.ExtraHeaders["x-opencode-session"] != sessionPlaceholder {
		t.Errorf("额外请求头未装载：%v", cfg.ExtraHeaders)
	}
	if cfg.UserAgent != "zhixingqiao-knowbr/1.0" {
		t.Errorf("User-Agent 未生效：%q", cfg.UserAgent)
	}
	if cfg.Timeout != 20*time.Second {
		t.Errorf("未设置超时秒数时应使用默认值，实际 %v", cfg.Timeout)
	}
}
