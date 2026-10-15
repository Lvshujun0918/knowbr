package api

import (
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"knowbr/server/internal/models"
	"knowbr/server/internal/services"
)

// agentDecision 汇总当前会话状态，交给策略引擎决定下一步动作。
func (s *Server) agentDecision(sess *models.Session) services.AgentStep {
	diag, _ := decodeJSON[services.Diagnosis](sess.DiagnosisJSON)
	metrics, _ := decodeJSON[services.MatrixMetrics](sess.MetricsJSON)
	hints, _ := decodeJSON[[]models.Hint](sess.HintsJSON)
	revisions, _ := decodeJSON[[]models.Revision](sess.RevisionsJSON)
	state, ok := decodeJSON[models.HintState](sess.HintStateJSON)
	if !ok {
		state = models.HintState{NextLevel: 1, MaxLevel: 3}
	}
	transfer, hasTransfer := decodeJSON[services.TransferState](sess.TransferJSON)

	return services.DecideNextAction(services.AgentInput{
		Status:            sess.Status,
		TaskType:          diag.TaskType,
		Diagnosis:         diag,
		Metrics:           metricsPtr(metrics),
		Hints:             hints,
		Revisions:         revisions,
		HintState:         state,
		HasTransfer:       hasTransfer && transfer.ItemID != "",
		TransferAnswered:  hasTransfer && transfer.Answered,
		TransferCompleted: hasTransfer && transfer.Completed,
		History:           s.agentHistory(sess.ID),
	})
}

// agentHistory 返回除当前会话以外的本机历史记录（智能体的长期记忆来源）。
func (s *Server) agentHistory(excludeID string) []models.Session {
	list, err := s.store.List(0)
	if err != nil {
		return nil
	}
	out := make([]models.Session, 0, len(list))
	for i := range list {
		if list[i].ID == excludeID {
			continue
		}
		out = append(out, list[i])
	}
	return out
}

// GetAgentDecision 处理 GET /api/sessions/:id/agent（只看决策，不产生任何副作用）。
func (s *Server) GetAgentDecision(c *gin.Context) {
	sess, err := s.store.Get(c.Param("id"))
	if err != nil {
		notFound(c)
		return
	}
	c.JSON(200, gin.H{"ok": true, "agent": s.agentDecision(sess)})
}

// AgentStepEndpoint 处理 POST /api/sessions/:id/agent/step：
// 先决策，再执行可执行的动作（给出建议等级的提示、标记教师介入），因此一次调用就是一个完整的“智能体步”。
func (s *Server) AgentStepEndpoint(c *gin.Context) {
	sess, err := s.store.Get(c.Param("id"))
	if err != nil {
		notFound(c)
		return
	}
	decision := s.agentDecision(sess)

	resp := gin.H{"ok": true, "agent": decision, "executed": false, "executedNote": ""}
	switch decision.Action {
	case services.AgentGiveHint:
		diag, _ := decodeJSON[services.Diagnosis](sess.DiagnosisJSON)
		metrics, _ := decodeJSON[services.MatrixMetrics](sess.MetricsJSON)
		hints, _ := decodeJSON[[]models.Hint](sess.HintsJSON)
		revisions, _ := decodeJSON[[]models.Revision](sess.RevisionsJSON)
		state, ok := decodeJSON[models.HintState](sess.HintStateJSON)
		if !ok {
			state = models.HintState{NextLevel: 1, MaxLevel: 3}
		}
		latestAnswer := ""
		if len(revisions) > 0 {
			latestAnswer = revisions[len(revisions)-1].Content
		}
		latestClaims := []services.Claim{}
		if strings.TrimSpace(latestAnswer) != "" {
			latestClaims = services.AnalyzeClaims(services.DiagnoseInput{Explanation: latestAnswer}, diag.TaskType, diag.Contradiction)
		}
		alreadyExists := false
		for _, h := range hints {
			if h.Level == decision.SuggestHintLevel {
				alreadyExists = true
			}
		}
		hint, newState, err := services.BuildHint(services.HintRequest{
			Level:              decision.SuggestHintLevel,
			Diagnosis:          diag,
			Metrics:            metricsPtr(metrics),
			Existing:           hints,
			State:              state,
			RequestedByStudent: false,
			TaskType:           diag.TaskType,
			LatestAnswer:       latestAnswer,
			LatestClaims:       latestClaims,
			LogTail:            lastNonEmptyLine(sess.ExecutionLog),
		})
		if err != nil {
			fail(c, 500, "AGENT_HINT_FAILED", "智能体无法生成提示："+err.Error(), "")
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
			fail(c, 500, "INTERNAL", "保存智能体提示失败："+err.Error(), "")
			return
		}
		resp["executed"] = true
		resp["executedNote"] = fmt.Sprintf("智能体已自动给出第 %d 级提示（由系统决定，非学生点击）。", decision.SuggestHintLevel)
		resp["hint"] = hint
		resp["hints"] = hints
		resp["hintState"] = newState

	case services.AgentEscalate:
		state, ok := decodeJSON[models.HintState](sess.HintStateJSON)
		if !ok {
			state = models.HintState{NextLevel: 1, MaxLevel: 3}
		}
		if !state.TeacherIntervention {
			state.TeacherIntervention = true
			state.TeacherReason = decision.Reason
			sess.HintStateJSON = encodeJSON(state)
			sess.TeacherIntervention = true
			sess.UpdatedAt = time.Now().UTC()
			if err := s.store.Save(sess); err != nil {
				fail(c, 500, "INTERNAL", "保存教师介入标记失败："+err.Error(), "")
				return
			}
			resp["executed"] = true
			resp["executedNote"] = "智能体已把本次学习标记为「建议教师介入」，并记录了原因。"
		} else {
			resp["executedNote"] = "此前已经标记为需要教师介入，本次不重复记录。"
		}
		resp["hintState"] = state

	case services.AgentAskEvidence:
		resp["executedNote"] = "智能体暂不给出提示：请先补齐必需证据，再决定提示等级。"

	default:
		resp["executedNote"] = "当前状态没有需要系统自动执行的动作，请按建议的下一步继续。"
	}

	resp["session"] = s.sessionDTO(sess)
	c.JSON(200, resp)
}
