package status

import "testing"

var legal = map[Status][]Status{
	Pending: {Queued, Failed},
	Queued:  {Running},
	Running: {Success, Failed, RolledBack, FailedRollback, Queued},
}

var states = []Status{Pending, Queued, Running, Success, Failed, RolledBack, FailedRollback}

// TestCanTransition_legal 验证状态表中声明的所有合法边
func TestCanTransition_legal(t *testing.T) {
	for from, tos := range legal {
		for _, to := range tos {
			if !CanTransition(from, to) {
				t.Errorf("CanTransition(%s, %s) = false, want true", from, to)
			}
		}
	}
}

// TestCanTransition_illegal 穷举状态组合并验证未声明的边全部被拒绝
func TestCanTransition_illegal(t *testing.T) {
	illegalCount := 0
	for _, from := range states {
		for _, to := range states {
			if isLegal(from, to) {
				continue
			}
			illegalCount++
			if CanTransition(from, to) {
				t.Errorf("CanTransition(%s, %s) = true, want false", from, to)
			}
		}
	}
	if illegalCount != 41 {
		t.Fatalf("illegal case count = %d, want 41 (49 − 8 legal)", illegalCount)
	}
}

// TestPendingToSuccess_rejected 固定验证任务不能绕过 queued 和 running 直接成功
func TestPendingToSuccess_rejected(t *testing.T) {
	if CanTransition(Pending, Success) {
		t.Error("CanTransition(pending, success) = true, want false")
	}
}

// isLegal 使用独立测试表判断状态边，避免复用被测实现产生同源错误
func isLegal(from, to Status) bool {
	for _, next := range legal[from] {
		if next == to {
			return true
		}
	}
	return false
}
