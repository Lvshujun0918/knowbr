package services

import (
	"fmt"
	"regexp"
	"strings"

	"knowbr/server/internal/content"
)

// 知识点状态：需求文档 2.4 要求至少区分以下六种状态。
const (
	ClaimMastered      = "mastered"              // 已掌握：给出正确且与问题相关的解释
	ClaimPartial       = "partial"               // 部分掌握：方向正确但缺少关键环节
	ClaimError         = "error"                 // 存在错误：与原理、证据或逻辑冲突
	ClaimInsufficient  = "insufficient_evidence" // 证据不足：结论需要的数据没有提供
	ClaimNotMentioned  = "not_mentioned"         // 未涉及：没有讨论，但不足以判断不会
	ClaimNotApplicable = "not_applicable"        // 不适用：不属于当前实验目标
)

var claimStatusLabels = map[string]string{
	ClaimMastered:      "已掌握",
	ClaimPartial:       "部分掌握",
	ClaimError:         "存在错误",
	ClaimInsufficient:  "证据不足",
	ClaimNotMentioned:  "未涉及",
	ClaimNotApplicable: "不适用",
}

// ClaimStatusLabel 返回知识点状态的中文标签。
func ClaimStatusLabel(status string) string {
	if l, ok := claimStatusLabels[status]; ok {
		return l
	}
	return status
}

// claimStatusOrder 用于在同一知识点出现多个判断时取更保守的一个。
var claimStatusOrder = []string{ClaimError, ClaimInsufficient, ClaimPartial, ClaimNotMentioned, ClaimNotApplicable, ClaimMastered}

// Claim 是一条被单独分析的学生主张（需求文档 2.1 / 2.7 / 4.1）。
// 每条主张都必须能对应到学生原话、判断、理由、证据边界与下一步追问。
type Claim struct {
	ID               string   `json:"id"`
	Quote            string   `json:"quote"`
	KnowledgePoint   string   `json:"knowledgePoint"`
	TaskType         string   `json:"taskType"`
	Status           string   `json:"status"`
	StatusLabel      string   `json:"statusLabel"`
	Judgment         string   `json:"judgment"`
	Reason           string   `json:"reason"`
	Evidence         string   `json:"evidence"`
	EvidenceBoundary string   `json:"evidenceBoundary"`
	NextQuestion     string   `json:"nextQuestion"`
	Signals          []string `json:"signals"`
}

// KnowledgeItem 是当前任务下的一个知识点及其判定状态。
// 未涉及与不适用不得被当作“未掌握”。
type KnowledgeItem struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	Status      string `json:"status"`
	StatusLabel string `json:"statusLabel"`
	Note        string `json:"note"`
	Required    bool   `json:"required"`
	Applicable  bool   `json:"applicable"`
}

// EvidenceNeed 是一项材料的必要性判断（需求文档 2.5）。
type EvidenceNeed struct {
	Name     string `json:"name"`
	Required bool   `json:"required"`
	Present  bool   `json:"present"`
	Impact   string `json:"impact"`
	Note     string `json:"note"`
}

type claimRule struct {
	Code             string
	TaskTypes        []string
	KnowledgePoint   string
	Groups           []content.KeywordGroup
	Exclude          []string
	RequireNumber    bool
	Status           string
	Judgment         string
	Reason           string
	EvidenceBoundary string
	NextQuestion     string
}

