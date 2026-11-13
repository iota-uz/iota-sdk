package e2e_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// Scenario 23: a human task assigned to a permission (a committee). Any holder
// decides it, the first decision wins, whoever lacks the permission is refused,
// and the management read model shows who the task is for.
func TestScenario23CommitteeTaskIsDecidedByAPermissionHolder(t *testing.T) {
	e := newEnv(t)
	admin := e.admin
	const committee = "e2e.approvals.committee"
	e.deploy(admin, "e2e.approvals", "1.0.0", loadTree(t, "approvals"))
	first := e.newActor("member-one", committee)
	second := e.newActor("member-two", committee)
	outsider := e.newActor("outsider")

	var out submitted
	e.mustRPC(admin, "command", "e2e.approvals.submitToCommittee", map[string]any{"title": "Treaty"}, &out)
	id, err := uuid.Parse(out.Instance)
	require.NoError(t, err)
	e.waitStatus(admin, id, "waiting")

	tasks := e.workflow(admin, id).Tasks
	require.Len(t, tasks, 1)
	task := tasks[0]
	require.Equal(t, "permission", task.AssigneeKind)
	require.Equal(t, committee, task.AssigneePermission)
	require.Zero(t, task.AssigneeUserID)
	decide := "/tasks/" + task.ID.String() + "/decide"
	body := map[string]any{"decision": map[string]any{"approved": true}}

	e.mgmt(outsider, http.MethodPost, decide, body, http.StatusForbidden, nil)
	e.mgmt(first, http.MethodPost, decide, map[string]any{"decision": map[string]any{"approved": "yes"}}, http.StatusBadRequest, nil)
	e.mgmt(first, http.MethodPost, decide, body, http.StatusNoContent, nil)
	e.mgmt(second, http.MethodPost, decide, body, http.StatusConflict, nil)

	done := e.waitStatus(admin, id, "completed")
	require.JSONEq(t, fmt.Sprintf(`{"outcome":"approved","by":"%d"}`, first.id), string(done.Result))
	require.Equal(t, "approved", e.approvalStatus(admin, out.Request))
}
