package repository

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	_ "github.com/Wei-Shaw/sub2api/ent/runtime"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func TestAdobeImageQualityPricingUpgradeAndPersistence(t *testing.T) {
	dsn := os.Getenv("OKAD_RECOVERY_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("requires disposable PostgreSQL")
	}
	root, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = root.Close() })
	name := fmt.Sprintf("adobe_quality_%d", time.Now().UnixNano())
	_, err = root.Exec("CREATE DATABASE " + name)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = root.Exec("DROP DATABASE " + name) })
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	u.Path = "/" + name
	query := u.Query()
	query.Del("search_path")
	u.RawQuery = query.Encode()
	db, err := sql.Open("postgres", u.String())
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	baseline := fstest.MapFS{}
	entries, err := migrations.FS.ReadDir(".")
	require.NoError(t, err)
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".sql") || (entry.Name() == "256_adobe_image_quality_pricing.sql" || entry.Name() == "257_adobe_multi_model_group_names.sql" || entry.Name() == "258_adobe_quality_pricing_enabled.sql") {
			continue
		}
		body, err := migrations.FS.ReadFile(entry.Name())
		require.NoError(t, err)
		baseline[entry.Name()] = &fstest.MapFile{Data: body}
	}
	bootstrap := fstest.MapFS{}
	for name, entry := range baseline {
		if name < "109_" || name == "121_auth_identity_migration_report_type_widen.sql" {
			bootstrap[name] = entry
		}
	}
	ctx := context.Background()
	require.NoError(t, applyMigrationsFS(ctx, db, bootstrap))
	require.NoError(t, applyMigrationsFS(ctx, db, baseline))
	_, err = db.Exec(`INSERT INTO users(id,email,password_hash) VALUES (1,'quality@example.test','synthetic');
 INSERT INTO groups(id,name,platform,image_price_4k) VALUES (900001,'Existing Adobe','adobe',0.19);
 INSERT INTO accounts(id,name,platform,type,credentials) VALUES (1,'test','adobe','oauth','{}');
 INSERT INTO api_keys(id,user_id,key,name,group_id) VALUES (1,1,'sk-synthetic-quality-test','test',900001);
 INSERT INTO usage_logs(user_id,api_key_id,account_id,request_id,model,image_count,image_size,total_cost,actual_cost) VALUES (1,1,1,'old-request','gpt-image-2.6-flare',1,'4K',0.19,0.19);`)
	require.NoError(t, err)
	var oldID int64
	require.NoError(t, db.QueryRow("SELECT id FROM usage_logs WHERE request_id='old-request'").Scan(&oldID))
	var before string
	require.NoError(t, db.QueryRow("SELECT json_object_agg(filename,checksum)::text FROM schema_migrations").Scan(&before))

	oldQualitySchema, err := migrations.FS.ReadFile("256_adobe_image_quality_pricing.sql")
	require.NoError(t, err)
	_, err = db.Exec(string(oldQualitySchema))
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO groups(id,name,platform,image_quality_prices) VALUES (900002,'Existing saved quality prices','adobe','{"gpt-image-2.6-flare":{"4K":{"max":0.1234}}}')`)
	require.NoError(t, err)
	require.NoError(t, ApplyMigrations(ctx, db))
	require.NoError(t, ApplyMigrations(ctx, db))
	var after string
	require.NoError(t, db.QueryRow("SELECT json_object_agg(filename,checksum)::text FROM schema_migrations WHERE filename NOT IN ('256_adobe_image_quality_pricing.sql', '257_adobe_multi_model_group_names.sql', '258_adobe_quality_pricing_enabled.sql')").Scan(&after))
	require.JSONEq(t, before, after)
	var unset bool
	require.NoError(t, db.QueryRow("SELECT image_quality_prices IS NULL FROM groups WHERE id=900001").Scan(&unset))
	require.True(t, unset)
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	groups := newGroupRepositoryWithSQL(client, db)
	group, err := groups.GetByIDLite(ctx, 900001)
	require.NoError(t, err)
	require.Empty(t, group.ImageQualityPrices)
	require.True(t, group.ImageQualityPricingEnabled)
	existing, err := groups.GetByIDLite(ctx, 900002)
	require.NoError(t, err)
	require.True(t, existing.ImageQualityPricingEnabled, "new mode defaults Adobe groups to quality billing without erasing prices")
	require.Equal(t, 0.1234, existing.ImageQualityPrices["gpt-image-2.6-flare"]["4K"]["max"])
	require.Equal(t, 0.19, *group.ImagePrice4K)
	prices := map[string]map[string]map[string]float64{"gpt-image-2": {"1K": {"low": 0.0001}}, "gpt-image-2.5-flare": {"2K": {"xhigh": 0.1234}}, "gpt-image-2.6-flare": {"4K": {"xhigh": 0.31, "max": 0}}, "gpt-image-2.6-sunburst": {"2K": {"max": 0.47}}}
	group.Name = "Adobe GPT Image 2 + GPT Image 2.5 Flare + GPT Image 2.5 Sunburst + GPT Image 2.6 Flare + GPT Image 2.6 Sunburst - preview"
	require.Greater(t, len(group.Name), 100)
	group.ImageQualityPricingEnabled = true
	group.ImageQualityPrices = prices
	require.NoError(t, groups.Update(ctx, group))
	reloaded, err := groups.GetByIDLite(ctx, 900001)
	require.NoError(t, err)
	require.Equal(t, prices, reloaded.ImageQualityPrices)
	require.True(t, reloaded.ImageQualityPricingEnabled)
	keys := newAPIKeyRepositoryWithSQL(client, db)
	for _, enabled := range []bool{true, false, true} {
		group.ImageQualityPricingEnabled = enabled
		require.NoError(t, groups.Update(ctx, group))
		// Replaying this unpublished migration must not reset an explicit off.
		modeSchema, err := migrations.FS.ReadFile("258_adobe_quality_pricing_enabled.sql")
		require.NoError(t, err)
		_, err = db.Exec(string(modeSchema))
		require.NoError(t, err)
		saved, err := groups.GetByIDLite(ctx, group.ID)
		require.NoError(t, err)
		require.Equal(t, enabled, saved.ImageQualityPricingEnabled)
		require.Equal(t, prices, saved.ImageQualityPrices)
		for _, read := range []func(context.Context, string) (*service.APIKey, error){keys.GetByKey, keys.GetByKeyForAuth} {
			key, err := read(ctx, "sk-synthetic-quality-test")
			require.NoError(t, err)
			require.Equal(t, enabled, key.Group.ImageQualityPricingEnabled)
			require.Equal(t, prices, key.Group.ImageQualityPrices)
		}
	}
	require.Equal(t, group.Name, reloaded.Name)
	for _, model := range []string{"GPT Image 2", "GPT Image 2.5 Flare", "GPT Image 2.5 Sunburst", "GPT Image 2.6 Flare", "GPT Image 2.6 Sunburst"} {
		items, page, err := groups.ListWithFilters(domain.WithGroupAdobeModelFilter(ctx, model), pagination.PaginationParams{Page: 1, PageSize: 10}, "adobe", "", "", nil)
		require.NoError(t, err)
		require.EqualValues(t, 1, page.Total)
		require.Equal(t, group.Name, items[0].Name)
	}
	repo := newUsageLogRepositoryWithSQL(client, db)
	old, err := repo.GetByID(ctx, oldID)
	require.NoError(t, err)
	require.Nil(t, old.ImageQuality)
	require.Equal(t, 0.19, old.ActualCost)
	for _, quality := range []string{"xhigh", "max"} {
		size := "4K"
		log := &service.UsageLog{UserID: 1, APIKeyID: 1, AccountID: 1, RequestID: "quality-" + quality, Model: "gpt-image-2.6-flare", ImageCount: 1, ImageSize: &size, ImageQuality: &quality, TotalCost: 0.31, ActualCost: 0.62, RateMultiplier: 2, CreatedAt: time.Now()}
		inserted, err := repo.Create(ctx, log)
		require.NoError(t, err)
		require.True(t, inserted)
		got, err := repo.GetByID(ctx, log.ID)
		require.NoError(t, err)
		require.Equal(t, quality, *got.ImageQuality)
		require.Equal(t, 0.62, got.ActualCost)
		inserted, err = repo.Create(ctx, log)
		require.NoError(t, err)
		require.False(t, inserted, "retry must not create another bill")
	}
	group.ImageQualityPrices = map[string]map[string]map[string]float64{}
	require.NoError(t, groups.Update(ctx, group))
	reloaded, err = groups.GetByIDLite(ctx, 900001)
	require.NoError(t, err)
	require.Empty(t, reloaded.ImageQualityPrices)
	require.Equal(t, 0.19, *reloaded.ImagePrice4K)
}