// claimRules 是主张判定规则。规则按“越具体越靠前”排列，同一句话命中第一条即归类。
// 每条规则都限定适用的任务类型，从机制上阻止跨任务模板漂移。
var claimRules = []claimRule{
	// ---------- 环境与代码排障 ----------
	{
		Code:             "EN-ERROR",
		TaskTypes:        []string{content.TaskCodeEnv},
		KnowledgePoint:   "报错信息定位",
		Groups:           []content.KeywordGroup{{"报错", "error", "traceback", "no module named", "modulenotfounderror", "异常"}},
		Status:           ClaimInsufficient,
		Judgment:         "这是一个环境或代码问题，不属于概念掌握判断。",
		Reason:           "报错类问题需要按工程排障处理：先定位缺失依赖或出错代码行，再讨论模型结论。",
		EvidenceBoundary: "排障结论只针对报错本身；在代码跑通之前，不对模型效果或概念理解下判断。",
		NextQuestion:     "请先按排查顺序执行，再把执行后的完整输出重新提交。",
	},
	// ---------- 训练稳定性 ----------
	{
		Code:           "TR-CERTAIN-CAUSE",
		TaskTypes:      []string{content.TaskTraining},
		KnowledgePoint: "结论的证据边界",
		Groups: []content.KeywordGroup{
			{"梯度爆炸", "梯度", "学习率", "数据", "代码", "实现"},
			{"肯定是", "一定是", "就是因为", "证明是", "说明就是", "已经确定", "根本原因就是", "必然是"},
		},
		Status:           ClaimError,
		Judgment:         "把单一原因说成了已经证实的结论。",
		Reason:           "日志只能说明训练出现异常（损失上升并出现非有限值），不能单独证明根因一定是梯度爆炸：损失计算或数据中的非有限值、实现错误同样会产生相同现象。",
		EvidenceBoundary: "现有材料支持“训练不稳定”这一事实，不支持“根因已确定”这一结论。",
		NextQuestion:     "请从日志里分别列出：已经发生的事实、你的推断、以及仍然未知的信息；再说明哪一种检查能把这些原因区分开。",
	},
	{
		Code:           "TR-DISTINGUISH",
		TaskTypes:      []string{content.TaskTraining},
		KnowledgePoint: "日志中的事实、推断与未知",
		Groups: []content.KeywordGroup{
			{"事实", "推断", "未知", "不确定", "不能确定", "无法确定", "无法排除", "不能排除", "区分"},
		},
		Status:           ClaimMastered,
		Judgment:         "能够区分日志中的事实、推断与仍然未知的信息。",
		Reason:           "把“观察到的数值变化”和“对原因的解释”分开表述，是这类问题最关键的一步。",
		EvidenceBoundary: "该判断只说明表述方式正确，根因是否成立仍需对照实验确认。",
		NextQuestion:     "请在已区分事实与推断的基础上，给出一个只改变单一变量的检查方案，并说明什么结果会推翻你的假设。",
	},
	{
		Code:           "TR-CONTROL",
		TaskTypes:      []string{content.TaskTraining},
		KnowledgePoint: "单变量对照实验设计",
		Groups: []content.KeywordGroup{
			{"对照", "控制变量", "只改变", "单独改变", "只调整", "固定其它", "分别跑", "两组实验", "重复实验"},
		},
		Status:           ClaimMastered,
		Judgment:         "提出了控制变量的对照方案。",
		Reason:           "把学习率、梯度裁剪等改动分开验证，才能判断是哪一项引起了变化。",
		EvidenceBoundary: "方案本身还不能证明假设成立，需要实际运行结果支持。",
		NextQuestion:     "请说明你预计对照实验会出现什么结果，以及哪一种结果会推翻当前的梯度爆炸假设。",
	},
	{
		Code:           "TR-CLIP-EXPLAIN",
		TaskTypes:      []string{content.TaskTraining},
		KnowledgePoint: "学习率与梯度裁剪的作用差异",
		Groups: []content.KeywordGroup{
			{"裁剪", "clip", "梯度范数", "grad_norm", "阈值", "上限"},
			{"学习率", "步长", "lr", "更新幅度"},
		},
		Status:           ClaimMastered,
		Judgment:         "同时说明了裁剪与学习率的作用差异。",
		Reason:           "裁剪限制单步更新的幅度上限，学习率决定每步更新的基准尺度；两者可以缓解同类现象，但改变的不是同一件事。",
		EvidenceBoundary: "两者都属于缓解手段，能否解决问题取决于根因，不能据此断定问题已消除。",
		NextQuestion:     "请说明如果只降低学习率、不加裁剪，你会预期损失曲线发生什么变化，并设计一次验证。",
	},
	{
		Code:           "TR-HYPOTHESIS",
		TaskTypes:      []string{content.TaskTraining},
		KnowledgePoint: "假设与结论的区分",
		Groups: []content.KeywordGroup{
			{"梯度爆炸", "梯度", "学习率", "数据", "实现", "非有限"},
			{"可能", "也许", "不一定", "假设", "推测", "怀疑", "倾向于"},
		},
		Status:           ClaimPartial,
		Judgment:         "提出了有依据的假设，但还没有把它与其它可能原因区分开。",
		Reason:           "方向合理：损失上升伴随梯度范数增大，符合数值不稳定的特征；但仅凭这段日志无法排除损失计算或数据中的非有限值、实现错误等同样能产生该现象的原因。",
		EvidenceBoundary: "可以说明“与数值不稳定一致”，不能说明“已经确认是梯度爆炸”。",
		NextQuestion:     "请看第 3—5 轮的 loss 与梯度范数：哪些是日志直接记录的事实，哪些是你的推断？哪一种检查能区分梯度爆炸与非有限值/实现错误？",
	},
	{
		Code:             "TR-NUMBER-ONLY",
		TaskTypes:        []string{content.TaskTraining},
		KnowledgePoint:   "训练损失与梯度范数的变化",
		Groups:           []content.KeywordGroup{{"loss", "损失", "梯度", "grad_norm", "范数"}},
		RequireNumber:    true,
		Status:           ClaimPartial,
		Judgment:         "引用了日志中的具体数值，但还没有说明这些数值支持什么、不能证明什么。",
		Reason:           "具体数值是可核查的证据，但目前只做了复述，还没有与假设或检查方案连接起来。",
		EvidenceBoundary: "数值本身能证明训练出现异常，不能直接证明异常的原因。",
		NextQuestion:     "请用这些数值回答：异常是从哪一轮开始的？这一轮同时发生了什么变化？",
	},
	// ---------- 模型结构 ----------
	{
		Code:           "AR-COMPLEX-CAUSE",
		TaskTypes:      []string{content.TaskArchitecture},
		KnowledgePoint: "模型复杂度与性能的因果关系",
		Groups: []content.KeywordGroup{
			{"复杂", "复杂度", "参数多", "模型更大", "结构更复杂"},
			{"一定", "必然", "肯定", "所以更", "因此更", "说明", "导致", "就能"},
		},
		Status:           ClaimError,
		Judgment:         "把模型复杂度当成了准确率更高的必然原因。",
		Reason:           "复杂度只说明假设空间更大，既可能带来更强的拟合能力，也可能带来更高的过拟合风险；准确率差异需要由结构差异（局部连接、权重共享、对空间信息的利用）和实际训练结果来解释。",
		EvidenceBoundary: "现有材料能支持“两个模型的准确率存在差距”，不支持“复杂度必然导致更高准确率”。",
		NextQuestion:     "请把这句话拆成两部分：结构上 CNN 与 MLP 到底差在哪里？这个差异为什么可能影响图像分类效果？",
	},
	{
		Code:           "AR-MORE-WINS",
		TaskTypes:      []string{content.TaskArchitecture},
		KnowledgePoint: "模型容量与数据规模的匹配",
		Groups: []content.KeywordGroup{
			{"增加", "加大", "继续加", "更多", "堆"},
			{"隐藏层", "神经元", "层数", "参数", "容量"},
			{"一定", "必然", "肯定", "就能", "超过", "保证"},
		},
		Status:           ClaimError,
		Judgment:         "认为增加模型容量就必然提升性能。",
		Reason:           "容量增大只提高拟合能力上限，是否转化为测试表现的提升取决于数据规模、正则化与训练是否充分；在数据量有限时，容量增大往往先表现为训练集与测试集表现差距扩大。",
		EvidenceBoundary: "要判断加层是否有效，需要训练集与测试集上的对照结果，当前材料还没有提供。",
		NextQuestion:     "如果给 MLP 增加隐藏层，你预期训练集准确率和测试集准确率分别会怎么变？请说明理由，并给出你会记录哪些对照数据。",
	},
	{
		Code:           "AR-COMPLEX-ASSERT",
		TaskTypes:      []string{content.TaskArchitecture},
		KnowledgePoint: "模型复杂度与性能的因果关系",
		Groups: []content.KeywordGroup{
			{"复杂", "复杂度", "参数多", "模型更大", "层更多"},
			{"cnn", "mlp", "卷积", "全连接", "模型", "网络"},
		},
		Exclude:          []string{"局部连接", "权重共享", "卷积核", "展平", "空间", "感受野", "参数量", "泛化"},
		Status:           ClaimPartial,
		Judgment:         "用一个笼统的原因解释两个模型的表现差异，没有回到结构本身。",
		Reason:           "“更复杂”既不是结构描述，也不能单独决定准确率；需要说明具体是哪一处结构差异在起作用，以及它为什么可能影响图像分类效果。",
		EvidenceBoundary: "这句话没有给出可以被数据检验的结构证据。",
		NextQuestion:     "请写出 CNN 与 MLP 在结构上的至少两处具体差异，并说明每一处为什么可能影响图像分类效果。",
	},
	{
		Code:           "AR-SINGLE-METRIC",
		TaskTypes:      []string{content.TaskArchitecture},
		KnowledgePoint: "训练集与测试集表现差异（过拟合）",
		Groups: []content.KeywordGroup{
			{"测试集", "准确率", "accuracy"},
			{"就说明", "就能说明", "只要", "等于", "等同于", "一定说明", "泛化能力更强"},
		},
		Status:           ClaimError,
		Judgment:         "把单一测试集准确率的变化直接等同于泛化能力增强。",
		Reason:           "测试集准确率上升可能来自模型学到了更通用的特征，也可能来自数据划分偶然性或超参数选择带来的波动；判断泛化能力需要同时看训练集与测试集的差距、多次划分或交叉验证的结果。",
		EvidenceBoundary: "现有材料能说明测试集准确率上升，不能单独证明泛化能力提升。",
		NextQuestion:     "请给出训练集与测试集两个数字，并说明二者的差距说明了什么。",
	},
	{
		Code:           "AR-STRUCTURE-EXPLAIN",
		TaskTypes:      []string{content.TaskArchitecture},
		KnowledgePoint: "全连接与卷积的结构差异",
		Groups: []content.KeywordGroup{
			{"局部连接", "卷积", "权重共享", "卷积核"},
			{"特征", "提取", "共享", "空间", "邻域"},
		},
		Status:           ClaimMastered,
		Judgment:         "能够从局部连接与权重共享解释卷积结构的特点。",
		Reason:           "把结构机制说清楚了：卷积通过局部连接与权重共享提取局部特征，参数随通道数而非像素数增长。",
		EvidenceBoundary: "结构优势可以解释“为什么可能更好”，但单次实验的准确率差距仍受数据与训练设置影响。",
		NextQuestion:     "请再说明：这种结构差异在什么数据条件下优势会变小，甚至消失？",
	},
	{
		Code:           "AR-FLATTEN",
		TaskTypes:      []string{content.TaskArchitecture},
		KnowledgePoint: "图像空间结构的利用",
		Groups: []content.KeywordGroup{
			{"展平", "拉平", "一维向量", "flatten"},
			{"空间", "像素", "位置", "图像"},
		},
		Status:           ClaimMastered,
		Judgment:         "说明了 MLP 需要把图像展平，空间结构会被打散。",
		Reason:           "这是 MLP 与 CNN 差异的起点：展平后相邻像素的空间关系不再被结构显式利用。",
		EvidenceBoundary: "这解释了结构差异，不直接等于准确率差距的全部原因。",
		NextQuestion:     "请把这一点与你观察到的准确率差距连起来：空间结构被打散后，模型需要靠什么来弥补？",
	},
	{
		Code:           "AR-OVERFIT",
		TaskTypes:      []string{content.TaskArchitecture},
		KnowledgePoint: "训练集与测试集表现差异（过拟合）",
		Groups: []content.KeywordGroup{
			{"训练集", "测试集", "训练准确率", "测试准确率"},
			{"过拟合", "泛化", "差距", "差很多", "一致", "分别", "对比"},
		},
		Status:           ClaimMastered,
		Judgment:         "能用训练集与测试集表现的差异讨论拟合情况。",
		Reason:           "这是判断“更复杂的结构是否真的更好”的必要证据：只看测试集单一数字无法区分拟合能力与泛化能力。",
		EvidenceBoundary: "还需要参数量或训练耗时等数据才能完整比较结构代价。",
		NextQuestion:     "请补充说明：如果两个模型的训练集准确率都接近 100%，你更愿意相信哪一个的测试表现？为什么？",
	},
	// ---------- 分类指标 ----------
	{
		Code:           "CL-ACC-ONLY",
		TaskTypes:      []string{content.TaskClassification},
		KnowledgePoint: "整体准确率在类别不平衡下的误导性",
		Groups: []content.KeywordGroup{
			{"准确率", "accuracy", "acc"},
			{"很好", "不错", "挺好", "优秀", "没问题", "可以直接用", "效果好", "表现好", "理想", "很高"},
		},
		Exclude:          []string{"召回率", "recall", "精确率", "precision", "逐类", "按类", "混淆矩阵", "不平衡", "多数类", "少数类"},
		Status:           ClaimError,
		Judgment:         "只用整体准确率就得出了模型可用的结论。",
		Reason:           "整体准确率对每个样本等权，当某一类占绝大多数时，即使少数类完全没有被识别，准确率也可能很高，因此它无法支撑“模型可用”这一结论。",
		EvidenceBoundary: "现有材料能说明整体准确率是多少，不足以说明各类别的识别情况。",
		NextQuestion:     "请逐类计算召回率（该真实类别中正确识别的比例），再回答：哪些类别实际上没有被识别出来？",
	},
	{
		Code:           "CL-RECALL-SAME",
		TaskTypes:      []string{content.TaskClassification},
		KnowledgePoint: "各分类别的召回率比较",
		Groups: []content.KeywordGroup{
			{"召回率", "recall", "查全率"},
			{"差不多", "都一样", "大致相同", "都高", "都很好", "很接近", "均衡"},
		},
		Status:           ClaimError,
		Judgment:         "在没有逐类计算的情况下假定各类别召回率接近。",
		Reason:           "多数分类算法以整体损失最小为目标，类别样本数相差悬殊时会系统性地偏向多数类，因此召回率是否接近必须逐类验证，不能作为默认假设。",
		EvidenceBoundary: "该判断需要逐类召回率数据支持，当前材料尚未提供。",
		NextQuestion:     "请列出每个类别的样本数与召回率，并回答：召回率的差异与类别样本数之间是否存在关系？",
	},
	{
		Code:           "CL-DIAGONAL-ONLY",
		TaskTypes:      []string{content.TaskClassification},
		KnowledgePoint: "混淆矩阵非对角元素的含义",
		Groups: []content.KeywordGroup{
			{"对角线"},
			{"只要", "只看", "就够", "足够", "不用看", "忽略", "光看", "不需要"},
		},
		Status:           ClaimError,
		Judgment:         "把混淆矩阵简化为对角线上的正确数。",
		Reason:           "对角线只统计预测正确的数量，非对角线元素才记录错误发生在哪些类别之间，丢掉它就无法定位模型偏向。",
		EvidenceBoundary: "当前矩阵中的非对角线数字已经可以提供错误去向，只是还没有被使用。",
		NextQuestion:     "请挑出一行非对角线数字，说明这些样本本来是哪个类别、被预测成了什么。",
	},
	{
		Code:           "CL-RECALL-CALC",
		TaskTypes:      []string{content.TaskClassification},
		KnowledgePoint: "按类别的召回率",
		Groups: []content.KeywordGroup{
			{"召回率", "recall", "查全率"},
			{"计算", "算出", "0%", "逐类", "按类", "每一类", "分别是"},
		},
		Status:           ClaimMastered,
		Judgment:         "已经按类别计算或解释了召回率。",
		Reason:           "这是判断模型能否在任务中使用的关键证据，说明分析已经落到逐类表现上。",
		EvidenceBoundary: "还要结合任务目标说明哪种错误代价更高，才能给出“是否可用”的结论。",
		NextQuestion:     "请结合本任务说明：哪一类的错误代价更高？据此应当以哪个指标为主指标？",
	},
	{
		Code:           "CL-IMBALANCE",
		TaskTypes:      []string{content.TaskClassification},
		KnowledgePoint: "类别分布对整体指标的影响",
		Groups: []content.KeywordGroup{
			{"不平衡", "不均衡", "多数类", "少数类", "占比", "样本数", "分布"},
			{"主导", "影响", "误导", "被多数", "导致", "反映不出", "掩盖"},
		},
		Status:           ClaimMastered,
		Judgment:         "把结论与类别分布联系起来了。",
		Reason:           "说明已经意识到整体指标会被多数类主导，这是正确指标选择的必要前提。",
		EvidenceBoundary: "还需要逐类指标才能量化少数类被忽略的程度。",
		NextQuestion:     "请用你矩阵里的数字说明：多数类占比多少？如果全部预测为多数类，整体准确率会是多少？",
	},
	{
		Code:           "CL-RUN-ONLY",
		TaskTypes:      []string{content.TaskClassification, content.TaskGeneric, content.TaskArchitecture},
		KnowledgePoint: "程序成功与概念掌握的区分",
		Groups: []content.KeywordGroup{
			{"跑通", "运行成功", "没有报错", "不报错", "能运行", "跑完"},
			{"学会", "掌握", "懂了", "没问题了", "可以了"},
		},
		Status:           ClaimError,
		Judgment:         "把程序运行成功当成了已经掌握知识。",
		Reason:           "程序可执行只能证明接口调用正确，不能证明能解释结果含义、选择合适指标或判断结论的适用边界。",
		EvidenceBoundary: "运行日志能证明流程可执行，不能证明对结果的理解。",
		NextQuestion:     "请用两三句话说明本次实验结果的含义，并指出一个会让该结论不成立的数据情形。",
	},
	{
		Code:           "CL-GOAL-METRIC",
		TaskTypes:      []string{content.TaskClassification},
		KnowledgePoint: "任务目标与错误代价决定指标",
		Groups: []content.KeywordGroup{
			{"任务目标", "漏诊", "漏报", "误报", "代价", "成本", "风险", "应用场景"},
			{"指标", "召回率", "精确率", "阈值", "主指标"},
		},
		Status:           ClaimMastered,
		Judgment:         "把指标选择与任务目标或错误代价联系起来了。",
		Reason:           "指标应由任务决定，而不是默认使用准确率；这一点说明判断依据已经落到任务层面。",
		EvidenceBoundary: "还需要说明具体阈值与可接受的错误范围。",
		NextQuestion:     "请给出本任务中可接受的错误范围，并说明你会据此把阈值设在哪里。",
	},
	{
		Code:             "CL-NUMBER-ONLY",
		TaskTypes:        []string{content.TaskClassification},
		KnowledgePoint:   "结果数值的解释",
		Groups:           []content.KeywordGroup{{"准确率", "accuracy", "召回率", "精确率", "f1", "%"}},
		RequireNumber:    true,
		Status:           ClaimPartial,
		Judgment:         "引用了具体数值，但还没有说明这个数值支持什么结论。",
		Reason:           "数值是可核查的证据，但单独一个数字无法说明模型是否满足任务要求。",
		EvidenceBoundary: "现有材料只能核对数值本身，无法据此判断各类别表现。",
		NextQuestion:     "请说明这个数值是在什么条件下得到的，以及它无法回答哪个问题。",
	},
	{
		Code:           "RG-R2-ONLY",
		TaskTypes:      []string{content.TaskRegression},
		KnowledgePoint: "误差指标与任务可接受范围",
		Groups: []content.KeywordGroup{
			{"r2", "r²", "决定系数", "拟合优度"},
			{"很好", "不错", "可以", "够用", "直接用", "没问题", "误差很小"},
		},
		Status:           ClaimError,
		Judgment:         "只用 R² 或“误差很小”就断定模型可用。",
		Reason:           "R² 衡量模型解释了目标变量多少比例的方差，它不携带误差的量纲，也无法说明这个误差在业务上能否接受；能否使用必须回到 MAE / RMSE 的绝对量级与任务允许范围。",
		EvidenceBoundary: "现有材料能说明训练集上的拟合程度，不足以说明在未见过数据上的偏差。",
		NextQuestion:     "请把 MAE 或 RMSE 换算成目标变量的单位，并说明本任务可以接受的误差上限是多少。",
	},
	{
		Code:           "RG-TRAIN-ONLY",
		TaskTypes:      []string{content.TaskRegression},
		KnowledgePoint: "训练集与验证集表现差异",
		Groups: []content.KeywordGroup{
			{"训练集", "训练数据", "同一批数据", "全量数据"},
			{"误差", "mae", "rmse", "mse"},
		},
		Exclude:          []string{"验证集", "测试集", "泛化", "划分", "过拟合", "未见", "新数据"},
		Status:           ClaimError,
		Judgment:         "用训练集上的误差代替对未见过数据的估计。",
		Reason:           "在训练数据上评估误差会低估真实误差：模型可能记住了训练样本。回归任务应当报告留出验证集或交叉验证上的误差。",
		EvidenceBoundary: "当前只有训练集指标，无法据此判断泛化表现。",
		NextQuestion:     "请说明你会如何留出验证集或做交叉验证，并给出验证集上要报告的误差指标。",
	},
	{
		Code:           "RG-UNIT",
		TaskTypes:      []string{content.TaskRegression},
		KnowledgePoint: "误差数值的业务含义",
		Groups: []content.KeywordGroup{
			{"mae", "rmse", "均方误差", "平均绝对误差", "误差"},
			{"单位", "万元", "千瓦时", "量纲", "相对", "百分比", "百分之", "占比"},
		},
		Status:           ClaimMastered,
		Judgment:         "把误差数值换算成了可解释的量级。",
		Reason:           "误差只有在目标变量的单位与相对量级下才能判断是否可接受，这一步说明解释已经落到业务含义上。",
		EvidenceBoundary: "还需要说明该误差是否满足任务的可接受标准。",
		NextQuestion:     "请写出本任务可接受的误差上限，并说明它来自哪一条业务要求。",
	},
	{
		Code:           "CLU-SUPERVISED",
		TaskTypes:      []string{content.TaskClustering},
		KnowledgePoint: "无监督任务的评价方式",
		Groups: []content.KeywordGroup{
			{"准确率", "accuracy", "精确率", "召回率", "f1", "混淆矩阵"},
			{"聚类", "分群", "无监督", "簇"},
		},
		Status:           ClaimError,
		Judgment:         "用需要真实标签的指标评价没有标签的聚类结果。",
		Reason:           "准确率、召回率与混淆矩阵都要与真实标签比较才能计算；聚类没有标签，只能看簇内与簇间的结构，或借助外部业务判断。",
		EvidenceBoundary: "现有材料能说明分群结果，但无法支撑任何有监督指标。",
		NextQuestion:     "请改用轮廓系数、惯性等只依赖数据的指标，并说明每个簇在业务上代表什么。",
	},
	{
		Code:           "CLU-METRIC-ONLY",
		TaskTypes:      []string{content.TaskClustering},
		KnowledgePoint: "内部指标的含义与局限",
		Groups: []content.KeywordGroup{
			{"轮廓系数", "silhouette", "惯性", "inertia", "sse"},
			{"很好", "不错", "可以直接用", "直接用", "够用", "可接受", "没问题"},
		},
		Status:           ClaimError,
		Judgment:         "只用一个内部指标的数值就断定分群可用。",
		Reason:           "轮廓系数与惯性只描述几何结构，不携带业务含义；惯性还会随簇数增加而单调下降，不能单独用来选择簇数或判定可用性。",
		EvidenceBoundary: "该数值只在特定簇数、特定特征处理下成立，尚未与业务含义和稳定性对照。",
		NextQuestion:     "请写出你尝试过的簇数与各自的内部指标，并说明最终选定的簇数对应哪些业务上的用户特征。",
	},
	{
		Code:           "CLU-MEANING",
		TaskTypes:      []string{content.TaskClustering},
		KnowledgePoint: "簇数与业务含义的匹配",
		Groups: []content.KeywordGroup{
			{"簇", "分群", "主题", "类用户"},
			{"业务", "含义", "解释", "画像", "对应", "代表", "营销", "场景"},
		},
		Status:           ClaimMastered,
		Judgment:         "把簇与业务含义对应起来了。",
		Reason:           "聚类结果只有能被业务解释才能使用，这一步说明已经越过了“只看指标数字”。",
		EvidenceBoundary: "还需要说明该分群在多簇数与不同初始化下是否稳定。",
		NextQuestion:     "请给出各簇的规模与关键特征，并说明哪个簇对应哪种运营策略。",
	},
	{
		Code:           "CLU-STANDARDIZE",
		TaskTypes:      []string{content.TaskClustering},
		KnowledgePoint: "特征标准化对距离度量的影响",
		Groups: []content.KeywordGroup{
			{"标准化", "归一化", "z-score", "scale", "量纲"},
			{"距离", "度量", "聚类", "kmeans", "k-means", "权重"},
		},
		Status:           ClaimMastered,
		Judgment:         "注意到特征量纲会主导距离度量。",
		Reason:           "K-means 等基于距离的算法会被量纲大的特征主导，因此聚类前需要先标准化，否则分群实际反映的是单个特征的范围。",
		EvidenceBoundary: "还需要说明标准化后的分群是否稳定、是否仍有业务解释。",
		NextQuestion:     "请说明你用了哪种标准化方式，并比较标准化前后的分群结果。",
	},
}

