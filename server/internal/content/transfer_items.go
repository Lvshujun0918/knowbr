package content

// RubricDim 是迁移题的评分维度。评分完全由关键词规则计算，并附带理由文本。
type RubricDim struct {
	Code          string         `json:"code"`
	Title         string         `json:"title"`
	MaxScore      int            `json:"maxScore"`
	KeywordGroups []KeywordGroup `json:"-"`
	FullHitHint   string         `json:"fullHitHint"`
	PartialHint   string         `json:"partialHint"`
	MissHint      string         `json:"missHint"`
}

// TransferItem 是一道新情境迁移题。
type TransferItem struct {
	ID              string      `json:"id"`
	Topic           string      `json:"topic"`
	Archetypes      []string    `json:"archetypes"`
	Context         string      `json:"context"`
	Question        string      `json:"question"`
	MaxScore        int         `json:"maxScore"`
	Rubric          []RubricDim `json:"rubric"`
	ExpectedPoints  []string    `json:"expectedPoints"`
	ReferenceAnswer string      `json:"referenceAnswer"`
}

// TransferDisclaimer 是评分口径声明，必须随分数一起展示。
const TransferDisclaimer = "评分由内置规则根据关键词与要点覆盖情况计算，仅代表本次演示任务的规则化反馈，不等于经过验证的心理测量或正式考试成绩。"

