package spotlight

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/meilisearch/meilisearch-go"
	meilimocks "github.com/meilisearch/meilisearch-go/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestMeilisearchEngine_SetupForSearchExistingIndexStaysReadOnly(t *testing.T) {
	service := meilimocks.NewMockmeilisearchServiceManager(t)
	index := meilimocks.NewMockmeilisearchIndexManager(t)
	engine := &MeilisearchEngine{
		client:    service,
		indexName: "spotlight",
	}

	service.EXPECT().
		GetIndex("spotlight").
		Return(&meilisearch.IndexResult{UID: "spotlight"}, nil).
		Once()
	service.EXPECT().
		Index("spotlight").
		Return(index).
		Once()
	index.EXPECT().
		GetSettings().
		Return(&meilisearch.Settings{
			FilterableAttributes: requiredFilterableAttributes(),
			SearchableAttributes: requiredSearchableAttributes(),
			SortableAttributes:   requiredSortableAttributes(),
		}, nil).
		Once()

	require.NoError(t, engine.setupForSearch())
	require.True(t, engine.searchReady.Load())
	require.False(t, engine.writeReady.Load())

	service.EXPECT().
		GetIndex("spotlight").
		Return(&meilisearch.IndexResult{UID: "spotlight"}, nil).
		Once()
	service.EXPECT().
		Index("spotlight").
		Return(index).
		Once()
	index.EXPECT().
		UpdateFilterableAttributes(mock.Anything).
		Return(&meilisearch.TaskInfo{TaskUID: 1}, nil).
		Once()
	service.EXPECT().
		WaitForTask(int64(1), 100*time.Millisecond).
		Return(&meilisearch.Task{}, nil).
		Once()
	index.EXPECT().
		UpdateSearchableAttributes(mock.Anything).
		Return(&meilisearch.TaskInfo{TaskUID: 2}, nil).
		Once()
	service.EXPECT().
		WaitForTask(int64(2), 100*time.Millisecond).
		Return(&meilisearch.Task{}, nil).
		Once()
	index.EXPECT().
		UpdateSortableAttributes(mock.Anything).
		Return(&meilisearch.TaskInfo{TaskUID: 3}, nil).
		Once()
	service.EXPECT().
		WaitForTask(int64(3), 100*time.Millisecond).
		Return(&meilisearch.Task{}, nil).
		Once()

	require.NoError(t, engine.setup())
	require.True(t, engine.searchReady.Load())
	require.True(t, engine.writeReady.Load())
}

func TestMeilisearchEngine_SetupForSearchMissingIndexBootstrapsIt(t *testing.T) {
	service := meilimocks.NewMockmeilisearchServiceManager(t)
	index := meilimocks.NewMockmeilisearchIndexManager(t)
	engine := &MeilisearchEngine{
		client:    service,
		indexName: "spotlight",
	}

	service.EXPECT().
		GetIndex("spotlight").
		Return(nil, &meilisearch.Error{StatusCode: http.StatusNotFound}).
		Once()
	service.EXPECT().
		CreateIndex(mock.AnythingOfType("*meilisearch.IndexConfig")).
		Return(&meilisearch.TaskInfo{TaskUID: 10}, nil).
		Once()
	service.EXPECT().
		WaitForTask(int64(10), 100*time.Millisecond).
		Return(&meilisearch.Task{}, nil).
		Once()
	service.EXPECT().
		Index("spotlight").
		Return(index).
		Once()
	index.EXPECT().
		UpdateFilterableAttributes(mock.Anything).
		Return(&meilisearch.TaskInfo{TaskUID: 11}, nil).
		Once()
	service.EXPECT().
		WaitForTask(int64(11), 100*time.Millisecond).
		Return(&meilisearch.Task{}, nil).
		Once()
	index.EXPECT().
		UpdateSearchableAttributes(mock.Anything).
		Return(&meilisearch.TaskInfo{TaskUID: 12}, nil).
		Once()
	service.EXPECT().
		WaitForTask(int64(12), 100*time.Millisecond).
		Return(&meilisearch.Task{}, nil).
		Once()
	index.EXPECT().
		UpdateSortableAttributes(mock.Anything).
		Return(&meilisearch.TaskInfo{TaskUID: 13}, nil).
		Once()
	service.EXPECT().
		WaitForTask(int64(13), 100*time.Millisecond).
		Return(&meilisearch.Task{}, nil).
		Once()

	require.NoError(t, engine.setupForSearch())
	require.True(t, engine.searchReady.Load())
	require.True(t, engine.writeReady.Load())
}

