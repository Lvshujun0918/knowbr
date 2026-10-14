package content

// 任务类型：每次诊断、提示与迁移题生成前必须先确定当前实验属于哪一类任务，
// 生成内容不得越出该类型的知识范围（需求文档 2.2 / 2.9 / 3.4）。
const (
	TaskCodeEnv        = "code_environment"       // 环境与代码排障
	TaskTraining       = "training_stability"     // 训练稳定性（损失、梯度、学习率）
	TaskArchitecture   = "model_architecture"     // 模型结构与容量
	TaskClassification = "classification_metrics" // 分类指标与类别不平衡
	TaskGeneric        = "generic"                // 未识别到明确任务类型
)

// EvidenceSpec 描述一项材料在当前任务中的必要性。
// Required 为 true 时缺失会导致无法得出结论；为 false 时只能作为可选补充。
type EvidenceSpec struct {
	Name     string `json:"name"`
	Required bool   `json:"required"`
	Impact   string `json:"impact"` // 缺失时限制了什么结论
	Optional string `json:"optional"`
}

// TaskType 是一个任务类型的约束集合：优先讨论什么、必须有什么证据、不得引入什么。
type TaskType struct {
	Code             string         `json:"code"`
	Label            string         `json:"label"`
	FocusKnowledge   []string       `json:"focusKnowledge"`
	Evidence         []EvidenceSpec `json:"evidence"`
	OffTopic         []string       `json:"offTopic"`
	NextLearning     []string       `json:"nextLearning"`
	CompletenessNote string         `json:"completenessNote"`
}

