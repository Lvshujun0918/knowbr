package services

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// ValidationError 描述一个面向用户的输入问题，接口层会把它转换为 HTTP 400。
type ValidationError struct {
	Code    string `json:"code"`
	Field   string `json:"field,omitempty"`
	Message string `json:"message"`
	Detail  string `json:"detail,omitempty"`
}

func (e *ValidationError) Error() string { return e.Message }

// NewValidationError 构造带有明确中文提示的校验错误。
func NewValidationError(code, field, message, detail string) *ValidationError {
	return &ValidationError{Code: code, Field: field, Message: message, Detail: detail}
}

// ClassMetric 保存单个类别的计算结果。
type ClassMetric struct {
	Index              int     `json:"index"`
	Label              string  `json:"label"`
	Support            float64 `json:"support"`
	Predicted          float64 `json:"predicted"`
	TruePositive       float64 `json:"truePositive"`
	FalsePositive      float64 `json:"falsePositive"`
	FalseNegative      float64 `json:"falseNegative"`
	Recall             float64 `json:"recall"`
	RecallUndefined    bool    `json:"recallUndefined"`
	Precision          float64 `json:"precision"`
	PrecisionUndefined bool    `json:"precisionUndefined"`
	F1                 float64 `json:"f1"`
}

// MatrixMetrics 是混淆矩阵的完整计算结果，全部由程序计算，不经过大模型。
type MatrixMetrics struct {
	Labels          []string      `json:"labels"`
	Values          [][]float64   `json:"values"`
	Total           float64       `json:"total"`
	Correct         float64       `json:"correct"`
	Accuracy        float64       `json:"accuracy"`
	Classes         []ClassMetric `json:"classes"`
	MacroRecall     float64       `json:"macroRecall"`
	MacroPrecision  float64       `json:"macroPrecision"`
	MacroF1         float64       `json:"macroF1"`
	WeightedF1      float64       `json:"weightedF1"`
	MaxClassRatio   float64       `json:"maxClassRatio"`
	ImbalanceRatio  float64       `json:"imbalanceRatio"`
	Notes           []string      `json:"notes"`
	ZeroRecallClass []string      `json:"zeroRecallClasses"`
	Imbalanced      bool          `json:"imbalanced"`
}

// DefaultLabel 生成第 i 个类别的默认名称（类别A、类别B……）。
func DefaultLabel(i int) string {
	letters := "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	if i < len(letters) {
		return "类别" + string(letters[i])
	}
	return fmt.Sprintf("类别%d", i+1)
}

// ComputeMetrics 计算混淆矩阵指标。
// 约定：行=真实类别，列=预测类别。
// 零分母约定：某类别没有任何预测样本时，其精确率分母为零，按本应用约定记为 0 并标记 PrecisionUndefined；
// 某类别在真实数据中不存在时，其召回率分母为零，记为 0 并标记 RecallUndefined。这些约定会在 Notes 中说明。
func ComputeMetrics(values [][]float64, labels []string) (*MatrixMetrics, error) {
	n, err := validateMatrix(values)
	if err != nil {
		return nil, err
	}

	if len(labels) != n {
		labels = nil
	}
	names := make([]string, n)
	for i := range names {
		if labels != nil && strings.TrimSpace(labels[i]) != "" {
			names[i] = strings.TrimSpace(labels[i])
		} else {
			names[i] = DefaultLabel(i)
		}
	}

	m := &MatrixMetrics{Labels: names, Values: values}
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			m.Total += values[i][j]
			if i == j {
				m.Correct += values[i][j]
			}
		}
	}
	m.Accuracy = safeDiv(m.Correct, m.Total)

	classes := make([]ClassMetric, n)
	for i := 0; i < n; i++ {
		var support, predicted, tp float64
		for j := 0; j < n; j++ {
			support += values[i][j]   // 真实为 i 的样本数
			predicted += values[j][i] // 被预测为 i 的样本数
			if i == j {
				tp = values[i][i]
			}
		}
		c := ClassMetric{
			Index:         i,
			Label:         names[i],
			Support:       support,
			Predicted:     predicted,
			TruePositive:  tp,
			FalsePositive: predicted - tp,
			FalseNegative: support - tp,
		}
		if support > 0 {
			c.Recall = safeDiv(tp, support)
		} else {
			c.RecallUndefined = true
		}
		if predicted > 0 {
			c.Precision = safeDiv(tp, predicted)
		} else {
			c.PrecisionUndefined = true
		}
		f1Denom := c.Precision + c.Recall
		c.F1 = safeDiv(2*c.Precision*c.Recall, f1Denom)
		classes[i] = c

		if c.RecallUndefined {
			m.Notes = append(m.Notes, fmt.Sprintf("%s 在数据中不存在（样本数为 0），其召回率分母为零，按本应用约定记为 0。", names[i]))
		} else if c.PrecisionUndefined {
			m.Notes = append(m.Notes, fmt.Sprintf("%s 没有任何预测样本，其精确率分母为零，按本应用约定记为 0（不参与正常意义上的精确率比较）。", names[i]))
		}
		if !c.RecallUndefined && c.Recall == 0 {
			m.ZeroRecallClass = append(m.ZeroRecallClass, names[i])
		}
	}
	m.Classes = classes

	var macroRecall, macroPrecision, macroF1, weightedF1 float64
	for _, c := range classes {
		macroRecall += c.Recall
		macroPrecision += c.Precision
		macroF1 += c.F1
		weightedF1 += c.F1 * c.Support
	}
	m.MacroRecall = safeDiv(macroRecall, float64(n))
	m.MacroPrecision = safeDiv(macroPrecision, float64(n))
	m.MacroF1 = safeDiv(macroF1, float64(n))
	m.WeightedF1 = safeDiv(weightedF1, m.Total)

	var maxSupport, minSupport float64
	for i, c := range classes {
		if i == 0 || c.Support > maxSupport {
			maxSupport = c.Support
		}
		if i == 0 || c.Support < minSupport {
			minSupport = c.Support
		}
	}
	m.MaxClassRatio = safeDiv(maxSupport, m.Total)
	if minSupport > 0 {
		m.ImbalanceRatio = maxSupport / minSupport
	}
	m.Imbalanced = m.MaxClassRatio >= 0.7 || (m.ImbalanceRatio >= 3 && minSupport > 0)

	m.Notes = append(m.Notes, fmt.Sprintf("宏平均与加权平均 F1 均按上述零分母约定计算：宏平均对每个类别等权，加权平均按各类别真实样本数加权。总样本数 %.0f，正确分类 %.0f。", m.Total, m.Correct))
	return m, nil
}