// TransferItems 是迁移题题库。题目与讲解案例的学习目标相关，但改变了类别分布、错误代价或任务目标。
var TransferItems = []TransferItem{
	{
		ID:         "T1",
		Topic:      "类别不平衡下的指标选择（疾病筛查）",
		Archetypes: []string{"E1", "E3", "E4"},
		Context:    "另一个疾病筛查任务中，阳性样本很少（约占 1%）。如果模型把绝大多数样本预测为阴性，整体准确率仍然可能很高。",
		Question:   "此时除了准确率，还应重点查看哪些信息？为什么？请结合“漏诊与误报的代价不同”说明你的选择。",
		MaxScore:   6,
		Rubric: []RubricDim{
			{
				Code:     "goal",
				Title:    "识别任务目标与数据分布（0—2 分）",
				MaxScore: 2,
				KeywordGroups: []KeywordGroup{
					{"阳性", "病", "筛查", "患者", "少数类", "罕见", "极端不平衡", "类别不平衡", "样本很少", "分布"},
					{"漏诊", "漏报", "不能漏", "高风险", "尽早发现", "召回"},
				},
				FullHitHint: "同时点出了数据分布（阳性极少/极不平衡）与任务目标（不能漏诊），说明你抓住了任务的核心约束。",
				PartialHint: "提到了数据分布或任务目标中的一项，建议把另一项也补上：任务对哪一类错误更敏感？",
				MissHint:    "没有提到阳性样本极少这一数据特征，也没有说明任务目标；建议先描述数据分布，再说清哪种错误更不可接受。",
			},
			{
				Code:     "metric",
				Title:    "选择或解释与任务匹配的评价指标（0—2 分）",
				MaxScore: 2,
				KeywordGroups: []KeywordGroup{
					{"召回率", "查全率", "敏感度", "recall", "真阳性率", "漏诊率"},
					{"精确率", "查准率", "precision", "F1", "AUC", "PR", "权衡", "综合"},
				},
				FullHitHint: "给出了主指标并考虑了与之配合的另一类指标，符合“任务决定指标”的思路。",
				PartialHint: "只给出了一类指标，建议补充说明为什么它优先，以及需要用什么指标平衡其不足。",
				MissHint:    "没有提出任何与任务匹配的指标；对筛查任务而言，至少应说明为什么要看阳性类的召回率。",
			},
			{
				Code:     "reasoning",
				Title:    "结合证据或代价说明理由（0—2 分）",
				MaxScore: 2,
				KeywordGroups: []KeywordGroup{
					{"漏诊", "假阴性", "FN", "漏报", "误报", "假阳性", "FP", "误诊"},
					{"混淆矩阵", "代价", "成本", "风险", "权衡", "阈值", "具体数字", "多少例"},
				},
				FullHitHint: "同时说明了错误的类型与相应的代价/阈值考量，理由可追溯到具体证据。",
				PartialHint: "提到了错误类型或代价中的一项，建议用具体数量或代价说明为什么这个指标更重要。",
				MissHint:    "理由停留在“准确率不够”的层面，没有结合漏诊/误报的代价或混淆矩阵中的具体数量展开。",
			},
		},
		ExpectedPoints: []string{
			"关注阳性类别的召回率，因为漏诊可能有较高代价。",
			"结合精确率，考虑误报与漏报之间的权衡。",
			"查看混淆矩阵，分析假阴性和假阳性的数量。",
			"根据具体应用风险和任务目标选择评价指标，而不是机械认定某一个指标永远最重要。",
		},
		ReferenceAnswer: "除准确率外，应重点查看阳性类的召回率、精确率以及混淆矩阵中的假阴性与假阳性数量。原因是：本任务中漏诊（假阴性）的代价明显高于一次误报，而阳性样本占比极低时，全部预测为阴性也能得到很高的准确率，因此准确率无法反映模型是否完成了筛查任务。一般做法是以阳性类召回率为主要指标，同时用精确率或 PR 曲线评估误报带来的额外成本，再根据可接受的漏诊率确定判别阈值。",
	},
	{
		ID:         "T2",
		Topic:      "多分类混淆矩阵的读图（垃圾分类）",
		Archetypes: []string{"E2", "E3"},
		Context:    "把同一个分类流程迁移到四分类的垃圾分类任务（类别：可回收物、厨余、有害、其他），总体准确率 82%。其中“有害垃圾”有 120 个样本、召回率 35%，大部分被预测为“其他”。",
		Question:   "请说明这份混淆矩阵中你最需要关注哪些位置？这些位置对应什么实际风险？如果要改进，你会优先处理哪一类？",
		MaxScore:   6,
		Rubric: []RubricDim{
			{
				Code:     "goal",
				Title:    "识别任务目标与数据分布（0—2 分）",
				MaxScore: 2,
				KeywordGroups: []KeywordGroup{
					{"有害", "类别", "四分类", "少数类", "分布", "样本数", "120"},
					{"优先", "风险", "危害", "错误分类后果", "任务目标"},
				},
				FullHitHint: "既注意到类别间样本不均衡，也说明了该任务中错误分类的后果，定位准确。",
				PartialHint: "指出了类别或风险中的一项，建议把“哪一类错得更严重”说清楚。",
				MissHint:    "没有识别出各类别样本量与错误后果的差异，建议先逐类看样本数与召回率。",
			},
			{
				Code:     "metric",
				Title:    "选择或解释与任务匹配的评价指标（0—2 分）",
				MaxScore: 2,
				KeywordGroups: []KeywordGroup{
					{"召回率", "查全率", "recall", "逐类"},
					{"宏平均", "macro", "加权", "weighted", "F1", "精确率", "按类"},
				},
				FullHitHint: "同时给出了逐类指标与整体聚合口径，避免了只看整体准确率的问题。",
				PartialHint: "提到了逐类或聚合中的一项，建议补充另一项以免结论被多数类掩盖。",
				MissHint:    "没有提出按类别分解的指标，整体准确率 82% 无法反映“有害”类的表现。",
			},
			{
				Code:     "reasoning",
				Title:    "结合证据或代价说明理由（0—2 分）",
				MaxScore: 2,
				KeywordGroups: []KeywordGroup{
					{"非对角", "混淆", "误判为", "被预测为", "其他类", "混入"},
					{"代价", "风险", "污染", "较严重", "优先处理", "非对角线"},
				},
				FullHitHint: "把矩阵中的具体错误去向与该任务的实际后果对应起来，理由完整。",
				PartialHint: "指出了错误位置或后果中的一项，建议用具体数字说明其严重程度。",
				MissHint:    "没有指出错误具体发生在哪两类之间，也没有说明后果，理由不够充分。",
			},
		},
		ExpectedPoints: []string{
			"关注“有害垃圾”这一行（真实类别）中非对角线上的数字，即它被误判成了哪些类别。",
			"该类的召回率只有 35%，说明约 65% 的有害垃圾没有被识别出来，而误分类会带来实际处理风险。",
			"整体准确率受多数类主导，应同时看逐类召回率与宏平均 F1。",
			"改进优先处理后果最严重的类别，而不是只追求整体准确率上升。",
		},
		ReferenceAnswer: "应重点关注“真实为有害垃圾”这一行中所有非对角线元素，它们表示有害垃圾被误判为“其他”等类别的数量；120 个样本中召回率 35%，意味着约 78 个有害垃圾没有被正确识别。由于该项误分类后果最严重，应优先处理：可通过调整类别权重、过采样或阈值调整提升该类召回率，同时用宏平均 F1 或逐类精确率监控是否带来大量新的误报。仅看 82% 的整体准确率无法发现这一问题。",
	},
	{
		ID:         "T3",
		Topic:      "结论适用边界（代码可运行的边界）",
		Archetypes: []string{"E5", "E0"},
		Context:    "一名同学把实验代码复制到另一份数据上，代码同样“运行成功”，但数据只有 60 条、“异常”类别只有 3 条。",
		Question:   "请说明这份新结果在什么条件下才可信，并给出你会主动检查的两项内容及原因。",
		MaxScore:   6,
		Rubric: []RubricDim{
			{
				Code:     "goal",
				Title:    "识别任务目标与数据分布（0—2 分）",
				MaxScore: 2,
				KeywordGroups: []KeywordGroup{
					{"样本少", "样本量", "60", "数据量", "3 条", "分布", "不平衡", "极少"},
					{"任务", "目标", "场景", "结论是否成立", "适用"},
				},
				FullHitHint: "认识到数据规模与分布的变化会改变结论的可信度，并把它与任务目标联系起来。",
				PartialHint: "提到了数据规模或任务目标中的一项，建议明确指出“样本太少会导致什么结论不可靠”。",
				MissHint:    "没有注意到样本量急剧变小带来的问题，仍默认结果可比。",
			},
			{
				Code:     "metric",
				Title:    "选择或解释与任务匹配的评价指标（0—2 分）",
				MaxScore: 2,
				KeywordGroups: []KeywordGroup{
					{"交叉验证", "划分", "训练集", "测试集", "划分比例", "随机种子"},
					{"逐类", "召回率", "F1", "混淆矩阵", "置信"},
				},
				FullHitHint: "给出了评估方案（如交叉验证/固定划分）并按类别评估，覆盖了可信度验证的两个关键点。",
				PartialHint: "提到了评估方案或逐类指标中的一项，建议补齐另一项。",
				MissHint:    "没有提出任何评估方案或逐类指标，无法判断结果稳定性。",
			},
			{
				Code:     "reasoning",
				Title:    "结合证据或代价说明理由（0—2 分）",
				MaxScore: 2,
				KeywordGroups: []KeywordGroup{
					{"波动", "不稳定", "偶然", "过拟合", "偏差", "小样本"},
					{"边界", "适用条件", "不能直接比较", "失效", "外推"},
				},
				FullHitHint: "说明了为什么要检查这两项，并指出原结论的适用边界，推理完整。",
				PartialHint: "提到了不稳定性或适用边界中的一项，建议把两者连起来说明。",
				MissHint:    "只重复了“再看一眼”，没有说明检查内容与结论可信度之间的关系。",
			},
		},
		ExpectedPoints: []string{
			"代码能运行只说明流程可执行，样本量只有 60 条时指标波动很大。",
			"应确认数据划分方式（交叉验证或固定随机种子的训练/测试划分），避免结果受单次划分影响。",
			"应逐类查看指标与样本数，3 条异常样本无法支撑该类别的任何结论。",
			"原实验结果不能直接外推到该数据上，需要说明适用条件。",
		},
		ReferenceAnswer: "新结果只有在数据划分与评估方式明确、且各类别样本量足够时才具备参考价值。建议主动检查：一是评估方式（是否使用交叉验证或固定随机种子的训练/测试划分），因为 60 条样本下单次划分的指标波动极大；二是逐类的样本数与召回率，因为“异常”类只有 3 条，任何指标变化都可能由单个样本决定。同时应说明原结论的适用边界：该结论只适用于与原数据规模、类别分布相近的场景，不能直接外推到小样本数据上。",
	},
}

// TransferItemByID 按编号查找迁移题。
func TransferItemByID(id string) (TransferItem, bool) {
	for _, item := range TransferItems {
		if item.ID == id {
			return item, true
		}
	}
	return TransferItem{}, false
}
