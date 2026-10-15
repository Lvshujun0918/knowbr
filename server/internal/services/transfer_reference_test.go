package services

import (
	"fmt"
	"testing"

	"knowbr/server/internal/content"
)

// 迁移题的不变量：参考答案是“完整作答”的基准，它在自己的评分维度下必须拿到满分。
// 如果参考答案都会掉分，说明关键词规则与该维度的满分标准不一致，
// 学生写出同等质量的作答也会被误判为“要点未覆盖”。
// 因此这里对全部题目族 × 变体 × 参数组合以及固定题库逐一核对。
func TestTransferReferenceAnswersScoreFull(t *testing.T) {
	checked := 0

	for _, fam := range content.TransferFamilies {
		combos := content.ParamCombinationCount(fam)
		for vi := range fam.Variants {
			for pi := 0; pi < combos; pi++ {
				item, ok := content.BuildVariantItem(fam, vi, pi)
				if !ok {
					continue
				}
				checked++
				assertReferenceScoresFull(t, fmt.Sprintf("%s / %s / 参数 %d", fam.Code, item.VariantCode, pi), item)
			}
		}
	}

	for _, item := range content.TransferItems {
		checked++
		assertReferenceScoresFull(t, "固定题库 / "+item.ID, item)
	}

	if checked == 0 {
		t.Fatal("没有核对到任何迁移题")
	}
	t.Logf("共核对 %d 道迁移题的参考答案", checked)
}

func assertReferenceScoresFull(t *testing.T, label string, item content.TransferItem) {
	t.Helper()
	state := ScoreTransfer(item, item.ReferenceAnswer)
	if state.Score == item.MaxScore {
		return
	}
	missing := []string{}
	for _, fb := range state.RubricFeedback {
		if fb.Score < fb.MaxScore {
			missing = append(missing, fmt.Sprintf("%s(%s) %d/%d", fb.Code, fb.Title, fb.Score, fb.MaxScore))
		}
	}
	t.Errorf("%s：参考答案只得 %d/%d，未满分的维度：%v", label, state.Score, item.MaxScore, missing)
}
