package services

import (
	"fmt"
	"strings"
	"time"

	"knowbr/server/internal/content"
	"knowbr/server/internal/models"
)

// MasteryStatusRef 是掌握情况的对外结构。
type MasteryStatusRef struct {
	Status string `json:"status"`
	Label  string `json:"label"`
}

// TransferSummary 是学习摘要中的迁移表现。
type TransferSummary struct {
	ItemID          string           `json:"itemId"`
	Topic           string           `json:"topic"`
	Scenario        string           `json:"scenario"`
	TaskType        string           `json:"taskType"`
	TaskTypeLabel   string           `json:"taskTypeLabel"`
	KnowledgePoint  string           `json:"knowledgePoint"`
	Question        string           `json:"question"`
	Answer          string           `json:"answer"`
	Score           int              `json:"score"`
	MaxScore        int              `json:"maxScore"`
	Scored          bool             `json:"scored"`
	Completed       bool             `json:"completed"`
	Source          string           `json:"source"`
	SourceLabel     string           `json:"sourceLabel"`
	SelectionReason string           `json:"selectionReason"`
	DedupNote       string           `json:"dedupNote"`
	GenerationNote  string           `json:"generationNote"`
	RubricFeedback  []RubricFeedback `json:"rubricFeedback"`
	MatchedKeywords []string         `json:"matchedKeywords"`
	MissingPoints   []string         `json:"missingPoints"`
	ReferenceAnswer string           `json:"referenceAnswer,omitempty"`
}

// Summary 是需求文档功能 G 要求的结构化学习反馈。
type Summary struct {
	GeneratedAt         string           `json:"generatedAt"`
	LearningGoal        string           `json:"learningGoal"`
	InitialExplanation  string           `json:"initialExplanation"`
	KeyEvidence         []string         `json:"keyEvidence"`
	TaskType            string           `json:"taskType"`
	TaskTypeLabel       string           `json:"taskTypeLabel"`
	Claims              []Claim          `json:"claims"`
	Knowledge           []KnowledgeItem  `json:"knowledge"`
	StillOpen           []string         `json:"stillOpen"`
	Resolved            []string         `json:"resolved"`
	EvidenceLimits      []string         `json:"evidenceLimits"`
	MainIssue           string           `json:"mainIssue"`
	IssueEvidence       []string         `json:"issueEvidence"`
	DiagnosisReasoning  string           `json:"diagnosisReasoning"`
	Categories          []CategoryRef    `json:"categories"`
	HintLevels          []int            `json:"hintLevels"`
	HintTrail           []models.Hint    `json:"hintTrail"`
	RevisionSummary     string           `json:"revisionSummary"`
	RevisionImproved    bool             `json:"revisionImproved"`
	RevisionCount       int              `json:"revisionCount"`
	Transfer            *TransferSummary `json:"transfer,omitempty"`
	Mastery             MasteryStatusRef `json:"mastery"`
	StudentNextSteps    []string         `json:"studentNextSteps"`
	TeacherNotes        []string         `json:"teacherNotes"`
	TeacherSummary      string           `json:"teacherSummary"`
	Mode                string           `json:"mode"`
	ModeLabel           string           `json:"modeLabel"`
	LLMUsed             bool             `json:"llmUsed"`
	DataNotice          string           `json:"dataNotice"`
	Disclaimer          string           `json:"disclaimer"`
	TeacherIntervention bool             `json:"teacherIntervention"`
}

const summaryDisclaimer = "本反馈由内置规则引擎根据学生提交的材料生成（如已配置大模型，仅用于组织自然语言表达，不修改任何计算结果）。分数仅代表本次演示任务的规则化反馈，不是正式考试成绩。"

// SummaryInput 是构建学习摘要所需的全部输入。
type SummaryInput struct {
	Session       *models.Session
	Diagnosis     Diagnosis
	Metrics       *MatrixMetrics
	Hints         []models.Hint
	Revisions     []models.Revision
	Transfer      *TransferState
	HintState     models.HintState
	LLMUsed       bool
	ModeLabel     string
	DataNotice    string
	IsDemo        bool
	ArchetypeName string
}

