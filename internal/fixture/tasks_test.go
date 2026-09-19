package fixture

import "testing"

func TestDay0TaskSuiteGoldenIDsExist(t *testing.T) {
	st, _ := Day0()
	for _, task := range Day0TaskSuite() {
		if task.ID == "" || task.Question == "" {
			t.Fatalf("task missing id/question: %+v", task)
		}
		if len(task.RelevantIDs) == 0 || len(task.RequiredPhrases) == 0 {
			t.Fatalf("task %s missing golden fields", task.ID)
		}
		for _, id := range task.RelevantIDs {
			if _, ok := st.Get(id); !ok {
				t.Errorf("task %s: golden ID %q missing from store", task.ID, id)
			}
		}
	}
}

func TestProjXDelayTaskMatchesDay0Primary(t *testing.T) {
	_, primary := Day0()
	suite := Day0TaskSuite()
	if suite[0].ID != primary.ID {
		t.Fatalf("suite[0]=%s primary=%s", suite[0].ID, primary.ID)
	}
	if suite[0].Question != primary.Question {
		t.Fatal("primary task drifted from Day0")
	}
}
