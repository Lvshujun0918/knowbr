package api

import (
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"knowbr/server/internal/content"
	"knowbr/server/internal/models"
	"knowbr/server/internal/services"
)

const defaultExperimentTitle = "分类模型评价实验"

// newSession 依据提交材料完成完整性检查、诊断与持久化（需求文档流程 B→C→D）。
func (s *Server) newSession(sub models.Submission) (*models.Session, error) {
	if strings.TrimSpace(sub.ExperimentTitle) == "" {
		sub.ExperimentTitle = defaultExperimentTitle
	}
	if strings.TrimSpace(sub.StudentInitialExplanation) == "" {
		return nil, services.NewValidationError("EXPLANATION_REQUIRED", "studentInitialExplanation",
			"请先填写“学生自己的实验解释”（必填）。系统需要基于你自己的判断来诊断，而不是替你判断。", "")
	}
	if sub.DemoCaseID != "" {
		if demo, ok := content.DemoCaseByID(sub.DemoCaseID); ok {
			fillFromDemo(&sub, demo)
		}
	}

	_, labels, metrics, err := analyzeMatrix(sub)
	if err != nil {
		return nil, err
	}

	input := services.DiagnoseInput{
		Explanation:        sub.StudentInitialExplanation,
		Question:           sub.Question,
		DatasetDescription: sub.DatasetDescription,
		ExperimentGoal:     sub.ExperimentGoal,
		MetricsText:        sub.MetricsText,
		ExecutionLog:       sub.ExecutionLog,
		CodeSnippet:        sub.CodeSnippet,
		HaveMatrix:         metrics != nil,
		Labels:             labels,
		Metrics:            metrics,
		DemoCaseID:         sub.DemoCaseID,
	}

	diag := services.Diagnose(input)
	mode := ModeRule
	llmUsed := false
	fallback := ""
	useLLM := sub.UseLLM == nil || *sub.UseLLM
	if useLLM && s.llm.Available() {
		enhanced, err := s.llm.EnhanceDiagnosis(ctxOrBackground(), diag, input)
		if err != nil {
			fallback = err.Error()
		} else {
			diag = enhanced
			llmUsed = true
		}
	} else if useLLM && !s.llm.Available() {
		fallback = ""
	}
	switch {
	case llmUsed:
		mode = ModeLLM
	case sub.DemoCaseID != "":
		mode = ModeDemo
	default:
		mode = ModeRule
	}

	now := time.Now().UTC()
	sess := &models.Session{
		ID:                        uuid.NewString(),
		CreatedAt:                 now,
		UpdatedAt:                 now,
		Status:                    models.StatusDiagnosed,
		Mode:                      mode,
		LLMUsed:                   llmUsed,
		LLMFallbackReason:         truncateForColumn(fallback, 240),
		IsDemo:                    sub.DemoCaseID != "",
		DemoCaseID:                sub.DemoCaseID,
		ExperimentTitle:           sub.ExperimentTitle,
		ExperimentGoal:            sub.ExperimentGoal,
		DatasetDescription:        sub.DatasetDescription,
		ModelName:                 sub.ModelName,
		CodeSnippet:               sub.CodeSnippet,
		ExecutionLog:              sub.ExecutionLog,
		ClassLabelsJSON:           encodeJSON(labels),
		MetricsText:               sub.MetricsText,
		StudentInitialExplanation: sub.StudentInitialExplanation,
		StudentQuestion:           sub.Question,
		DiagnosisJSON:             encodeJSON(diag),
		MainIssue:                 truncateForColumn(diag.MainIssue, 240),
		CategoriesCSV:             services.CategoriesCSV(diag.Categories),
		PrimaryArchetype:          diag.PrimaryArchetype,
		TeacherIntervention:       diag.TeacherIntervention,
		TransferScore:             -1,
	}
	if metrics != nil {
		sess.MatrixJSON = encodeJSON(metrics.Values)
		sess.MetricsJSON = encodeJSON(metrics)
		sess.MatrixRaw = sub.ConfusionMatrixText
	}
	sess.HintStateJSON = encodeJSON(models.HintState{NextLevel: 1, MaxLevel: 3, UsedLevels: []int{}})
	sess.HintsJSON = encodeJSON([]models.Hint{})
	sess.RevisionsJSON = encodeJSON([]models.Revision{})
	if err := s.store.Create(sess); err != nil {
		return nil, err
	}
	return sess, nil
}

