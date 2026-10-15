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
	{
		ID:          "case-mlp-cnn",
		Name:        "案例六：MLP 与 CNN 图像分类对比（模型结构，模拟数据）",
		Description: "学生把准确率差距归因于“结构更复杂”，并认为继续加层就一定能超过 CNN。用于演示同一份回答中多个不同推理问题被分别诊断。",
		DataNotice:  DemoDataNotice,
		ExpectedFocus: []string{
			"把复杂度与准确率建立必然因果",
			"认为增加模型容量必然提升性能",
			"把单一测试集准确率等同于泛化能力",
		},
		Submission: DemoSubmission{
			ExperimentTitle:    "MLP 与 CNN 图像分类模型对比实验",
			ExperimentGoal:     "在同一图像数据集上训练 MLP 与 CNN，比较两者的分类表现并解释差异来源。",
			DatasetDescription: "图像分类数据集共 10000 张图，训练集 8000 张、测试集 2000 张，10 个类别（模拟数据）。",
			ModelName:          "MLP（两个隐藏层，全连接）与 CNN（两层卷积 + 全连接）",
			CodeSnippet: `mlp = MLP(hidden=[256, 128]).fit(X_train_flat, y_train)   # 图像先展平为一维向量
cnn = CNN(conv_layers=2).fit(X_train, y_train)            # 保留图像空间结构
print(mlp.score(X_test_flat, y_test), cnn.score(X_test, y_test))`,
			ExecutionLog: `MLP  test_accuracy = 0.72   params = 610000
CNN  test_accuracy = 0.92   params = 31000
（以上为人工构造的模拟教学数据，不是真实训练结果）`,
			MetricsText:               "MLP 测试准确率 0.72，参数量约 61 万；CNN 测试准确率 0.92，参数量约 3.1 万（模拟数据）。",
			StudentInitialExplanation: "CNN 的准确率比 MLP 高 20 个百分点，说明 CNN 的网络结构更复杂，所以准确率一定更高。如果继续增加 MLP 的隐藏层和神经元数量，它的准确率就一定能超过 CNN。只要测试集准确率提高，就说明模型的泛化能力更强，不需要再分析其他指标。",
			Question:                  "我是不是只要把 MLP 继续加深就行了？",
		},
	},
	{
		ID:          "case-rnn-instability",
		Name:        "案例七：RNN 训练损失异常与梯度裁剪（训练稳定性，模拟数据）",
		Description: "逐轮日志显示损失与梯度范数快速增大并出现 NaN。用于演示训练稳定性任务不会转向分类指标，而是要求区分日志中的事实、推断与未知信息。",
		DataNotice:  DemoDataNotice,
		ExpectedFocus: []string{
			"引用日志中损失与梯度范数的具体数值",
			"梯度爆炸只是有依据的假设，尚未证实",
			"需要单变量对照实验才能区分原因",
		},
		Submission: DemoSubmission{
			ExperimentTitle:    "RNN 训练损失异常与梯度裁剪实验",
			ExperimentGoal:     "判断 RNN 训练过程是否稳定，并分析损失异常的原因。",
			DatasetDescription: "字符级语料 5000 行，词表大小 128（模拟数据）。",
			ModelName:          "单层 RNN（hidden=128），学习率 0.5，未使用梯度裁剪",
			CodeSnippet: `for epoch in range(1, 6):
    loss = train_one_epoch(model, loader, optimizer)   # lr=0.5，未加梯度裁剪
    grad_norm = compute_grad_norm(model)
    print("epoch", epoch, "train_loss", round(loss, 2), "grad_norm", round(grad_norm, 1))`,
			ExecutionLog: `epoch 1 train_loss=1.92 grad_norm=3.1
epoch 2 train_loss=2.05 grad_norm=6.7
epoch 3 train_loss=2.37 grad_norm=18.9
epoch 4 train_loss=6.84 grad_norm=95.3
epoch 5 train_loss=nan grad_norm=nan
（以上为人工构造的模拟教学数据，不是真实训练结果）`,
			MetricsText:               "训练损失与梯度范数见运行日志（模拟数据）。",
			StudentInitialExplanation: "可能发生了梯度爆炸，学习率 0.5 也许让问题更严重。但我不确定能不能仅凭这份日志确定根本原因。",
			Question:                  "我该怎么判断是不是梯度爆炸？还需要补什么实验？",
		},
	},
	{
		ID:          "case-regression",
		Name:        "案例八：回归任务只看训练集误差（模拟数据）",
		Description: "二手房成交价预测中，学生在同一批数据上训练并评估，报告 R² = 0.93、MAE = 4.8 万元，据此认为模型可以上线；没有划分验证集，也没有与基线对照。用于演示回归任务中误差量纲、泛化估计与验收标准三类问题。",
		DataNotice:  DemoDataNotice,
		ExpectedFocus: []string{
			"误差数值要换算成目标变量的单位与相对量级",
			"训练集误差不等于泛化误差",
			"需要留出验证集与基线对照才能谈上线",
		},
		Submission: DemoSubmission{
			ExperimentTitle:    "房价预测回归实验",
			ExperimentGoal:     "预测二手房成交价（单位：万元），判断模型能否用于线上报价参考。",
			DatasetDescription: "某城市二手房成交数据 4200 条，目标变量为成交价、单位万元，全量样本的均价约 15 万元。",
			ModelName:          "随机森林回归（默认参数，未划分验证集）",
			CodeSnippet: `from sklearn.ensemble import RandomForestRegressor
from sklearn.metrics import mean_absolute_error, r2_score

model = RandomForestRegressor(random_state=0)
model.fit(X, y)            # 在全量数据上训练
pred = model.predict(X)    # 又在同一批数据上评估
print("MAE:", mean_absolute_error(y, pred))
print("R2:", r2_score(y, pred))`,
			ExecutionLog: `MAE: 4.8
R2: 0.93
（以上为人工构造的模拟教学数据，不是真实训练结果）`,
			MetricsText:               "训练集 MAE = 4.8（万元），R² = 0.93；未计算验证集指标，也没有基线对照。",
			StudentInitialExplanation: "我在训练集上算出的 R² 达到 0.93，MAE 只有 4.8 万元，误差很小，所以模型可以直接用来给房子定价了。",
			Question:                  "R² 这么高、误差看着也小，为什么我还是不确定它上线后准不准？",
		},
	},
	{
		ID:          "case-clustering",
		Name:        "案例九：K-means 分群只看轮廓系数（模拟数据）",
		Description: "客户分群任务中，学生报告 K=5、轮廓系数 0.62，认为“聚类效果很好、可以直接用于营销”，但特征未标准化、未固定随机种子，也没有说明每个簇的业务含义。用于演示聚类任务中内部指标的局限、簇数选择与稳定性检查。",
		DataNotice:  DemoDataNotice,
		ExpectedFocus: []string{
			"轮廓系数只描述几何结构，不携带业务含义",
			"簇数要在多个取值间比较，不能只看一个数值",
			"量纲会主导距离，聚类前需要标准化并验证稳定性",
		},
		Submission: DemoSubmission{
			ExperimentTitle:    "客户分群聚类实验",
			ExperimentGoal:     "对会员按消费行为分群，用于差异化运营策略。",
			DatasetDescription: "某电商会员行为数据 8000 条，特征包括年消费金额（元，几百到几万）与月访问次数（0—60 次），两者量纲差异很大。",
			ModelName:          "K-means（K=5，未做标准化，未设置随机种子与多次初始化）",
			CodeSnippet: `from sklearn.cluster import KMeans
from sklearn.metrics import silhouette_score

model = KMeans(n_clusters=5)        # 未标准化，未固定 random_state，默认 n_init
labels = model.fit_predict(X)       # X 同时包含金额（元）与访问次数（次）
print("silhouette:", silhouette_score(X, labels))`,
			ExecutionLog: `silhouette: 0.62
（以上为人工构造的模拟教学数据，不是真实训练结果）`,
			MetricsText:               "K=5，轮廓系数 0.62；未做特征标准化，未重复运行比较稳定性，未统计各簇规模。",
			StudentInitialExplanation: "我用 K-means 把会员分成 5 群，轮廓系数达到 0.62，说明聚类效果很好，可以直接用来制定营销策略了。",
			Question:                  "轮廓系数这个数值，能不能说明分出来的群有业务意义？",
		},
	},
	{
		ID:          "case-leakage",
		Name:        "案例十：先标准化后划分的数据泄漏（模拟数据）",
		Description: "用户在划分训练集与测试集之前，用全部数据完成了标准化与缺失值填充，报告测试集 AUC = 0.93；把流程改成“先划分、只在训练集上拟合参数”后，同一模型的 AUC 下降 8 个百分点。用于演示预处理与划分顺序导致的数据泄漏（E15）。",
		DataNotice:  DemoDataNotice,
		ExpectedFocus: []string{
			"标准化与填充的参数应在划分之后、只在训练集上估计",
			"用全部数据估参数会把测试集信息带进训练过程",
			"指标下降不等于模型变差，而是回到可信水平",
		},
		Submission: DemoSubmission{
			ExperimentTitle:    "用户流失预测实验",
			ExperimentGoal:     "预测用户下月是否流失，用于提前做挽留运营。",
			DatasetDescription: "某 App 用户行为数据 12000 条，36 个特征（访问频次、使用时长、会员等级等），部分特征存在缺失值。",
			ModelName:          "逻辑回归（标准化后训练）",
			CodeSnippet: `from sklearn.preprocessing import StandardScaler
from sklearn.impute import SimpleImputer
from sklearn.model_selection import train_test_split
from sklearn.linear_model import LogisticRegression

# 先用全部数据做填充与标准化，再划分训练集与测试集
imputer = SimpleImputer(strategy="mean")
scaler = StandardScaler()
X_filled = imputer.fit_transform(X)
X_scaled = scaler.fit_transform(X_filled)

X_train, X_test, y_train, y_test = train_test_split(X_scaled, y, test_size=0.3, random_state=0)
model = LogisticRegression().fit(X_train, y_train)
print("test AUC:", roc_auc_score(y_test, model.predict_proba(X_test)[:, 1]))`,
			ExecutionLog: `test AUC: 0.93
（以上为人工构造的模拟教学数据，不是真实训练结果）`,
			MetricsText:               "测试集 AUC = 0.93；标准化与缺失值填充都在划分之前用全部数据完成；改成先划分后，同一模型 AUC 下降 8 个百分点。",
			StudentInitialExplanation: "我先用全部数据做了缺失值填充和标准化，然后再划分训练集与测试集，测试集 AUC 达到 0.93，说明模型效果很好，可以直接上线了。",
			Question:                  "这样做出来的 AUC，能代表模型上线后的真实效果吗？",
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
