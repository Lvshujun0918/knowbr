package services

import (
	"fmt"
	"hash/fnv"
	"sort"
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
	ItemID            string            `json:"itemId"`
	Topic             string            `json:"topic"`
	Scenario          string            `json:"scenario"`
	KnowledgePoint    string            `json:"knowledgePoint"`
	TaskType          string            `json:"taskType"`
	TaskTypeLabel     string            `json:"taskTypeLabel"`
	Context           string            `json:"context"`
	Question          string            `json:"question"`
	RequiredData      []string          `json:"requiredData"`
	AnswerRequirement string            `json:"answerRequirement"`
	MaxScore          int               `json:"maxScore"`
	Rubric            []RubricDimRef    `json:"rubric"`
	Answer            string            `json:"answer,omitempty"`
	Answered          bool              `json:"answered"`
	Score             int               `json:"score"`
	ScoreLabel        string            `json:"scoreLabel"`
	RubricFeedback    []RubricFeedback  `json:"rubricFeedback"`
	ReferenceAnswer   string            `json:"referenceAnswer,omitempty"`
	ExpectedPoints    []string          `json:"expectedPoints,omitempty"`
	ScoredAt          string            `json:"scoredAt,omitempty"`
	Completed         bool              `json:"completed"`
	Disclaimer        string            `json:"disclaimer"`
	GenerationNote    string            `json:"generationNote"`
	Source            string            `json:"source"`
	SourceLabel       string            `json:"sourceLabel"`
	VariantCode       string            `json:"variantCode,omitempty"`
	FamilyLabel       string            `json:"familyLabel,omitempty"`
	ParamsUsed        map[string]string `json:"paramsUsed,omitempty"`
	SelectionReason   string            `json:"selectionReason"`
	DedupNote         string            `json:"dedupNote"`
	ComparedItems     []string          `json:"comparedItems"`
	SimilarityMax     float64           `json:"similarityMax"`
	MatchedKeywords   []string          `json:"matchedKeywords"`
	MissingPoints     []string          `json:"missingPoints"`
}

// GenerateInput 描述一次迁移题生成请求：当前任务类型、学生尚未解决的问题（以命中的问题类型表示）、
// 已经掌握的知识点、已使用过的题目与已经出现过的示例情境。
type GenerateInput struct {
	TaskType       string
	Archetypes     []string
	MasteredPoints []string
	FocusPoints    []string
	UsedItemIDs    []string
	UsedTopics     []string
	ExampleTexts   []string
	Seed           string
}

// GenerationInfo 记录生成过程，便于界面标注题目来源并接受复核（需求文档 5 节）。
type GenerationInfo struct {
	Source          string
	FamilyCode      string
	FamilyLabel     string
	VariantCode     string
	ParamsUsed      map[string]string
	SelectionReason string
	SimilarityMax   float64
	DedupNote       string
	ComparedItems   []string
	Attempts        int
	ReusedFallback  bool
}

const transferSourceLabels = "规则模板组合生成"

// GenerateTransfer 依据当前任务类型与学生薄弱点生成迁移题。
// 生成过程完全不使用大模型：情境与数据条件来自题目模板的参数组合，
// 并对演示案例、提示示例与历史题目做相似度检查，避免把示例改个标题当成新题。
func GenerateTransfer(in GenerateInput) (content.TransferItem, GenerationInfo) {
	families := content.FamiliesByTaskType(in.TaskType)
	if len(families) == 0 {
		families = content.FamiliesByTaskType(content.TaskGeneric)
	}
	used := map[string]bool{}
	for _, id := range in.UsedItemIDs {
		used[id] = true
	}
	usedTopicList := append([]string{}, in.UsedTopics...)
	exampleList := append([]string{}, in.ExampleTexts...)

	type candidate struct {
		item    content.TransferItem
		score   float64
		sim     float64
		simNote string
	}
	var best *candidate
	attempts := 0
	// 最多三轮：每轮更换参数组合，从而在同一模板上产生不同数据条件的新题。
	for pass := 0; pass < 3; pass++ {
		for _, fam := range families {
			combos := content.ParamCombinationCount(fam)
			for vi := range fam.Variants {
				paramIndex := (hashInt(in.Seed, 17) + pass*3) % max(combos, 1)
				item, ok := content.BuildVariantItem(fam, vi, paramIndex)
				if !ok {
					continue
				}
				attempts++
				sim, simNote := maxSimilarity(item, exampleList, usedTopicList)
				score := scoreCandidate(item, in, used, sim)
				// 同一轮内先到者优先，避免不同参数组合之间出现不稳定排序。
				score -= 0.01 * float64(pass)
				c := candidate{item: item, score: score, sim: sim, simNote: simNote}
				if best == nil || c.score > best.score {
					cp := c
					best = &cp
				}
			}
		}
		if best != nil && best.score > 0 {
			break
		}
	}
	// 题库回退：任务类型下没有任何可用模板时，仍然保证流程不中断，但明确标注来源。
	if best == nil {
		fallback := legacyFallback(in)
		attempts++
		sim, simNote := maxSimilarity(fallback, exampleList, usedTopicList)
		best = &candidate{item: fallback, score: -100, sim: sim, simNote: simNote}
	}
	info := GenerationInfo{
		Source:          best.item.Source,
		FamilyCode:      best.item.FamilyCode,
		FamilyLabel:     best.item.FamilyLabel,
		VariantCode:     best.item.VariantCode,
		ParamsUsed:      best.item.ParamsUsed,
		SimilarityMax:   best.sim,
		DedupNote:       best.simNote,
		ComparedItems:   comparedLabels(in),
		Attempts:        attempts,
		ReusedFallback:  false,
		SelectionReason: selectionReason(best.item, in, best.sim),
	}
	if best.item.Source == content.SourceFixedBank {
		info.ReusedFallback = true
	}
	return best.item, info
}