var claimSentenceSplitter = regexp.MustCompile(`[。！？；\n!?;]+`)
var claimNumberPattern = regexp.MustCompile(`[0-9]|％|%`)

// AnalyzeClaims 把学生解释拆分为独立的主张，并逐条给出判断、理由、证据边界与追问。
// 拆分的目的是避免把多个不同的推理问题合并成一句笼统的“主要问题”。
func AnalyzeClaims(in DiagnoseInput, taskType string, contradiction Contradiction) []Claim {
	text := strings.TrimSpace(in.Explanation)
	if text == "" {
		return []Claim{}
	}
	var claims []Claim
	usedRules := map[string]bool{}
	for _, sentence := range claimSentenceSplitter.Split(text, -1) {
		sentence = strings.TrimSpace(sentence)
		if len([]rune(sentence)) < 4 {
			continue
		}
		rule, ok := matchClaimRule(sentence, taskType)
		if !ok {
			claims = append(claims, unclassifiedClaim(sentence, taskType, in))
			continue
		}
		usedRules[rule.Code] = true
		claims = append(claims, claimFromRule(rule, sentence, taskType, in, contradiction))
		if len(claims) >= 8 {
			break
		}
	}
	// 同一知识点只保留一条主张，避免重复追问同一个问题。
	seen := map[string]bool{}
	out := make([]Claim, 0, len(claims))
	for i, c := range claims {
		c.ID = fmt.Sprintf("C%d", i+1)
		key := c.KnowledgePoint + "|" + c.Status
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, c)
	}
	return out
}

