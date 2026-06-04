package drivers

import (
	"testing"

	"github.com/natansdj/lets/types"
)

func resetSQLDriverConfigs() {
	MySQLConfig = nil
	PostgresConfig = nil
	SqLiteConfig = nil
}

func TestResolveSQLDriversStrict_NoPrimaryConfig_AllowsNoEngine(t *testing.T) {
	t.Setenv("DB_ENGINE", "")
	t.Setenv("LETS_SQL_DRIVERS", "")
	resetSQLDriverConfigs()

	runners, err := resolveSQLDriversStrict()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(runners) != 0 {
		t.Fatalf("expected no runners, got %d", len(runners))
	}
}

func TestResolveSQLDriversStrict_DefaultsToMariaDBWhenEngineMissing(t *testing.T) {
	t.Setenv("DB_ENGINE", "")
	t.Setenv("LETS_SQL_DRIVERS", "")
	resetSQLDriverConfigs()
	MySQLConfig = []types.IMySQL{&types.MySQL{}}

	runners, err := resolveSQLDriversStrict()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(runners) != 1 {
		t.Fatalf("expected default mariadb runner, got %d", len(runners))
	}
}

func TestResolveSQLDriversStrict_DefaultEngineIgnoresLegacyAutoDetect(t *testing.T) {
	t.Setenv("DB_ENGINE", "")
	t.Setenv("LETS_SQL_DRIVERS", "")
	t.Setenv("LETS_STRICT_DB_ENGINE", "false")
	resetSQLDriverConfigs()
	MySQLConfig = []types.IMySQL{&types.MySQL{}}
	PostgresConfig = []types.IPostgres{&types.Postgres{}}
	sqliteCfg := []*types.SqLite{{DBPath: "main.db"}}
	SqLiteConfig = &sqliteCfg

	runners, err := resolveSQLDriversStrict()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(runners) != 2 {
		t.Fatalf("expected default mariadb plus sqlite runners, got %d", len(runners))
	}
}

func TestResolveSQLDriversStrict_MariaDBEngine(t *testing.T) {
	t.Setenv("DB_ENGINE", "mariadb")
	t.Setenv("LETS_SQL_DRIVERS", "")
	resetSQLDriverConfigs()
	MySQLConfig = []types.IMySQL{&types.MySQL{}}

	runners, err := resolveSQLDriversStrict()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(runners) != 1 {
		t.Fatalf("expected 1 runner, got %d", len(runners))
	}
}

func TestResolveSQLDriversStrict_PostgresPlusSQLite(t *testing.T) {
	t.Setenv("DB_ENGINE", "postgres")
	t.Setenv("LETS_SQL_DRIVERS", "")
	resetSQLDriverConfigs()
	PostgresConfig = []types.IPostgres{&types.Postgres{}}
	sqliteCfg := []*types.SqLite{{DBPath: "main.db"}}
	SqLiteConfig = &sqliteCfg

	runners, err := resolveSQLDriversStrict()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(runners) != 2 {
		t.Fatalf("expected 2 runners (postgres + sqlite), got %d", len(runners))
	}
}

func TestResolveSQLDriversStrict_InvalidEngine(t *testing.T) {
	t.Setenv("DB_ENGINE", "oracle")
	t.Setenv("LETS_SQL_DRIVERS", "")
	resetSQLDriverConfigs()
	MySQLConfig = []types.IMySQL{&types.MySQL{}}

	_, err := resolveSQLDriversStrict()
	if err == nil {
		t.Fatal("expected invalid DB_ENGINE error")
	}
}

func TestResolveSQLDriversStrict_RejectsMultiplePrimaryInList(t *testing.T) {
	t.Setenv("DB_ENGINE", "")
	t.Setenv("LETS_SQL_DRIVERS", "mariadb,postgres")
	resetSQLDriverConfigs()
	MySQLConfig = []types.IMySQL{&types.MySQL{}}
	PostgresConfig = []types.IPostgres{&types.Postgres{}}

	_, err := resolveSQLDriversStrict()
	if err == nil {
		t.Fatal("expected LETS_SQL_DRIVERS multi-primary error")
	}
}

func TestResolveSQLDriversStrict_RejectsPrimaryListWithDBEngine(t *testing.T) {
	t.Setenv("DB_ENGINE", "postgres")
	t.Setenv("LETS_SQL_DRIVERS", "mariadb")
	resetSQLDriverConfigs()
	MySQLConfig = []types.IMySQL{&types.MySQL{}}
	PostgresConfig = []types.IPostgres{&types.Postgres{}}

	_, err := resolveSQLDriversStrict()
	if err == nil {
		t.Fatal("expected conflict error between DB_ENGINE and LETS_SQL_DRIVERS")
	}
}

func TestResolveSQLDriversStrict_AllowSQLiteOnlyList(t *testing.T) {
	t.Setenv("DB_ENGINE", "")
	t.Setenv("LETS_SQL_DRIVERS", "sqlite")
	resetSQLDriverConfigs()
	sqliteCfg := []*types.SqLite{{DBPath: "main.db"}}
	SqLiteConfig = &sqliteCfg

	runners, err := resolveSQLDriversStrict()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(runners) != 1 {
		t.Fatalf("expected sqlite-only runner, got %d", len(runners))
	}
}
