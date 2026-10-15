package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// LLMConfig 是大模型接入配置。未配置 API Key 时应用自动使用规则模式。
type LLMConfig struct {
	Enabled  bool
	BaseURL  string
	APIKey   string
	Model    string
	Timeout  time.Duration
	// ExtraHeaders 是随每次请求发送的额外请求头。值中的 {{session}} 会被替换为本轮学习会话编号，
	// 用于像 OpenCode Go 这样要求客户端上报稳定会话标识的服务。
	ExtraHeaders map[string]string
	// UserAgent 标识调用方身份；默认值不是通用 SDK，便于服务端区分流量。
	UserAgent string
}

// defaultUserAgent 是本应用对外声明的客户端标识。
const defaultUserAgent = "zhixingqiao-knowbr/1.0"

// sessionPlaceholder 是额外请求头中代表学习会话编号的占位符。
const sessionPlaceholder = "{{session}}"

// LoadLLMConfig 从环境变量读取配置，密钥只保存在服务端。
func LoadLLMConfig() LLMConfig {
	timeout := 20 * time.Second
	if v := strings.TrimSpace(os.Getenv("ZXQ_LLM_TIMEOUT_SECONDS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			timeout = time.Duration(n) * time.Second
		}
	}
	baseURL := strings.TrimRight(firstNonEmpty(os.Getenv("ZXQ_LLM_BASE_URL"), "https://api.openai.com/v1"), "/")
	apiKey := firstNonEmpty(os.Getenv("ZXQ_LLM_API_KEY"), os.Getenv("OPENAI_API_KEY"))
	model := firstNonEmpty(os.Getenv("ZXQ_LLM_MODEL"), "gpt-4o-mini")
	enabled := apiKey != "" && strings.ToLower(strings.TrimSpace(os.Getenv("ZXQ_LLM_DISABLED"))) != "true"
	return LLMConfig{
		Enabled:      enabled,
		BaseURL:      baseURL,
		APIKey:       apiKey,
		Model:        model,
		Timeout:      timeout,
		ExtraHeaders: parseExtraHeaders(os.Getenv("ZXQ_LLM_EXTRA_HEADERS")),
		UserAgent:    firstNonEmpty(os.Getenv("ZXQ_LLM_USER_AGENT"), defaultUserAgent),
	}
}

// parseExtraHeaders 解析形如 `x-opencode-session={{session}};X-Other=1` 的额外请求头配置。
// 分隔符可用分号、逗号或换行；条目缺少名称或取值时直接跳过，不阻断启动。
func parseExtraHeaders(raw string) map[string]string {
	out := map[string]string{}
	for _, part := range strings.FieldsFunc(raw, func(r rune) bool {
		return r == ';' || r == ',' || r == '\n'
	}) {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		i := strings.Index(part, "=")
		if i <= 0 {
			continue
		}
		name := strings.TrimSpace(part[:i])
		value := strings.TrimSpace(part[i+1:])
		if name == "" || value == "" {
			continue
		}
		out[name] = value
	}
	return out
}

// LLMClient 是 OpenAI 兼容 Chat Completions 接口的最小客户端。
type LLMClient struct {
	cfg   LLMConfig
	httpc *http.Client
}

// NewLLMClient 构造客户端。
func NewLLMClient(cfg LLMConfig) *LLMClient {
	return &LLMClient{cfg: cfg, httpc: &http.Client{Timeout: cfg.Timeout}}
}

// Available 表示当前是否真的可以调用大模型。
func (c *LLMClient) Available() bool {
	return c != nil && c.cfg.Enabled && strings.TrimSpace(c.cfg.APIKey) != ""
}

