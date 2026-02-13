package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	_ "github.com/lib/pq"
	"gopkg.in/yaml.v2"
)

type adminConfig struct {
	DB struct {
		DataSource string `yaml:"DataSource"`
	} `yaml:"DB"`
}

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "admin/etc/admin-api.yaml", "path to admin-api.yaml")
	flag.Parse()

	cfgBytes, err := os.ReadFile(configPath)
	if err != nil {
		exitErr(fmt.Errorf("read config failed: %w", err))
	}

	var cfg adminConfig
	if err := yaml.Unmarshal(cfgBytes, &cfg); err != nil {
		exitErr(fmt.Errorf("parse yaml failed: %w", err))
	}

	dsn := strings.TrimSpace(cfg.DB.DataSource)
	if len(dsn) == 0 {
		exitErr(fmt.Errorf("DB.DataSource is empty in %s", configPath))
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		exitErr(fmt.Errorf("open db failed: %w", err))
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		exitErr(fmt.Errorf("db ping failed: %w", err))
	}

	tx, err := db.Begin()
	if err != nil {
		exitErr(fmt.Errorf("begin tx failed: %w", err))
	}

	if err := ensureNodeServerId(tx); err != nil {
		_ = tx.Rollback()
		exitErr(err)
	}
	if err := ensureNodeConfig(tx); err != nil {
		_ = tx.Rollback()
		exitErr(err)
	}
	if err := ensureConfigKeySequence(tx); err != nil {
		_ = tx.Rollback()
		exitErr(err)
	}
	if err := migrateConfigServerIdToCfgNamespace(tx); err != nil {
		_ = tx.Rollback()
		exitErr(err)
	}
	if err := ensureConfigServerIdUnique(tx); err != nil {
		_ = tx.Rollback()
		exitErr(err)
	}
	if err := migrateNativeEndpointToConfig(tx); err != nil {
		_ = tx.Rollback()
		exitErr(err)
	}
	if err := ensureExternalServicesAccountIdSequence(tx); err != nil {
		_ = tx.Rollback()
		exitErr(err)
	}
	if err := backfillNodeServerId(tx); err != nil {
		_ = tx.Rollback()
		exitErr(err)
	}
	if err := dropDeprecatedColumns(tx); err != nil {
		_ = tx.Rollback()
		exitErr(err)
	}

	if err := tx.Commit(); err != nil {
		exitErr(fmt.Errorf("commit failed: %w", err))
	}

	if err := verifySchema(db); err != nil {
		exitErr(err)
	}
	fmt.Println("OK: migration applied (server_id-only linkage, deprecated columns dropped)")
}

func ensureNodeServerId(tx *sql.Tx) error {
	stmts := []string{
		`ALTER TABLE "public"."ae_mcp_task_node" ADD COLUMN IF NOT EXISTS "server_id" varchar(64)`,
		`CREATE INDEX IF NOT EXISTS "idx_ae_mcp_task_node_server_id" ON "public"."ae_mcp_task_node" ("server_id")`,
	}
	for _, stmt := range stmts {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("exec failed: %s: %w", stmt, err)
		}
	}
	return nil
}

func ensureNodeConfig(tx *sql.Tx) error {
	stmts := []string{
		`ALTER TABLE "public"."ae_mcp_task_node" ADD COLUMN IF NOT EXISTS "node_config" jsonb`,
		`ALTER TABLE "public"."ae_mcp_task_node" ALTER COLUMN "node_config" SET DEFAULT '{}'::jsonb`,
	}
	for _, stmt := range stmts {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("exec failed: %s: %w", stmt, err)
		}
	}
	return nil
}

func ensureConfigKeySequence(tx *sql.Tx) error {
	if _, err := tx.Exec(`CREATE SEQUENCE IF NOT EXISTS "public"."ae_mcp_config_key_seq" INCREMENT 1 MINVALUE 1 MAXVALUE 9223372036854775807 START 1 CACHE 1`); err != nil {
		return fmt.Errorf("create ae_mcp_config_key_seq failed: %w", err)
	}
	if _, err := tx.Exec(`SELECT setval('"public"."ae_mcp_config_key_seq"', (SELECT COALESCE(MAX(id), 0) + 1 FROM "public"."ae_mcp_external_services_config"), false)`); err != nil {
		return fmt.Errorf("setval ae_mcp_config_key_seq failed: %w", err)
	}
	return nil
}