// analyzeMatrix 解析并计算混淆矩阵；未提供矩阵时返回 nil 指标。
func analyzeMatrix(sub models.Submission) ([][]float64, []string, *services.MatrixMetrics, error) {
	values := sub.ConfusionMatrix
	labels := sub.ClassLabels
	if len(values) == 0 && strings.TrimSpace(sub.ConfusionMatrixText) != "" {
		parsed, parsedLabels, err := services.ParseMatrixText(sub.ConfusionMatrixText)
		if err != nil {
			return nil, nil, nil, err
		}
		values = parsed
		if len(labels) == 0 {
			labels = parsedLabels
		}
	}
	if len(values) == 0 {
		return nil, labels, nil, nil
	}
	clean := make([][]float64, len(values))
	for i, row := range values {
		clean[i] = append([]float64(nil), row...)
	}
	metrics, err := services.ComputeMetrics(clean, labels)
	if err != nil {
		return nil, nil, nil, err
	}
	return clean, metrics.Labels, metrics, nil
}

func fillFromDemo(sub *models.Submission, demo content.DemoCase) {
	d := demo.Submission
	setIfEmpty(&sub.ExperimentTitle, d.ExperimentTitle)
	setIfEmpty(&sub.ExperimentGoal, d.ExperimentGoal)
	setIfEmpty(&sub.DatasetDescription, d.DatasetDescription)
	setIfEmpty(&sub.ModelName, d.ModelName)
	setIfEmpty(&sub.CodeSnippet, d.CodeSnippet)
	setIfEmpty(&sub.ExecutionLog, d.ExecutionLog)
	setIfEmpty(&sub.ConfusionMatrixText, d.ConfusionMatrixText)
	setIfEmpty(&sub.MetricsText, d.MetricsText)
	setIfEmpty(&sub.StudentInitialExplanation, d.StudentInitialExplanation)
	setIfEmpty(&sub.Question, d.Question)
	if len(sub.ClassLabels) == 0 {
		sub.ClassLabels = d.ClassLabels
	}
}

func setIfEmpty(dst *string, v string) {
	if strings.TrimSpace(*dst) == "" {
		*dst = v
	}
}

// CreateSession 处理 POST /api/sessions。
func (s *Server) CreateSession(c *gin.Context) {
	var sub models.Submission
	if err := c.ShouldBindJSON(&sub); err != nil {
		fail(c, 400, "BAD_JSON", "提交内容格式不正确，请检查后重试。你的输入不会丢失。", err.Error())
		return
	}
	sess, err := s.newSession(sub)
	if err != nil {
		if isValidationError(err) {
			failValidation(c, err)
			return
		}
		fail(c, 500, "INTERNAL", "服务器处理失败："+err.Error(), "")
		return
	}
	c.JSON(200, gin.H{"ok": true, "session": s.sessionDTO(sess)})
}

// ListSessions 处理 GET /api/sessions。
func (s *Server) ListSessions(c *gin.Context) {
	list, err := s.store.List(0)
	if err != nil {
		fail(c, 500, "INTERNAL", "读取学习记录失败："+err.Error(), "")
		return
	}
	records := make([]models.RecordDTO, 0, len(list))
	for i := range list {
		records = append(records, s.recordDTO(&list[i]))
	}
	real, demo := countDemo(list)
	c.JSON(200, gin.H{
		"ok":      true,
		"records": records,
		"storage": gin.H{
			"location":    "服务端本地 SQLite（data/knowbr.db）",
			"total":       len(list),
			"demoRecords": demo,
			"realRecords": real,
			"note":        "记录保存在本机数据库，用于演示与教学回顾；不会收集姓名、学号等个人信息。",
		},
	})
}

// GetSession 处理 GET /api/sessions/:id。
func (s *Server) GetSession(c *gin.Context) {
	sess, err := s.store.Get(c.Param("id"))
	if err != nil {
		notFound(c)
		return
	}
	c.JSON(200, gin.H{"ok": true, "session": s.sessionDTO(sess)})
}