// Model 返回当前模型名称，用于界面标注。
func (c *LLMClient) Model() string { return c.cfg.Model }

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Chat 调用 Chat Completions 接口，任何失败都会返回错误交由上层回退到规则模式。
// sessionID 是本轮学习会话的编号，用于填充额外请求头中的 {{session}} 占位符。
func (c *LLMClient) Chat(ctx context.Context, system, user, sessionID string) (string, error) {
	if !c.Available() {
		return "", errors.New("未配置大模型 API Key，当前为规则模式")
	}
	body, err := json.Marshal(chatRequest{
		Model: c.cfg.Model,
		Messages: []chatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
		Temperature: 0.2,
	})
	if err != nil {
		return "", err
	}
	reqCtx, cancel := context.WithTimeout(ctx, c.cfg.Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, c.cfg.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	ua := c.cfg.UserAgent
	if strings.TrimSpace(ua) == "" {
		ua = defaultUserAgent
	}
	req.Header.Set("User-Agent", ua)
	for name, value := range c.cfg.ExtraHeaders {
		value = strings.ReplaceAll(value, sessionPlaceholder, sessionID)
		if strings.TrimSpace(value) == "" {
			continue
		}
		req.Header.Set(name, value)
	}
	resp, err := c.httpc.Do(req)
	if err != nil {
		return "", fmt.Errorf("调用大模型接口失败：%w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("大模型接口返回状态码 %d：%s", resp.StatusCode, truncate(string(raw), 200))
	}
	var parsed chatResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", fmt.Errorf("大模型返回内容不是有效 JSON：%w", err)
	}
	if parsed.Error != nil {
		return "", errors.New("大模型接口报错：" + parsed.Error.Message)
	}
	if len(parsed.Choices) == 0 {
		return "", errors.New("大模型没有返回任何内容")
	}
	return strings.TrimSpace(parsed.Choices[0].Message.Content), nil
}

// EnhanceDiagnosis 使用大模型润色诊断的自然语言部分。
// 约束：类别、证据、指标数值一律沿用规则引擎结果；若模型引入未提供的数据，则整段回退。
// sessionID 用于填充额外请求头中的 {{session}}，便于服务端做路由与缓存。
func (c *LLMClient) EnhanceDiagnosis(ctx context.Context, diag Diagnosis, in DiagnoseInput, sessionID string) (Diagnosis, error) {
	if !c.Available() {
		return diag, errors.New("未配置大模型 API Key")
	}
	system := "你是《人工智能基础》课程的教学助教，只负责把已有的诊断结论组织成更自然的教师口吻表达。" +
		"严禁引入未提供的数据、严禁修改任何指标数值、严禁给出新的结论。只输出 JSON。"
	var facts strings.Builder
	facts.WriteString("学生解释：" + in.Explanation + "\n")
	facts.WriteString("学生困惑：" + in.Question + "\n")
	if in.Metrics != nil {
		facts.WriteString(fmt.Sprintf("程序计算的整体准确率：%.4f\n", in.Metrics.Accuracy))
		facts.WriteString(fmt.Sprintf("程序计算的宏平均F1：%.4f\n", in.Metrics.MacroF1))
		facts.WriteString("矩阵事实：\n- " + strings.Join(diag.MatrixFacts, "\n- ") + "\n")
		facts.WriteString("召回率为0的类别：" + strings.Join(in.Metrics.ZeroRecallClass, "、") + "\n")
	} else {
		facts.WriteString("未提供混淆矩阵，禁止编造任何类别指标数值。\n")
	}
	facts.WriteString("规则引擎给出的主要问题：" + diag.MainIssue + "\n")
	facts.WriteString("规则引擎给出的诊断理由：" + diag.Reasoning + "\n")
	facts.WriteString("规则引擎给出的下一步建议：" + diag.NextStep + "\n")
	facts.WriteString(`请输出 JSON：{"mainIssue":"一句话主要问题","reasoning":"诊断理由","nextStep":"下一步建议"}`)

	out, err := c.Chat(ctx, system, facts.String(), sessionID)
	if err != nil {
		return diag, err
	}
	var payload struct {
		MainIssue string `json:"mainIssue"`
		Reasoning string `json:"reasoning"`
		NextStep  string `json:"nextStep"`
	}
	if err := unmarshalJSONBlock(out, &payload); err != nil {
		return diag, err
	}
	if len([]rune(payload.MainIssue)) < 6 || len([]rune(payload.Reasoning)) < 10 || len([]rune(payload.NextStep)) < 6 {
		return diag, errors.New("大模型返回内容结构不完整，已回退规则模板")
	}
	allowed := allowedNumbers(diag, in)
	if bad, _ := ungroundedNumber(payload.MainIssue+" "+payload.Reasoning+" "+payload.NextStep, allowed); bad != "" {
		return diag, fmt.Errorf("大模型输出引入了未提供的数值「%s」，已回退规则模板", bad)
	}
	enhanced := diag
	enhanced.MainIssue = payload.MainIssue
	enhanced.Reasoning = payload.Reasoning
	enhanced.NextStep = payload.NextStep
	enhanced.LLMUsed = true
	enhanced.Source = "llm"
	enhanced.LLMNote = "自然语言表达由大模型组织，指标数值与诊断类别仍来自规则引擎与程序计算。"
	return enhanced, nil
}

// EnhanceSummary 使用大模型润色教师摘要（失败时保留规则版本）。
func (c *LLMClient) EnhanceSummary(ctx context.Context, summary Summary, sessionID string) (string, error) {
	if !c.Available() {
		return summary.TeacherSummary, errors.New("未配置大模型 API Key")
	}
	system := "你是课程助教，负责把结构化学习记录改写为面向教师的简要反馈。" +
		"只能使用给定事实，禁止编造成绩、通过率或学生画像。输出纯文本，不要 Markdown 标题。"
	user := "结构化记录：\n" + summary.TeacherSummary + "\n请改写为 4—6 句的教师反馈。"
	out, err := c.Chat(ctx, system, user, sessionID)
	if err != nil {
		return summary.TeacherSummary, err
	}
	out = strings.TrimSpace(out)
	if len([]rune(out)) < 30 {
		return summary.TeacherSummary, errors.New("大模型返回内容过短，保留规则版本")
	}
	if bad, _ := ungroundedNumber(out, allowedNumbersFromSummary(summary)); bad != "" {
		return summary.TeacherSummary, fmt.Errorf("大模型输出引入了未提供的数值「%s」，保留规则版本", bad)
	}
	return out, nil
}

var numberPatternLLM = regexp.MustCompile(`\d+(?:\.\d+)?`)

func allowedNumbers(diag Diagnosis, in DiagnoseInput) map[string]bool {
	set := map[string]bool{}
	add := func(v float64) {
		set[strconv.FormatFloat(v, 'f', -1, 64)] = true
		set[fmt.Sprintf("%.0f", v)] = true
		set[fmt.Sprintf("%.1f", v)] = true
		set[fmt.Sprintf("%.2f", v)] = true
		set[fmt.Sprintf("%.3f", v)] = true
		set[fmt.Sprintf("%.0f", v*100)] = true
		set[fmt.Sprintf("%.1f", v*100)] = true
	}
	if in.Metrics != nil {
		add(in.Metrics.Total)
		add(in.Metrics.Correct)
		add(in.Metrics.Accuracy)
		add(in.Metrics.MacroF1)
		add(in.Metrics.WeightedF1)
		add(in.Metrics.MaxClassRatio)
		for _, c := range in.Metrics.Classes {
			add(c.Support)
			add(c.Recall)
			add(c.Precision)
			add(c.F1)
		}
	}
	// 学生自己提交的所有文本都算“已提供的数值”：模型只是复述它们不算编造。
	for _, s := range []string{
		in.ExperimentTitle, in.ExperimentGoal, in.DatasetDescription, in.ModelName,
		in.CodeSnippet, in.ExecutionLog, in.MetricsText, in.Explanation, in.Question,
		strings.Join(diag.Evidence, " "), diag.MainIssue, diag.Reasoning, diag.NextStep,
	} {
		for _, m := range numberPatternLLM.FindAllString(s, -1) {
			set[m] = true
		}
	}
	return set
}

func allowedNumbersFromSummary(s Summary) map[string]bool {
	set := map[string]bool{}
	for _, part := range []string{s.InitialExplanation, s.MainIssue, s.DiagnosisReasoning, s.TeacherSummary, strings.Join(s.KeyEvidence, " "), strings.Join(s.IssueEvidence, " ")} {
		for _, m := range numberPatternLLM.FindAllString(part, -1) {
			set[m] = true
		}
	}
	return set
}

// numbersGrounded 检查模型输出中的数字是否都出现在允许范围内（防止编造指标）。
func numbersGrounded(text string, allowed map[string]bool) bool {
	_, bad := ungroundedNumber(text, allowed)
	return bad == ""
}

// ungroundedNumber 返回第一个不在允许范围内的数字，供回退原因使用，便于定位模型具体编造了什么。
func ungroundedNumber(text string, allowed map[string]bool) (string, string) {
	for _, m := range numberPatternLLM.FindAllString(text, -1) {
		if allowed[m] {
			continue
		}
		if v, err := strconv.ParseFloat(m, 64); err == nil {
			matched := false
			for k := range allowed {
				if av, err := strconv.ParseFloat(k, 64); err == nil && math.Abs(av-v) < 1e-9 {
					matched = true
					break
				}
			}
			if matched {
				continue
			}
		}
		// 只放行单个数字：它们通常是“第 3 轮”“2 类”这类序数与数量表达，不构成指标。
		// 两位及以上的数值必须能在学生提交的材料或规则引擎文本中找到，
		// 因此“准确率 97%”这类编造的两位数指标会被拦住并回退规则模式。
		if len(m) == 1 {
			continue
		}
		return m, text
	}
	return "", ""
}

func unmarshalJSONBlock(raw string, target any) error {
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start < 0 || end <= start {
		return errors.New("大模型未返回 JSON 结构")
	}
	if err := json.Unmarshal([]byte(raw[start:end+1]), target); err != nil {
		return fmt.Errorf("大模型返回的 JSON 无法解析：%w", err)
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
