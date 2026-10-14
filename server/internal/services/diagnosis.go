package services

import (
	"fmt"
	"regexp"
	"strings"

	"knowbr/server/internal/content"
	"knowbr/server/internal/models"
)

// CategoryRef 是诊断类别引用。
type CategoryRef struct {
	Code  string `json:"code"`
	Label string `json:"label"`
}

// ArchetypeRef 是命中的典型误区引用。
type ArchetypeRef struct {
	Code           string `json:"code"`
	Name           string `json:"name"`
	KnowledgePoint string `json:"knowledgePoint"`
}

// Contradiction 描述学生陈述与提交证据相互矛盾的情况。
type Contradiction struct {
	Detected bool   `json:"detected"`
	Claim    string `json:"claim"`
	Detail   string `json:"detail"`
}

// Completeness 是材料完整性检查结果（需求文档功能 C）。
type Completeness struct {
	Sufficient          bool     `json:"sufficient"`
	HaveExplanation     bool     `json:"haveExplanation"`
	HaveMatrix          bool     `json:"haveMatrix"`
	HavePerClassMetrics bool     `json:"havePerClassMetrics"`
	HaveDistribution    bool     `json:"haveDistribution"`
	HaveTaskGoal        bool     `json:"haveTaskGoal"`
	HaveRunEvidence     bool     `json:"haveRunEvidence"`
	Missing             []string `json:"missing"`
	Limitations         []string `json:"limitations"`
}

// Diagnosis 是诊断结果（需求文档功能 D）。
type Diagnosis struct {
	TaskType            string          `json:"taskType"`
	TaskTypeLabel       string          `json:"taskTypeLabel"`
	TaskFocus           []string        `json:"taskFocus"`
	TaskOffTopic        []string        `json:"taskOffTopic"`
	Categories          []CategoryRef   `json:"categories"`
	PrimaryCategory     string          `json:"primaryCategory"`
	PrimaryCategoryName string          `json:"primaryCategoryName"`
	MainIssue           string          `json:"mainIssue"`
	Evidence            []string        `json:"evidence"`
	EvidenceSource      []string        `json:"evidenceSource"` // 每条证据来自哪里：学生输入 / 混淆矩阵计算 / 运行日志
	EvidenceNeeds       []EvidenceNeed  `json:"evidenceNeeds"`  // 必需与可选材料的区分
	Claims              []Claim         `json:"claims"`         // 逐个学生主张的诊断对象
	Knowledge           []KnowledgeItem `json:"knowledge"`      // 知识点状态（含未涉及/不适用）
	PrimaryClaimID      string          `json:"primaryClaimId"`
	Reasoning           string          `json:"reasoning"`
	MasteryStatus       string          `json:"masteryStatus"`
	MasteryLabel        string          `json:"masteryLabel"`
	NextStep            string          `json:"nextStep"`
	MatchedArchetypes   []ArchetypeRef  `json:"matchedArchetypes"`
	PrimaryArchetype    string          `json:"primaryArchetype"`
	Contradiction       Contradiction   `json:"contradiction"`
	TeacherIntervention bool            `json:"teacherIntervention"`
	TeacherReason       string          `json:"teacherReason"`
	Completeness        Completeness    `json:"completeness"`
	MatrixFacts         []string        `json:"matrixFacts"`
	GeneratedAt         string          `json:"generatedAt"`
	LLMUsed             bool            `json:"llmUsed"`
	LLMNote             string          `json:"llmNote"`
	Source              string          `json:"source"` // rule | llm（若大模型可用则标注其实际参与情况）
}

// DiagnoseInput 是诊断所需的学生材料。
type DiagnoseInput struct {
	ExperimentTitle    string
	ExperimentGoal     string
	ModelName          string
	Explanation        string
	Question           string
	DatasetDescription string
	MetricsText        string
	ExecutionLog       string
	CodeSnippet        string
	HaveMatrix         bool
	Labels             []string
	Metrics            *MatrixMetrics
	DemoCaseID         string
}

// Mastery 常量对应需求文档中的掌握情况枚举。
const (
	MasteryDemonstrated = "demonstrated"
	MasteryPartial      = "partial"
	MasteryNotShown     = "not_demonstrated"
	MasteryInsufficient = "insufficient_evidence"
)