func matchClaimRule(sentence, taskType string) (claimRule, bool) {
	low := strings.ToLower(sentence)
	for _, r := range claimRules {
		if !ruleApplies(r, taskType) {
			continue
		}
		if len(r.Exclude) > 0 && containsAny(low, r.Exclude) {
			continue
		}
		if !matchAllGroups(low, r.Groups) {
			continue
		}
		if r.RequireNumber && !claimNumberPattern.MatchString(sentence) {
			continue
		}
		return r, true
	}
	return claimRule{}, false
}

func ruleApplies(r claimRule, taskType string) bool {
	for _, t := range r.TaskTypes {
		if t == taskType {
			return true
		}
	}
	return false
}

func claimFromRule(r claimRule, sentence, taskType string, in DiagnoseInput, contra Contradiction) Claim {
	boundary := r.EvidenceBoundary
	if contra.Detected && r.Status == ClaimError {
		boundary = contra.Detail
	}
	return Claim{
		Quote:            sentence,
		KnowledgePoint:   r.KnowledgePoint,
		TaskType:         taskType,
		Status:           r.Status,
		StatusLabel:      ClaimStatusLabel(r.Status),
		Judgment:         r.Judgment,
		Reason:           r.Reason,
		Evidence:         "学生原话：" + sentence,
		EvidenceBoundary: boundary,
		NextQuestion:     r.NextQuestion,
		Signals:          []string{r.Code},
	}
}

