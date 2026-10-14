package content

// DemoSubmission 是演示案例中的实验材料字段，与接口层提交结构保持一致。
type DemoSubmission struct {
	ExperimentTitle           string   `json:"experimentTitle"`
	ExperimentGoal            string   `json:"experimentGoal"`
	DatasetDescription        string   `json:"datasetDescription"`
	ModelName                 string   `json:"modelName"`
	CodeSnippet               string   `json:"codeSnippet"`
	ExecutionLog              string   `json:"executionLog"`
	ConfusionMatrixText       string   `json:"confusionMatrixText"`
	ClassLabels               []string `json:"classLabels"`
	MetricsText               string   `json:"metricsText"`
	StudentInitialExplanation string   `json:"studentInitialExplanation"`
	Question                  string   `json:"question"`
}

// DemoCase 是一个预置的模拟教学案例。所有案例数据均为人工构造，不是真实训练实验结果。
type DemoCase struct {
	ID            string         `json:"id"`
	Name          string         `json:"name"`
	Description   string         `json:"description"`
	DataNotice    string         `json:"dataNotice"`
	ExpectedFocus []string       `json:"expectedFocus"`
	Submission    DemoSubmission `json:"submission"`
}

// DemoDataNotice 统一标注演示数据来源。
const DemoDataNotice = "模拟案例：以下数据为人工构造的教学演示数据，不是真实训练实验结果，也不代表任何真实学生的学习成效。"