func migrateConfigServerIdToCfgNamespace(tx *sql.Tx) error {
	if _, err := tx.Exec(`
		UPDATE "public"."ae_mcp_external_services_config"
		SET server_id = 'cfg_' || lpad(id::text, 7, '0')
		WHERE server_id IS NULL
		   OR server_id = ''
		   OR server_id NOT LIKE 'cfg_%'
	`); err != nil {
		return fmt.Errorf("migrate config server_id to cfg_ namespace failed: %w", err)
	}
	return nil
}

func ensureConfigServerIdUnique(tx *sql.Tx) error {
	if _, err := tx.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS "uk_ae_mcp_external_services_config_server_id" ON "public"."ae_mcp_external_services_config" ("server_id")`); err != nil {
		return fmt.Errorf("create unique index failed: %w", err)
	}
	return nil
}

func backfillNodeServerId(tx *sql.Tx) error {
	if ok, err := columnExists(tx, "ae_mcp_task_node", "bind_service_id"); err != nil {
		return err
	} else if ok {
		if _, err := tx.Exec(`
			UPDATE "public"."ae_mcp_task_node" n
			SET server_id = s.server_id
			FROM "public"."ae_mcp_services" s
			WHERE (n.server_id IS NULL OR n.server_id = '')
			  AND n.bind_service_id IS NOT NULL
			  AND n.bind_service_id = s.id
		`); err != nil {
			return fmt.Errorf("backfill node server_id from bind_service_id failed: %w", err)
		}
	}
	ok1, err := columnExists(tx, "ae_mcp_task_node", "external_service_id")
	if err != nil {
		return err
	}
	ok2, err := columnExists(tx, "ae_mcp_external_services_config", "external_service_id")
	if err != nil {
		return err
	}
	if ok1 && ok2 {
		if _, err := tx.Exec(`
			UPDATE "public"."ae_mcp_task_node" n
			SET server_id = c.server_id
			FROM "public"."ae_mcp_external_services_config" c
			WHERE (n.server_id IS NULL OR n.server_id = '')
			  AND n.external_service_id IS NOT NULL
			  AND c.external_service_id IS NOT NULL
			  AND n.external_service_id::text = c.external_service_id::text
		`); err != nil {
			return fmt.Errorf("backfill node server_id from external_service_id failed: %w", err)
		}
	}
	return nil
}

func migrateNativeEndpointToConfig(tx *sql.Tx) error {
	ok, err := columnExists(tx, "ae_mcp_services", "endpoint_json")
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	hasSourceType, err := columnExists(tx, "ae_mcp_services", "source_type")
	if err != nil {
		return err
	}

	type row struct {
		ServerId     string
		ServerName   string
		Description  string
		ProjectName  string
		EndpointJson string
	}
	query := `
		SELECT server_id, server_name, coalesce(description, ''), coalesce(project_name, ''), coalesce(endpoint_json, '')
		FROM "public"."ae_mcp_services"
		WHERE endpoint_json IS NOT NULL
		  AND endpoint_json <> ''`
	if hasSourceType {
		query += ` AND source_type = 'native'`
	}
	rows, err := tx.Query(query)
	if err != nil {
		return fmt.Errorf("query native endpoint_json failed: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var r row
		if err := rows.Scan(&r.ServerId, &r.ServerName, &r.Description, &r.ProjectName, &r.EndpointJson); err != nil {
			return fmt.Errorf("scan native endpoint_json failed: %w", err)
		}
		serverId := strings.TrimSpace(r.ServerId)
		if len(serverId) == 0 {
			continue
		}
		endpointJson := strings.TrimSpace(r.EndpointJson)
		if len(endpointJson) == 0 {
			continue
		}
		endpointType := inferEndpointType(endpointJson)

		var configId int64
		var configType string
		var connectInfo string
		var name string
		var projectName string
		var description string
		err := tx.QueryRow(`
			SELECT id, coalesce(type, ''), coalesce(connect_info, ''), coalesce(name, ''), coalesce(project_name, ''), coalesce(description, '')
			FROM "public"."ae_mcp_external_services_config"
			WHERE server_id = $1
			LIMIT 1
		`, serverId).Scan(&configId, &configType, &connectInfo, &name, &projectName, &description)
		if err != nil {
			if err == sql.ErrNoRows {
				if _, err := tx.Exec(`
					INSERT INTO "public"."ae_mcp_external_services_config"
						(name, type, launch_info, connect_info, max_instance, create_status, description, project_name, server_id, install_info, account_required, test_status, online_status)
					VALUES
						($1, $2, $3, $4, 1, false, $5, $6, $7, NULL, 0, 0, 0)
				`, strings.TrimSpace(r.ServerName), endpointType, "{}", endpointJson, strings.TrimSpace(r.Description), strings.TrimSpace(r.ProjectName), serverId); err != nil {
					return fmt.Errorf("insert config for native failed (server_id=%s): %w", serverId, err)
				}
				continue
			}
			return fmt.Errorf("query config by server_id failed (server_id=%s): %w", serverId, err)
		}

		if _, err := tx.Exec(`
			UPDATE "public"."ae_mcp_external_services_config"
			SET
				connect_info = CASE WHEN connect_info IS NULL OR connect_info = '' THEN $2 ELSE connect_info END,
				type = CASE WHEN type IS NULL OR type = '' THEN $3 ELSE type END,
				name = CASE WHEN name IS NULL OR name = '' THEN $4 ELSE name END,
				project_name = CASE WHEN project_name IS NULL OR project_name = '' THEN $5 ELSE project_name END,
				description = CASE WHEN description IS NULL OR description = '' THEN $6 ELSE description END
			WHERE id = $1
		`, configId, endpointJson, endpointType, strings.TrimSpace(r.ServerName), strings.TrimSpace(r.ProjectName), strings.TrimSpace(r.Description)); err != nil {
			return fmt.Errorf("update config for native failed (server_id=%s): %w", serverId, err)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("native endpoint_json rows error: %w", err)
	}
	return nil
}

func ensureExternalServicesAccountIdSequence(tx *sql.Tx) error {
	tableOk, err := tableExists(tx, "ae_mcp_external_services_account")
	if err != nil {
		return err
	}
	if !tableOk {
		return nil
	}
	seqOk, err := sequenceExists(tx, "ae_mcp_external_services_account_id_seq")
	if err != nil {
		return err
	}
	if !seqOk {
		return nil
	}

	var maxId int64
	if err := tx.QueryRow(`SELECT COALESCE(MAX(id), 0) FROM "public"."ae_mcp_external_services_account"`).Scan(&maxId); err != nil {
		return fmt.Errorf("query max account id failed: %w", err)
	}
	var lastValue int64
	if err := tx.QueryRow(`SELECT last_value FROM "public"."ae_mcp_external_services_account_id_seq"`).Scan(&lastValue); err != nil {
		return fmt.Errorf("query account id sequence last_value failed: %w", err)
	}
	if lastValue >= maxId+1 {
		return nil
	}
	if _, err := tx.Exec(`SELECT setval('public.ae_mcp_external_services_account_id_seq', $1, false)`, maxId+1); err != nil {
		return fmt.Errorf("setval account id sequence failed: %w", err)
	}
	return nil
}

func inferEndpointType(endpointJson string) string {
	var v map[string]any
	if err := json.Unmarshal([]byte(endpointJson), &v); err != nil {
		return "sse"
	}
	if _, ok := v["command"]; ok {
		return "stdio"
	}
	if rawURL, ok := v["url"]; ok {
		if url, ok := rawURL.(string); ok {
			if strings.Contains(url, "/sse") || strings.Contains(url, "sse") {
				return "sse"
			}
			return "httpStreamable"
		}
	}
	return "sse"
}

func dropDeprecatedColumns(tx *sql.Tx) error {
	stmts := []string{
		`DROP INDEX IF EXISTS "idx_ae_mcp_services_source_config_id"`,
		`DROP INDEX IF EXISTS "idx_ae_mcp_task_node_bind_service_id"`,

		`ALTER TABLE "public"."ae_mcp_services" DROP COLUMN IF EXISTS "source_config_id"`,
		`ALTER TABLE "public"."ae_mcp_services" DROP COLUMN IF EXISTS "source_type"`,
		`ALTER TABLE "public"."ae_mcp_services" DROP COLUMN IF EXISTS "endpoint_json"`,
		`ALTER TABLE "public"."ae_mcp_task_node" DROP COLUMN IF EXISTS "bind_kind"`,
		`ALTER TABLE "public"."ae_mcp_task_node" DROP COLUMN IF EXISTS "bind_service_id"`,
		`ALTER TABLE "public"."ae_mcp_task_node" DROP COLUMN IF EXISTS "external_service_id"`,
		`ALTER TABLE "public"."ae_mcp_external_services_config" DROP COLUMN IF EXISTS "external_service_id"`,
	}
	for _, stmt := range stmts {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("exec failed: %s: %w", stmt, err)
		}
	}
	return nil
}

func verifySchema(db *sql.DB) error {
	type need struct {
		Table  string
		Column string
		Want   bool
	}
	checks := []need{
		{Table: "ae_mcp_task_node", Column: "server_id", Want: true},
		{Table: "ae_mcp_task_node", Column: "node_config", Want: true},
		{Table: "ae_mcp_external_services_config", Column: "server_id", Want: true},

		{Table: "ae_mcp_services", Column: "source_config_id", Want: false},
		{Table: "ae_mcp_services", Column: "source_type", Want: false},
		{Table: "ae_mcp_services", Column: "endpoint_json", Want: false},
		{Table: "ae_mcp_task_node", Column: "bind_kind", Want: false},
		{Table: "ae_mcp_task_node", Column: "bind_service_id", Want: false},
		{Table: "ae_mcp_task_node", Column: "external_service_id", Want: false},
		{Table: "ae_mcp_external_services_config", Column: "external_service_id", Want: false},
	}

	for _, c := range checks {
		var count int
		if err := db.QueryRow(
			`SELECT count(*) FROM information_schema.columns WHERE table_schema='public' AND table_name=$1 AND column_name=$2`,
			c.Table,
			c.Column,
		).Scan(&count); err != nil {
			return fmt.Errorf("verify query failed: %w", err)
		}
		exists := count > 0
		if c.Want != exists {
			return fmt.Errorf("schema mismatch: %s.%s want=%v exists=%v", c.Table, c.Column, c.Want, exists)
		}
	}

	var nullCount int64
	if err := db.QueryRow(`SELECT count(*) FROM "public"."ae_mcp_external_services_config" WHERE server_id IS NULL OR server_id = ''`).Scan(&nullCount); err != nil {
		return fmt.Errorf("verify server_id not null failed: %w", err)
	}
	if nullCount != 0 {
		return fmt.Errorf("verify failed: ae_mcp_external_services_config has %d rows with empty server_id", nullCount)
	}

	return nil
}

func queryInt64Slice(tx *sql.Tx, query string, dest *[]int64) error {
	rows, err := tx.Query(query)
	if err != nil {
		return fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var v int64
		if err := rows.Scan(&v); err != nil {
			return fmt.Errorf("scan failed: %w", err)
		}
		*dest = append(*dest, v)
	}
	return rows.Err()
}

func columnExists(tx *sql.Tx, table string, column string) (bool, error) {
	var count int
	if err := tx.QueryRow(
		`SELECT count(*) FROM information_schema.columns WHERE table_schema='public' AND table_name=$1 AND column_name=$2`,
		table,
		column,
	).Scan(&count); err != nil {
		return false, fmt.Errorf("columnExists query failed: %w", err)
	}
	return count > 0, nil
}

func tableExists(tx *sql.Tx, table string) (bool, error) {
	var count int
	if err := tx.QueryRow(
		`SELECT count(*) FROM information_schema.tables WHERE table_schema='public' AND table_name=$1`,
		table,
	).Scan(&count); err != nil {
		return false, fmt.Errorf("tableExists query failed: %w", err)
	}
	return count > 0, nil
}

func sequenceExists(tx *sql.Tx, seq string) (bool, error) {
	var count int
	if err := tx.QueryRow(
		`SELECT count(*) FROM information_schema.sequences WHERE sequence_schema='public' AND sequence_name=$1`,
		seq,
	).Scan(&count); err != nil {
		return false, fmt.Errorf("sequenceExists query failed: %w", err)
	}
	return count > 0, nil
}

func formatServerId(id int64) string {
	return fmt.Sprintf("server_%07d", id)
}

func nextServiceNumeric(tx *sql.Tx) (int64, error) {
	query := `
		SELECT nextval('"public"."ae_mcp_server_id_seq"')
		FROM (
			SELECT 
				nextval('"public"."ae_mcp_server_id_seq"') as seq_val,
				(SELECT coalesce(max(id), 0) FROM "public"."ae_mcp_services") as max_id
		) s, generate_series(1, GREATEST(1, (max_id - seq_val + 1 + floor(random() * 9))::int))
		ORDER BY 1 DESC
		LIMIT 1`
	var id int64
	if err := tx.QueryRow(query).Scan(&id); err != nil {
		return 0, fmt.Errorf("next server numeric failed: %w", err)
	}
	return id, nil
}

func exitErr(err error) {
	fmt.Fprintln(os.Stderr, "ERROR:", err.Error())
	os.Exit(1)
}