// BuildSummary 生成结构化学习摘要。
func BuildSummary(in SummaryInput) Summary {
	s := Summary{
		GeneratedAt:         nowISO(),
		LearningGoal:        boldGoal(in.Session),
		InitialExplanation:  in.Session.StudentInitialExplanation,
		KeyEvidence:         nonEmptyStrings(in.Diagnosis.Evidence),
		MainIssue:           in.Diagnosis.MainIssue,
		IssueEvidence:       issueEvidence(in.Diagnosis),
		DiagnosisReasoning:  in.Diagnosis.Reasoning,
		Categories:          nonEmptyCategories(in.Diagnosis.Categories),
		HintTrail:           nonEmptyHints(in.Hints),
		HintLevels:          []int{},
		RevisionCount:       len(in.Revisions),
		Mastery:             MasteryStatusRef{Status: in.Diagnosis.MasteryStatus, Label: MasteryLabel(in.Diagnosis.MasteryStatus)},
		Mode:                in.Session.Mode,
		ModeLabel:           in.ModeLabel,
		LLMUsed:             in.LLMUsed,
		TeacherIntervention: in.HintState.TeacherIntervention || in.Diagnosis.TeacherIntervention,
		Disclaimer:          summaryDisclaimer,
		StudentNextSteps:    []string{},
		TeacherNotes:        []string{},
	}
	for _, h := range in.Hints {
		s.HintLevels = append(s.HintLevels, h.Level)
	}
	s.RevisionSummary, s.RevisionImproved = revisionSummary(in.Revisions)
	if len(in.Revisions) > 0 {
		last := in.Revisions[len(in.Revisions)-1]
		if last.Progress == "improved" || last.Progress == "partial" {
			s.Mastery.Status = MasteryPartial
			if last.Progress == "improved" && in.Transfer != nil && in.Transfer.Answered && in.Transfer.MaxScore > 0 && in.Transfer.Score == in.Transfer.MaxScore {
				s.Mastery.Status = MasteryDemonstrated
			}
			s.Mastery.Label = MasteryLabel(s.Mastery.Status)
		}
	}
	if in.Transfer != nil {
		ts := &TransferSummary{
			ItemID:          in.Transfer.ItemID,
			Topic:           in.Transfer.Topic,
			Scenario:        in.Transfer.Scenario,
			TaskType:        in.Transfer.TaskType,
			TaskTypeLabel:   in.Transfer.TaskTypeLabel,
			KnowledgePoint:  in.Transfer.KnowledgePoint,
			Question:        in.Transfer.Question,
			Answer:          in.Transfer.Answer,
			Score:           in.Transfer.Score,
			MaxScore:        in.Transfer.MaxScore,
			Scored:          in.Transfer.Answered,
			Completed:       in.Transfer.Completed,
			Source:          in.Transfer.Source,
			SourceLabel:     in.Transfer.SourceLabel,
			SelectionReason: in.Transfer.SelectionReason,
			DedupNote:       in.Transfer.DedupNote,
			GenerationNote:  in.Transfer.GenerationNote,
			RubricFeedback:  nonEmptyRubricFeedback(in.Transfer.RubricFeedback),
			MatchedKeywords: nonEmptyStrings(in.Transfer.MatchedKeywords),
			MissingPoints:   nonEmptyStrings(in.Transfer.MissingPoints),
		}
		if in.Transfer.Answered {
			ts.ReferenceAnswer = in.Transfer.ReferenceAnswer
		}
		s.Transfer = ts
	}
	if in.IsDemo {
		s.DataNotice = in.DataNotice
	}
	s.TaskType = in.Diagnosis.TaskType
	s.TaskTypeLabel = in.Diagnosis.TaskTypeLabel
	s.Claims = nonEmptyClaims(in.Diagnosis.Claims)
	s.Knowledge = nonEmptyKnowledge(in.Diagnosis.Knowledge)
	// 只记录真实发生的过程：哪些主张仍未解决、哪些已解决、哪些材料确实缺失。
	for _, c := range s.Claims {
		switch c.Status {
		case ClaimError, ClaimPartial, ClaimInsufficient:
			s.StillOpen = append(s.StillOpen, fmt.Sprintf("%s（%s）：%s", c.KnowledgePoint, c.StatusLabel, c.Judgment))
		case ClaimMastered:
			s.Resolved = append(s.Resolved, c.KnowledgePoint)
		}
	}
	for _, need := range in.Diagnosis.EvidenceNeeds {
		if need.Present {
			continue
		}
		kind := "可选材料"
		if need.Required {
			kind = "必需材料"
		}
		s.EvidenceLimits = append(s.EvidenceLimits, fmt.Sprintf("%s（%s）：%s", need.Name, kind, need.Impact))
	}
	s.StudentNextSteps = studentNextSteps(in)
	s.TeacherNotes = teacherNotes(in)
	s.TeacherSummary = teacherSummary(&s, in)
	return s
}