// unclassifiedClaim 处理没有命中任何规则的主张：只做保守表述，不臆断学生是否掌握。
func unclassifiedClaim(sentence, taskType string, in DiagnoseInput) Claim {
	status := ClaimPartial
	judgment := "这句话还没有与当前任务需要回答的问题建立联系。"
	reason := "系统在这句话里没有找到可以直接核查的判断（例如某个指标、某段日志数值或某种因果说法），因此不会把它判定为错误。"
	next := "请针对当前问题补充一句可以直接被数据检验的判断：你依据哪一项数据？它支持什么结论？"
	if len([]rune(sentence)) >= 6 && !claimNumberPattern.MatchString(sentence) {
		status = ClaimNotMentioned
		judgment = "这句话涉及的内容与当前任务目标的关系还不明确。"
		reason = "没有提到与当前任务直接相关的指标、证据或推理，系统不据此判断你是否掌握相关知识点。"
	}
	return Claim{
		Quote:            sentence,
		KnowledgePoint:   "与当前任务的相关性",
		TaskType:         taskType,
		Status:           status,
		StatusLabel:      ClaimStatusLabel(status),
		Judgment:         judgment,
		Reason:           reason,
		Evidence:         "学生原话：" + sentence,
		EvidenceBoundary: "本句不构成掌握与否的证据。",
		NextQuestion:     next,
		Signals:          []string{"UNCLASSIFIED"},
	}
}