func validateMatrix(values [][]float64) (int, error) {
	if len(values) == 0 {
		return 0, NewValidationError("MATRIX_EMPTY", "confusionMatrix", "混淆矩阵为空，请至少填写 2×2 的矩阵。", "")
	}
	n := len(values)
	if n < 2 {
		return 0, NewValidationError("MATRIX_TOO_SMALL", "confusionMatrix", "混淆矩阵至少需要 2 个类别（2×2），当前只有 1 行。", "")
	}
	var total float64
	for i, row := range values {
		if len(row) != n {
			return 0, NewValidationError("MATRIX_NOT_SQUARE", "confusionMatrix",
				fmt.Sprintf("混淆矩阵必须为方阵：第 %d 行有 %d 列，但矩阵有 %d 个类别（应为 %d 列）。行表示真实类别，列表示预测类别。", i+1, len(row), n, n), "")
		}
		for j, v := range row {
			if math.IsNaN(v) || math.IsInf(v, 0) {
				return 0, NewValidationError("MATRIX_INVALID_NUMBER", "confusionMatrix",
					fmt.Sprintf("第 %d 行第 %d 列不是有效数字，请填写普通数值。", i+1, j+1), "")
			}
			if v < 0 {
				return 0, NewValidationError("MATRIX_NEGATIVE", "confusionMatrix",
					fmt.Sprintf("第 %d 行第 %d 列的数值为 %.0f，混淆矩阵不能包含负数。", i+1, j+1, v), "")
			}
			total += v
		}
	}
	if total == 0 {
		return 0, NewValidationError("MATRIX_ALL_ZERO", "confusionMatrix", "混淆矩阵所有取值都为 0，无法计算任何指标。", "")
	}
	return n, nil
}

// ParseMatrixText 解析粘贴的混淆矩阵文本。
// 支持空格、制表符、逗号、分号分隔；忽略空行与以 # 开头的注释行。
// 只有当每一行的首个字段都不是数字、且各行字段数量一致时，才把首列当作类别名称；
// 否则按纯数字矩阵解析，遇到非数字字段会报出具体行号与内容。
func ParseMatrixText(raw string) (values [][]float64, labels []string, err error) {
	sep := regexp.MustCompile(`[\s,;，、]+`)
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")

	type rawRow struct {
		index  int
		fields []string
	}
	var rows []rawRow
	rowIndex := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "//") {
			continue
		}
		rowIndex++
		fields := sep.Split(trimmed, -1)
		for len(fields) > 0 && fields[0] == "" {
			fields = fields[1:]
		}
		for len(fields) > 0 && fields[len(fields)-1] == "" {
			fields = fields[:len(fields)-1]
		}
		if len(fields) == 0 {
			rowIndex--
			continue
		}
		rows = append(rows, rawRow{index: rowIndex, fields: fields})
	}
	if len(rows) == 0 {
		return nil, nil, NewValidationError("MATRIX_EMPTY", "confusionMatrix", "没有从粘贴内容中解析出混淆矩阵，请检查后重试。", "")
	}

	width := len(rows[0].fields)
	sameWidth := true
	for _, r := range rows {
		if len(r.fields) != width {
			sameWidth = false
		}
	}
	labelMode := sameWidth
	for _, r := range rows {
		if _, convErr := strconv.ParseFloat(r.fields[0], 64); convErr == nil {
			labelMode = false
		}
	}

	for _, r := range rows {
		start := 0
		if labelMode {
			labels = append(labels, r.fields[0])
			start = 1
		}
		row := make([]float64, 0, len(r.fields)-start)
		for _, f := range r.fields[start:] {
			v, convErr := strconv.ParseFloat(strings.TrimSpace(f), 64)
			if convErr != nil {
				return nil, nil, NewValidationError("MATRIX_PARSE_ERROR", "confusionMatrix",
					fmt.Sprintf("第 %d 行中的 “%s” 不是有效数字，请只填写数字（可用空格、制表符或逗号分隔）。", r.index, f), "")
			}
			row = append(row, v)
		}
		if len(row) == 0 {
			return nil, nil, NewValidationError("MATRIX_EMPTY_ROW", "confusionMatrix",
				fmt.Sprintf("第 %d 行没有读到任何数字，请检查该行内容。", r.index), "")
		}
		values = append(values, row)
	}
	if len(labels) != len(values) {
		labels = nil
	}
	return values, labels, nil
}

func safeDiv(a, b float64) float64 {
	if b == 0 {
		return 0
	}
	return a / b
}