var masteryLabels = map[string]string{
	MasteryDemonstrated: "已体现理解",
	MasteryPartial:      "部分理解",
	MasteryNotShown:     "尚未体现",
	MasteryInsufficient: "证据不足",
}

// MasteryLabel 返回掌握情况的中文标签。
func MasteryLabel(status string) string {
	if l, ok := masteryLabels[status]; ok {
		return l
	}
	return status
}

// Diagnose 依据学生材料与计算结果生成诊断（规则引擎，不依赖外部模型）。
func Diagnose(in DiagnoseInput) Diagnosis {
	taskType := DetectTaskType(in)
	tt := content.TaskTypeOrDefault(taskType)
	completeness := CheckCompleteness(in, taskType)
	text := in.Explanation + " " + in.Question

	matches := matchArchetypes(text, in.Metrics, in.DatasetDescription, taskType)
	contra := detectContradiction(text, in.Metrics)
	// 矛盾场景会插入「混淆矩阵读数」规则，后续统一使用 matched。
	matched := matches
	diag := Diagnosis{
		GeneratedAt:       nowISO(),
		Source:            "rule",
		Completeness:      completeness,
		Evidence:          []string{},
		EvidenceSource:    []string{},
		Categories:        []CategoryRef{},
		MatchedArchetypes: []ArchetypeRef{},
		MatrixFacts:       MatrixFacts(in.Metrics),
		TaskType:          taskType,
		TaskTypeLabel:     tt.Label,
		TaskFocus:         nonNil(tt.FocusKnowledge),
		TaskOffTopic:      nonNil(tt.OffTopic),
		EvidenceNeeds:     BuildEvidenceNeeds(in, taskType),
		Claims:            []Claim{},
		Knowledge:         []KnowledgeItem{},
		Contradiction:     contra,
	}

	if in.Metrics != nil {
		diag.Evidence = append(diag.Evidence, MatrixFacts(in.Metrics)...)
		for range MatrixFacts(in.Metrics) {
			diag.EvidenceSource = append(diag.EvidenceSource, "混淆矩阵计算")
		}
	}
	quotes, quoteSources := explanationQuotes(in.Explanation)
	diag.Evidence = append(diag.Evidence, quotes...)
	diag.EvidenceSource = append(diag.EvidenceSource, quoteSources...)
	// 训练稳定性任务只引用日志中真实出现的数值行，不引用任何未提交的数据。
	if taskType == content.TaskTraining {
		facts := logFacts(in.ExecutionLog)
		diag.Evidence = append(diag.Evidence, facts...)
		for range facts {
			diag.EvidenceSource = append(diag.EvidenceSource, "运行日志")
		}
	}
	if in.MetricsText != "" {
		diag.Evidence = append(diag.Evidence, fmt.Sprintf("学生填写的评价指标：%s", firstLine(in.MetricsText)))
		diag.EvidenceSource = append(diag.EvidenceSource, "学生输入")
	}

	// 0) 环境/代码异常优先处理：直接给出可执行的排障步骤，不用苏格拉底式提问。
	if env, ok := matchArchetypeByCode(text, in.Metrics, in.DatasetDescription, taskType, "E0"); ok {
		diag.Categories = []CategoryRef{{content.CategoryEnvironment, content.CategoryLabel(content.CategoryEnvironment)}}
		diag.PrimaryCategory = content.CategoryEnvironment
		diag.PrimaryCategoryName = content.CategoryLabel(content.CategoryEnvironment)
		diag.MatchedArchetypes = []ArchetypeRef{{Code: env.Code, Name: env.Name, KnowledgePoint: env.KnowledgePoint}}
		diag.PrimaryArchetype = env.Code
		diag.MainIssue = buildMainIssue(in, taskType, &env, Contradiction{}, nil)
		diag.Reasoning = env.DiagnosisReason
		diag.MasteryStatus = MasteryInsufficient
		diag.MasteryLabel = "证据不足（本次为环境排障，未评估概念掌握）"
		diag.NextStep = env.NextStep
		if log := lastLine(in.ExecutionLog); log != "" {
			diag.Evidence = append(diag.Evidence, "运行日志末行："+log)
			diag.EvidenceSource = append(diag.EvidenceSource, "学生输入")
		}
		return diag
	}

	// 1) 证据不足优先：不能凭一句准确率下结论。
	if !completeness.Sufficient {
		diag.Categories = []CategoryRef{{content.CategoryInsufficient, content.CategoryLabel(content.CategoryInsufficient)}}
		diag.PrimaryCategory = content.CategoryInsufficient
		diag.PrimaryCategoryName = content.CategoryLabel(content.CategoryInsufficient)
		diag.MainIssue = insufficientMainIssue(in, completeness, taskType)
		diag.Reasoning = insufficientReasoning(in, completeness, taskType)
		diag.MasteryStatus = MasteryInsufficient
		diag.NextStep = insufficientNextStep(completeness, taskType)
		diag.PrimaryArchetype = "NONE"
		diag.MasteryLabel = MasteryLabel(diag.MasteryStatus)
		claims := AnalyzeClaims(in, taskType, Contradiction{})
		diag.Claims = claims
		diag.Knowledge = BuildKnowledge(taskType, claims)
		diag.PrimaryClaimID = primaryClaimID(claims)
		return diag
	}

	// 陈述与证据矛盾：即使没有命中其它规则，也按“混淆矩阵读数”归类，才能给出针对性的提示与迁移题。
	diag.Contradiction = contra
	if diag.Contradiction.Detected {
		if e2, ok := archetypeByCode("E2"); ok {
			already := false
			for _, m := range matched {
				if m.Code == "E2" {
					already = true
				}
			}
			if !already {
				matched = append([]content.Archetype{e2}, matched...)
			}
		}
	}
	for _, a := range matched {
		diag.MatchedArchetypes = append(diag.MatchedArchetypes, ArchetypeRef{Code: a.Code, Name: a.Name, KnowledgePoint: a.KnowledgePoint})
	}
	for _, a := range matched {
		for _, c := range a.Categories {
			if !hasCategory(diag.Categories, c) {
				diag.Categories = append(diag.Categories, CategoryRef{Code: c, Label: content.CategoryLabel(c)})
			}
		}
	}

	// 逐个学生主张拆分：每个主张单独给出判断、理由、证据边界与下一步追问。
	claims := AnalyzeClaims(in, taskType, diag.Contradiction)
	diag.Claims = claims
	diag.Knowledge = BuildKnowledge(taskType, claims)
	diag.PrimaryClaimID = primaryClaimID(claims)

	var primary *content.Archetype
	if len(matched) > 0 {
		primary = &matched[0]
		diag.PrimaryArchetype = primary.Code
	}

	diag.MainIssue = buildMainIssue(in, taskType, primary, diag.Contradiction, claims)
	if primary != nil {
		diag.Reasoning = primary.DiagnosisReason
		if diag.Contradiction.Detected {
			diag.Reasoning = diag.Contradiction.Detail + " " + diag.Reasoning
		}
	} else {
		diag.Reasoning = genericReasoning(in, diag.Contradiction)
	}
	diag.MasteryStatus = judgeMasteryFromClaims(claims)
	if diag.Contradiction.Detected {
		diag.MasteryStatus = MasteryNotShown
	}
	diag.MasteryLabel = MasteryLabel(diag.MasteryStatus)
	diag.NextStep = buildNextStep(in, taskType, primary, diag.Contradiction, claims)

	// 教师介入：陈述与证据矛盾、或存在零召回类别却完全没有被提及，需要教师关注。
	if diag.Contradiction.Detected {
		diag.TeacherIntervention = true
		diag.TeacherReason = "学生陈述与提交的混淆矩阵明显矛盾，建议教师确认其是否理解矩阵行列含义。"
	} else if primary != nil && in.Metrics != nil && len(in.Metrics.ZeroRecallClass) > 0 {
		diag.TeacherReason = primary.TeacherCondition
	}

	if len(diag.Categories) == 0 {
		diag.Categories = []CategoryRef{{content.CategoryBoundary, content.CategoryLabel(content.CategoryBoundary)}}
	}
	if diag.PrimaryCategory == "" {
		diag.PrimaryCategory = diag.Categories[0].Code
		diag.PrimaryCategoryName = diag.Categories[0].Label
	}
	if diag.PrimaryCategory == "" || diag.PrimaryCategoryName == "" {
		diag.PrimaryCategory = diag.Categories[0].Code
		diag.PrimaryCategoryName = diag.Categories[0].Label
	}
	return diag
}