func scoreCandidate(item content.TransferItem, in GenerateInput, used map[string]bool, sim float64) float64 {
	score := 0.0
	// 命中的典型误区就是学生尚未解决的问题，与它们的重叠度是选题的首要依据。
	for _, a := range item.Archetypes {
		for _, m := range in.Archetypes {
			if a == m {
				score += 3
			}
		}
	}
	for _, f := range in.FocusPoints {
		if pointMatch(f, item.KnowledgePoint) {
			score += 1.5
		}
	}
	// 已经掌握的内容不应在迁移题里反复考查。
	for _, mp := range in.MasteredPoints {
		if pointMatch(mp, item.KnowledgePoint) {
			score -= 5
		}
	}
	// 与示例情境相同的题目必须降权：迁移题不能只是把示例换个标题。
	if item.ExampleScenario != "" {
		score -= 4
	}
	if used[item.ID] {
		score -= 6
	}
	if sim >= content.SimilarityThreshold {
		score -= 12
	}
	score -= sim
	return score
}

// pointMatch 是对知识点名称的宽松匹配：完全相等、互相包含，或字符二元组相似度达阈值。
func pointMatch(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	if a == b || strings.Contains(a, b) || strings.Contains(b, a) {
		return true
	}
	return content.Similarity(a, b) >= 0.3
}

func legacyFallback(in GenerateInput) content.TransferItem {
	best := content.TransferItems[0]
	bestOverlap := -1
	for _, item := range content.TransferItems {
		if item.TaskType != in.TaskType && in.TaskType != content.TaskGeneric {
			continue
		}
		overlap := 0
		for _, a := range item.Archetypes {
			for _, m := range in.Archetypes {
				if a == m {
					overlap++
				}
			}
		}
		if overlap > bestOverlap {
			best, bestOverlap = item, overlap
		}
	}
	best.Source = content.SourceFixedBank
	return best
}

func maxSimilarity(item content.TransferItem, exampleTexts, usedTopics []string) (float64, string) {
	text := item.Context + item.Question
	best, bestLabel := 0.0, ""
	for _, e := range exampleTexts {
		if s := content.Similarity(text, e); s > best {
			best, bestLabel = s, "示例情境"
		}
	}
	for _, t := range usedTopics {
		if t == "" || t == item.Topic {
			continue
		}
		if s := content.Similarity(text, t); s > best {
			best, bestLabel = s, "历史题目"
		}
	}
	note := fmt.Sprintf("已与 %d 条示例情境、%d 道历史题目比对，最高相似度 %.2f（阈值 %.2f）",
		len(exampleTexts), len(usedTopics), best, content.SimilarityThreshold)
	if best >= content.SimilarityThreshold {
		note += fmt.Sprintf("：与%s过于接近，已改为其它情境组合", bestLabel)
	} else {
		note += "：未发现与示例或历史题目的重复"
	}
	return best, note
}

func comparedLabels(in GenerateInput) []string {
	out := []string{}
	for i := range in.ExampleTexts {
		out = append(out, fmt.Sprintf("示例情境 %d", i+1))
	}
	for _, t := range in.UsedTopics {
		if t != "" {
			out = append(out, "历史题目："+t)
		}
	}
	return out
}

