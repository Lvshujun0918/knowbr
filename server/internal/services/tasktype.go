package services

import (
	"strings"

	"knowbr/server/internal/content"
)

// taskSignals 是任务类型识别用的关键词。识别只使用学生提交的材料与计算结果，
// 不依赖任何预设案例编号，避免把不同实验都归到同一个模板上。
var taskSignals = map[string][]string{
	content.TaskCodeEnv: {
		"报错", "traceback", "modulenotfounderror", "importerror", "no module named",
		"cannot import", "运行失败", "跑不起来", "闪退", "依赖冲突", "版本冲突", "环境",
	},
	content.TaskTraining: {
		"loss", "损失", "梯度", "grad_norm", "gradient", "nan", "inf", "梯度裁剪", "clip",
		"裁剪", "学习率", "收敛", "发散", "爆炸", "rnn", "lstm", "gru", "训练不稳定", "优化器",
		"训练", "稳定性",
	},
	content.TaskArchitecture: {
		"cnn", "卷积", "mlp", "全连接", "隐藏层", "层数", "参数量", "网络结构", "池化",
		"感受野", "权重共享", "局部连接", "过拟合", "容量", "结构差异", "推理耗时", "参数量",
	},
	content.TaskClassification: {
		"准确率", "accuracy", "混淆矩阵", "召回", "精确", "f1", "分类", "类别", "样本",
		"precision", "recall", "不平衡",
	},
	content.TaskRegression: {
		"回归", "预测值", "房价", "成交量", "气温", "用电量", "mae", "mse", "rmse", "r2", "r²",
		"决定系数", "均方误差", "平均绝对误差", "残差", "拟合优度", "目标变量", "连续值",
	},
}

// DetectTaskType 判定当前实验属于哪一类任务（需求文档 3.1「当前实验」上下文）。
// 判定依据按字段重要性加权：实验名称/目标 > 运行日志/代码 > 学生解释 > 数据与指标描述。
func DetectTaskType(in DiagnoseInput) string {
	title := in.ExperimentTitle + " " + in.ExperimentGoal
	code := in.CodeSnippet + " " + in.ModelName
	log := in.ExecutionLog
	explanation := in.Explanation + " " + in.Question
	data := in.DatasetDescription + " " + in.MetricsText

	scores := map[string]float64{}
	for taskType, words := range taskSignals {
		s := 0.0
		s += 3 * countSignals(title, words)
		s += 2.5 * countSignals(log, words)
		s += 2 * countSignals(code, words)
		s += 2 * countSignals(explanation, words)
		s += 1 * countSignals(data, words)
		scores[taskType] = s
	}
	// 提供了混淆矩阵时，分类任务的证据已经存在，给一个下限权重。
	if in.Metrics != nil || in.HaveMatrix {
		if scores[content.TaskClassification] < 2 {
			scores[content.TaskClassification] = 2
		}
	}
	best, bestScore := "", 0.0
	for _, t := range content.TaskTypes {
		if t.Code == content.TaskGeneric {
			continue
		}
		if scores[t.Code] > bestScore {
			best, bestScore = t.Code, scores[t.Code]
		}
	}
	if best == "" {
		if in.Metrics != nil || in.HaveMatrix {
			return content.TaskClassification
		}
		return content.TaskGeneric
	}
	return best
}

func countSignals(text string, words []string) float64 {
	low := strings.ToLower(text)
	n := 0.0
	for _, w := range words {
		if w == "" {
			continue
		}
		if strings.Contains(low, strings.ToLower(w)) {
			n++
		}
	}
	return n
}

// archetypeAppliesToTask 判断一条规则是否适用于当前任务类型。
// 这是阻止跨任务模板漂移的机制：训练稳定性任务不会再命中分类指标类规则。
func archetypeAppliesToTask(a content.Archetype, taskType string) bool {
	if len(a.TaskTypes) == 0 {
		return true
	}
	for _, t := range a.TaskTypes {
		if t == taskType {
			return true
		}
	}
	return false
}
