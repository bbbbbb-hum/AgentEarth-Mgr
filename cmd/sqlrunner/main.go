package main

import (
	"database/sql"
	"flag"
	"fmt"
	"os"
	"strings"

	_ "github.com/lib/pq"
)

type stringSlice []string

func (s *stringSlice) String() string {
	return strings.Join(*s, ",")
}

func (s *stringSlice) Set(v string) error {
	*s = append(*s, v)
	return nil
}

type querySlice []string

func (s *querySlice) String() string {
	return strings.Join(*s, "\n")
}

func (s *querySlice) Set(v string) error {
	*s = append(*s, v)
	return nil
}

func main() {
	var dsn string
	var files stringSlice
	var queries querySlice
	flag.StringVar(&dsn, "dsn", "", "postgres DSN, e.g. postgres://user:pass@localhost:5432/db?sslmode=disable")
	flag.Var(&files, "f", "SQL file path (repeatable)")
	flag.Var(&queries, "q", "SQL query to execute (repeatable)")
	flag.Parse()

	if strings.TrimSpace(dsn) == "" {
		exitErr(fmt.Errorf("missing -dsn"))
	}
	if len(files) == 0 && len(queries) == 0 {
		exitErr(fmt.Errorf("missing -f or -q"))
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		exitErr(fmt.Errorf("open db failed: %w", err))
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		exitErr(fmt.Errorf("db ping failed: %w", err))
	}

	for _, fp := range files {
		content, err := os.ReadFile(fp)
		if err != nil {
			exitErr(fmt.Errorf("read sql file failed (%s): %w", fp, err))
		}
		tx, err := db.Begin()
		if err != nil {
			exitErr(fmt.Errorf("begin tx failed (%s): %w", fp, err))
		}

		if _, err := tx.Exec(string(content)); err != nil {
			_ = tx.Rollback()
			exitErr(fmt.Errorf("exec sql failed (%s): %w", fp, err))
		}
		if err := tx.Commit(); err != nil {
			exitErr(fmt.Errorf("commit failed (%s): %w", fp, err))
		}
		fmt.Printf("OK: executed %s\n", fp)
	}

	for _, q := range queries {
		q = strings.TrimSpace(q)
		if q == "" {
			continue
		}
		if _, err := db.Exec(q); err != nil {
			exitErr(fmt.Errorf("exec query failed: %w", err))
		}
		fmt.Printf("OK: executed query\n")
	}

	verify(db)
}

func verify(db *sql.DB) {
	checks := []struct {
		label string
		query string
		want  string
	}{
		{
			label: "table",
			query: `SELECT COALESCE(to_regclass('public.ae_mcp_external_services_config_v2')::text, '')`,
			want:  "ae_mcp_external_services_config_v2",
		},
		{
			label: "sequence",
			query: `SELECT COALESCE(to_regclass('public.ae_mcp_external_services_config_v2_id_seq')::text, '')`,
			want:  "ae_mcp_external_services_config_v2_id_seq",
		},
		{
			label: "index",
			query: `SELECT COALESCE(to_regclass('public.uk_ae_mcp_external_services_config_v2_wemcp_name')::text, '')`,
			want:  "uk_ae_mcp_external_services_config_v2_wemcp_name",
		},
		{
			label: "task_node.server_id",
			query: `SELECT COALESCE((SELECT column_name FROM information_schema.columns WHERE table_schema='public' AND table_name='ae_mcp_task_node' AND column_name='server_id' LIMIT 1), '')`,
			want:  "server_id",
		},
		{
			label: "task_node.node_config",
			query: `SELECT COALESCE((SELECT column_name FROM information_schema.columns WHERE table_schema='public' AND table_name='ae_mcp_task_node' AND column_name='node_config' LIMIT 1), '')`,
			want:  "node_config",
		},
	}
	for _, c := range checks {
		var got string
		if err := db.QueryRow(c.query).Scan(&got); err != nil {
			exitErr(fmt.Errorf("verify %s failed: %w", c.label, err))
		}
		if got == "" || !strings.Contains(got, c.want) {
			exitErr(fmt.Errorf("verify %s failed: expected %s, got %s", c.label, c.want, got))
		}
		fmt.Printf("OK: %s exists (%s)\n", c.label, got)
	}
}

func exitErr(err error) {
	fmt.Fprintln(os.Stderr, err.Error())
	os.Exit(1)
}
