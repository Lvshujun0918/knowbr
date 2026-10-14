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
	TaskTypes             []string       `json:"taskTypes"` // 适用的任务类型；为空表示不限
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
		TaskTypes:             []string{TaskCodeEnv},
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
		TaskTypes:         []string{TaskClassification},
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
		TaskTypes:      []string{TaskClassification},
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
		TaskTypes:      []string{TaskClassification},
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
		TaskTypes:      []string{TaskClassification},
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
		TaskTypes:      []string{TaskClassification, TaskArchitecture, TaskGeneric},
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
	{
		Code:           "E6",
		Name:           "训练异常归因缺少可区分的检查",
		KnowledgePoint: "训练损失与梯度范数的变化、假设与结论的区分",
		TaskTypes:      []string{TaskTraining},
		Groups: []KeywordGroup{
			{"梯度爆炸", "梯度", "grad_norm", "梯度范数", "loss", "损失", "nan", "不稳定", "发散"},
			{"可能", "也许", "不确定", "不能确定", "无法确定", "怀疑", "推测", "应该是", "肯定是", "一定是", "因为"},
		},
		Categories:      []string{CategoryInterpretation, CategoryDesign},
		DiagnosisReason: "日志记录了损失与梯度范数的变化，这是事实；把原因归结为梯度爆炸或学习率偏高属于推断。当前材料还不能排除损失或数据中的非有限值、实现错误等同样会产生该现象的原因，也还没有提供能把这些原因区分开的检查结果，因此假设有依据但尚未证实。",
		Hints: HintSet{
			Level1: "先把日志拆成三类信息：第 3—5 轮记录了什么数值（事实）、你对原因的判断（推断）、以及日志里查不到的信息（未知）。请各写一行，并指出哪一项检查能把“梯度爆炸”与“损失或数据中出现非有限值”区分开。",
			Level2: "梯度裁剪限制单步更新的幅度上限，学习率决定每次更新的基准尺度：两者都能抑制发散，但改变的不是同一件事——裁剪不改变更新方向，学习率改变步长。要把原因缩小到一个变量，需要单变量对照：固定其它设置，分别只调学习率、只加裁剪，并在训练前检查输入与损失是否出现非有限值，再比较 loss 与梯度范数的曲线。",
			Level3: "局部示例（与你的训练稳定性问题同类型）：如果某轮梯度范数从 18.9 涨到 95.3、下一轮 loss 变成 NaN，可以确定的事实是“训练在该轮出现数值异常”，不能确定的是它由梯度爆炸、损失计算中的非有限值还是实现错误引起。最小对照可以这样设计：把学习率降为原来的十分之一再跑一次——若仍然出现 NaN，说明问题不只是学习率偏大；在训练循环里加一行断言检查输入与损失是否为有限值，可以排除数据或实现问题。",
		},
		NextStep:         "请按“事实 / 推断 / 未知”各写一行，并给出一个只改变单一变量的检查方案，说明什么结果会推翻“梯度爆炸”这一假设。",
		TeacherCondition: "若学生连续两轮仍无法区分日志中的事实与推断，建议教师面谈演示一次单变量对照的排查过程。",
	},
	{
		Code:           "E8",
		Name:           "把模型复杂度当作准确率更高的必然原因",
		KnowledgePoint: "全连接与卷积的结构差异",
		TaskTypes:      []string{TaskArchitecture},
		Groups: []KeywordGroup{
			{"复杂", "复杂度", "参数多", "层数多", "更大"},
			{"所以", "因此", "说明", "一定", "必然", "肯定", "导致", "就能"},
		},
		Categories:      []string{CategoryConcept, CategoryInterpretation},
		DiagnosisReason: "把“结构更复杂”当成了准确率更高的原因，跳过了具体的结构差异。卷积与全连接的区别在于局部连接、权重共享以及对图像空间信息的利用，是否带来更好的分类效果需要由这些机制和对照结果来解释，而不是由“更复杂”直接推出。",
		Hints: HintSet{
			Level1: "先把这句话拆成两半：一半是你在结构上的判断，一半是“更复杂所以更准”的因果判断。哪一半是数据支持的？哪一半只是推断？",
			Level2: "复杂度只说明假设空间更大，它既可能带来更强的拟合能力，也可能带来更高的过拟合风险。两个模型真正的结构差异在于：MLP 需要把图像展平成一维向量，空间关系被打破；CNN 通过局部连接与权重共享提取局部特征，参数随通道数增长而不是随像素数增长。",
			Level3: "局部示例：若 CNN 用了 3 万个参数就达到 92%，而 MLP 用了 60 万个参数只有 85%，那“更复杂所以更准”就无法解释——参数更少的模型反而更好。请用你实验里的参数量与准确率数字做同样的比对，再说明结构差异在其中起什么作用。",
		},
		NextStep:         "请写出两个模型在结构上的两处具体差异，并说明每一处为什么可能影响图像分类效果。",
		TransferItemIDs:  []string{},
		TeacherCondition: "若学生仍以“模型更复杂”解释全部差异，建议教师在实验课上安排一次参数量与准确率的对照统计。",
	},
	{
		Code:           "E9",
		Name:           "认为增加模型容量必然提升性能",
		KnowledgePoint: "模型容量与数据规模的匹配",
		TaskTypes:      []string{TaskArchitecture},
		Groups: []KeywordGroup{
			{"增加", "加大", "继续加", "更多", "堆"},
			{"隐藏层", "神经元", "层数", "参数", "容量"},
			{"一定", "必然", "肯定", "就能", "超过", "保证"},
		},
		Categories:      []string{CategoryDesign, CategoryBoundary},
		DiagnosisReason: "容量增大只提高拟合能力的上限，能否转化为测试表现的提升取决于数据规模、正则化与训练是否充分；在数据量有限时，容量增大往往先表现为训练集与测试集表现差距扩大，而不是整体变好。",
		Hints: HintSet{
			Level1: "如果给 MLP 增加隐藏层，你预期训练集准确率和测试集准确率分别会怎么变？为什么要分开看这两个数字？",
			Level2: "模型容量、数据量、正则化三者共同决定泛化表现。只增加容量而不增加数据或正则化时，模型更容易把训练样本记住，训练集表现上升但测试集表现不一定上升。判断加层是否有效，需要同时记录训练集与测试集的表现，并保持数据划分一致。",
			Level3: "局部示例：如果一个模型训练集准确率 99%、测试集 91%，而加层后训练集升到 100%、测试集降到 88%，说明增加的容量被用去记忆训练样本。请按这个格式写下你实验中的两个数字，再判断加层是否值得。",
		},
		NextStep:         "请写出增加容量后你会记录哪些对照数据，并说明哪一种结果会让你放弃“加层就能超过 CNN”的想法。",
		TeacherCondition: "若学生无法说出训练集与测试集的区别，建议教师先补充一节过拟合与泛化的讲解。",
	},
	{
		Code:           "E10",
		Name:           "只用单一测试结果下结论，未比较训练集与测试集表现",
		KnowledgePoint: "训练集与测试集表现差异（过拟合）",
		TaskTypes:      []string{TaskArchitecture},
		Exclude:        []string{"训练集", "过拟合", "泛化", "验证集"},
		Groups: []KeywordGroup{
			{"准确率", "accuracy", "测试集", "结果"},
			{"最高", "更好", "超过", "所以", "说明"},
		},
		Categories:      []string{CategoryInterpretation, CategoryBoundary},
		DiagnosisReason: "只用单一测试结果下结论时，无法区分“模型拟合能力更强”与“模型在新数据上表现更好”。要判断结构差异的影响，至少需要同时给出训练集与测试集上的表现。",
		Hints: HintSet{
			Level1: "这两个准确率分别是在哪个数据集上得到的？请把训练集与测试集的数字分别写出来，再比较差距。",
			Level2: "训练集表现反映拟合能力，测试集表现反映在新数据上的效果，两者的差距才是判断过拟合的依据。只看一个测试数字，无法知道模型是把特征学到了还是把训练样本记住了。",
			Level3: "局部示例：如果两个模型测试集都是 90%，一个训练集 91%、另一个训练集 100%，后者说明容量大量用于记忆训练样本。请按这个格式补全你实验中的四个数字（两模型 × 训练/测试）。",
		},
		NextStep:         "请补全两个模型在训练集与测试集上的表现，并说明差距说明了什么。",
		TeacherCondition: "若学生无法区分训练集与测试集的作用，建议教师结合实验记录做一次逐项讲解。",
	},
}