func selectionReason(item content.TransferItem, in GenerateInput, sim float64) string {
	taskLabel := content.TaskTypeLabel(in.TaskType)
	reason := fmt.Sprintf("按当前任务类型「%s」生成", taskLabel)
	if item.KnowledgePoint != "" {
		reason += fmt.Sprintf("，考查知识点「%s」", item.KnowledgePoint)
	}
	matched := []string{}
	for _, a := range item.Archetypes {
		for _, m := range in.Archetypes {
			if a == m {
				matched = append(matched, a)
			}
		}
	}
	if len(matched) > 0 {
		reason += fmt.Sprintf("；该题与你本次命中的问题类型（%s）直接相关", strings.Join(dedupe(matched), "、"))
	} else {
		reason += "；当前诊断未命中内置规则，因此按任务类型的默认目标出题"
	}
	if len(in.MasteredPoints) > 0 {
		reason += fmt.Sprintf("；已避开你认为已掌握的内容（%s）", strings.Join(firstN(in.MasteredPoints, 2), "、"))
	}
	if len(in.UsedItemIDs) > 0 {
		reason += fmt.Sprintf("；已比对本机保存的 %d 道历史题目", len(in.UsedItemIDs))
	}
	reason += fmt.Sprintf("。与示例情境及历史题目的最高相似度为 %.2f。", sim)
	return reason
}

// TransferMasteredPoints 汇总学生已经掌握的知识点，用于避免在迁移题中重复考查。
func TransferMasteredPoints(diag Diagnosis) []string {
	out := []string{}
	for _, k := range diag.Knowledge {
		if k.Status == ClaimMastered {
			out = append(out, k.Name)
		}
	}
	return dedupe(out)
}

// TransferFocusPoints 返回当前任务类型关注的知识点，作为迁移题的次要选择依据。
func TransferFocusPoints(taskType string) []string {
	return nonNil(content.TaskTypeOrDefault(taskType).FocusKnowledge)
}

// SelectTransferItem 依据命中的典型误区在固定题库中选择迁移题（保留为回退路径）。
// 正常情况下应使用 GenerateTransfer，由任务类型与学生薄弱点共同决定题目。
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

// NewTransferState 依据题目构造迁移测试状态。
func NewTransferState(item content.TransferItem) TransferState {
	rubric := make([]RubricDimRef, 0, len(item.Rubric))
	for _, d := range item.Rubric {
		rubric = append(rubric, RubricDimRef{Code: d.Code, Title: d.Title, MaxScore: d.MaxScore})
	}
	source := item.Source
	if source == "" {
		source = content.SourceFixedBank
	}
	return TransferState{
		ItemID:            item.ID,
		Topic:             item.Topic,
		Scenario:          item.Scenario,
		KnowledgePoint:    item.KnowledgePoint,
		TaskType:          item.TaskType,
		TaskTypeLabel:     content.TaskTypeLabel(item.TaskType),
		Context:           item.Context,
		Question:          item.Question,
		RequiredData:      nonNil(item.RequiredData),
		AnswerRequirement: item.AnswerRequirement,
		MaxScore:          item.MaxScore,
		Rubric:            rubric,
		ScoreLabel:        fmt.Sprintf("0 / %d", item.MaxScore),
		Disclaimer:        content.TransferDisclaimer,
		GenerationNote:    content.TransferGenerationNotice,
		Source:            source,
		SourceLabel:       transferSourceLabels,
		VariantCode:       item.VariantCode,
		FamilyLabel:       item.FamilyLabel,
		ParamsUsed:        item.ParamsUsed,
		RubricFeedback:    []RubricFeedback{},
		MatchedKeywords:   []string{},
		MissingPoints:     []string{},
	}
}

func nonNil(list []string) []string {
	if list == nil {
		return []string{}
	}
	return list
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

// ScoreTransferState 依据已保存的题目编号重建题目并评分，用于学生记录中只保存题目编号的场景。
func ScoreTransferState(state TransferState, answer string) (TransferState, bool) {
	item, ok := content.ResolveTransferItem(state.ItemID)
	if !ok {
		return state, false
	}
	scored := ScoreTransfer(item, answer)
	scored.SelectionReason = state.SelectionReason
	scored.DedupNote = state.DedupNote
	scored.ComparedItems = state.ComparedItems
	scored.SimilarityMax = state.SimilarityMax
	return scored, true
}

// RevealReference 在学生已作答后填充参考答案与期望要点。
func RevealReference(state TransferState) TransferState {
	item, ok := content.ResolveTransferItem(state.ItemID)
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

func hashInt(seed string, salt uint32) int {
	h := fnv.New32a()
	_, _ = h.Write([]byte(seed))
	_, _ = h.Write([]byte{byte(salt)})
	return int(h.Sum32() % 100000)
}

// SortedKeys 返回映射的排序键，便于输出可复现的参数说明。
func SortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