// CheckCompleteness 检查材料是否足以开展诊断（需求文档功能 C）。
func CheckCompleteness(in DiagnoseInput, taskType string) Completeness {
	c := Completeness{
		HaveExplanation: strings.TrimSpace(in.Explanation) != "",
		HaveMatrix:      in.HaveMatrix,
		HaveTaskGoal:    strings.TrimSpace(in.ExperimentGoal) != "",
		HaveRunEvidence: strings.TrimSpace(in.ExecutionLog) != "" || strings.TrimSpace(in.CodeSnippet) != "",
		Missing:         []string{},
		Limitations:     []string{},
	}
	c.HavePerClassMetrics = in.HaveMatrix || containsAny(in.MetricsText, []string{"召回", "recall", "精确", "precision", "f1", "F1", "每类", "逐类", "各类", "混淆矩阵"})
	c.HaveDistribution = in.HaveMatrix || containsAny(in.DatasetDescription, []string{"类", "样本", "条", "占比", "分布", "比例"})
	if !c.HaveExplanation {
		c.Missing = append(c.Missing, "学生自己的实验解释（必填）")
	}
	// 混淆矩阵与类别分布属于分类类任务的必需材料，对训练稳定性/结构比较任务则不是。
	needsMatrix := taskType == content.TaskClassification || taskType == content.TaskGeneric
	if !c.HaveMatrix && needsMatrix {
		c.Missing = append(c.Missing, "混淆矩阵或按类别的指标")
		c.Limitations = append(c.Limitations, "本次未提供混淆矩阵，系统无法逐类计算召回率与精确率，以下判断只能基于你填写的文字信息。")
	} else if !c.HaveMatrix {
		c.Limitations = append(c.Limitations, "本任务不需要混淆矩阵或按类别的指标，缺失这些材料不构成本次诊断的缺口。")
	}
	if !c.HaveDistribution && needsMatrix {
		c.Missing = append(c.Missing, "数据集或类别分布")
	}
	if !c.HaveTaskGoal {
		c.Missing = append(c.Missing, "实验目标（判断结论是否成立需要任务目标）")
		c.Limitations = append(c.Limitations, "未提供任务目标，只能结合常见任务类型讨论，不能断言你的判断一定正确或错误。")
	}
	if !c.HaveRunEvidence {
		c.Limitations = append(c.Limitations, "未提供运行日志或代码片段，系统没有运行你的代码，也不对你的代码质量作判断。")
	}
	// 材料是否“必需”按当前任务类型判断：训练稳定性任务不需要混淆矩阵与分类指标。
	switch taskType {
	case content.TaskTraining:
		c.Sufficient = c.HaveExplanation && c.HaveRunEvidence
	case content.TaskArchitecture:
		c.Sufficient = c.HaveExplanation && (c.HaveTaskGoal || c.HaveDistribution) && (c.HaveRunEvidence || c.HavePerClassMetrics)
	default:
		// 能讨论结果解释的前提：有解释，且有矩阵，或者既有数据分布又有任务背景/运行证据。
		// 只有一句准确率、连数据分布都没有时，仍然判定为证据不足（需求文档测试 2）。
		c.Sufficient = c.HaveExplanation && (c.HaveMatrix || (c.HaveDistribution && (c.HavePerClassMetrics || c.HaveTaskGoal || c.HaveRunEvidence)))
	}
	if !c.HaveMatrix && c.Sufficient && taskType == content.TaskClassification {
		c.Limitations = append(c.Limitations, "本次诊断不含混淆矩阵，无法逐类计算召回率与精确率，只针对解释的完整性与推理依据给出反馈。")
	}
	return c
}