// TaskTypes 按判定优先级排列：命中靠前的类型优先。
var TaskTypes = []TaskType{
	{
		Code:  TaskCodeEnv,
		Label: "环境与代码排障",
		FocusKnowledge: []string{
			"报错信息定位",
			"依赖与版本一致性",
			"数据路径与形状检查",
		},
		Evidence: []EvidenceSpec{
			{Name: "完整报错信息", Required: true, Impact: "没有报错原文时只能给出通用排查顺序，无法定位具体原因。"},
			{Name: "相关代码片段", Required: false, Optional: "有代码片段时可以指出更具体的出错位置。"},
		},
		OffTopic:         []string{"概念辨析式提问"},
		NextLearning:     []string{"按排查顺序执行后重新提交运行结果"},
		CompletenessNote: "环境问题按排障处理，不做概念掌握判断。",
	},
	{
		Code:  TaskTraining,
		Label: "训练稳定性与优化",
		FocusKnowledge: []string{
			"训练损失与梯度范数的变化",
			"假设与结论的区分",
			"日志中的事实、推断与未知",
			"学习率与梯度裁剪的作用差异",
			"单变量对照实验设计",
		},
		Evidence: []EvidenceSpec{
			{Name: "训练日志（含 loss 与梯度范数等数值）", Required: true, Impact: "没有逐轮日志时无法判断异常出现的时机，也无法比较不同配置。"},
			{Name: "实验目标与该模型/数据的基本设置", Required: true, Impact: "没有任务目标与设置时无法判断当前损失水平是否异常。"},
			{Name: "关键代码片段（优化器、学习率、裁剪设置）", Required: false, Optional: "有代码片段时可以定位到具体配置项。"},
			{Name: "对照实验记录", Required: false, Optional: "有对照记录时可以把假设缩小到更少的变量。"},
		},
		OffTopic: []string{
			"混淆矩阵", "按类别的召回率/精确率/F1", "类别分布与少数类",
			"疾病筛查的漏诊与误报", "垃圾分类", "筛选任务的指标选择",
		},
		NextLearning: []string{
			"区分日志中的事实、推断与未知信息",
			"用单变量对照实验检验梯度爆炸假设",
		},
		CompletenessNote: "训练稳定性任务不需要混淆矩阵或分类指标，缺失这些材料不影响本任务的分析。",
	},
	{
		Code:  TaskArchitecture,
		Label: "模型结构与容量",
		FocusKnowledge: []string{
			"全连接与卷积的结构差异（局部连接、权重共享）",
			"图像的空间结构信息",
			"模型容量与参数量",
			"训练集与测试集表现差异（过拟合）",
			"推理效率与结构代价",
		},
		Evidence: []EvidenceSpec{
			{Name: "实验目标与对比的模型结构", Required: true, Impact: "没有对比对象时无法判断结构差异是否解释了性能差异。"},
			{Name: "两种模型在训练集与测试集上的结果", Required: true, Impact: "只有单一测试集结果时无法区分拟合能力差异与泛化差异。"},
			{Name: "参数量或训练耗时", Required: false, Optional: "有参数量/耗时数据时可以讨论效率代价。"},
			{Name: "逐类结果或混淆矩阵", Required: false, Optional: "有逐类结果时可以进一步分析错误模式。"},
		},
		OffTopic:         []string{"疾病筛查的漏诊与误报", "类别不平衡下的指标轮换"},
		NextLearning:     []string{"结合训练集与测试集表现判断过拟合", "用参数量与推理代价说明结构选择的取舍"},
		CompletenessNote: "结构比较任务优先看结构差异与拟合情况，分类别指标属于可选补充。",
	},
	{
		Code:  TaskClassification,
		Label: "分类指标与类别不平衡",
		FocusKnowledge: []string{
			"整体准确率在类别不平衡下的误导性",
			"按类别的召回率与精确率",
			"混淆矩阵中错误的去向",
			"任务目标与错误代价",
		},
		Evidence: []EvidenceSpec{
			{Name: "学生自己的实验解释", Required: true, Impact: "没有学生自己的判断就无法诊断其理解情况。"},
			{Name: "混淆矩阵或按类别的指标", Required: true, Impact: "没有逐类结果时无法计算召回率，只能针对解释的推理依据给出反馈。"},
			{Name: "类别分布", Required: true, Impact: "不知道各类别样本量时无法判断整体准确率是否被多数类主导。"},
			{Name: "任务目标", Required: false, Optional: "有任务目标时才能判断哪种错误代价更高。"},
			{Name: "运行日志或代码片段", Required: false, Optional: "有运行证据时可以核对结果是否来自实际运行。"},
		},
		OffTopic:         []string{"训练稳定性与梯度", "模型结构的归纳偏置"},
		NextLearning:     []string{"逐类计算召回率并解释错误去向", "结合任务目标说明哪种错误更不能接受"},
		CompletenessNote: "分类指标任务的核心证据是逐类结果与类别分布。",
	},
	{
		Code:  TaskGeneric,
		Label: "未识别到明确任务类型",
		FocusKnowledge: []string{
			"结论依据的证据",
			"结论的适用边界",
		},
		Evidence: []EvidenceSpec{
			{Name: "学生自己的实验解释", Required: true, Impact: "没有学生自己的判断就无法诊断其理解情况。"},
			{Name: "实验目标", Required: true, Impact: "没有任务目标时只能讨论通用推理原则。"},
			{Name: "结果数据", Required: false, Optional: "有数据时才能核对结论是否被证据支持。"},
		},
		OffTopic:         []string{"与当前实验无关的其它任务指标"},
		NextLearning:     []string{"把结论与它依据的证据对应起来"},
		CompletenessNote: "任务类型不明确时只做通用推理诊断，不引入任何具体任务的指标要求。",
	},
}

// TaskTypeByCode 按编号查找任务类型。
func TaskTypeByCode(code string) (TaskType, bool) {
	for _, t := range TaskTypes {
		if t.Code == code {
			return t, true
		}
	}
	return TaskType{}, false
}

// TaskTypeOrDefault 返回任务类型，未识别时返回通用类型。
func TaskTypeOrDefault(code string) TaskType {
	if t, ok := TaskTypeByCode(code); ok {
		return t
	}
	t, _ := TaskTypeByCode(TaskGeneric)
	return t
}

// TaskTypeLabel 返回任务类型中文名称。
func TaskTypeLabel(code string) string {
	if t, ok := TaskTypeByCode(code); ok {
		return t.Label
	}
	return TaskTypeOrDefault(TaskGeneric).Label
}