// BuildKnowledge 依据任务类型的关注点与学生主张，生成知识点状态清单。
// 没有讨论到的知识点标记为“未涉及”，并明确说明它不等于未掌握（需求文档 2.4）。
func BuildKnowledge(taskType string, claims []Claim) []KnowledgeItem {
	tt := content.TaskTypeOrDefault(taskType)
	items := make([]KnowledgeItem, 0, len(tt.FocusKnowledge))
	for i, name := range tt.FocusKnowledge {
		item := KnowledgeItem{
			Code:        fmt.Sprintf("K%d", i+1),
			Name:        name,
			Status:      ClaimNotMentioned,
			StatusLabel: ClaimStatusLabel(ClaimNotMentioned),
			Required:    true,
			Applicable:  true,
			Note:        "你没有在本次提交中讨论这一点。未涉及不等于未掌握，系统不会据此扣减进展。",
		}
		var matched []string
		for _, c := range claims {
			if c.KnowledgePoint == name {
				matched = append(matched, c.Status)
			}
		}
		if len(matched) > 0 {
			item.Status = pickWorstStatus(matched)
			item.StatusLabel = ClaimStatusLabel(item.Status)
			item.Note = knowledgeNote(item.Status)
		}
		items = append(items, item)
	}
	return items
}

func knowledgeNote(status string) string {
	switch status {
	case ClaimMastered:
		return "你的解释已经覆盖这一点，后续不会再重复追问。"
	case ClaimPartial:
		return "方向正确，但还缺少一个与当前目标相关的关键环节。"
	case ClaimError:
		return "你的说法与原理或提交的证据存在冲突，需要先修正再推进。"
	case ClaimInsufficient:
		return "结论需要的数据还没有提供，系统不会替你补全。"
	case ClaimNotApplicable:
		return "该知识点不属于当前实验目标，不作为你的不足项。"
	default:
		return "未涉及不等于未掌握。"
	}
}

