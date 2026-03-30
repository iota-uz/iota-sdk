package spotlight

import (
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
	index.EXPECT().
		GetStats().
		Return(&meilisearch.StatsIndex{}, nil).
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

func TestMeilisearchEngine_SetupForSearchRejectsStaleSchemaDocuments(t *testing.T) {
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
	index.EXPECT().
		GetStats().
		Return(&meilisearch.StatsIndex{NumberOfDocuments: 3}, nil).
		Once()
	index.EXPECT().
		Search("", mock.MatchedBy(func(req *meilisearch.SearchRequest) bool {
			return req != nil && req.Filter == `schema_version = "`+IndexSchemaVersion+`"` && req.Limit == 1
		})).
		Return(&meilisearch.SearchResponse{EstimatedTotalHits: 2}, nil).
		Once()

	err := engine.setupForSearch()
	require.Error(t, err)
	require.Contains(t, err.Error(), "schema mismatch")
	require.False(t, engine.searchReady.Load())
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

func uuidMustParse(raw string) uuid.UUID {
	parsed, err := uuid.Parse(raw)
	if err != nil {
		panic(err)
	}
	return parsed
}
