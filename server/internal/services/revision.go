package services

import (
	"fmt"
	"regexp"
	"strings"

	"knowbr/server/internal/content"
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
	TaskType           string
}

// RevisionResult 是修正解释的分析结果。
type RevisionResult struct {
	Revision models.Revision
	State    models.HintState
	Improved bool
}

// evidenceGroup 是修正解释中可核查的证据要点，按当前实验的任务类型选择。
type evidenceGroup struct {
	Name          string
	Words         []string
	MissingAdvice string
}

// classificationEvidenceGroups 适用于分类指标类任务。
var classificationEvidenceGroups = []evidenceGroup{
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

// trainingEvidenceGroups 适用于训练稳定性任务：只考查日志证据、假设边界与对照设计。
var trainingEvidenceGroups = []evidenceGroup{
	{
		Name:          "日志中的事实、推断与未知",
		Words:         []string{"事实", "推断", "未知", "不确定", "不能确定", "无法确定", "无法排除", "区分"},
		MissingAdvice: "还没有把日志中的事实、你的推断与仍然未知的信息分开",
	},
	{
		Name:          "假设与结论的区分",
		Words:         []string{"假设", "可能", "推测", "怀疑", "尚未证实", "不能证明", "未证实", "未必"},
		MissingAdvice: "还没有说明这个原因目前只是假设、尚未被证实",
	},
	{
		Name:          "触发异常的数值与轮次",
		Words:         []string{"loss", "损失", "梯度", "grad_norm", "范数", "nan", "inf", "轮"},
		MissingAdvice: "还没有引用日志中的具体数值与异常出现的轮次",
	},
	{
		Name:          "学习率与梯度裁剪的作用差异",
		Words:         []string{"学习率", "lr", "步长", "裁剪", "clip", "阈值", "上限"},
		MissingAdvice: "还没有说清学习率与梯度裁剪各自改变了什么",
	},
	{
		Name:          "单变量对照实验设计",
		Words:         []string{"对照", "控制变量", "只改变", "只调整", "单独改变", "分别跑", "固定其它", "重复实验"},
		MissingAdvice: "还没有给出只改变单一变量的对照方案，也没有说明什么结果会推翻假设",
	},
}

// architectureEvidenceGroups 适用于模型结构比较任务。
var architectureEvidenceGroups = []evidenceGroup{
	{
		Name:          "结构差异（局部连接与权重共享）",
		Words:         []string{"局部连接", "权重共享", "卷积", "卷积核", "感受野", "共享参数"},
		MissingAdvice: "还没有说明卷积与全连接在连接方式与参数共享上的差异",
	},
	{
		Name:          "图像空间信息的利用",
		Words:         []string{"展平", "拉平", "一维向量", "空间", "像素", "位置"},
		MissingAdvice: "还没有说明 MLP 展平图像后空间结构被破坏这一点",
	},
	{
		Name:          "训练集与测试集表现的比较",
		Words:         []string{"训练集", "测试集", "过拟合", "泛化", "差距", "验证集"},
		MissingAdvice: "还没有同时给出训练集与测试集的表现来讨论拟合情况",
	},
	{
		Name:          "容量与数据规模的匹配",
		Words:         []string{"容量", "隐藏层", "神经元", "层数", "过拟合", "样本量", "数据量"},
		MissingAdvice: "还没有说明模型容量与数据规模之间的关系",
	},
	{
		Name:          "参数量或推理代价",
		Words:         []string{"参数量", "参数数量", "推理", "耗时", "计算量", "效率", "flops"},
		MissingAdvice: "还没有用参数量或推理耗时说明结构带来的代价",
	},
}

// envEvidenceGroups 适用于环境与代码排障任务。
var envEvidenceGroups = []evidenceGroup{
	{
		Name:          "报错信息的定位",
		Words:         []string{"报错", "最后一行", "traceback", "模块名", "no module named", "importerror", "filenotfound"},
		MissingAdvice: "还没有从报错信息本身定位问题（例如缺失的模块名或出错代码行）",
	},
	{
		Name:          "依赖与运行环境的一致性",
		Words:         []string{"版本", "pip list", "conda", "虚拟环境", "解释器", "requirements", "安装"},
		MissingAdvice: "还没有检查依赖版本与运行环境是否一致",
	},
	{
		Name:          "路径或数据检查",
		Words:         []string{"路径", "工作目录", "abspath", "相对路径", "绝对路径", "shape", "维度"},
		MissingAdvice: "还没有检查数据路径或形状是否符合预期",
	},
	{
		Name:          "修复后的验证方式",
		Words:         []string{"重新运行", "再跑", "验证", "确认", "复现", "输出"},
		MissingAdvice: "还没有说明如何验证问题已经解决",
	},
}

// genericEvidenceGroups 用于任务类型不明确的情况：只考查结论与证据的对应关系。
var genericEvidenceGroups = []evidenceGroup{
	{
		Name:          "结论依据的证据",
		Words:         []string{"依据", "证据", "数据", "来自", "根据"},
		MissingAdvice: "还没有说明结论依据的是哪一条证据",
	},
	{
		Name:          "结论的适用边界",
		Words:         []string{"适用", "边界", "条件", "不成立", "失效", "外推"},
		MissingAdvice: "还没有说明这个结论在什么情况下会不成立",
	},
}

// EvidenceGroupsFor 返回当前任务类型下可核查的证据要点。
func EvidenceGroupsFor(taskType string) []evidenceGroup {
	switch taskType {
	case content.TaskTraining:
		return trainingEvidenceGroups
	case content.TaskArchitecture:
		return architectureEvidenceGroups
	case content.TaskCodeEnv:
		return envEvidenceGroups
	case content.TaskGeneric:
		return genericEvidenceGroups
	default:
		return classificationEvidenceGroups
	}
}

// numberPattern 判定回答中是否引用了可核查的具体数值（含小数，不强求单位）。
var numberPattern = regexp.MustCompile(`\d+(\.\d+)?`)

// AnalyzeRevision 判断学生修正后的解释是否有所推进，并给出简短反馈。
// 判定标准按当前实验的任务类型选择，避免把其它任务的知识点当成必须回答的内容。
func AnalyzeRevision(req RevisionRequest) RevisionResult {
	state := req.State
	if state.MaxLevel == 0 {
		state.MaxLevel = 3
	}
	taskType := req.TaskType
	if taskType == "" {
		taskType = req.Diagnosis.TaskType
	}
	groups := EvidenceGroupsFor(taskType)
	current := detectEvidence(req.Content, groups)
	var previous map[string]bool
	if len(req.Previous) > 0 {
		previous = detectEvidence(req.Previous[len(req.Previous)-1].Content, groups)
	} else {
		previous = detectEvidence(req.InitialExplanation, groups)
	}

	var added, stillMissing []string
	for _, g := range groups {
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
	case len(added) >= 2, len(added) == 1 && hasNumber:
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
		points = append(points, "未引用具体数值，建议用你提交的材料中的数字说明判断")
	}
	if len(stillMissing) > 0 {
		feedback += " 仍然缺少：" + strings.Join(firstN(stillMissing, 2), "；") + "。"
		points = append(points, "仍缺失："+strings.Join(firstN(stillMissing, 2), "；"))
	} else {
		feedback += fmt.Sprintf(" 本任务（%s）需要的证据要点都已覆盖，可以进入迁移测试检验能否迁移到新情境。",
			content.TaskTypeLabel(taskType))
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

func detectEvidence(text string, groups []evidenceGroup) map[string]bool {
	low := strings.ToLower(text)
	out := make(map[string]bool, len(groups))
	for _, g := range groups {
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
