package services

import (
	"fmt"
	"regexp"
	"strings"

	"knowbr/server/internal/models"
)

// RevisionRequest 描述一次学生修正解释的分析请求。
type RevisionRequest struct {
	Content            string
	InitialExplanation string
	Previous           []models.Revision
	Diagnosis          Diagnosis
	Metrics            *MatrixMetrics
	State              models.HintState
}

// RevisionResult 是修正解释的分析结果。
type RevisionResult struct {
	Revision models.Revision
	State    models.HintState
	Improved bool
}

// evidenceGroup 是修正解释中可核查的证据要点。
type evidenceGroup struct {
	Name          string
	Words         []string
	MissingAdvice string
}

var revisionEvidenceGroups = []evidenceGroup{
	{
		Name:          "按类别的召回率",
		Words:         []string{"召回率", "查全率", "recall", "漏检率"},
		MissingAdvice: "还没有按类别计算召回率（某类真实样本中被正确识别的比例）",
	},
	{
		Name:          "精确率或其他分类指标",
		Words:         []string{"精确率", "查准率", "precision", "f1", "auc", "混淆矩阵"},
		MissingAdvice: "还没有提到精确率/F1 等指标与召回率的分工",
	},
	{
		Name:          "错误的具体去向",
		Words:         []string{"非对角", "被预测为", "误判", "判成", "假阴", "假阳", "误分类", "错分"},
		MissingAdvice: "还没有指出错误具体发生在哪两类之间（非对角线元素）",
	},
	{
		Name:          "类别分布或不平衡",
		Words:         []string{"不平衡", "不均衡", "多数类", "少数类", "占比", "样本数量", "样本数", "分布"},
		MissingAdvice: "还没有把结论与类别分布联系起来",
	},
	{
		Name:          "任务目标与错误代价",
		Words:         []string{"任务目标", "漏诊", "漏报", "误报", "代价", "成本", "风险", "阈值", "应用场景"},
		MissingAdvice: "还没有结合任务目标说明哪种错误更不能接受",
	},
}

var numberPattern = regexp.MustCompile(`\d+(\.\d+)?\s*(%|个|例|条|类)`)

// AnalyzeRevision 判断学生修正后的解释是否有所推进，并给出简短反馈。
func AnalyzeRevision(req RevisionRequest) RevisionResult {
	state := req.State
	if state.MaxLevel == 0 {
		state.MaxLevel = 3
	}
	current := detectEvidence(req.Content)
	var previous map[string]bool
	if len(req.Previous) > 0 {
		previous = detectEvidence(req.Previous[len(req.Previous)-1].Content)
	} else {
		previous = detectEvidence(req.InitialExplanation)
	}

	var added, stillMissing []string
	for _, g := range revisionEvidenceGroups {
		if current[g.Name] && !previous[g.Name] {
			added = append(added, g.Name)
		}
		if !current[g.Name] {
			stillMissing = append(stillMissing, g.MissingAdvice)
		}
	}
	hasNumber := numberPattern.MatchString(req.Content)

	progress := "none"
	switch {
	case len(added) >= 2 || (len(added) == 1 && hasNumber && !numberPattern.MatchString(req.InitialExplanation)):
		progress = "improved"
	case len(added) == 1:
		progress = "partial"
	default:
		progress = "none"
	}

	var points []string
	var feedback string
	switch progress {
	case "improved":
		feedback = fmt.Sprintf("这次解释有推进：你补充了%s。", strings.Join(added, "、"))
		points = append(points, "新增证据要点："+strings.Join(added, "、"))
	case "partial":
		feedback = fmt.Sprintf("这次解释比上一次多了一点内容（%s），但关键的推理链条还没有接上。", strings.Join(added, "、"))
		points = append(points, "新增证据要点："+strings.Join(added, "、"))
	default:
		feedback = "这次解释相比上一次没有新增可以被核查的证据点，仍然停留在原来的判断上。"
		points = append(points, "未检测到新的证据要点")
	}
	if hasNumber {
		points = append(points, "引用了具体数值，属于可核查的证据")
	} else {
		points = append(points, "未引用具体数值，建议用矩阵中的数字说明判断")
	}
	if len(stillMissing) > 0 {
		feedback += " 仍然缺少：" + strings.Join(firstN(stillMissing, 2), "；") + "。"
		points = append(points, "仍缺失："+strings.Join(firstN(stillMissing, 2), "；"))
	} else {
		feedback += " 主要的证据要点都已覆盖，可以进入迁移测试检验能否迁移到新情境。"
	}

	if m := req.Metrics; m != nil && len(m.ZeroRecallClass) > 0 &&
		containsAny(strings.ToLower(req.Content), []string{"召回率", "识别", "0%"}) {
		feedback += fmt.Sprintf(" 结合你的矩阵看，%s 的召回率是 0%%，这一点已经可以直接用来支撑结论。", strings.Join(m.ZeroRecallClass, "、"))
	}

	if progress == "none" {
		state.NoProgressRounds++
	} else {
		state.NoProgressRounds = 0
	}
	if state.NoProgressRounds >= 2 && !state.TeacherIntervention {
		state.TeacherIntervention = true
		state.TeacherReason = "连续两轮修正都没有新增可核查的证据点，规则模式的提示难以继续推进，建议教师介入并当面澄清核心概念。"
		feedback += " 已连续两轮没有进展，系统建议请教师介入辅导。"
	}

	// 列表字段统一输出空数组，避免前端拿到 null。
	if points == nil {
		points = []string{}
	}
	if added == nil {
		added = []string{}
	}
	if stillMissing == nil {
		stillMissing = []string{}
	}
	revision := models.Revision{
		Content:           req.Content,
		CreatedAt:         nowISO(),
		Feedback:          feedback,
		Progress:          progress,
		Points:            points,
		AddressedKeywords: added,
		MissingKeywords:   stillMissing,
	}
	return RevisionResult{Revision: revision, State: state, Improved: progress == "improved"}
}

func detectEvidence(text string) map[string]bool {
	low := strings.ToLower(text)
	out := make(map[string]bool, len(revisionEvidenceGroups))
	for _, g := range revisionEvidenceGroups {
		out[g.Name] = containsAny(low, g.Words)
	}
	return out
}

func firstN(list []string, n int) []string {
	if len(list) <= n {
		return list
	}
	return list[:n]
}