func insufficientMainIssue(in DiagnoseInput, c Completeness, taskType string) string {
	if !c.HaveExplanation {
		return "缺少你自己的实验解释，暂时无法判断你的理解情况。"
	}
	switch taskType {
	case content.TaskTraining:
		return "本任务的必需证据还没到位：没有逐轮训练日志或实验设置，无法判断损失与梯度是否异常、异常从哪一轮开始。"
	case content.TaskArchitecture:
		return "本任务的必需证据还没到位：只有单一结果，没有两个模型的结构说明，也没有训练集与测试集上的表现，无法比较结构差异带来的影响。"
	}
	metrics := strings.TrimSpace(in.MetricsText)
	if metrics == "" {
		metrics = "（未填写指标）"
	}
	return fmt.Sprintf("目前只有“%s”这一条结果，信息不足以判断模型好坏：既没有类别分布，也没有按类别分解的指标。", firstLine(metrics))
}

// insufficientReasoning 说明当前缺什么、它限制了什么结论。
func insufficientReasoning(in DiagnoseInput, c Completeness, taskType string) string {
	switch taskType {
	case content.TaskTraining:
		return "本任务需要的是训练过程中的证据（逐轮损失、梯度范数等），不需要混淆矩阵或分类指标。现有材料不足以判断训练是否真正异常，也不能确定异常的原因，因此不做任何根因结论。"
	case content.TaskArchitecture:
		return "本任务需要的是结构说明与训练/测试集上的对照结果。现有材料缺少必要证据，因此不能判断准确率差异是否来自结构差异，也不把分类别指标当作本任务的缺口。"
	}
	return "现有材料缺少判断模型是否可用的关键证据：只有整体结果，没有类别分布、任务目标或按类别的指标。整体准确率在不平衡数据上具有误导性，因此在信息补齐之前不能对模型好坏作结论。"
}

