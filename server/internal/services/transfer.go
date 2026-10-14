package services

import (
	"fmt"
	"strings"

	"knowbr/server/internal/content"
)

// RubricDimRef 是评分维度的对外结构（不含规则关键词）。
type RubricDimRef struct {
	Code     string `json:"code"`
	Title    string `json:"title"`
	MaxScore int    `json:"maxScore"`
}

// RubricFeedback 是单个维度的评分与理由。
type RubricFeedback struct {
	Code            string   `json:"code"`
	Title           string   `json:"title"`
	Score           int      `json:"score"`
	MaxScore        int      `json:"maxScore"`
	Reason          string   `json:"reason"`
	MatchedKeywords []string `json:"matchedKeywords"`
}

// TransferState 是一次迁移测试的完整状态。
// 参考答案与期望要点只有在学生提交答案之后才会被填充，避免提前泄露。
type TransferState struct {
	ItemID          string           `json:"itemId"`
	Topic           string           `json:"topic"`
	Context         string           `json:"context"`
	Question        string           `json:"question"`
	MaxScore        int              `json:"maxScore"`
	Rubric          []RubricDimRef   `json:"rubric"`
	Answer          string           `json:"answer,omitempty"`
	Answered        bool             `json:"answered"`
	Score           int              `json:"score"`
	ScoreLabel      string           `json:"scoreLabel"`
	RubricFeedback  []RubricFeedback `json:"rubricFeedback"`
	ReferenceAnswer string           `json:"referenceAnswer,omitempty"`
	ExpectedPoints  []string         `json:"expectedPoints,omitempty"`
	ScoredAt        string           `json:"scoredAt,omitempty"`
	Completed       bool             `json:"completed"`
	Disclaimer      string           `json:"disclaimer"`
	MatchedKeywords []string         `json:"matchedKeywords"`
	MissingPoints   []string         `json:"missingPoints"`
}

// SelectTransferItem 依据命中的典型误区选择迁移题；已使用过的题目不会被重复选择。
func SelectTransferItem(matchedArchetypes []string, usedItemIDs []string) content.TransferItem {
	used := map[string]bool{}
	for _, id := range usedItemIDs {
		used[id] = true
	}
	best := content.TransferItem{}
	bestOverlap := -1
	for _, item := range content.TransferItems {
		if used[item.ID] {
			continue
		}
		overlap := 0
		for _, a := range item.Archetypes {
			for _, m := range matchedArchetypes {
				if a == m {
					overlap++
				}
			}
		}
		if overlap > bestOverlap {
			best, bestOverlap = item, overlap
		}
	}
	if best.ID == "" {
		// 全部题目都用过时，退回题库第一题（保证流程不中断）。
		return content.TransferItems[0]
	}
	return best
}

// NewTransferState 依据题库构造迁移测试状态。
func NewTransferState(item content.TransferItem) TransferState {
	rubric := make([]RubricDimRef, 0, len(item.Rubric))
	for _, d := range item.Rubric {
		rubric = append(rubric, RubricDimRef{Code: d.Code, Title: d.Title, MaxScore: d.MaxScore})
	}
	return TransferState{
		ItemID:          item.ID,
		Topic:           item.Topic,
		Context:         item.Context,
		Question:        item.Question,
		MaxScore:        item.MaxScore,
		Rubric:          rubric,
		ScoreLabel:      fmt.Sprintf("0 / %d", item.MaxScore),
		Disclaimer:      content.TransferDisclaimer,
		RubricFeedback:  []RubricFeedback{},
		MatchedKeywords: []string{},
		MissingPoints:   []string{},
	}
}

// ScoreTransfer 按评分维度对学生作答进行规则化评分，并给出每个维度的理由。
func ScoreTransfer(item content.TransferItem, answer string) TransferState {
	low := strings.ToLower(answer)
	state := NewTransferState(item)
	state.Answer = answer
	state.Answered = true
	state.ScoredAt = nowISO()

	total := 0
	var matchedAll, missing []string
	for _, dim := range item.Rubric {
		hits := 0
		var matched []string
		for _, group := range dim.KeywordGroups {
			groupHit := false
			for _, w := range group {
				if w != "" && strings.Contains(low, strings.ToLower(w)) {
					groupHit = true
					matched = append(matched, w)
				}
			}
			if groupHit {
				hits++
			}
		}
		score := hits
		if score > dim.MaxScore {
			score = dim.MaxScore
		}
		reason := dim.MissHint
		switch {
		case score >= dim.MaxScore:
			reason = dim.FullHitHint
		case score > 0:
			reason = dim.PartialHint
		}
		total += score
		matchedAll = append(matchedAll, matched...)
		if score < dim.MaxScore {
			missing = append(missing, dim.Title)
		}
		state.RubricFeedback = append(state.RubricFeedback, RubricFeedback{
			Code:            dim.Code,
			Title:           dim.Title,
			Score:           score,
			MaxScore:        dim.MaxScore,
			Reason:          reason,
			MatchedKeywords: dedupe(matched),
		})
	}
	state.Score = total
	state.ScoreLabel = fmt.Sprintf("%d / %d", total, item.MaxScore)
	state.MatchedKeywords = dedupe(matchedAll)
	state.MissingPoints = missing
	state.Disclaimer = content.TransferDisclaimer
	return state
}

// RevealReference 在学生已作答后填充参考答案与期望要点。
func RevealReference(state TransferState) TransferState {
	item, ok := content.TransferItemByID(state.ItemID)
	if !ok || !state.Answered {
		return state
	}
	state.ReferenceAnswer = item.ReferenceAnswer
	state.ExpectedPoints = item.ExpectedPoints
	return state
}

func dedupe(list []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(list))
	for _, s := range list {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