func TestMeilisearchEngine_SetupForSearchRejectsExistingIndexWithoutRequiredSettings(t *testing.T) {
	service := meilimocks.NewMockmeilisearchServiceManager(t)
	index := meilimocks.NewMockmeilisearchIndexManager(t)
	engine := &MeilisearchEngine{
		client:    service,
		indexName: "spotlight",
	}

	service.EXPECT().
		GetIndex("spotlight").
		Return(&meilisearch.IndexResult{UID: "spotlight"}, nil).
		Once()
	service.EXPECT().
		Index("spotlight").
		Return(index).
		Once()
	index.EXPECT().
		GetSettings().
		Return(&meilisearch.Settings{
			FilterableAttributes: []string{"tenant_id"},
			SearchableAttributes: requiredSearchableAttributes(),
			SortableAttributes:   requiredSortableAttributes(),
		}, nil).
		Once()

	err := engine.setupForSearch()
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing filterable attributes")
	require.False(t, engine.searchReady.Load())
}

func TestMeilisearchEngine_SetupForSearchAllowsStaleActiveIndex(t *testing.T) {
	service := meilimocks.NewMockmeilisearchServiceManager(t)
	index := meilimocks.NewMockmeilisearchIndexManager(t)
	engine := &MeilisearchEngine{
		client:    service,
		indexName: "spotlight",
	}

	service.EXPECT().
		GetIndex("spotlight").
		Return(&meilisearch.IndexResult{UID: "spotlight"}, nil).
		Once()
	service.EXPECT().
		Index("spotlight").
		Return(index).
		Once()
	index.EXPECT().
		GetSettings().
		Return(&meilisearch.Settings{
			FilterableAttributes: requiredFilterableAttributes(),
			SearchableAttributes: requiredSearchableAttributes(),
			SortableAttributes:   requiredSortableAttributes(),
		}, nil).
		Once()

	require.NoError(t, engine.setupForSearch())
	require.True(t, engine.searchReady.Load())
}

func TestMeilisearchEngine_SetupForSearchFallsBackToReadyBuildIndexWhenActiveEmpty(t *testing.T) {
	service := meilimocks.NewMockmeilisearchServiceManager(t)
	activeIndex := meilimocks.NewMockmeilisearchIndexManager(t)
	buildIndex := meilimocks.NewMockmeilisearchIndexManager(t)
	engine := &MeilisearchEngine{
		client:     service,
		indexName:  "spotlight",
		activeName: "spotlight",
	}
	buildIndexName := rebuildIndexName("spotlight")

	service.EXPECT().
		Index("spotlight").
		Return(activeIndex).
		Once()
	activeIndex.EXPECT().
		GetStats().
		Return(&meilisearch.StatsIndex{
			NumberOfDocuments: 0,
		}, nil).
		Once()

	service.EXPECT().
		Index(buildIndexName).
		Return(buildIndex).
		Times(3)
	buildIndex.EXPECT().
		GetStats().
		Return(&meilisearch.StatsIndex{
			NumberOfDocuments: 42,
			FieldDistribution: map[string]int64{
				"domain":              42,
				"description":         42,
				"search_text":         42,
				"exact_terms":         42,
				"schema_version":      42,
				"access_visibility":   42,
				"owner_id":            42,
				"allowed_users":       42,
				"allowed_roles":       42,
				"allowed_permissions": 42,
			},
		}, nil).
		Once()
	buildIndex.EXPECT().
		Search("", mock.AnythingOfType("*meilisearch.SearchRequest")).
		Return(&meilisearch.SearchResponse{
			Hits: meilisearch.Hits{
				meilisearch.Hit{
					"schema_version": json.RawMessage(`"` + IndexSchemaVersion + `"`),
				},
			},
		}, nil).
		Once()
	buildIndex.EXPECT().
		GetSettings().
		Return(&meilisearch.Settings{
			FilterableAttributes: requiredFilterableAttributes(),
			SearchableAttributes: requiredSearchableAttributes(),
			SortableAttributes:   requiredSortableAttributes(),
		}, nil).
		Once()

	require.NoError(t, engine.setupForSearch())
	require.True(t, engine.searchReady.Load())
}

func TestBuildSearchFilterEscapesDynamicValues(t *testing.T) {
	req := SearchRequest{
		TenantID:         uuidMustParse("11111111-1111-1111-1111-111111111111"),
		PreferredDomains: []ResultDomain{ResultDomain(`people" OR domain = "admin`)},
		UserID:           `user" OR access_visibility = "public`,
		Roles:            []string{`ops" OR allowed_roles = "admin`},
		Permissions:      []string{`read" OR allowed_permissions = "write`},
	}

	filter := buildSearchFilter(req)
	require.Contains(t, filter, `domain = "people\" OR domain = \"admin"`)
	require.Contains(t, filter, `owner_id = "user\" OR access_visibility = \"public"`)
	require.Contains(t, filter, `allowed_roles = "ops\" OR allowed_roles = \"admin"`)
	require.Contains(t, filter, `allowed_permissions = "read\" OR allowed_permissions = \"write"`)
}