func insufficientNextStep(c Completeness, taskType string) string {
	var steps []string
	switch taskType {
	case content.TaskTraining:
		steps = []string{"请补充：1) 逐轮训练日志（至少包含损失与梯度范数等数值）；2) 本次实验的任务目标与模型/数据的基本设置。"}
	case content.TaskArchitecture:
		steps = []string{"请补充：1) 两个待比较模型的具体结构；2) 它们在训练集与测试集上的结果。"}
	default:
		steps = []string{"请补充：1) 各类别的样本数量（或类别分布）；2) 本次实验的任务目标（更怕漏报还是更怕误报）；3) 混淆矩阵或按类别的召回率/精确率。"}
	}
	if len(c.Missing) > 0 {
		steps = append(steps, "当前缺失："+strings.Join(c.Missing, "、")+"。")
	}
	return strings.Join(steps, "")
}

// matchArchetypeByCode 判断指定规则是否命中。
func matchArchetypeByCode(text string, metrics *MatrixMetrics, dataset, taskType, code string) (content.Archetype, bool) {
	for _, a := range matchArchetypes(text, metrics, dataset, taskType) {
		if a.Code == code {
			return a, true
		}
	}
	return content.Archetype{}, false
}

// matchArchetypes 按内置规则匹配典型误区，可能命中多个。
func matchArchetypes(text string, metrics *MatrixMetrics, dataset, taskType string) []content.Archetype {
	low := strings.ToLower(text)
	datasetLow := strings.ToLower(dataset)
	var out []content.Archetype
	for _, a := range content.Archetypes {
		if !archetypeAppliesToTask(a, taskType) {
			continue
		}
		if len(a.Exclude) > 0 && containsAny(low, a.Exclude) {
			continue
		}
		if !matchAllGroups(low, a.Groups) {
			continue
		}
		if a.RequiresImbalance && !imbalanceEvidence(metrics, datasetLow) {
			continue
		}
		if a.RequiresZeroRecall && (metrics == nil || len(metrics.ZeroRecallClass) == 0) {
			continue
		}
		out = append(out, a)
	}
	return out
}

func matchAllGroups(low string, groups []content.KeywordGroup) bool {
	for _, g := range groups {
		if !containsAny(low, g) {
			return false
		}
	}
	return true
}

func imbalanceEvidence(metrics *MatrixMetrics, datasetLow string) bool {
	if metrics != nil && metrics.Imbalanced {
		return true
	}
	return containsAny(datasetLow, []string{"不平衡", "失衡", "多数类", "少数类", "极不均衡", "长尾", "罕见"})
}

