package content

// 诊断类别：需求文档 5.1 功能 D 与 7 节诊断规则中定义的七类问题。
const (
	CategoryEnvironment     = "environment"      // 环境或代码问题
	CategoryConcept         = "concept"          // 概念理解问题
	CategoryDesign          = "design"           // 实验设计问题
	CategoryMetricSelection = "metric_selection" // 评价指标选择问题
	CategoryInterpretation  = "interpretation"   // 实验结果解释问题
	CategoryBoundary        = "boundary"         // 结论适用边界或模型局限性问题
	CategoryInsufficient    = "insufficient"     // 当前证据不足，暂时无法诊断
)

// Category 是一个诊断类别的注册信息。
type Category struct {
	Code  string `json:"code"`
	Label string `json:"label"`
}

// Categories 按需求文档顺序列出全部诊断类别。
var Categories = []Category{
	{Code: CategoryEnvironment, Label: "环境或代码问题"},
	{Code: CategoryConcept, Label: "概念理解问题"},
	{Code: CategoryDesign, Label: "实验设计问题"},
	{Code: CategoryMetricSelection, Label: "评价指标选择问题"},
	{Code: CategoryInterpretation, Label: "实验结果解释问题"},
	{Code: CategoryBoundary, Label: "结论适用边界或模型局限性问题"},
	{Code: CategoryInsufficient, Label: "当前证据不足，暂时无法诊断"},
}

// CategoryLabel 返回类别中文名称。
func CategoryLabel(code string) string {
	for _, c := range Categories {
		if c.Code == code {
			return c.Label
		}
	}
	return code
}

// KeywordGroup 是一组同义关键词，任意命中一个即认为该组命中。
type KeywordGroup []string

// HintSet 是三级提示文本。
type HintSet struct {
	Level1 string `json:"level1"`
	Level2 string `json:"level2"`
	Level3 string `json:"level3"`
}

// Archetype 是需求文档 7 节要求的典型误区规则：触发语句、知识点、诊断理由、三级提示、
// 迁移题建议与教师介入条件。
type Archetype struct {
	Code                  string         `json:"code"`
	Name                  string         `json:"name"`
	KnowledgePoint        string         `json:"knowledgePoint"`
	Groups                []KeywordGroup `json:"-"`
	Exclude               []string       `json:"-"`
	RequiresImbalance     bool           `json:"-"`
	RequiresZeroRecall    bool           `json:"-"`
	DirectTroubleshooting bool           `json:"-"`
	Categories            []string       `json:"categories"`
	DiagnosisReason       string         `json:"diagnosisReason"`
	Hints                 HintSet        `json:"hints"`
	NextStep              string         `json:"nextStep"`
	TransferItemIDs       []string       `json:"transferItemIds"`
	TeacherCondition      string         `json:"teacherCondition"`
}

// RawKeywords 仅用于前端展示规则触发词（便于回归测试与教学演示说明）。
func (a Archetype) RawKeywords() []string {
	var out []string
	for _, g := range a.Groups {
		out = append(out, g...)
	}
	return out
}