// DeleteSession 处理 DELETE /api/sessions/:id。
func (s *Server) DeleteSession(c *gin.Context) {
	if err := s.store.Delete(c.Param("id")); err != nil {
		notFound(c)
		return
	}
	c.JSON(200, gin.H{"ok": true})
}

// ClearSessions 处理 DELETE /api/sessions（清空记录）。
func (s *Server) ClearSessions(c *gin.Context) {
	n, err := s.store.DeleteAll()
	if err != nil {
		fail(c, 500, "INTERNAL", "清空记录失败："+err.Error(), "")
		return
	}
	c.JSON(200, gin.H{"ok": true, "deleted": n})
}

// AddHint 处理 POST /api/sessions/:id/hints（功能 E：分层提示）。
func (s *Server) AddHint(c *gin.Context) {
	sess, err := s.store.Get(c.Param("id"))
	if err != nil {
		notFound(c)
		return
	}
	var body struct {
		Level              int  `json:"level"`
		RequestedByStudent bool `json:"requestedByStudent"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, 400, "BAD_JSON", "请求格式不正确。", err.Error())
		return
	}
	diag, _ := decodeJSON[services.Diagnosis](sess.DiagnosisJSON)
	metrics, _ := decodeJSON[services.MatrixMetrics](sess.MetricsJSON)
	hints, _ := decodeJSON[[]models.Hint](sess.HintsJSON)
	state, ok := decodeJSON[models.HintState](sess.HintStateJSON)
	if !ok {
		state = models.HintState{NextLevel: 1, MaxLevel: 3}
	}

	alreadyExists := false
	for _, h := range hints {
		if h.Level == body.Level {
			alreadyExists = true
		}
	}
	hint, newState, err := services.BuildHint(services.HintRequest{
		Level:              body.Level,
		Diagnosis:          diag,
		Metrics:            metricsPtr(metrics),
		Existing:           hints,
		State:              state,
		RequestedByStudent: body.RequestedByStudent,
	})
	if err != nil {
		fail(c, 400, "HINT_LEVEL_INVALID", err.Error(), "提示等级只能是 1、2 或 3。")
		return
	}
	if !alreadyExists {
		hints = append(hints, hint)
	}
	sess.HintsJSON = encodeJSON(hints)
	sess.HintStateJSON = encodeJSON(newState)
	if sess.Status == models.StatusDiagnosed {
		sess.Status = models.StatusTutoring
	}
	sess.UpdatedAt = time.Now().UTC()
	if err := s.store.Save(sess); err != nil {
		fail(c, 500, "INTERNAL", "保存提示失败："+err.Error(), "")
		return
	}
	c.JSON(200, gin.H{"ok": true, "hint": hint, "hints": hints, "hintState": newState, "alreadyProvided": alreadyExists})
}

// AddRevision 处理 POST /api/sessions/:id/revisions（功能 E：学生修正解释）。
func (s *Server) AddRevision(c *gin.Context) {
	sess, err := s.store.Get(c.Param("id"))
	if err != nil {
		notFound(c)
		return
	}
	var body struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, 400, "BAD_JSON", "请求格式不正确。", err.Error())
		return
	}
	if strings.TrimSpace(body.Content) == "" {
		fail(c, 400, "REVISION_EMPTY", "请先写下你现在的解释，再提交。提交后系统会判断是否有所修正。", "")
		return
	}
	diag, _ := decodeJSON[services.Diagnosis](sess.DiagnosisJSON)
	metrics, _ := decodeJSON[services.MatrixMetrics](sess.MetricsJSON)
	revisions, _ := decodeJSON[[]models.Revision](sess.RevisionsJSON)
	state, ok := decodeJSON[models.HintState](sess.HintStateJSON)
	if !ok {
		state = models.HintState{NextLevel: 1, MaxLevel: 3}
	}

	res := services.AnalyzeRevision(services.RevisionRequest{
		Content:            body.Content,
		InitialExplanation: sess.StudentInitialExplanation,
		Previous:           revisions,
		Diagnosis:          diag,
		Metrics:            metricsPtr(metrics),
		State:              state,
	})
	revisions = append(revisions, res.Revision)
	sess.RevisionsJSON = encodeJSON(revisions)
	sess.HintStateJSON = encodeJSON(res.State)
	if sess.Status == models.StatusDiagnosed {
		sess.Status = models.StatusTutoring
	}
	sess.UpdatedAt = time.Now().UTC()
	if err := s.store.Save(sess); err != nil {
		fail(c, 500, "INTERNAL", "保存解释失败："+err.Error(), "")
		return
	}
	c.JSON(200, gin.H{
		"ok":        true,
		"revision":  res.Revision,
		"revisions": revisions,
		"hintState": res.State,
		"improved":  res.Improved,
	})
}

// TransferQuestion 处理 POST /api/sessions/:id/transfer/question（功能 F）。
func (s *Server) TransferQuestion(c *gin.Context) {
	sess, err := s.store.Get(c.Param("id"))
	if err != nil {
		notFound(c)
		return
	}
	if existing, ok := decodeJSON[services.TransferState](sess.TransferJSON); ok && existing.ItemID != "" {
		if !existing.Answered {
			existing.ReferenceAnswer = ""
			existing.ExpectedPoints = nil
		}
		c.JSON(200, gin.H{"ok": true, "transfer": existing, "reused": true})
		return
	}
	diag, _ := decodeJSON[services.Diagnosis](sess.DiagnosisJSON)
	var archetypes []string
	for _, a := range diag.MatchedArchetypes {
		archetypes = append(archetypes, a.Code)
	}
	if len(archetypes) == 0 && diag.PrimaryArchetype != "" && diag.PrimaryArchetype != "NONE" {
		archetypes = append(archetypes, diag.PrimaryArchetype)
	}
	item := services.SelectTransferItem(archetypes, nil)
	state := services.NewTransferState(item)
	sess.TransferJSON = encodeJSON(state)
	sess.Status = models.StatusTransfer
	sess.UpdatedAt = time.Now().UTC()
	if err := s.store.Save(sess); err != nil {
		fail(c, 500, "INTERNAL", "生成迁移题失败："+err.Error(), "")
		return
	}
	c.JSON(200, gin.H{"ok": true, "transfer": state, "reused": false})
}

// TransferAnswer 处理 POST /api/sessions/:id/transfer/answer（提交答案并获得评分）。
func (s *Server) TransferAnswer(c *gin.Context) {
	sess, err := s.store.Get(c.Param("id"))
	if err != nil {
		notFound(c)
		return
	}
	var body struct {
		Answer string `json:"answer"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, 400, "BAD_JSON", "请求格式不正确。", err.Error())
		return
	}
	if strings.TrimSpace(body.Answer) == "" {
		fail(c, 400, "ANSWER_EMPTY", "请先写下你的答案再提交；参考答案会在提交之后显示。", "")
		return
	}
	current, ok := decodeJSON[services.TransferState](sess.TransferJSON)
	if !ok || current.ItemID == "" {
		fail(c, 400, "TRANSFER_NOT_STARTED", "还没有生成迁移题，请先进入迁移测试。", "")
		return
	}
	item, ok := content.TransferItemByID(current.ItemID)
	if !ok {
		fail(c, 500, "ITEM_MISSING", "迁移题题库中找不到对应题目。", current.ItemID)
		return
	}
	scored := services.ScoreTransfer(item, body.Answer)
	scored.Completed = false
	sess.TransferJSON = encodeJSON(scored)
	sess.TransferScore = scored.Score
	sess.Status = models.StatusTransfer
	sess.UpdatedAt = time.Now().UTC()
	if err := s.store.Save(sess); err != nil {
		fail(c, 500, "INTERNAL", "保存答案失败："+err.Error(), "")
		return
	}
	c.JSON(200, gin.H{"ok": true, "transfer": scored})
}

