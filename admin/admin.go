package main

import (
	"context"
	"flag"
	"fmt"

	"AgentEarth-Mgr/admin/internal/config"
	"AgentEarth-Mgr/admin/internal/handler"
	"AgentEarth-Mgr/admin/internal/middleware"
	"AgentEarth-Mgr/admin/internal/svc"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/rest"
)

var configFile = flag.String("f", "etc/admin-api.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)

	fmt.Printf("DB DataSource: %s\n", c.DB.DataSource)
	server := rest.MustNewServer(c.RestConf)
	defer server.Stop()

	server.Use(middleware.ErrorHandlerMiddleware)

	ctx := svc.NewServiceContext(c)
	{
		var dbInfo struct {
			Database string `db:"db"`
			Schema   string `db:"schema"`
			User     string `db:"user"`
			Addr     string `db:"addr"`
			Port     int    `db:"port"`
			DataDir  string `db:"data_dir"`
			Version  string `db:"version"`
		}
		err := ctx.DB.QueryRowCtx(context.Background(), &dbInfo, `
			select
				current_database() as db,
				current_schema() as schema,
				current_user as user,
				inet_server_addr()::text as addr,
				inet_server_port() as port,
				current_setting('data_directory') as data_dir,
				version() as version
		`)
		if err != nil {
			fmt.Printf("DB Info query failed: %v\n", err)
		} else {
			fmt.Printf("DB Info: db=%s schema=%s user=%s addr=%s port=%d data_dir=%s\n", dbInfo.Database, dbInfo.Schema, dbInfo.User, dbInfo.Addr, dbInfo.Port, dbInfo.DataDir)
			fmt.Printf("DB Version: %s\n", dbInfo.Version)
		}
	}
	{
		var tableCheck struct {
			Regclass *string `db:"regclass"`
		}
		err := ctx.DB.QueryRowCtx(context.Background(), &tableCheck, `
			select to_regclass('public.ae_mgrsystem_user') as regclass
		`)
		if err != nil {
			fmt.Printf("DB Table check failed: %v\n", err)
		} else if tableCheck.Regclass == nil {
			fmt.Printf("DB Table check: public.ae_mgrsystem_user = <nil>\n")
		} else {
			fmt.Printf("DB Table check: public.ae_mgrsystem_user = %s\n", *tableCheck.Regclass)
		}
	}
	{
		type relInfo struct {
			Schema string `db:"schema"`
			Name   string `db:"name"`
			Kind   string `db:"kind"`
		}
		var rels []relInfo
		err := ctx.DB.QueryRowsCtx(context.Background(), &rels, `
			select n.nspname as schema, c.relname as name, c.relkind as kind
			from pg_class c
			join pg_namespace n on n.oid = c.relnamespace
			where c.relname ilike '%mgrsystem%'
			order by n.nspname, c.relname
		`)
		if err != nil {
			fmt.Printf("DB Rel search failed: %v\n", err)
		} else if len(rels) == 0 {
			fmt.Printf("DB Rel search: no relname like %%mgrsystem%%\n")
		} else {
			fmt.Printf("DB Rel search: found %d\n", len(rels))
			for _, rel := range rels {
				fmt.Printf("  - %s.%s (%s)\n", rel.Schema, rel.Name, rel.Kind)
			}
		}
	}
	handler.RegisterHandlers(server, ctx)

	fmt.Printf("Starting server at %s:%d...\n", c.Host, c.Port)
	server.Start()
}