func pickWorstStatus(statuses []string) string {
	for _, want := range claimStatusOrder {
		for _, got := range statuses {
			if got == want {
				return want
			}
		}
	}
	return ClaimNotMentioned
}

// BuildEvidenceNeeds 依据任务类型判断每项材料是否必需，并说明缺失限制了哪些结论。
func BuildEvidenceNeeds(in DiagnoseInput, taskType string) []EvidenceNeed {
	tt := content.TaskTypeOrDefault(taskType)
	out := make([]EvidenceNeed, 0, len(tt.Evidence))
	for _, spec := range tt.Evidence {
		present := evidencePresent(spec.Name, in)
		note := ""
		switch {
		case present && spec.Required:
			note = "已提供，可以支撑当前任务的结论。"
		case present:
			note = "已提供，可作为补充论据。"
		case spec.Required:
			note = "缺失，本任务的结论因此受限。"
		default:
			note = "缺失。它属于可选材料，不影响本次诊断成立，可进一步补充。"
		}
		impact := spec.Impact
		if !spec.Required && spec.Optional != "" {
			impact = spec.Optional
		}
		out = append(out, EvidenceNeed{
			Name:     spec.Name,
			Required: spec.Required,
			Present:  present,
			Impact:   impact,
			Note:     note,
		})
	}
	return out
}