// TransferReference 处理 POST /api/sessions/:id/transfer/reference（学生作答后查看参考答案）。
func (s *Server) TransferReference(c *gin.Context) {
	sess, err := s.store.Get(c.Param("id"))
	if err != nil {
		notFound(c)
		return
	}
	state, ok := decodeJSON[services.TransferState](sess.TransferJSON)
	if !ok || state.ItemID == "" {
		fail(c, 400, "TRANSFER_NOT_STARTED", "还没有生成迁移题。", "")
		return
	}
	if !state.Answered {
		fail(c, 400, "ANSWER_REQUIRED", "请先独立作答并提交，系统才会显示参考答案与评分依据。", "")
		return
	}
	revealed := services.RevealReference(state)
	sess.TransferJSON = encodeJSON(revealed)
	sess.UpdatedAt = time.Now().UTC()
	if err := s.store.Save(sess); err != nil {
		fail(c, 500, "INTERNAL", "保存失败："+err.Error(), "")
		return
	}
	c.JSON(200, gin.H{"ok": true, "transfer": revealed})
}

// TransferComplete 处理 POST /api/sessions/:id/transfer/complete（完成本次学习并生成反馈）。
func (s *Server) TransferComplete(c *gin.Context) {
	sess, err := s.store.Get(c.Param("id"))
	if err != nil {
		notFound(c)
		return
	}
	state, ok := decodeJSON[services.TransferState](sess.TransferJSON)
	if !ok || state.ItemID == "" || !state.Answered {
		fail(c, 400, "TRANSFER_INCOMPLETE", "请先提交迁移题答案，再完成本次学习。", "")
		return
	}
	state = services.RevealReference(state)
	state.Completed = true
	sess.TransferJSON = encodeJSON(state)
	sess.Status = models.StatusCompleted
	sess.UpdatedAt = time.Now().UTC()
	summary, markdown, err := s.buildAndStoreSummary(sess)
	if err != nil {
		fail(c, 500, "INTERNAL", "生成学习摘要失败："+err.Error(), "")
		return
	}
	c.JSON(200, gin.H{"ok": true, "transfer": state, "summary": summary, "markdown": markdown, "session": s.sessionDTO(sess)})
}

