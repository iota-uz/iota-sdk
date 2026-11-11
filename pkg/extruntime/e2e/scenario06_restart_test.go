package e2e_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type submitted struct {
	Request  map[string]string `json:"request"`
	Instance string            `json:"instance"`
}

func (e *env) submitApproval(by *actor, title string, approver *actor, waitSeconds, cooldownSeconds int) (submitted, uuid.UUID) {
	e.t.Helper()
	var out submitted
	e.mustRPC(by, "command", "e2e.approvals.submit", map[string]any{
		"title": title, "approver": itoa(approver.id), "waitSeconds": waitSeconds, "cooldownSeconds": cooldownSeconds,
	}, &out)
	id, err := uuid.Parse(out.Instance)
	require.NoError(e.t, err)
	return out, id
}

func (e *env) approvalStatus(a *actor, req map[string]string) string {
	e.t.Helper()
	var out *struct {
		Status string `json:"status"`
	}
	e.mustRPC(a, "query", "e2e.approvals.statusOf", map[string]any{"request": req}, &out)
	if out == nil {
		return ""
	}
	return out.Status
}

// Scenario 6: a workflow waits on a human decision and on a timer; the engine
// restarts (a new runtime on the same database) in both waits, and the
// instance continues from its journal. A task that is not decided in time
// times out and takes its other branch.
func TestScenario06HumanDecisionAndTimerSurviveEngineRestart(t *testing.T) {
	e := newEnv(t)
	admin := e.admin
	e.deploy(admin, "e2e.approvals", "1.0.0", loadTree(t, "approvals"))
	approver := e.newActor("approver")
	stranger := e.newActor("stranger")

	// ---- decision + timer across restarts
	req, id := e.submitApproval(admin, "Budget", approver, 3600, 2)
	inst := e.waitStatus(admin, id, "waiting")
	require.Equal(t, "pending", e.approvalStatus(admin, req.Request))
	h := e.workflow(admin, id)
	require.Len(t, h.Tasks, 1)
	task := h.Tasks[0]
	require.Equal(t, "open", task.Status)
	require.EqualValues(t, approver.id, task.AssigneeUserID)
	runs := inst.RunCount

	e.restart() // the engine goes away while the task is open

	// Only the assignee (or an operator) can decide.
	e.mgmt(stranger, http.MethodPost, "/tasks/"+task.ID.String()+"/decide", map[string]any{"decision": map[string]any{"approved": true}}, http.StatusForbidden, nil)
	e.mgmt(approver, http.MethodPost, "/tasks/"+task.ID.String()+"/decide", map[string]any{"decision": map[string]any{"approved": "yes"}}, http.StatusBadRequest, nil)
	decidedAt := time.Now()
	e.mgmt(approver, http.MethodPost, "/tasks/"+task.ID.String()+"/decide", map[string]any{"decision": map[string]any{"approved": true, "note": "ok"}}, http.StatusNoContent, nil)
	e.mgmt(approver, http.MethodPost, "/tasks/"+task.ID.String()+"/decide", map[string]any{"decision": map[string]any{"approved": false}}, http.StatusConflict, nil)

	// The decision resumed the instance on the new engine; it now sleeps on its timer.
	e.eventually("instance is waiting on the cooldown timer", 30*time.Second, func() bool {
		hh := e.workflow(admin, id)
		for _, o := range hh.Occurrences {
			if o.Name == "cooldown" && o.Status == "waiting" {
				return true
			}
		}
		return false
	})
	require.Equal(t, "pending", e.approvalStatus(admin, req.Request), "the effect has not happened before the timer")
	e.restart() // ... and goes away again while the timer runs
	done := e.waitStatus(admin, id, "completed")
	require.GreaterOrEqual(t, time.Since(decidedAt), 2*time.Second, "the timer was honored across the restart")
	require.JSONEq(t, `{"outcome":"approved"}`, string(done.Result))
	require.Equal(t, "approved", e.approvalStatus(admin, req.Request))
	require.Greater(t, done.RunCount, runs, "the instance was replayed after the restarts")
	final := e.workflow(admin, id)
	var names []string
	for _, o := range final.Occurrences {
		require.Equal(t, "completed", o.Status, o.Name)
		names = append(names, o.Name)
	}
	require.ElementsMatch(t, []string{"approve", "cooldown", "apply"}, names, "each primitive recorded exactly once")
	require.Equal(t, 1, e.scalarInt(`SELECT count(*) FROM extruntime.human_tasks WHERE instance_id = $1`, id), "replay did not create a second task")

	// ---- an undecided task times out
	req2, id2 := e.submitApproval(admin, "Late", approver, 1, 1)
	done2 := e.waitStatus(admin, id2, "completed")
	require.JSONEq(t, `{"outcome":"expired"}`, string(done2.Result))
	require.Equal(t, "expired", e.approvalStatus(admin, req2.Request))
	h2 := e.workflow(admin, id2)
	require.Equal(t, "expired", h2.Tasks[0].Status)
	e.mgmt(approver, http.MethodPost, "/tasks/"+h2.Tasks[0].ID.String()+"/decide", map[string]any{"decision": map[string]any{"approved": true}}, http.StatusConflict, nil)

	// ---- a rejected decision takes the other branch
	req3, id3 := e.submitApproval(admin, "No", approver, 3600, 1)
	e.waitStatus(admin, id3, "waiting")
	tasks := e.workflow(admin, id3).Tasks
	e.mgmt(approver, http.MethodPost, "/tasks/"+tasks[0].ID.String()+"/decide", map[string]any{"decision": map[string]any{"approved": false}}, http.StatusNoContent, nil)
	e.waitStatus(admin, id3, "completed")
	require.Equal(t, "rejected", e.approvalStatus(admin, req3.Request))
}