// Archetypes 是内置的典型误区规则库（E0 为环境排障，E1—E5 对应需求文档表格）。
var Archetypes = []Archetype{
	{
		Code:                  "E0",
		Name:                  "环境或代码异常",
		KnowledgePoint:        "运行环境、依赖与错误定位",
		DirectTroubleshooting: true,
		Exclude:               []string{"没有报错", "不报错", "无报错", "没有错误", "未报错", "without error", "no error"},
		Groups: []KeywordGroup{
			{"报错", "error", "Error", "ERROR", "traceback", "Traceback", "异常", "崩溃", "闪退", "跑不起来", "运行失败", "ModuleNotFoundError", "ImportError", "No module named", "cannot import", "版本冲突", "依赖冲突"},
		},
		Categories:      []string{CategoryEnvironment},
		DiagnosisReason: "提交内容提到程序报错、依赖缺失或环境异常。这类问题属于工程排障，不适合用苏格拉底式提问绕圈，应先给出可直接执行的排查步骤。",
		Hints: HintSet{
			Level1: "先不看模型结论，先把报错本身读清楚：报错最后一行通常写着缺失的模块或出错的代码行。请把完整报错信息（含最后 3—5 行）贴出来，再对照下面的排查顺序。",
			Level2: "大多数 scikit-learn 实验环境问题来自三类原因：依赖未安装或版本不匹配、数据路径/编码错误、随机种子或数据形状不一致。请在终端依次执行 `python -c \"import sklearn; print(sklearn.__version__)\"` 与 `pip list | grep -i -E \"scikit|pandas|numpy\"`，确认版本与课程环境要求一致。",
			Level3: "可直接执行的排查步骤：1) 复制报错最后一行的模块名，执行 `pip install <模块名>`；2) 若报 FileNotFoundError，用 `os.path.abspath()` 打印实际读取路径，确认数据文件位置；3) 若报形状不一致，打印 `X.shape`、`y.shape` 与 `len(y)`；4) 仍然失败时，把“完整报错 + 相关代码片段 + 数据规模”一起记录，按下面的教师介入条件处理。",
		},
		NextStep:         "按三级提示中的排查顺序逐条执行，把执行后的完整报错或成功输出重新提交，再回到结果解释环节。",
		TransferItemIDs:  []string{"T3"},
		TeacherCondition: "若按排障步骤执行后仍报错，且错误涉及课程环境本身的版本问题（例如集体出现同一 ImportError），建议教师统一检查实验环境镜像。",
	},
	{
		Code:              "E1",
		Name:              "以整体准确率代替全部评价（忽视类别不平衡）",
		KnowledgePoint:    "准确率在不平衡数据上的误导性、各类别召回率、混淆矩阵",
		RequiresImbalance: true,
		Groups: []KeywordGroup{
			{"准确率", "accuracy", "acc", "正确率"},
			{"很好", "不错", "挺好", "优秀", "没问题", "肯定好", "效果好", "效果不错", "表现好", "理想", "完美", "很高", "达到", "较高"},
		},
		Categories:      []string{CategoryInterpretation, CategoryMetricSelection},
		DiagnosisReason: "解释只用整体准确率下结论，没有查看按类别分解的指标。在类别分布不平衡的数据上，整体准确率会被多数类主导，无法说明少数类的识别能力，因此这一判断缺少证据支撑。",
		Hints: HintSet{
			Level1: "先不要只看整体准确率。请观察混淆矩阵中真实类别 B 和 C 对应的两行：这两个类别的样本分别被预测成了什么？你认为这说明模型对每个类别的识别能力相同吗？",
			Level2: "准确率等于“正确样本数 / 总样本数”，它对每个样本等权，因此当某一类样本占绝大多数时，只要模型一直预测多数类，准确率也会很高。判断模型能不能用，需要回到按类别分解的指标：每类的召回率（该真实类别的样本有多少被找出来）和精确率（被判为该类的样本有多少是对的）。请把混淆矩阵逐行读一遍，分别算出各真实类别的召回率，再和整体准确率作比较。",
			Level3: "局部示例：若混淆矩阵中真实类别 B 一行是“被预测为 A 的 90 个、其余为 0”，则 B 的召回率 = 0 / 90 = 0%；类别 C 同理为 0 / 10 = 0%；而整体准确率仍为 900/1000 = 90%。请用同样的方式算一下你这份矩阵里每个类别的召回率，再回答：哪些类别实际上没有被识别出来？",
		},
		NextStep:         "请逐类计算召回率，并回答：在本次任务目标下，模型对少数类是否可用？如果不可用，你打算从数据层面还是损失函数层面改进？",
		TransferItemIDs:  []string{"T1"},
		TeacherCondition: "若学生连续两轮仍只重复整体准确率、未触及按类别指标，建议教师结合类别分布专门讲解一次“准确率陷阱”。",
	},
	{
		Code:           "E2",
		Name:           "只关注混淆矩阵对角线",
		KnowledgePoint: "混淆矩阵的非对角元素与类别间错误分布",
		Groups: []KeywordGroup{
			{"对角线"},
			{"只看", "只要", "只需要", "就够", "足够", "不用看", "不需要", "不用管", "忽略", "光看"},
		},
		Categories:      []string{CategoryInterpretation, CategoryConcept},
		DiagnosisReason: "把混淆矩阵简化为对角线上的正确数，等于只统计了正确率，丢掉了错误发生在哪些类别之间的信息，也就无法定位模型偏向。",
		Hints: HintSet{
			Level1: "对角线只能告诉你“预测对了多少”，不能告诉你“错在哪里”。请挑一行非对角线上的数字，问自己：这些样本本来是哪个类别，被模型认成了什么？",
			Level2: "混淆矩阵的第 i 行第 j 列表示“真实是类别 i、却被预测为类别 j”的样本数。对角线以外的数字正是错误的具体去向：它们反映模型倾向于把哪些类别混在一起。请列出错误最多的那一对类别（真实 X → 预测 Y）。",
			Level3: "局部示例：在“真实 B 行的 90 个样本全部落在预测 A 列”的情况下，非对角线元素 90 说明模型把整个 B 类判成了 A 类，B 的召回率为 0%。请在你的矩阵中指出错误最集中的行与列，并说明它对应哪一种实际风险（例如漏检某一类别）。",
		},
		NextStep:         "请写出你这份矩阵中错误最集中的“真实类别 → 预测类别”组合，并说明该错误在任务场景中意味着什么风险。",
		TransferItemIDs:  []string{"T2"},
		TeacherCondition: "若学生仍无法解释行与列的含义（把行当成预测类别），建议教师用一次板书示例澄清矩阵方向约定。",
	},
	{
		Code:           "E3",
		Name:           "假设各类别召回率大致相同",
		KnowledgePoint: "分类器对多数类的偏向、逐类召回率比较",
		Groups: []KeywordGroup{
			{"召回率", "recall", "查全率"},
			{"差不多", "大致相同", "都一样", "一样", "都差不多", "应该相同", "不会差", "都高", "都很好", "很接近", "均衡"},
		},
		Categories:      []string{CategoryConcept, CategoryInterpretation},
		DiagnosisReason: "在没有逐类计算的情况下假定各类别召回率接近，属于用先验直觉代替实证检查；类别不平衡时，分类器通常会偏向多数类，少数类召回率往往显著偏低。",
		Hints: HintSet{
			Level1: "这个“差不多”是怎么得到的？请先在混淆矩阵上把每个真实类别那一行的正确数除以该行总数，逐个算出召回率，再回来看是否接近。",
			Level2: "多数分类算法以整体损失最小为目标，当类别样本数相差悬殊时，把少数类判成多数类往往能换来更低的总损失，因此模型会系统性地偏向多数类。召回率是否接近是一个需要逐类验证的经验问题，不能靠默认假设。",
			Level3: "局部示例：若真实类别 A 有 900 个样本、B 有 90 个、C 有 10 个，而模型全部预测为 A，则三类召回率分别是 100%、0%、0%——差异极大。请按此方法列出你矩阵中各类别的样本数与召回率，并指出差异最大的类别。",
		},
		NextStep:         "请列出每个类别的样本数与召回率，并回答：召回率的差异与类别样本数之间是否存在关系？为什么？",
		TransferItemIDs:  []string{"T1"},
		TeacherCondition: "若学生算出逐类召回率后仍认为差异属于随机波动，建议教师补充一节关于类别不平衡对损失函数影响的讲解。",
	},
	{
		Code:           "E4",
		Name:           "指标选择未结合任务目标",
		KnowledgePoint: "漏报与误报的代价差异、指标与任务目标的匹配",
		Exclude:        []string{"漏报", "漏诊", "误报", "误诊", "代价", "成本", "任务目标", "业务", "风险", "召回率", "精确率"},
		Groups: []KeywordGroup{
			{"提高", "提升", "优化", "增加", "加大", "改进"},
			{"准确率", "accuracy", "acc"},
		},
		Categories:      []string{CategoryMetricSelection, CategoryDesign},
		DiagnosisReason: "把“继续提高准确率”当作唯一改进方向，而没有说明任务目标、错误代价与指标选择依据。指标应当由任务决定，而不是默认选择准确率。",
		Hints: HintSet{
			Level1: "在回答“怎样提高”之前，先回答“这个任务里哪种错误更不能接受”。漏掉一个正例和误判一个正例，代价一样吗？",
			Level2: "不同任务对错误的容忍度不同：筛检类任务更怕漏报，倾向关注召回率并接受一定误报；给用户强提醒的任务更怕误报，倾向关注精确率。请说明你的实验场景属于哪一类，以及对应的主指标是什么、辅助指标是什么。",
			Level3: "局部示例：若“未知样本”占比 90%，把所有样本都判为零风险，准确率可达 90%，但漏报率 100%。这说明单看准确率无法反映任务是否达标。请写出你任务的两种错误（假阳性、假阴性）各自的实际后果，再据此给出主指标与可接受的阈值范围。",
		},
		NextStep:         "请分别写出本任务中假阳性与假阴性的实际后果，并据此说明应当以哪个指标为主指标、以哪个为辅助指标。",
		TransferItemIDs:  []string{"T1"},
		TeacherCondition: "若学生无法说出任务目标与错误代价，说明实验背景理解不足，建议教师补充任务书解读环节。",
	},
	{
		Code:           "E5",
		Name:           "把程序运行成功等同于掌握知识",
		KnowledgePoint: "程序成功与概念掌握的区分、结果解释与迁移",
		Groups: []KeywordGroup{
			{"代码", "程序", "跑通", "运行成功", "能运行", "编译通过", "跑完", "顺利完成", "没有报错"},
			{"学会", "掌握", "懂了", "会了", "没问题了", "学会了", "已经理解", "完成学习"},
		},
		Categories:      []string{CategoryConcept, CategoryBoundary, CategoryDesign},
		DiagnosisReason: "把“没有报错”当作“已经学会”。程序跑通只能证明代码可执行，不能证明能解释结果、判断模型局限；需要补充对结果含义、指标选择与适用边界的说明。",
		Hints: HintSet{
			Level1: "代码跑通说明流程没问题，但它没有回答三个问题：结果说明了什么？哪个指标支撑结论？这个结论在什么情况下会失效？请先回答第一个。",
			Level2: "“能运行”和“能解释”是两件事：能运行只说明接口调用正确，能解释需要说清数据、标签、指标与结论之间的关系。请用 2—3 句话说明本次实验结果的含义，并指出一个你认为模型可能失效的场景。",
			Level3: "局部示例：同样的代码换到样本更少或类别更不平衡的数据上，结论可能完全不同。请设计一个小验证：把某一类样本数量减半，重新训练并比较各召回率的变化，写出你预测的结果与理由。",
		},
		NextStep:         "请写出本次实验结论的适用条件，并说出一个会让该结论不成立的数据情形。",
		TransferItemIDs:  []string{"T3"},
		TeacherCondition: "若学生无法说出任何适用边界，建议教师在实验课中增加“结果解释口述”环节。",
	},
}