// GetSummary 处理 GET /api/sessions/:id/summary（功能 G）。
func (s *Server) GetSummary(c *gin.Context) {
	sess, err := s.store.Get(c.Param("id"))
	if err != nil {
		notFound(c)
		return
	}
	summary, markdown, err := s.buildAndStoreSummary(sess)
	if err != nil {
		fail(c, 500, "INTERNAL", "生成学习摘要失败："+err.Error(), "")
		return
	}
	c.JSON(200, gin.H{"ok": true, "summary": summary, "markdown": markdown, "teacherSummary": sess.TeacherSummary})
}

func (s *Server) buildAndStoreSummary(sess *models.Session) (services.Summary, string, error) {
	diag, _ := decodeJSON[services.Diagnosis](sess.DiagnosisJSON)
	metrics, _ := decodeJSON[services.MatrixMetrics](sess.MetricsJSON)
	hints, _ := decodeJSON[[]models.Hint](sess.HintsJSON)
	revisions, _ := decodeJSON[[]models.Revision](sess.RevisionsJSON)
	state, _ := decodeJSON[models.HintState](sess.HintStateJSON)
	var transfer *services.TransferState
	if t, ok := decodeJSON[services.TransferState](sess.TransferJSON); ok && t.ItemID != "" {
		transfer = &t
		if transfer.Answered {
			*transfer = services.RevealReference(*transfer)
		}
	}
	label, _ := modeInfo(sess.Mode, sess.LLMUsed, sess.LLMFallbackReason, s.llmModel, s.llm.Available())
	summary := services.BuildSummary(services.SummaryInput{
		Session:    sess,
		Diagnosis:  diag,
		Metrics:    metricsPtr(metrics),
		Hints:      hints,
		Revisions:  revisions,
		Transfer:   transfer,
		HintState:  state,
		LLMUsed:    sess.LLMUsed,
		ModeLabel:  label,
		DataNotice: demoNotice(sess.DemoCaseID),
		IsDemo:     sess.IsDemo,
	})
	if s.llm.Available() && sess.LLMUsed {
		if text, err := s.llm.EnhanceSummary(ctxOrBackground(), summary); err == nil {
			summary.TeacherSummary = text
		}
	}
	sess.SummaryJSON = encodeJSON(summary)
	sess.TeacherSummary = summary.TeacherSummary
	sess.UpdatedAt = time.Now().UTC()
	if err := s.store.Save(sess); err != nil {
		return summary, "", err
	}
	return summary, services.SummaryMarkdown(summary), nil
}

