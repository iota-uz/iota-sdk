// Package persistence provides this package.
package persistence

import (
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	"github.com/iota-uz/iota-sdk/pkg/repo"
)

const (
	selectMessageColumns = `
		SELECT
			m.id,
			m.session_id,
			m.role,
			m.content,
			m.author_user_id,
			COALESCE(u.first_name, ''),
			COALESCE(u.last_name, ''),
			m.tool_calls,
			m.tool_call_id,
			m.citations,
			m.debug_trace,
			m.question_data,
			m.created_at
	`
	selectMessageFrom = `
		FROM bichat.messages m
		JOIN bichat.sessions s ON m.session_id = s.id
		LEFT JOIN public.users u ON u.id = m.author_user_id AND u.tenant_id = s.tenant_id
	`
)

var (
	insertMessageQuery = repo.Insert(
		"bichat.messages",
		[]string{
			"id",
			"session_id",
			"role",
			"content",
			"author_user_id",
			"tool_calls",
			"tool_call_id",
			"citations",
			"debug_trace",
			"question_data",
			"created_at",
		},
	)
	updateMessageQuestionDataQuery = repo.Join(
		"UPDATE bichat.messages m",
		"SET question_data = $1",
		"FROM bichat.sessions s",
		repo.JoinWhere("m.session_id = s.id", "s.tenant_id = $2", "m.id = $3"),
	)
)

func buildSelectMessageQuery() string {
	return repo.Join(
		selectMessageColumns,
		selectMessageFrom,
		repo.JoinWhere("s.tenant_id = $1", "m.id = $2"),
	)
}

func buildSelectSessionMessagesQuery(opts domain.ListOptions) string {
	parts := []string{
		selectMessageColumns,
		selectMessageFrom,
		repo.JoinWhere("s.tenant_id = $1", "m.session_id = $2"),
		"ORDER BY m.created_at ASC",
	}

	if pagination := repo.FormatLimitOffset(opts.Limit, opts.Offset); pagination != "" {
		parts = append(parts, pagination)
	}

	return repo.Join(parts...)
}

func buildSelectPendingQuestionMessageQuery() string {
	return repo.Join(
		selectMessageColumns,
		selectMessageFrom,
		repo.JoinWhere("s.tenant_id = $1", "m.session_id = $2", "m.question_data->>'status' = 'PENDING'"),
		"ORDER BY m.created_at DESC, m.id DESC",
		"LIMIT 1",
	)
}