// detectContradiction 检查学生对矩阵的陈述与矩阵本身是否冲突。
func detectContradiction(text string, metrics *MatrixMetrics) Contradiction {
	claimsAllCorrect := containsAny(text, []string{"所有类别", "每类", "每一类", "各个类别", "三个类别", "三类", "都预测", "全部预测"}) &&
		containsAny(text, []string{"都能正确分类", "都正确", "全部正确", "识别得很好", "都能识别", "都能被正确分类", "没有错误", "都很准", "没有错分", "都识别"})
	claimsDiagonalEnough := containsAny(text, []string{"对角线都对", "对角线正确", "对角线都对就", "对角线没问题"})
	if metrics == nil || (!claimsAllCorrect && !claimsDiagonalEnough) {
		return Contradiction{}
	}
	offDiagonal := 0.0
	for i, row := range metrics.Values {
		for j, v := range row {
			if i != j {
				offDiagonal += v
			}
		}
	}
	if offDiagonal == 0 {
		return Contradiction{}
	}
	claim := "所有类别都能被正确分类"
	if !claimsAllCorrect {
		claim = "只要对角线正确，模型就没有问题"
	}
	detail := fmt.Sprintf("你在解释中提到“%s”，但混淆矩阵中共有 %.0f 个样本不在对角线上，即被错误分类。", claim, offDiagonal)
	if len(metrics.ZeroRecallClass) > 0 {
		parts := make([]string, 0, len(metrics.ZeroRecallClass))
		for _, name := range metrics.ZeroRecallClass {
			parts = append(parts, name)
		}
		detail += fmt.Sprintf("其中 %s 的召回率为 0%%，这些类别的样本没有任何一个被正确识别。", strings.Join(parts, "、"))
	}
	detail += "你的陈述与提交的证据不一致，建议先确认矩阵的行（真实类别）与列（预测类别）的含义。"
	return Contradiction{Detected: true, Claim: claim, Detail: detail}
}

func buildMainIssue(in DiagnoseInput, taskType string, primary *content.Archetype, contra Contradiction, claims []Claim) string {
	if contra.Detected {
		return fmt.Sprintf("解释中的结论与混淆矩阵证据矛盾：%s", contra.Claim)
	}
	if primary != nil {
		switch primary.Code {
		case "E1":
			if in.Metrics != nil && len(in.Metrics.ZeroRecallClass) > 0 {
				return fmt.Sprintf("只用整体准确率（%.0f%%）评价模型，忽略了 %s 的召回率为 0%%。",
					in.Metrics.Accuracy*100, strings.Join(in.Metrics.ZeroRecallClass, "、"))
			}
			return "只用整体准确率评价模型，没有结合类别分布与各类别指标。"
		case "E2":
			return "把混淆矩阵简化为对角线上的正确数，未分析错误具体发生在哪些类别之间。"
		case "E3":
			return "在未逐类计算的情况下假定各类别召回率接近，未验证模型是否存在多数类偏向。"
		case "E4":
			return "把“继续提高准确率”当作唯一改进方向，没有把指标选择与任务目标、错误代价联系起来。"
		case "E5":
			return "把程序运行成功当作已经掌握知识，未解释结果含义与结论的适用边界。"
		case "E0":
			return "提交内容为环境或代码异常，需要先完成工程排障，再讨论模型结论。"
		case "E6":
			return "把训练异常的原因当成了已证实的结论，没有把日志中的事实、推断与未知信息分开，也没有提出能区分各种原因的检查。"
		case "E8":
			return "把模型复杂度当成了准确率更高的必然原因，没有回到局部连接、权重共享等结构差异上。"
		case "E9":
			return "认为增加模型容量就必然提升性能，没有考虑数据规模、正则化与训练集/测试集表现的差异。"
		case "E10":
			return "只用单一测试结果下结论，没有比较训练集与测试集的表现，无法区分拟合能力差异与泛化差异。"
		}
		return primary.Name
	}
	if in.Metrics != nil {
		if in.Metrics.Imbalanced && !containsAny(strings.ToLower(in.Explanation), []string{"不平衡", "多数类", "少数类", "召回率", "recall", "非对角"}) {
			return fmt.Sprintf("解释没有结合这份矩阵的特点展开：%s 类占比最高（%.0f%%），需要按类别分析而不是停留在整体结论。",
				in.Metrics.Labels[0], in.Metrics.MaxClassRatio*100)
		}
		return "解释已涉及部分结果，但还没有把结论与具体指标证据、适用条件完整对应起来。"
	}
	// 没有命中内置规则时，直接引用尚未解决的最优先主张，避免给出笼统结论。
	if id := primaryClaimID(claims); id != "" {
		for _, c := range claims {
			if c.ID == id {
				return fmt.Sprintf("你的解释中「%s」这一点尚未解决：%s", c.KnowledgePoint, c.Judgment)
			}
		}
	}
	switch taskType {
	case content.TaskTraining:
		return "解释已引用训练过程，但还没有把日志中的事实、推断与未知信息分开，也没有提出可区分的检查方案。"
	case content.TaskArchitecture:
		return "解释已涉及模型表现，但还没有把结构差异、容量与训练/测试集表现对应起来。"
	}
	return "解释停留在过程描述，缺少可核查的结果证据与判断依据。"
}

