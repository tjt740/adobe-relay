package repository

import (
	"context"
	"testing"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

// Both count and page selection must contain the version predicate. A client-side
// page filter would give incorrect totals and empty pages.
func TestGroupListAdobeModelBeforePagination(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	repo := &groupRepository{client: client, sql: db}
	pattern, _ := domain.AdobeGroupModelPattern("2.6")
	mock.ExpectQuery(`SELECT COUNT.*FROM "groups".*"platform" = \$1.*"name" ~\* \$2.*"status" = \$3`).
		WithArgs("adobe", pattern, "inactive").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(21))
	mock.ExpectQuery(`SELECT .*FROM "groups".*"platform" = \$1.*"name" ~\* \$2.*"status" = \$3.*LIMIT 10 OFFSET 10`).
		WithArgs("adobe", pattern, "inactive").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}))
	items, page, err := repo.ListWithFilters(domain.WithGroupAdobeModelFilter(context.Background(), "2.6"), pagination.PaginationParams{Page: 2, PageSize: 10}, "", "inactive", "", nil)
	require.NoError(t, err)
	require.Empty(t, items)
	require.EqualValues(t, 21, page.Total)
	require.Equal(t, 3, page.Pages)
	require.NoError(t, mock.ExpectationsWereMet())
}
