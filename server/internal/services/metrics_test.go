package services

import (
	"math"
	"strings"
	"testing"
)

// 核心演示案例（需求文档 6.1）：1000 个样本，模型全部预测为类别 A。
func coreDemoMatrix() [][]float64 {
	return [][]float64{
		{900, 0, 0},
		{90, 0, 0},
		{10, 0, 0},
	}
}

func TestComputeMetricsCoreDemoCase(t *testing.T) {
	m, err := ComputeMetrics(coreDemoMatrix(), []string{"类别A", "类别B", "类别C"})
	if err != nil {
		t.Fatalf("计算失败：%v", err)
	}
	if m.Total != 1000 || m.Correct != 900 {
		t.Fatalf("总数/正确数错误：total=%v correct=%v", m.Total, m.Correct)
	}
	if math.Abs(m.Accuracy-0.9) > 1e-9 {
		t.Fatalf("准确率应为 0.9，实际 %v", m.Accuracy)
	}
	wantRecall := []float64{1, 0, 0}
	for i, c := range m.Classes {
		if math.Abs(c.Recall-wantRecall[i]) > 1e-9 {
			t.Errorf("%s 召回率应为 %v，实际 %v", c.Label, wantRecall[i], c.Recall)
		}
	}
	// 类别 A 的召回率为 1（900 个 A 样本全部识别正确），但所有样本都被预测成 A，
	// 因此 A 的精确率 = 900/1000 = 0.9，F1 = 2*0.9*1/(0.9+1)。
	f1A := 2 * 0.9 * 1.0 / (0.9 + 1.0)
	if math.Abs(m.Classes[0].Precision-0.9) > 1e-9 {
		t.Errorf("类别A 精确率应为 0.9（900/1000），实际 %v", m.Classes[0].Precision)
	}
	if math.Abs(m.Classes[0].F1-f1A) > 1e-9 {
		t.Errorf("类别A F1 应为 %v，实际 %v", f1A, m.Classes[0].F1)
	}
	for _, i := range []int{1, 2} {
		c := m.Classes[i]
		if !c.PrecisionUndefined {
			t.Errorf("%s 应标记精确率分母为零", c.Label)
		}
		if c.Precision != 0 {
			t.Errorf("%s 按约定精确率应记为 0，实际 %v", c.Label, c.Precision)
		}
		if c.F1 != 0 {
			t.Errorf("%s 的 F1 应为 0，实际 %v", c.Label, c.F1)
		}
	}
	// 宏平均 F1 = (F1A + 0 + 0) / 3，加权平均 F1 = F1A * 900/1000。
	if math.Abs(m.MacroF1-f1A/3) > 1e-9 {
		t.Errorf("宏平均 F1 应为 %v，实际 %v", f1A/3, m.MacroF1)
	}
	if math.Abs(m.WeightedF1-f1A*0.9) > 1e-9 {
		t.Errorf("加权平均 F1 应为 %v，实际 %v", f1A*0.9, m.WeightedF1)
	}
	if len(m.ZeroRecallClass) != 2 || m.ZeroRecallClass[0] != "类别B" || m.ZeroRecallClass[1] != "类别C" {
		t.Errorf("零召回类别错误：%v", m.ZeroRecallClass)
	}
	if !m.Imbalanced {
		t.Error("该矩阵应被判定为类别不平衡")
	}
	if math.Abs(m.MaxClassRatio-0.9) > 1e-9 {
		t.Errorf("最大类别占比应为 0.9，实际 %v", m.MaxClassRatio)
	}
	if math.Abs(m.ImbalanceRatio-90) > 1e-9 {
		t.Errorf("不平衡比应为 90，实际 %v", m.ImbalanceRatio)
	}
	if !strings.Contains(strings.Join(m.Notes, " "), "精确率分母为零") {
		t.Errorf("应说明零分母约定，notes=%v", m.Notes)
	}
}

// 手工核对：平衡二分类矩阵 [[45,5],[5,45]]。
// acc = 90/100 = 0.9；两类 recall = 45/50 = 0.9，precision = 45/50 = 0.9，F1 = 0.9；
// 宏平均 F1 = 0.9，加权平均 F1 = 0.9，最大类别占比 0.5（不属于不平衡）。
func TestComputeMetricsBalancedCase(t *testing.T) {
	m, err := ComputeMetrics([][]float64{{45, 5}, {5, 45}}, nil)
	if err != nil {
		t.Fatalf("计算失败：%v", err)
	}
	if math.Abs(m.Accuracy-0.9) > 1e-9 {
		t.Errorf("准确率应为 0.9，实际 %v", m.Accuracy)
	}
	for _, c := range m.Classes {
		if math.Abs(c.Recall-0.9) > 1e-9 || math.Abs(c.Precision-0.9) > 1e-9 || math.Abs(c.F1-0.9) > 1e-9 {
			t.Errorf("%s 指标应为 0.9，实际 recall=%v precision=%v f1=%v", c.Label, c.Recall, c.Precision, c.F1)
		}
	}
	if math.Abs(m.MacroF1-0.9) > 1e-9 || math.Abs(m.WeightedF1-0.9) > 1e-9 {
		t.Errorf("宏平均/加权平均 F1 应为 0.9，实际 %v / %v", m.MacroF1, m.WeightedF1)
	}
	if m.Imbalanced {
		t.Error("平衡矩阵不应被判定为类别不平衡")
	}
	if math.Abs(m.MaxClassRatio-0.5) > 1e-9 {
		t.Errorf("最大类别占比应为 0.5，实际 %v", m.MaxClassRatio)
	}
}