func TestMeiliRebuildSessionCommitCreatesActiveIndexBeforeSwapWhenMissing(t *testing.T) {
	service := meilimocks.NewMockmeilisearchServiceManager(t)

	session := &meiliRebuildSession{
		client:          service,
		activeIndexName: "spotlight",
		buildIndexName:  "spotlight_build_v4",
		engine: &MeilisearchEngine{
			client:    service,
			indexName: "spotlight",
		},
	}

	service.EXPECT().
		GetIndex("spotlight").
		Return(nil, &meilisearch.Error{StatusCode: http.StatusNotFound}).
		Once()
	service.EXPECT().
		CreateIndex(mock.AnythingOfType("*meilisearch.IndexConfig")).
		Return(&meilisearch.TaskInfo{TaskUID: 20}, nil).
		Once()
	service.EXPECT().
		WaitForTask(int64(20), 100*time.Millisecond).
		Return(&meilisearch.Task{}, nil).
		Once()
	service.EXPECT().
		SwapIndexesWithContext(mock.Anything, []*meilisearch.SwapIndexesParams{{
			Indexes: []string{"spotlight_build_v4", "spotlight"},
		}}).
		Return(&meilisearch.TaskInfo{TaskUID: 21}, nil).
		Once()
	service.EXPECT().
		WaitForTask(int64(21), 100*time.Millisecond).
		Return(&meilisearch.Task{}, nil).
		Once()
	service.EXPECT().
		DeleteIndexWithContext(mock.Anything, "spotlight_build_v4").
		Return(&meilisearch.TaskInfo{TaskUID: 22}, nil).
		Once()
	service.EXPECT().
		WaitForTask(int64(22), 100*time.Millisecond).
		Return(&meilisearch.Task{}, nil).
		Once()

	require.NoError(t, session.Commit(t.Context()))
}

func TestMeilisearchEngine_DeleteTenant_RemovesOnlyRequestedTenantDocuments(t *testing.T) {
	service := meilimocks.NewMockmeilisearchServiceManager(t)
	index := meilimocks.NewMockmeilisearchIndexManager(t)
	tenantID := uuidMustParse("11111111-1111-1111-1111-111111111111")
	engine := &MeilisearchEngine{
		client:    service,
		indexName: "spotlight",
	}

	service.EXPECT().
		GetIndex("spotlight").
		Return(&meilisearch.IndexResult{UID: "spotlight"}, nil).
		Once()
	service.EXPECT().
		Index("spotlight").
		Return(index).
		Once()
	index.EXPECT().
		UpdateFilterableAttributes(mock.Anything).
		Return(&meilisearch.TaskInfo{TaskUID: 31}, nil).
		Once()
	service.EXPECT().
		WaitForTask(int64(31), 100*time.Millisecond).
		Return(&meilisearch.Task{}, nil).
		Once()
	index.EXPECT().
		UpdateSearchableAttributes(mock.Anything).
		Return(&meilisearch.TaskInfo{TaskUID: 32}, nil).
		Once()
	service.EXPECT().
		WaitForTask(int64(32), 100*time.Millisecond).
		Return(&meilisearch.Task{}, nil).
		Once()
	index.EXPECT().
		UpdateSortableAttributes(mock.Anything).
		Return(&meilisearch.TaskInfo{TaskUID: 33}, nil).
		Once()
	service.EXPECT().
		WaitForTask(int64(33), 100*time.Millisecond).
		Return(&meilisearch.Task{}, nil).
		Once()
	service.EXPECT().
		Index("spotlight").
		Return(index).
		Once()
	index.EXPECT().
		DeleteDocumentsByFilterWithContext(mock.Anything, `tenant_id = "11111111-1111-1111-1111-111111111111"`, (*meilisearch.DocumentOptions)(nil)).
		Return(&meilisearch.TaskInfo{TaskUID: 34}, nil).
		Once()
	service.EXPECT().
		WaitForTask(int64(34), 100*time.Millisecond).
		Return(&meilisearch.Task{}, nil).
		Once()

	require.NoError(t, engine.DeleteTenant(t.Context(), tenantID))
}

func uuidMustParse(raw string) uuid.UUID {
	parsed, err := uuid.Parse(raw)
	if err != nil {
		panic(err)
	}
	return parsed
}
