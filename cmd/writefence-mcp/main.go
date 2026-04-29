package main

import (
	"flag"

	"github.com/writefence/writefence/internal/config"
	"github.com/writefence/writefence/internal/mcp"
)

func main() {
	defaults := config.Defaults()
	violLog := flag.String("violations-log", defaults.Proxy.ViolationsLog, "path to violations JSONL log")
	flag.Parse()
	mcp.NewServer(*violLog).Run()
}
