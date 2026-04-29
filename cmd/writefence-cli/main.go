package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/writefence/writefence/internal/config"
	"github.com/writefence/writefence/internal/quarantine"
	"github.com/writefence/writefence/internal/replay"
)

var defaults = config.Defaults()

const usageText = "Usage: writefence-cli <command>"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	switch os.Args[1] {
	case "status":
		cmdStatus()
	case "rules":
		if len(os.Args) > 2 && os.Args[2] == "list" {
			cmdRulesList()
		} else {
			usage()
		}
	case "violations":
		if len(os.Args) > 2 {
			switch os.Args[2] {
			case "tail":
				cmdViolationsTail(50)
			case "report":
				cmdViolationsReport()
			default:
				usage()
			}
		} else {
			usage()
		}
	case "quarantine":
		if len(os.Args) > 2 {
			switch os.Args[2] {
			case "list":
				cmdQuarantineList()
			case "approve":
				if len(os.Args) < 4 {
					usage()
					os.Exit(1)
				}
				cmdQuarantineApprove(os.Args[3])
			case "reject":
				if len(os.Args) < 4 {
					usage()
					os.Exit(1)
				}
				cmdQuarantineReject(os.Args[3])
			default:
				usage()
			}
		} else {
			usage()
		}
	case "replay":
		cmdReplay(os.Args[2:])
	default:
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Println(usageText)
	fmt.Println("  status                  Show session state")
	fmt.Println("  rules list              List active enforcement rules")
	fmt.Println("  violations tail         Show last 50 violations")
	fmt.Println("  violations report       Summary: violations by rule")
	fmt.Println("  quarantine list         Show quarantined writes")
	fmt.Println("  quarantine approve ID   Approve and forward a quarantined write")
	fmt.Println("  quarantine reject ID    Reject a quarantined write")
	fmt.Println("  replay [--wal path] [--config path]  Re-evaluate WAL against current policy")
}

func cmdStatus() {
	b, err := os.ReadFile(defaults.Proxy.StateFile)
	if err != nil {
		fmt.Println("State file not found.")
		return
	}
	var s map[string]interface{}
	json.Unmarshal(b, &s)
	fmt.Println("=== WriteFence Session State ===")
	for k, v := range s {
		fmt.Printf("  %-25s %v\n", k, v)
	}
}

func cmdRulesList() {
	rules := []struct{ name, desc string }{
		{"english_only", "Blocks documents with >5% Cyrillic characters."},
		{"prefix_required", "Blocks documents not starting with [STATUS],[DECISION],[SETUP],[CONFIG],[RUNBOOK]."},
		{"context_shield", "Blocks [DECISION] writes unless decisions were queried this session."},
		{"status_dedup", "Auto-deletes previous [STATUS] docs before writing new one."},
		{"semantic_dedup", "Blocks near-duplicate writes (embedding similarity >= 0.98)."},
	}
	fmt.Println("=== Active Rules ===")
	for _, r := range rules {
		fmt.Printf("  %-20s %s\n", r.name, r.desc)
	}
}

func readViolationEntries() []map[string]interface{} {
	f, err := os.Open(defaults.Proxy.ViolationsLog)
	if err != nil {
		return nil
	}
	defer f.Close()
	var entries []map[string]interface{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var e map[string]interface{}
		if json.Unmarshal(scanner.Bytes(), &e) == nil {
			entries = append(entries, e)
		}
	}
	return entries
}

func cmdViolationsTail(n int) {
	entries := readViolationEntries()
	if len(entries) == 0 {
		fmt.Println("No violations recorded.")
		return
	}
	start := 0
	if len(entries) > n {
		start = len(entries) - n
	}
	fmt.Printf("=== Last %d Violations ===\n", n)
	for _, e := range entries[start:] {
		rule := fmt.Sprintf("%v", e["rule"])
		ts := fmt.Sprintf("%v", e["ts"])
		reason := fmt.Sprintf("%v", e["reason"])
		if strings.Contains(rule, "english") {
			fmt.Printf("  * %s  %-18s  %s\n", ts, rule, reason)
		} else {
			fmt.Printf("    %s  %-18s  %s\n", ts, rule, reason)
		}
	}
}

func quarantineStore() *quarantine.Store {
	return quarantine.New(defaults.Proxy.QuarantineLog, defaults.Proxy.Upstream)
}

func cmdQuarantineList() {
	entries, err := quarantineStore().List()
	if err != nil {
		fmt.Printf("Failed to read quarantine log: %v\n", err)
		return
	}
	if len(entries) == 0 {
		fmt.Println("No quarantined writes recorded.")
		return
	}
	fmt.Println("=== Quarantine Entries ===")
	for _, entry := range entries {
		fmt.Printf("  %s  %-9s  %-16s  %s\n", entry.TraceID, entry.Status, entry.Rule, previewText(entry.Doc.Text, 60))
	}
}

func cmdQuarantineApprove(traceID string) {
	if err := quarantineStore().Approve(traceID); err != nil {
		fmt.Printf("Approve failed: %v\n", err)
		return
	}
	fmt.Printf("Approved %s\n", traceID)
}

func cmdQuarantineReject(traceID string) {
	if err := quarantineStore().Reject(traceID); err != nil {
		fmt.Printf("Reject failed: %v\n", err)
		return
	}
	fmt.Printf("Rejected %s\n", traceID)
}

func previewText(s string, limit int) string {
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	return string(runes[:limit]) + "…"
}

func cmdViolationsReport() {
	entries := readViolationEntries()
	if len(entries) == 0 {
		fmt.Println("No violations recorded.")
		return
	}
	counts := map[string]int{}
	for _, e := range entries {
		counts[fmt.Sprintf("%v", e["rule"])]++
	}
	type kv struct {
		k string
		v int
	}
	var sorted []kv
	for k, v := range counts {
		sorted = append(sorted, kv{k, v})
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].v > sorted[j].v })
	fmt.Printf("=== Violation Report (%d total) ===\n", len(entries))
	for _, item := range sorted {
		fmt.Printf("  %-20s %d\n", item.k, item.v)
	}
}

func cmdReplay(args []string) {
	fs := flag.NewFlagSet("replay", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	walPath := fs.String("wal", defaults.Proxy.WALLog, "path to WAL JSONL")
	configPath := fs.String("config", "", "path to YAML config file")
	if err := fs.Parse(args); err != nil {
		return
	}

	cfg := defaults
	if *configPath != "" {
		loaded, err := config.Load(*configPath)
		if err != nil {
			fmt.Printf("Failed to load config: %v\n", err)
			return
		}
		cfg = loaded
	}

	engine := replay.New(cfg)
	results, err := engine.Run(*walPath)
	if err != nil {
		fmt.Printf("Replay failed: %v\n", err)
		return
	}
	if len(results) == 0 {
		fmt.Println("No replayable WAL entries found.")
		return
	}

	fmt.Printf("=== Replay Report (%d entries) ===\n", len(results))
	for _, result := range results {
		marker := " "
		if result.Changed {
			marker = "!"
		}
		ruleInfo := result.Rule
		if result.ReasonCode != "" {
			if ruleInfo != "" {
				ruleInfo += "/"
			}
			ruleInfo += result.ReasonCode
		}
		if ruleInfo == "" {
			ruleInfo = "-"
		}
		fmt.Printf("  %s %-12s -> %-12s  %-32s %s\n", marker, result.OrigDecision, result.NewDecision, ruleInfo, result.TextPreview)
	}
}
