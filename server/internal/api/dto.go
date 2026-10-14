package api

import (
	"strings"
	"time"

	"knowbr/server/internal/content"
	"knowbr/server/internal/models"
	"knowbr/server/internal/services"
)

var statusLabels = map[string]string{
	models.StatusDiagnosed: "已完成诊断",
	models.StatusTutoring:  "分层辅导中",
	models.StatusTransfer:  "迁移测试中",
	models.StatusCompleted: "已完成学习",
}

// statusLabel 返回状态中文标签。
func statusLabel(status string) string {
	if l, ok := statusLabels[status]; ok {
		return l
	}
	return status
}

// sessionDTO 把数据库记录转换为前端使用的完整结构。
func (s *Server) sessionDTO(sess *models.Session) models.SessionDTO {
	label, notice := modeInfo(sess.Mode, sess.LLMUsed, sess.LLMFallbackReason, s.llmModel, s.llm.Available())
	hintState, ok := decodeJSON[models.HintState](sess.HintStateJSON)
	if !ok {
		hintState = models.HintState{NextLevel: 1, MaxLevel: 3}
	}
	if hintState.UsedLevels == nil {
		hintState.UsedLevels = []int{}
	}
	hints, _ := decodeJSON[[]models.Hint](sess.HintsJSON)
	revisions, _ := decodeJSON[[]models.Revision](sess.RevisionsJSON)
	if hints == nil {
		hints = []models.Hint{}
	}
	if revisions == nil {
		revisions = []models.Revision{}
	}

	dto := models.SessionDTO{
		ID:                sess.ID,
		CreatedAt:         sess.CreatedAt.Format(time.RFC3339),
		UpdatedAt:         sess.UpdatedAt.Format(time.RFC3339),
		Status:            sess.Status,
		Mode:              sess.Mode,
		ModeLabel:         label,
		ModeNotice:        notice,
		LLMUsed:           sess.LLMUsed,
		LLMFallbackReason: sess.LLMFallbackReason,
		IsDemo:            sess.IsDemo,
		DemoCaseID:        sess.DemoCaseID,
		Experiment: models.ExperimentDTO{
			ExperimentTitle:    sess.ExperimentTitle,
			ExperimentGoal:     sess.ExperimentGoal,
			DatasetDescription: sess.DatasetDescription,
			ModelName:          sess.ModelName,
			CodeSnippet:        sess.CodeSnippet,
			ExecutionLog:       sess.ExecutionLog,
			ClassLabels:        decodeJSONOrNil[[]string](sess.ClassLabelsJSON),
			MetricsText:        sess.MetricsText,
			InitialExplanation: sess.StudentInitialExplanation,
			Question:           sess.StudentQuestion,
		},
		Matrix:         rawOrNil(sess.MatrixJSON),
		Metrics:        rawOrNil(sess.MetricsJSON),
		Diagnosis:      rawOrNil(sess.DiagnosisJSON),
		Hints:          hints,
		HintState:      hintState,
		Revisions:      revisions,
		Transfer:       rawOrNil(sess.TransferJSON),
		Summary:        rawOrNil(sess.SummaryJSON),
		TeacherSummary: sess.TeacherSummary,
		MainIssue:      sess.MainIssue,
		Categories:     csvToList(sess.CategoriesCSV),
		TransferScore:  sess.TransferScore,
	}
	if sess.IsDemo {
		dto.DataNotice = demoNotice(sess.DemoCaseID)
	}
	return dto
}

func decodeJSONOrNil[T any](s string) T {
	v, _ := decodeJSON[T](s)
	return v
}

// recordDTO 把数据库记录转换为记录列表行。
func (s *Server) recordDTO(sess *models.Session) models.RecordDTO {
	label, _ := modeInfo(sess.Mode, sess.LLMUsed, sess.LLMFallbackReason, s.llmModel, s.llm.Available())
	state, _ := decodeJSON[models.HintState](sess.HintStateJSON)
	if state.UsedLevels == nil {
		state.UsedLevels = []int{}
	}
	revisions, _ := decodeJSON[[]models.Revision](sess.RevisionsJSON)
	transfer, hasTransfer := decodeJSON[services.TransferState](sess.TransferJSON)
	improved := false
	for _, r := range revisions {
		if r.Progress == "improved" {
			improved = true
		}
	}
	cats := csvToList(sess.CategoriesCSV)
	labels := make([]string, 0, len(cats))
	for _, c := range cats {
		labels = append(labels, categoryLabel(c))
	}
	maxScore := 0
	if hasTransfer {
		maxScore = transfer.MaxScore
	}
	title := sess.ExperimentTitle
	if strings.TrimSpace(title) == "" {
		title = "未命名实验"
	}
	return models.RecordDTO{
		ID:                  sess.ID,
		Title:               title,
		CreatedAt:           sess.CreatedAt.Format(time.RFC3339),
		UpdatedAt:           sess.UpdatedAt.Format(time.RFC3339),
		Status:              sess.Status,
		StatusLabel:         statusLabel(sess.Status),
		Mode:                sess.Mode,
		ModeLabel:           label,
		LLMUsed:             sess.LLMUsed,
		IsDemo:              sess.IsDemo,
		MainIssue:           sess.MainIssue,
		Categories:          cats,
		CategoryLabels:      labels,
		PrimaryArchetype:    sess.PrimaryArchetype,
		HintLevelsUsed:      state.UsedLevels,
		RevisionImproved:    improved,
		RevisionCount:       len(revisions),
		TransferScore:       sess.TransferScore,
		TransferMaxScore:    maxScore,
		TeacherIntervention: sess.TeacherIntervention,
	}
}

func csvToList(csv string) []string {
	if strings.TrimSpace(csv) == "" {
		return []string{}
	}
	out := []string{}
	for _, p := range strings.Split(csv, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func categoryLabel(code string) string { return content.CategoryLabel(code) }

func demoNotice(caseID string) string {
	if c, ok := content.DemoCaseByID(caseID); ok {
		return c.DataNotice
	}
	return "本记录来自预置演示案例，数据为人工构造的教学演示数据。"
}