func boldGoal(sess *models.Session) string {
	if strings.TrimSpace(sess.ExperimentGoal) != "" {
		return sess.ExperimentGoal
	}
	return fmt.Sprintf("完成《%s》并能够解释实验结果、判断模型局限。", sess.ExperimentTitle)
}

func issueEvidence(d Diagnosis) []string {
	out := []string{}
	if d.Contradiction.Detected {
		out = append(out, d.Contradiction.Detail)
	}
	out = append(out, d.MatrixFacts...)
	return out
}

func nonEmptyStrings(in []string) []string {
	if in == nil {
		return []string{}
	}
	return in
}

func nonEmptyClaims(in []Claim) []Claim {
	if in == nil {
		return []Claim{}
	}
	return in
}

func nonEmptyKnowledge(in []KnowledgeItem) []KnowledgeItem {
	if in == nil {
		return []KnowledgeItem{}
	}
	return in
}

func nonEmptyCategories(in []CategoryRef) []CategoryRef {
	if in == nil {
		return []CategoryRef{}
	}
	return in
}

func nonEmptyHints(in []models.Hint) []models.Hint {
	if in == nil {
		return []models.Hint{}
	}
	return in
}

func nonEmptyRubricFeedback(in []RubricFeedback) []RubricFeedback {
	if in == nil {
		return []RubricFeedback{}
	}
	return in
}

func revisionSummary(revisions []models.Revision) (string, bool) {
	if len(revisions) == 0 {
		return "学生尚未提交修正后的解释。", false
	}
	last := revisions[len(revisions)-1]
	improved := false
	for _, r := range revisions {
		if r.Progress == "improved" {
			improved = true
		}
	}
	label := map[string]string{"improved": "有所修正", "partial": "部分修正", "none": "未见到修正"}[last.Progress]
	return fmt.Sprintf("共提交 %d 次修正，最近一次判定为「%s」：%s", len(revisions), label, last.Feedback), improved
}

func studentNextSteps(in SummaryInput) []string {
	steps := []string{}
	for _, c := range in.Diagnosis.Claims {
		switch c.Status {
		case ClaimError, ClaimPartial, ClaimInsufficient:
			steps = append(steps, fmt.Sprintf("仍未解决：%s —— %s", c.KnowledgePoint, c.NextQuestion))
		}
	}
	if in.Transfer != nil && in.Transfer.Answered {
		if len(in.Transfer.MissingPoints) > 0 {
			steps = append(steps, "迁移测试中仍不足的方面："+strings.Join(in.Transfer.MissingPoints, "、")+"，建议针对性重做一次。")
		} else {
			steps = append(steps, "迁移测试的评分维度均已覆盖，可以尝试更极端的情境（如某一类只有个位数样本）。")
		}
	} else {
		steps = append(steps, "完成迁移测试后再结束本次学习，以便检验知识是否可迁移。")
	}
	steps = append(steps, "复习要点："+knowledgePoint(in)+"。")
	if in.Metrics != nil && len(in.Metrics.ZeroRecallClass) > 0 {
		steps = append(steps, fmt.Sprintf("重新计算 %s 的召回率与精确率，并说明在你的任务场景中这意味着什么风险。", strings.Join(in.Metrics.ZeroRecallClass, "、")))
	}
	return steps
}

func knowledgePoint(in SummaryInput) string {
	if a, ok := archetypeByCode(in.Diagnosis.PrimaryArchetype); ok {
		return a.KnowledgePoint
	}
	// 没有命中规则时，回到当前任务类型自己的知识点，而不是默认分类指标。
	if tt, ok := content.TaskTypeByCode(in.Diagnosis.TaskType); ok && len(tt.FocusKnowledge) > 0 {
		return strings.Join(firstN(tt.FocusKnowledge, 3), "、")
	}
	return "当前实验目标所要求的关键知识点"
}

func teacherNotes(in SummaryInput) []string {
	notes := []string{}
	if in.Diagnosis.Contradiction.Detected {
		notes = append(notes, "学生陈述与混淆矩阵证据矛盾，建议确认其是否理解矩阵行（真实类别）与列（预测类别）的方向约定。")
	}
	if in.HintState.TeacherIntervention {
		notes = append(notes, in.HintState.TeacherReason)
	}
	if in.Diagnosis.PrimaryArchetype != "" && in.Diagnosis.PrimaryArchetype != "NONE" {
		if a, ok := archetypeByCode(in.Diagnosis.PrimaryArchetype); ok {
			notes = append(notes, fmt.Sprintf("命中典型误区 %s（%s）：%s", a.Code, a.Name, a.TeacherCondition))
		}
	}
	if in.Diagnosis.Completeness.Sufficient == false {
		notes = append(notes, "学生提交的材料不完整，诊断结论仅覆盖现有证据。")
	}
	if len(notes) == 0 {
		notes = append(notes, "本次未触发需要教师介入的条件；如后续多份记录出现同一误区，可考虑集中讲解。")
	}
	return notes
}