func evidencePresent(name string, in DiagnoseInput) bool {
	switch {
	case strings.Contains(name, "解释"):
		return strings.TrimSpace(in.Explanation) != ""
	case strings.Contains(name, "簇数") || strings.Contains(name, "簇规模"):
		return containsAny(strings.ToLower(in.MetricsText+in.ExecutionLog+in.DatasetDescription), []string{"簇", "群", "k=", "k =", "cluster"})
	case strings.Contains(name, "轮廓系数") || strings.Contains(name, "内部指标"):
		return containsAny(strings.ToLower(in.MetricsText+in.ExecutionLog), []string{"轮廓", "silhouette", "惯性", "inertia", "sse", "肘部"})
	case strings.Contains(name, "特征与标准化"):
		return containsAny(strings.ToLower(in.DatasetDescription+in.CodeSnippet+in.MetricsText), []string{"标准化", "归一化", "z-score", "scale", "量纲", "特征"})
	case strings.Contains(name, "业务解释") || strings.Contains(name, "稳定性"):
		return containsAny(strings.ToLower(in.Explanation+in.MetricsText+in.ExecutionLog+in.DatasetDescription),
			[]string{"业务", "含义", "画像", "对应", "代表", "随机种子", "重复", "稳定"})
	case strings.Contains(name, "任务目标") || strings.Contains(name, "实验目标"):
		return strings.TrimSpace(in.ExperimentGoal) != "" || strings.TrimSpace(in.DatasetDescription) != ""
	case strings.Contains(name, "训练日志"):
		return strings.TrimSpace(in.ExecutionLog) != "" || strings.TrimSpace(in.CodeSnippet) != ""
	case strings.Contains(name, "报错"):
		return strings.TrimSpace(in.ExecutionLog) != ""
	case strings.Contains(name, "代码"):
		return strings.TrimSpace(in.CodeSnippet) != ""
	case strings.Contains(name, "混淆矩阵") || strings.Contains(name, "按类别"):
		return in.HaveMatrix || in.Metrics != nil
	case strings.Contains(name, "类别分布"):
		return in.HaveMatrix || containsAny(strings.ToLower(in.DatasetDescription), []string{"类", "样本", "条", "占比", "分布", "比例"})
	case strings.Contains(name, "模型结构") || strings.Contains(name, "训练集"):
		return strings.TrimSpace(in.ExperimentGoal) != "" || strings.TrimSpace(in.CodeSnippet) != "" || in.Metrics != nil
	case strings.Contains(name, "参数量"):
		return containsAny(strings.ToLower(in.MetricsText+in.ExecutionLog), []string{"参数", "耗时", "时间", "params"})
	case strings.Contains(name, "对照"):
		return containsAny(strings.ToLower(in.ExecutionLog), []string{"对照", "两组", "run"})
	case strings.Contains(name, "结果数据"):
		return strings.TrimSpace(in.MetricsText) != "" || in.Metrics != nil
	default:
		return strings.TrimSpace(in.ExecutionLog) != "" || in.Metrics != nil || strings.TrimSpace(in.MetricsText) != ""
	}
}