// ExportSummary 处理 GET /api/sessions/:id/export?format=md|txt（功能 G：复制/导出）。
func (s *Server) ExportSummary(c *gin.Context) {
	sess, err := s.store.Get(c.Param("id"))
	if err != nil {
		notFound(c)
		return
	}
	summary, markdown, err := s.buildAndStoreSummary(sess)
	if err != nil {
		fail(c, 500, "INTERNAL", "生成学习摘要失败："+err.Error(), "")
		return
	}
	format := strings.ToLower(c.DefaultQuery("format", "md"))
	name := safeFileName(sess.ExperimentTitle) + "-学习反馈"
	var content, contentType, filename string
	if format == "txt" {
		content = summary.TeacherSummary + "\n\n" + summary.Disclaimer + "\n"
		contentType = "text/plain; charset=utf-8"
		filename = name + ".txt"
	} else {
		content = markdown
		contentType = "text/markdown; charset=utf-8"
		filename = name + ".md"
	}
	if c.Query("download") == "1" {
		c.Header("Content-Disposition", "attachment; filename*=UTF-8''"+url.PathEscape(filename))
	}
	c.Data(200, contentType, []byte(content))
}

// ComputeMetricsEndpoint 处理 POST /api/metrics/compute（功能：指标计算与校验）。
func (s *Server) ComputeMetricsEndpoint(c *gin.Context) {
	var body struct {
		ConfusionMatrix     [][]float64 `json:"confusionMatrix"`
		ConfusionMatrixText string      `json:"confusionMatrixText"`
		ClassLabels         []string    `json:"classLabels"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, 400, "BAD_JSON", "请求格式不正确。", err.Error())
		return
	}
	_, labels, metrics, err := analyzeMatrix(models.Submission{
		ConfusionMatrix:     body.ConfusionMatrix,
		ConfusionMatrixText: body.ConfusionMatrixText,
		ClassLabels:         body.ClassLabels,
	})
	if err != nil {
		failValidation(c, err)
		return
	}
	if metrics == nil {
		c.JSON(200, gin.H{"ok": true, "metrics": nil, "message": "未提供混淆矩阵，未做任何计算。"})
		return
	}
	c.JSON(200, gin.H{"ok": true, "metrics": metrics, "labels": labels})
}

func metricsPtr(m services.MatrixMetrics) *services.MatrixMetrics {
	if m.Total == 0 && len(m.Classes) == 0 {
		return nil
	}
	return &m
}

func countDemo(list []models.Session) (real, demo int) {
	for _, s := range list {
		if s.IsDemo {
			demo++
		} else {
			real++
		}
	}
	return real, demo
}

func truncateForColumn(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

func safeFileName(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "知行桥"
	}
	replacer := strings.NewReplacer("/", "_", "\\", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_")
	return replacer.Replace(s)
}

// commonIssues 统计已保存记录中的共性问题（仅基于真实存在的记录，不做任何推算）。
func commonIssues(list []models.Session) []gin.H {
	type bucket struct {
		Label   string
		Count   int
		Records []string
	}
	buckets := map[string]*bucket{}
	for i := range list {
		for _, code := range csvToList(list[i].CategoriesCSV) {
			b, ok := buckets[code]
			if !ok {
				b = &bucket{Label: categoryLabel(code)}
				buckets[code] = b
			}
			b.Count++
			b.Records = append(b.Records, list[i].ID)
		}
	}
	out := make([]gin.H, 0, len(buckets))
	for code, b := range buckets {
		item := gin.H{"code": code, "label": b.Label, "count": b.Count, "recordIds": b.Records}
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		ci, _ := out[i]["count"].(int)
		cj, _ := out[j]["count"].(int)
		if ci != cj {
			return ci > cj
		}
		return fmt.Sprint(out[i]["label"]) < fmt.Sprint(out[j]["label"])
	})
	return out
}