func teacherSummary(s *Summary, in SummaryInput) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("【学习摘要｜%s】\n", in.Session.ExperimentTitle))
	b.WriteString(fmt.Sprintf("任务类型：%s\n", s.TaskTypeLabel))
	b.WriteString(fmt.Sprintf("学习目标：%s\n", s.LearningGoal))
	b.WriteString(fmt.Sprintf("主要误区：%s\n", s.MainIssue))
	if len(s.IssueEvidence) > 0 {
		b.WriteString("关键证据：" + strings.Join(firstN(s.IssueEvidence, 3), " ") + "\n")
	}
	if len(s.HintLevels) > 0 {
		b.WriteString(fmt.Sprintf("已提供提示：%s 级\n", levelsText(s.HintLevels)))
	} else {
		b.WriteString("已提供提示：无\n")
	}
	b.WriteString("修正情况：" + s.RevisionSummary + "\n")
	if len(s.StillOpen) > 0 {
		b.WriteString("仍未解决：" + strings.Join(firstN(s.StillOpen, 3), "；") + "\n")
	} else if len(s.Claims) > 0 {
		b.WriteString("仍未解决：无（本次提交的主张均已解决）\n")
	}
	if len(s.EvidenceLimits) > 0 {
		b.WriteString("材料限制：" + strings.Join(firstN(s.EvidenceLimits, 2), "；") + "\n")
	}
	if s.Transfer != nil && s.Transfer.Scored {
		b.WriteString(fmt.Sprintf("迁移题表现：%d/%d 分（%s，任务类型：%s）。", s.Transfer.Score, s.Transfer.MaxScore, s.Transfer.Topic, s.Transfer.TaskTypeLabel))
		if s.Transfer.SourceLabel != "" {
			b.WriteString("题目来源：" + s.Transfer.SourceLabel + "。")
		}
		if len(s.Transfer.MissingPoints) > 0 {
			b.WriteString("仍不足：" + strings.Join(s.Transfer.MissingPoints, "、") + "。")
		}
		b.WriteString("\n")
	} else if s.Transfer != nil {
		b.WriteString("迁移题表现：已生成题目但尚未作答，不产生评分结论。\n")
	} else {
		b.WriteString("迁移题表现：尚未完成\n")
	}
	b.WriteString(fmt.Sprintf("当前掌握情况：%s\n", s.Mastery.Label))
	b.WriteString("建议教师关注：" + strings.Join(s.TeacherNotes, " / ") + "\n")
	b.WriteString(fmt.Sprintf("运行模式：%s（大模型实际参与：%t）\n", s.ModeLabel, s.LLMUsed))
	if s.DataNotice != "" {
		b.WriteString("数据说明：" + s.DataNotice + "\n")
	}
	return b.String()
}

func levelsText(levels []int) string {
	seen := map[int]bool{}
	var parts []string
	for _, l := range levels {
		if !seen[l] {
			seen[l] = true
			parts = append(parts, fmt.Sprintf("%d", l))
		}
	}
	return strings.Join(parts, "、")
}

