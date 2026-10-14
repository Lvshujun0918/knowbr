package services

import (
	"testing"

	"knowbr/server/internal/models"
)

func TestSummaryMasteryUsesRevisionAndTransfer(t *testing.T) {
	in := SummaryInput{
		Session: &models.Session{},
		Diagnosis: Diagnosis{MasteryStatus: MasteryNotShown},
		Revisions: []models.Revision{{Progress: "improved"}},
		Transfer: &TransferState{Answered: true, Score: 6, MaxScore: 6},
	}
	if got := BuildSummary(in).Mastery.Status; got != MasteryDemonstrated {
		t.Fatalf("修正并完成迁移后应体现理解，实际 %s", got)
	}
	in.Transfer.Score = 3
	if got := BuildSummary(in).Mastery.Status; got != MasteryPartial {
		t.Fatalf("迁移仍有缺漏时应为部分理解，实际 %s", got)
	}
	in.Revisions = nil
	if got := BuildSummary(in).Mastery.Status; got != MasteryNotShown {
		t.Fatalf("未修正时保留初始证据判断，实际 %s", got)
	}
}