func genericReasoning(in DiagnoseInput, contra Contradiction) string {
	if contra.Detected {
		return contra.Detail
	}
	base := "诊断依据为你提交的文字解释与（若提供的）混淆矩阵计算结果，未引用任何你未提供的信息。"
	if in.Metrics != nil {
		return base + fmt.Sprintf("本次矩阵总样本 %.0f 个、正确分类 %.0f 个，整体准确率 %.0f%%；判断模型是否可用还需要按类别查看召回率与错误去向。",
			in.Metrics.Total, in.Metrics.Correct, in.Metrics.Accuracy*100)
	}
	return base + "由于未提供混淆矩阵，系统无法逐类计算指标，因此这里不对具体类别的表现作判断。"
}

// primaryClaimID 选出最需要优先处理的主张：错误优先，其次部分掌握，再次证据不足。
func primaryClaimID(claims []Claim) string {
	for _, want := range []string{ClaimError, ClaimPartial, ClaimInsufficient} {
		for _, c := range claims {
			if c.Status == want {
				return c.ID
			}
		}
	}
	return ""
}

// judgeMasteryFromClaims 依据逐个主张的状态推导整体掌握情况，
// 不再用关键词覆盖数量代替真实回答（需求文档 2.4 / 2.8）。
func judgeMasteryFromClaims(claims []Claim) string {
	hasError, hasPartial, hasMastered := false, false, false
	for _, c := range claims {
		switch c.Status {
		case ClaimError:
			hasError = true
		case ClaimPartial:
			hasPartial = true
		case ClaimMastered:
			hasMastered = true
		}
	}
	switch {
	case hasError && hasMastered:
		return MasteryPartial
	case hasError:
		return MasteryNotShown
	case hasPartial:
		return MasteryPartial
	case hasMastered:
		return MasteryDemonstrated
	default:
		return MasteryInsufficient
	}
}

func buildNextStep(in DiagnoseInput, taskType string, primary *content.Archetype, contra Contradiction, claims []Claim) string {
	if contra.Detected {
		return "请先重新读一遍混淆矩阵：第 i 行第 j 列表示“真实是类别 i、却被预测为类别 j”的样本数。请指出你的解释与矩阵中哪几个数字不符，并写出正确读法。"
	}
	if primary != nil {
		return primary.NextStep
	}
	if id := primaryClaimID(claims); id != "" {
		for _, c := range claims {
			if c.ID == id {
				return c.NextQuestion
			}
		}
	}
	switch taskType {
	case content.TaskTraining:
		return "请从日志中分别写出：已经发生的事实、你的推断、以及仍然未知的信息，并给出一个能把这些原因区分开的检查方案。"
	case content.TaskArchitecture:
		return "请说明两个模型在结构上的具体差异，并列出你会记录哪些对照数据来判断性能差异的来源。"
	}
	if in.Metrics != nil {
		return "请挑一个召回率最低的类别，说明它被误判成了哪个类别，以及这种误判在实际任务中意味着什么。"
	}
	return "请补充一段说明：本次实验的结论是什么，它依据哪一个指标，在什么情况下该结论会不成立。"
}

// explanationQuotes 从学生解释中抽取可作为证据的原句。
func explanationQuotes(explanation string) ([]string, []string) {
	text := strings.TrimSpace(explanation)
	if text == "" {
		return nil, nil
	}
	splitter := regexp.MustCompile(`[。！？；\n!?;]+`)
	parts := splitter.Split(text, -1)
	var quotes, sources []string
	priority := []string{"准确率", "accuracy", "召回", "精确", "f1", "F1", "混淆矩阵", "对角线", "类别", "跑通", "报错", "error", "学会", "掌握"}
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if containsAny(strings.ToLower(p), priority) || strings.ContainsAny(p, "0123456789") {
			quotes = append(quotes, "学生原话："+p)
			sources = append(sources, "学生输入")
		}
		if len(quotes) >= 4 {
			break
		}
	}
	if len(quotes) == 0 {
		q := []rune(text)
		if len(q) > 60 {
			q = q[:60]
		}
		quotes = append(quotes, "学生原话："+string(q))
		sources = append(sources, "学生输入")
	}
	return quotes, sources
}