// DemoCases 是首页“体验演示案例”使用的预置案例。
var DemoCases = []DemoCase{
	{
		ID:          "case-imbalance",
		Name:        "案例一：类别极不平衡的三分类任务（核心演示）",
		Description: "模型把 1000 个样本全部预测为类别 A，整体准确率仍有 90%，但类别 B、C 的召回率均为 0。用于演示“准确率高不等于模型可用”。",
		DataNotice:  DemoDataNotice,
		ExpectedFocus: []string{
			"整体准确率被多数类主导",
			"类别 B、C 召回率为 0",
			"需要结合任务目标与错误代价选择指标",
		},
		Submission: DemoSubmission{
			ExperimentTitle:    "分类模型评价实验",
			ExperimentGoal:     "完成一个三分类任务，并判断该模型是否可以直接交付使用。",
			DatasetDescription: "真实类别分布：类别 A 900 个、类别 B 90 个、类别 C 10 个，总计 1000 个样本，属于明显不平衡数据。",
			ModelName:          "默认参数的 KNN 分类器（predict 后直接取 accuracy_score，未做类别权重或重采样）",
			CodeSnippet: `from sklearn.model_selection import train_test_split
from sklearn.neighbors import KNeighborsClassifier
from sklearn.metrics import accuracy_score, confusion_matrix

X_train, X_test, y_train, y_test = train_test_split(X, y, test_size=0.3, random_state=42)
model = KNeighborsClassifier()   # 未设置 class_weight，未做重采样
model.fit(X_train, y_train)
pred = model.predict(X_test)
print("accuracy:", accuracy_score(y_test, pred))
print(confusion_matrix(y_test, pred))`,
			ExecutionLog: `accuracy: 0.90
[[900   0   0]
 [ 90   0   0]
 [ 10   0   0]]`,
			ConfusionMatrixText: `900 0 0
90 0 0
10 0 0`,
			ClassLabels:               []string{"类别A", "类别B", "类别C"},
			MetricsText:               "准确率 accuracy = 0.90，其他指标没有计算。",
			StudentInitialExplanation: "我的模型准确率达到 90%，所以我认为模型效果很好，可以直接使用了。",
			Question:                  "为什么准确率这么高，我还是不太确定这个模型能不能用？",
		},
	},
	{
		ID:            "case-accuracy-only",
		Name:          "案例二：只提交准确率",
		Description:   "学生只提供“准确率 90%”，没有混淆矩阵、类别分布与逐类指标。用于演示证据不足时系统应追问而不是武断下结论。",
		DataNotice:    DemoDataNotice,
		ExpectedFocus: []string{"现有信息不足以判断模型好坏", "需要类别分布、任务目标与逐类指标"},
		Submission: DemoSubmission{
			ExperimentTitle:           "分类模型准确率实验",
			ExperimentGoal:            "",
			DatasetDescription:        "",
			ModelName:                 "逻辑回归",
			ConfusionMatrixText:       "",
			MetricsText:               "准确率 90%",
			StudentInitialExplanation: "模型准确率为 90%，这个效果怎么样？",
			Question:                  "只看一个准确率数字，能不能说明模型好不好？",
		},
	},
	{
		ID:            "case-misread-matrix",
		Name:          "案例三：误读混淆矩阵",
		Description:   "学生声称“所有类别都能被正确分类”，但混淆矩阵显示类别 B、C 的样本全部被预测为 A。用于演示证据矛盾时的诊断。",
		DataNotice:    DemoDataNotice,
		ExpectedFocus: []string{"学生陈述与矩阵证据矛盾", "指出错误具体发生在哪两类之间", "用数据解释，不贬低学生"},
		Submission: DemoSubmission{
			ExperimentTitle:           "三分类混淆矩阵练习",
			ExperimentGoal:            "读懂混淆矩阵，并据此判断模型是否对所有类别都有识别能力。",
			DatasetDescription:        "样本共 1000 个：类别 A 900 个、类别 B 90 个、类别 C 10 个。",
			ModelName:                 "决策树分类器",
			ConfusionMatrixText:       "900 0 0\n90 0 0\n10 0 0",
			ClassLabels:               []string{"类别A", "类别B", "类别C"},
			MetricsText:               "混淆矩阵如上，准确率约 90%。",
			StudentInitialExplanation: "从混淆矩阵看，我把所有样本都预测成了类别A，所以三个类别都能被正确分类，模型对每一类都识别得很好。",
			Question:                  "我不太确定怎么看混淆矩阵，是不是对角线都对就说明模型好？",
		},
	},
	{
		ID:            "case-run-only",
		Name:          "案例四：代码跑通就认为学会了",
		Description:   "学生只强调代码运行成功，没有解释结果含义与适用边界。用于演示“程序成功 ≠ 概念掌握”的诊断与迁移检验。",
		DataNotice:    DemoDataNotice,
		ExpectedFocus: []string{"程序可执行不等于掌握知识", "需要解释结果含义与适用边界", "用迁移题验证理解"},
		Submission: DemoSubmission{
			ExperimentTitle:    "文本情感分类实验",
			ExperimentGoal:     "运行情感分类实验并解释模型表现。",
			DatasetDescription: "训练集 2000 条评论，测试集 500 条，正负样本比例约 1:1。",
			ModelName:          "多项式朴素贝叶斯",
			CodeSnippet: `model = MultinomialNB().fit(X_train_vec, y_train)
print("accuracy:", model.score(X_test_vec, y_test))`,
			ExecutionLog:              "accuracy: 0.873\n程序运行结束，没有报错。",
			MetricsText:               "准确率 0.873",
			StudentInitialExplanation: "代码跑通了而且没有报错，说明我已经学会了这个实验，可以做下一个了。",
			Question:                  "既然程序运行成功，是不是就说明我掌握了？",
		},
	},
	{
		ID:            "case-env-error",
		Name:          "案例五：环境报错（排障演示）",
		Description:   "学生提交的是环境报错。用于演示对工程排障类问题直接给出可执行步骤，而不是苏格拉底式提问。",
		DataNotice:    DemoDataNotice,
		ExpectedFocus: []string{"直接给出可执行的排障步骤", "不把环境问题误判为概念问题"},
		Submission: DemoSubmission{
			ExperimentTitle: "第一次运行分类实验",
			ExperimentGoal:  "跑通课程提供的分类实验代码。",
			ModelName:       "scikit-learn 分类器",
			CodeSnippet:     "from sklearn.datasets import load_iris\nclf = LogisticRegression()",
			ExecutionLog: `Traceback (most recent call last):
  File "exp.py", line 1, in <module>
    from sklearn.datasets import load_iris
ModuleNotFoundError: No module named 'sklearn'`,
			StudentInitialExplanation: "运行代码时报错了：ModuleNotFoundError: No module named 'sklearn'，请问怎么解决？",
			Question:                  "环境问题应该先装包还是先改代码？",
		},
	},
}

// DemoCaseByID 按编号查找演示案例。
func DemoCaseByID(id string) (DemoCase, bool) {
	for _, c := range DemoCases {
		if c.ID == id {
			return c, true
		}
	}
	return DemoCase{}, false
}