func TestZeroSupportClassConvention(t *testing.T) {
	m, err := ComputeMetrics([][]float64{{10, 0}, {0, 0}}, nil)
	if err != nil {
		t.Fatalf("计算失败：%v", err)
	}
	if m.Classes[1].Support != 0 || !m.Classes[1].RecallUndefined {
		t.Errorf("样本数为 0 的类别应标记召回率未定义：%+v", m.Classes[1])
	}
	if m.Classes[1].Recall != 0 {
		t.Errorf("未定义召回率按约定记为 0，实际 %v", m.Classes[1].Recall)
	}
	if !strings.Contains(strings.Join(m.Notes, " "), "不存在") {
		t.Errorf("应说明该类别在数据中不存在：%v", m.Notes)
	}
}

func TestValidateMatrixRejectsBadInput(t *testing.T) {
	cases := []struct {
		name   string
		values [][]float64
		code   string
	}{
		{"空矩阵", nil, "MATRIX_EMPTY"},
		{"只有一个类别", [][]float64{{3, 1}}, "MATRIX_TOO_SMALL"},
		{"行列不一致", [][]float64{{3, 1}, {0, 2, 1}}, "MATRIX_NOT_SQUARE"},
		{"负数", [][]float64{{3, -1}, {0, 2}}, "MATRIX_NEGATIVE"},
		{"全零", [][]float64{{0, 0}, {0, 0}}, "MATRIX_ALL_ZERO"},
		{"NaN", [][]float64{{3, math.NaN()}, {0, 2}}, "MATRIX_INVALID_NUMBER"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ComputeMetrics(tc.values, nil)
			ve, ok := err.(*ValidationError)
			if !ok {
				t.Fatalf("应返回 ValidationError，实际 %v", err)
			}
			if ve.Code != tc.code {
				t.Errorf("错误码应为 %s，实际 %s（%s）", tc.code, ve.Code, ve.Message)
			}
			if ve.Message == "" {
				t.Error("校验错误必须带有面向用户的中文提示")
			}
		})
	}
}

func TestParseMatrixText(t *testing.T) {
	t.Run("带类别名称与制表符", func(t *testing.T) {
		values, labels, err := ParseMatrixText("# 演示矩阵\n类别A\t900\t0\t0\n类别B\t90\t0\t0\n类别C\t10\t0\t0\n")
		if err != nil {
			t.Fatalf("解析失败：%v", err)
		}
		if len(values) != 3 || len(values[0]) != 3 {
			t.Fatalf("矩阵尺寸错误：%v", values)
		}
		if len(labels) != 3 || labels[1] != "类别B" {
			t.Fatalf("类别名称解析错误：%v", labels)
		}
		m, err := ComputeMetrics(values, labels)
		if err != nil {
			t.Fatalf("计算失败：%v", err)
		}
		if m.Labels[0] != "类别A" || math.Abs(m.Accuracy-0.9) > 1e-9 {
			t.Errorf("标签或准确率错误：%v %v", m.Labels, m.Accuracy)
		}
	})

	t.Run("逗号分隔", func(t *testing.T) {
		values, _, err := ParseMatrixText("3, 1\n0, 2\n")
		if err != nil {
			t.Fatalf("解析失败：%v", err)
		}
		if values[0][1] != 1 || values[1][0] != 0 {
			t.Fatalf("解析结果错误：%v", values)
		}
	})

	t.Run("非数字给出定位提示", func(t *testing.T) {
		_, _, err := ParseMatrixText("3 1\nx 2\n")
		ve, ok := err.(*ValidationError)
		if !ok {
			t.Fatalf("应返回 ValidationError，实际 %v", err)
		}
		if ve.Code != "MATRIX_PARSE_ERROR" {
			t.Errorf("错误码应为 MATRIX_PARSE_ERROR，实际 %s", ve.Code)
		}
		if !strings.Contains(ve.Message, "第 2 行") || !strings.Contains(ve.Message, "x") {
			t.Errorf("错误信息应指出出错行与内容，实际：%s", ve.Message)
		}
	})

	t.Run("空内容", func(t *testing.T) {
		_, _, err := ParseMatrixText("   \n\n")
		if err == nil {
			t.Fatal("空内容应返回错误")
		}
		if _, ok := err.(*ValidationError); !ok {
			t.Fatalf("应返回 ValidationError，实际 %v", err)
		}
	})
}
