package drivers

import (
	"fmt"
	"os"
	"strings"

	"github.com/natansdj/lets"
)

type sqlDriverRegistration struct {
	name    string
	run     func() []func()
	enabled func() bool
}

var sqlDriverRegistry = map[string]sqlDriverRegistration{}

func init() {
	RegisterSQLDriver("postgres", Postgres, func() bool {
		return len(PostgresConfig) > 0
	})

	RegisterSQLDriver("mariadb", MySQL, func() bool {
		return len(MySQLConfig) > 0
	})

	RegisterSQLDriver("mysql", MySQL, func() bool {
		return len(MySQLConfig) > 0
	})

	RegisterSQLDriver("sqlite", SqLiteClient, func() bool {
		return SqLiteConfig != nil && len(*SqLiteConfig) > 0
	})
}

func RegisterSQLDriver(name string, run func() []func(), enabled func() bool) {
	normalized := normalizeSQLDriverName(name)
	if normalized == "" || run == nil || enabled == nil {
		return
	}

	sqlDriverRegistry[normalized] = sqlDriverRegistration{
		name:    normalized,
		run:     run,
		enabled: enabled,
	}
}

func ResolveSQLDrivers() []func() []func() {
	resolvedRunners, err := resolveSQLDriversStrict()
	if err != nil {
		lets.LogF("[ResolveSQLDrivers] %v", err)
		return []func() []func(){}
	}

	return resolvedRunners
}

func resolveSQLDriversStrict() ([]func() []func(), error) {
	configuredDrivers := strings.TrimSpace(os.Getenv("LETS_SQL_DRIVERS"))
	if configuredDrivers != "" {
		resolvedNames := parseSQLDriverList(configuredDrivers)
		if err := validatePrimaryDriverList(resolvedNames); err != nil {
			return nil, err
		}

		if strings.TrimSpace(os.Getenv("DB_ENGINE")) != "" && hasPrimaryDriver(resolvedNames) {
			return nil, fmt.Errorf("DB_ENGINE cannot be combined with primary engines in LETS_SQL_DRIVERS")
		}

		resolvedRunners := []func() []func(){}
		for _, name := range resolvedNames {
			registration := runnerByName(name)
			if registration == nil {
				return nil, fmt.Errorf("unknown LETS_SQL_DRIVERS entry: %s", name)
			}
			if !registration.enabled() {
				return nil, fmt.Errorf("LETS_SQL_DRIVERS entry %s is selected but not configured", name)
			}

			resolvedRunners = append(resolvedRunners, registration.run)
		}

		return resolvedRunners, nil
	}

	hasPrimaryConfig := hasEnabledPrimaryConfig()
	if !hasPrimaryConfig {
		resolvedRunners := []func() []func(){}
		for _, name := range []string{"sqlite"} {
			registration := runnerByName(name)
			if registration == nil || !registration.enabled() {
				continue
			}

			resolvedRunners = append(resolvedRunners, registration.run)
		}

		return resolvedRunners, nil
	}

	primaryEngine, err := parsePrimaryEngine(os.Getenv("DB_ENGINE"))
	if err != nil {
		return nil, err
	}

	resolvedNames := []string{primaryEngine}
	sqliteRegistration := runnerByName("sqlite")
	if sqliteRegistration != nil && sqliteRegistration.enabled() {
		resolvedNames = appendUnique(resolvedNames, "sqlite")
	}

	resolvedRunners := []func() []func(){}
	for _, name := range resolvedNames {
		registration := runnerByName(name)
		if registration == nil {
			return nil, fmt.Errorf("unknown SQL engine registration: %s", name)
		}
		if !registration.enabled() {
			return nil, fmt.Errorf("selected SQL engine %s is not configured", name)
		}

		resolvedRunners = append(resolvedRunners, registration.run)
	}

	return resolvedRunners, nil
}

func parsePrimaryEngine(raw string) (string, error) {
	engine := normalizeSQLDriverName(raw)
	if engine == "" {
		return "mariadb", nil
	}

	switch engine {
	case "mariadb", "mysql":
		return "mariadb", nil
	case "postgres":
		return "postgres", nil
	default:
		return "", fmt.Errorf("invalid DB_ENGINE value %q, allowed values: mariadb, mysql, postgres", engine)
	}
}

func validatePrimaryDriverList(drivers []string) error {
	primaryCount := 0
	for _, driver := range drivers {
		if isPrimaryDriver(driver) {
			primaryCount++
		}
	}

	if primaryCount > 1 {
		return fmt.Errorf("LETS_SQL_DRIVERS cannot contain more than one primary engine (mariadb/mysql/postgres)")
	}

	return nil
}

func hasPrimaryDriver(drivers []string) bool {
	for _, driver := range drivers {
		if isPrimaryDriver(driver) {
			return true
		}
	}

	return false
}

func isPrimaryDriver(driver string) bool {
	normalized := normalizeSQLDriverName(driver)
	return normalized == "mariadb" || normalized == "mysql" || normalized == "postgres"
}

func hasEnabledPrimaryConfig() bool {
	for _, name := range []string{"postgres", "mariadb"} {
		registration := runnerByName(name)
		if registration != nil && registration.enabled() {
			return true
		}
	}

	return false
}

func normalizeSQLDriverName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func parseSQLDriverList(raw string) []string {
	resolved := []string{}
	for _, item := range strings.Split(raw, ",") {
		normalized := normalizeSQLDriverName(item)
		if normalized == "" {
			continue
		}

		resolved = appendUnique(resolved, normalized)
	}

	return resolved
}

func appendUnique(items []string, item string) []string {
	normalized := normalizeSQLDriverName(item)
	if normalized == "" {
		return items
	}

	for _, existing := range items {
		if existing == normalized {
			return items
		}
	}

	return append(items, normalized)
}

func runnerByName(name string) *sqlDriverRegistration {
	normalized := normalizeSQLDriverName(name)
	if normalized == "" {
		return nil
	}

	registration, ok := sqlDriverRegistry[normalized]
	if !ok {
		return nil
	}

	return &registration
}