// SummaryMarkdown 把学习摘要导出为 Markdown 文本。
func SummaryMarkdown(s Summary) string {
	var b strings.Builder
	b.WriteString("# 知行桥·学习反馈摘要\n\n")
	b.WriteString(fmt.Sprintf("- 生成时间：%s\n- 运行模式：%s（大模型实际参与：%t）\n\n", s.GeneratedAt, s.ModeLabel, s.LLMUsed))

	b.WriteString("## 一、本次学习目标\n" + s.LearningGoal + "\n\n")
	b.WriteString("## 二、学生提交的关键证据\n")
	b.WriteString("初始解释：" + s.InitialExplanation + "\n\n")
	for _, e := range s.KeyEvidence {
		b.WriteString("- " + e + "\n")
	}
	if len(s.EvidenceLimits) > 0 {
		b.WriteString("\n### 材料限制（缺失或可补充的部分）\n")
		for _, e := range s.EvidenceLimits {
			b.WriteString("- " + e + "\n")
		}
	}
	b.WriteString("\n## 三、主要误区\n" + s.MainIssue + "\n\n")
	b.WriteString("诊断理由：" + s.DiagnosisReasoning + "\n\n")
	if len(s.Claims) > 0 {
		b.WriteString("### 逐条主张诊断\n")
		for _, c := range s.Claims {
			b.WriteString(fmt.Sprintf("- [%s] %s\n  - 学生证据：%s\n  - 判断：%s；原因：%s\n  - 证据边界：%s\n  - 下一步追问：%s\n",
				c.StatusLabel, c.KnowledgePoint, c.Evidence, c.Judgment, c.Reason, c.EvidenceBoundary, c.NextQuestion))
		}
		b.WriteString("\n")
	}
	if len(s.Knowledge) > 0 {
		b.WriteString("### 知识点状态\n")
		for _, k := range s.Knowledge {
			b.WriteString(fmt.Sprintf("- %s：%s（%s）\n", k.Name, k.StatusLabel, k.Note))
		}
		b.WriteString("\n")
	}
	if len(s.IssueEvidence) > 0 {
		b.WriteString("支撑证据：\n")
		for _, e := range s.IssueEvidence {
			b.WriteString("- " + e + "\n")
		}
		b.WriteString("\n")
	}
	b.WriteString("## 四、已使用的提示等级\n")
	if len(s.HintTrail) == 0 {
		b.WriteString("未使用提示。\n\n")
	} else {
		for _, h := range s.HintTrail {
			b.WriteString(fmt.Sprintf("- 第 %d 级提示（%s）：%s\n", h.Level, h.CreatedAt, strings.ReplaceAll(h.Content, "\n", " ")))
		}
		b.WriteString("\n")
	}
	b.WriteString("## 五、学生修正情况\n" + s.RevisionSummary + "\n\n")
	b.WriteString("## 六、迁移测试\n")
	if s.Transfer == nil {
		b.WriteString("尚未进入迁移测试。\n\n")
	} else {
		b.WriteString("题目：" + s.Transfer.Question + "\n\n")
		if s.Transfer.KnowledgePoint != "" {
			b.WriteString("考查知识点：" + s.Transfer.KnowledgePoint + "\n\n")
		}
		if s.Transfer.SourceLabel != "" {
			b.WriteString("题目来源：" + s.Transfer.SourceLabel + "（" + s.Transfer.Source + "）\n\n")
		}
		if s.Transfer.SelectionReason != "" {
			b.WriteString("选题依据：" + s.Transfer.SelectionReason + "\n\n")
		}
		if s.Transfer.DedupNote != "" {
			b.WriteString("重复检查：" + s.Transfer.DedupNote + "\n\n")
		}
		b.WriteString("答案：" + s.Transfer.Answer + "\n\n")
		for _, f := range s.Transfer.RubricFeedback {
			b.WriteString(fmt.Sprintf("- %s：%d/%d —— %s\n", f.Title, f.Score, f.MaxScore, f.Reason))
		}
		b.WriteString(fmt.Sprintf("\n合计：%d/%d\n\n", s.Transfer.Score, s.Transfer.MaxScore))
		if s.Transfer.ReferenceAnswer != "" {
			b.WriteString("参考答案：" + s.Transfer.ReferenceAnswer + "\n\n")
		}
	}
	b.WriteString(fmt.Sprintf("## 七、当前掌握情况\n%s\n\n", s.Mastery.Label))
	b.WriteString("## 八、建议学生继续学习的内容\n")
	for _, t := range s.StudentNextSteps {
		b.WriteString("- " + t + "\n")
	}
	b.WriteString("\n## 九、建议教师进一步关注的问题\n")
	for _, t := range s.TeacherNotes {
		b.WriteString("- " + t + "\n")
	}
	b.WriteString("\n## 十、说明\n" + s.Disclaimer + "\n")
	if s.DataNotice != "" {
		b.WriteString(s.DataNotice + "\n")
	}
	return b.String()
}

func timeNowISO() string { return time.Now().Format(time.RFC3339) }

// CategoryRefsFromCSV 把存储的类别 CSV 还原为类别引用。
func CategoryRefsFromCSV(csv string) []CategoryRef {
	if strings.TrimSpace(csv) == "" {
		return nil
	}
	out := []CategoryRef{}
	for _, code := range strings.Split(csv, ",") {
		code = strings.TrimSpace(code)
		if code == "" {
			continue
		}
		out = append(out, CategoryRef{Code: code, Label: content.CategoryLabel(code)})
	}
	return out
}

// CategoriesCSV 把类别列表序列化为逗号分隔文本。
func CategoriesCSV(list []CategoryRef) string {
	parts := make([]string, 0, len(list))
	for _, c := range list {
		parts = append(parts, c.Code)
	}
	return strings.Join(parts, ",")
}