// MatrixFacts 生成基于混淆矩阵的具体证据（全部来自程序计算）。
func MatrixFacts(m *MatrixMetrics) []string {
	if m == nil {
		return []string{}
	}
	facts := []string{fmt.Sprintf("由你提交的矩阵计算：总样本 %.0f 个，正确分类 %.0f 个，整体准确率 %.1f%%，宏平均 F1 %.3f，加权平均 F1 %.3f。",
		m.Total, m.Correct, m.Accuracy*100, m.MacroF1, m.WeightedF1)}
	for _, c := range m.Classes {
		if c.Support == 0 {
			continue
		}
		if c.Recall == 0 {
			dominant := dominantPrediction(m, c.Index)
			if dominant != "" {
				facts = append(facts, fmt.Sprintf("真实%s：%s 个样本，全部被预测为%s，召回率 0%%。", c.Label, trimNum(c.Support), dominant))
			} else {
				facts = append(facts, fmt.Sprintf("真实%s：%s 个样本，正确识别 0 个，召回率 0%%。", c.Label, trimNum(c.Support)))
			}
			continue
		}
		if c.Recall < 1 {
			facts = append(facts, fmt.Sprintf("真实%s：%s 个样本，正确识别 %s 个，召回率 %.1f%%。", c.Label, trimNum(c.Support), trimNum(c.TruePositive), c.Recall*100))
		}
	}
	if len(m.ZeroRecallClass) > 0 {
		facts = append(facts, fmt.Sprintf("召回率为 0 的类别：%s，即这些类别的样本没有被任何一次正确识别。", strings.Join(m.ZeroRecallClass, "、")))
	}
	return facts
}

func dominantPrediction(m *MatrixMetrics, row int) string {
	best, bestIdx := 0.0, -1
	for j, v := range m.Values[row] {
		if j == row {
			continue
		}
		if v > best {
			best, bestIdx = v, j
		}
	}
	if bestIdx < 0 {
		return ""
	}
	return m.Labels[bestIdx]
}

func containsAny(low string, words []string) bool {
	for _, w := range words {
		if w == "" {
			continue
		}
		if strings.Contains(low, strings.ToLower(w)) {
			return true
		}
	}
	return false
}

func hasCategory(list []CategoryRef, code string) bool {
	for _, c := range list {
		if c.Code == code {
			return true
		}
	}
	return false
}

func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if l := strings.TrimSpace(lines[i]); l != "" {
			return truncate(l, 120)
		}
	}
	return ""
}

// logFacts 从学生提交的运行日志中抽取带数值的行，作为可直接核查的证据。
// 只引用学生自己提交的内容，不补全、不推测任何数值。
func logFacts(log string) []string {
	out := []string{}
	for _, line := range strings.Split(log, "\n") {
		l := strings.TrimSpace(line)
		if l == "" || !strings.ContainsAny(l, "0123456789") {
			continue
		}
		out = append(out, "运行日志："+truncate(l, 120))
		if len(out) >= 5 {
			break
		}
	}
	return out
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	lines := strings.Split(s, "\n")
	head := strings.TrimSpace(lines[0])
	if len([]rune(head)) > 60 {
		head = string([]rune(head)[:60]) + "…"
	}
	return head
}

func trimNum(v float64) string {
	if v == float64(int64(v)) {
		return fmt.Sprintf("%.0f", v)
	}
	return fmt.Sprintf("%.2f", v)
}

func nowISO() string { return timeNowISO() }

// NewHint 构造一条提示记录。
func NewHint(level int, content string, archetypeCode string, socratic, requested bool) models.Hint {
	return models.Hint{
		Level:              level,
		Content:            content,
		CreatedAt:          nowISO(),
		ArchetypeCode:      archetypeCode,
		Socratic:           socratic,
		RequestedByStudent: requested,
	}
}
