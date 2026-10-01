// Command flowc compiles flow Markdown into flow.json for the Flow Tracker
// canvas, and manages the central project store.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/madddtone/flowc/internal/compile"
	"github.com/madddtone/flowc/internal/schema"
	"github.com/madddtone/flowc/internal/state"
	"github.com/madddtone/flowc/internal/store"
	"github.com/madddtone/flowc/internal/tui"
	"github.com/madddtone/flowc/internal/validate"
)

const version = "0.2.0"

const pluginID = "io.github.madddtone.flow-tracker"

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		usage()
		os.Exit(2)
	}
	switch args[0] {
	case "new", "create":
		cmdNew(args[1:])
	case "list", "ls", "projects":
		cmdList()
	case "use", "switch":
		cmdUse(args[1:])
	case "rm", "remove":
		cmdRemove(args[1:])
	case "init":
		cmdInit(args[1:])
	case "guide":
		cmdGuide(args[1:])
	case "schema", "format":
		fmt.Print(schema.Schema)
	case "prompt":
		cmdPrompt(args[1:])
	case "agent":
		fmt.Print(schema.AgentPrompt)
	case "compile":
		cmdCompile(args[1:])
	case "check":
		cmdCheck(args[1:])
	case "open":
		cmdOpen(args[1:])
	case "watch":
		cmdWatch(args[1:])
	case "tui":
		cmdTUI(args[1:])
	case "version", "--version", "-v":
		fmt.Println("flowc " + version)
	case "help", "--help", "-h":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "flowc: unknown command %q\n\n", args[0])
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `flowc `+version+` - flow compiler + project store for Flow Tracker

Usage:
  flowc new "<name>" [--repo PATH] [--description TEXT]
  flowc list                       list projects
  flowc use <project>              set the active project
  flowc rm <project>               delete a project
  flowc prompt <project>           print AI/human authoring instructions
  flowc agent                      print a generic prompt for an AI agent
  flowc guide                      print the full authoring guide
  flowc open [project]             compile, set active, open the canvas
  flowc watch [project]            recompile on save
  flowc compile [project|flow.md]  compile to flow.json (-o PATH)
  flowc check [project|flow.md]    lint without writing
  flowc init [dir]                 scaffold a local flow.md (not centralised)
  flowc tui [project|flow.md]      interactive capture wizard
  flowc version

Projects live in `+store.ProjectsDir()+`
`)
}

// ---------------------------------------------------------------- projects

func cmdNew(args []string) {
	fs := flag.NewFlagSet("new", flag.ExitOnError)
	repo := fs.String("repo", "", "path to the documented repo")
	desc := fs.String("description", "", "short description")
	parseInterspersed(fs, args)
	name := strings.Join(fs.Args(), " ")
	if name == "" {
		fatal(fmt.Errorf("a project name is required, e.g. flowc new \"Checkout Service\""))
	}
	e, err := store.New(name, *repo, *desc)
	if err != nil {
		fatal(err)
	}
	res, cerr := compile.Write(e.FlowFile, e.JSONFile)
	if cerr == nil {
		_ = store.UpdateStats(e.ID, res.Doc.Meta.Stats.Nodes, res.Doc.Meta.Stats.Edges)
	}
	fmt.Printf("created project %q (%s)\n\n  %s\n\nnext:\n  flowc prompt %s   # instructions for you or an AI\n  flowc open %s\n", e.Name, e.ID, e.FlowFile, e.ID, e.ID)
}

func cmdList() {
	entries, err := store.List()
	if err != nil {
		fatal(err)
	}
	if len(entries) == 0 {
		fmt.Println("no projects yet — create one with: flowc new \"My Project\"")
		return
	}
	active, _ := state.ReadActive()
	fmt.Printf("%-24s %-28s %-9s %s\n", "ID", "NAME", "GRAPH", "UPDATED")
	for _, e := range entries {
		marker := " "
		if e.ID == active.ProjectID {
			marker = "*"
		}
		name := e.Name
		if len(name) > 27 {
			name = name[:26] + "…"
		}
		fmt.Printf("%s%-23s %-28s %-9s %s\n", marker, e.ID, name,
			fmt.Sprintf("%d/%d", e.Nodes, e.Edges), shortTime(e.UpdatedAt))
	}
	fmt.Println("\n(* = active)")
}

func cmdUse(args []string) {
	if len(args) == 0 {
		fatal(fmt.Errorf("usage: flowc use <project>"))
	}
	t, err := resolveTarget(args[0])
	if err != nil {
		fatal(err)
	}
	if !t.project {
		fatal(fmt.Errorf("use expects a project name or id"))
	}
	res, cerr := compile.Write(t.flowFile, t.jsonFile)
	if res != nil {
		printDiags(res)
		_ = store.UpdateStats(t.id, res.Doc.Meta.Stats.Nodes, res.Doc.Meta.Stats.Edges)
	} else if cerr != nil {
		fmt.Fprintln(os.Stderr, "flowc: compile:", cerr)
	}
	if err := writeActive(t, res); err != nil {
		fatal(err)
	}
	fmt.Printf("active project: %s (%s)\n", t.name, t.id)
}

func cmdRemove(args []string) {
	if len(args) == 0 {
		fatal(fmt.Errorf("usage: flowc rm <project>"))
	}
	e, err := store.Remove(args[0])
	if err != nil {
		fatal(err)
	}
	if active, _ := state.ReadActive(); active.ProjectID == e.ID {
		_ = state.WriteActive(state.Active{})
	}
	fmt.Printf("removed project %q\n", e.Name)
}

// ---------------------------------------------------------------- authoring

func cmdGuide(args []string) {
	fmt.Print(schema.Guide)
}

func cmdPrompt(args []string) {
	ref := ""
	if len(args) > 0 {
		ref = args[0]
	}
	t, err := resolveTarget(ref)
	if err != nil {
		fatal(err)
	}
	if !t.project {
		fatal(fmt.Errorf("prompt expects a project (create one with `flowc new`) or set an active project"))
	}
	fmt.Print(schema.Prompt(t.name, t.id, t.flowFile, t.repo))
}

func cmdInit(args []string) {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	force := fs.Bool("force", false, "overwrite an existing flow.md")
	parseInterspersed(fs, args)
	dir := "."
	if fs.NArg() > 0 {
		dir = fs.Arg(0)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fatal(err)
	}
	path := filepath.Join(dir, "flow.md")
	if _, err := os.Stat(path); err == nil && !*force {
		fatal(fmt.Errorf("%s already exists (use --force)", path))
	}
	if err := os.WriteFile(path, []byte(schema.Sample), 0o644); err != nil {
		fatal(err)
	}
	fmt.Printf("created %s (local; to centralise use `flowc new`)\n", path)
}

// ---------------------------------------------------------------- compile

func cmdCompile(args []string) {
	t, out := parseTarget("compile", args)
	res, err := compile.Write(t.flowFile, out)
	printDiags(res)
	if err != nil {
		fatal(err)
	}
	if t.project {
		_ = store.UpdateStats(t.id, res.Doc.Meta.Stats.Nodes, res.Doc.Meta.Stats.Edges)
	}
	fmt.Printf("compiled %s -> %s (%d nodes, %d edges)\n", t.flowFile, out, res.Doc.Meta.Stats.Nodes, res.Doc.Meta.Stats.Edges)
}

func cmdCheck(args []string) {
	t, _ := parseTarget("check", args)
	res, err := compile.Compile(t.flowFile)
	if err != nil {
		fatal(err)
	}
	printDiags(res)
	if res.Forbidden() {
		os.Exit(1)
	}
	fmt.Printf("ok: %d nodes, %d edges\n", res.Doc.Meta.Stats.Nodes, res.Doc.Meta.Stats.Edges)
}

func cmdTUI(args []string) {
	t, _ := parseTarget("tui", args)
	if err := tui.Run(t.flowFile); err != nil {
		fatal(err)
	}
}

// ---------------------------------------------------------------- open/watch

func cmdOpen(args []string) {
	fs := flag.NewFlagSet("open", flag.ExitOnError)
	noSummon := fs.Bool("no-summon", false, "do not summon the canvas")
	parseInterspersed(fs, args)
	ref := ""
	if fs.NArg() > 0 {
		ref = fs.Arg(0)
	}
	t, err := resolveTarget(ref)
	if err != nil {
		fatal(err)
	}
	res, cerr := compile.Write(t.flowFile, t.jsonFile)
	printDiags(res)
	if cerr != nil && res == nil {
		fatal(cerr)
	}
	if t.project {
		_ = store.UpdateStats(t.id, res.Doc.Meta.Stats.Nodes, res.Doc.Meta.Stats.Edges)
	}
	if err := writeActive(t, res); err != nil {
		fatal(err)
	}
	fmt.Printf("active flow: %s\n", t.flowFile)
	if !*noSummon {
		summon()
	}
}

// ---------------------------------------------------------------- targets

type target struct {
	project  bool
	id       string
	name     string
	repo     string
	flowFile string
	jsonFile string
}

func resolveTarget(ref string) (target, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		active, err := state.ReadActive()
		if err != nil || active.FlowFile == "" {
			return target{}, fmt.Errorf("no active project; run `flowc new` or `flowc use <project>`")
		}
		t := target{flowFile: active.FlowFile, jsonFile: active.JSONFile, id: active.ProjectID, name: active.Name}
		if active.Name == "" {
			t.name = filepath.Base(filepath.Dir(active.FlowFile))
		}
		if t.id != "" {
			t.project = true
			if e, ferr := store.Find(t.id); ferr == nil {
				t.repo = e.Repo
			}
		}
		return t, nil
	}
	if looksLikePath(ref) {
		p := abs(ref)
		return target{flowFile: p, jsonFile: defaultJSON(p), name: filepath.Base(p)}, nil
	}
	e, err := store.Find(ref)
	if err != nil {
		return target{}, err
	}
	return target{project: true, id: e.ID, name: e.Name, repo: e.Repo, flowFile: e.FlowFile, jsonFile: e.JSONFile}, nil
}

func parseTarget(name string, args []string) (target, string) {
	fs := flag.NewFlagSet(name, flag.ExitOnError)
	jsonPath := fs.String("json", "", "output json path")
	o := fs.String("o", "", "output json path")
	parseInterspersed(fs, args)
	ref := ""
	if fs.NArg() > 0 {
		ref = fs.Arg(0)
	}
	t, err := resolveTarget(ref)
	if err != nil {
		fatal(err)
	}
	if out := firstNonEmpty(*o, *jsonPath); out != "" {
		t.jsonFile = abs(out)
	}
	return t, t.jsonFile
}

func looksLikePath(s string) bool {
	return strings.HasSuffix(s, ".md") || strings.ContainsRune(s, '/')
}

func writeActive(t target, res *compile.Result) error {
	a := state.Active{
		ProjectID: t.id,
		Name:      t.name,
		FlowFile:  t.flowFile,
		JSONFile:  t.jsonFile,
		RepoRoot:  t.repo,
	}
	if res != nil && res.Forbidden() {
		for _, d := range res.Diags {
			if d.Severity == validate.Error {
				a.Error = d.Message
				break
			}
		}
	}
	return state.WriteActive(a)
}

func summon() {
	if _, err := exec.LookPath("omarchy-shell"); err != nil {
		fmt.Fprintln(os.Stderr, "flowc: omarchy-shell not found; canvas not summoned")
		return
	}
	cmd := exec.Command("omarchy-shell", "shell", "toggle", pluginID)
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "flowc: could not summon canvas: %v\n", err)
	}
}

func defaultJSON(in string) string {
	base := strings.TrimSuffix(in, filepath.Ext(in))
	if base == "" || base == in {
		return filepath.Join(filepath.Dir(in), "flow.json")
	}
	return base + ".json"
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func abs(p string) string {
	if a, err := filepath.Abs(p); err == nil {
		return a
	}
	return p
}

func shortTime(iso string) string {
	if len(iso) >= 16 {
		return strings.Replace(iso[:16], "T", " ", 1)
	}
	return iso
}

// ---------------------------------------------------------------- arg parsing

// parseInterspersed parses flags even when they appear after positional
// arguments (Go's flag package otherwise stops at the first positional).
func parseInterspersed(fs *flag.FlagSet, args []string) {
	var flags, pos []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if strings.HasPrefix(a, "-") && a != "-" && a != "--" {
			flags = append(flags, a)
			if !strings.Contains(a, "=") && flagTakesValue(fs, a) && i+1 < len(args) {
				flags = append(flags, args[i+1])
				i++
			}
			continue
		}
		pos = append(pos, a)
	}
	_ = fs.Parse(append(flags, pos...))
}

func flagTakesValue(fs *flag.FlagSet, name string) bool {
	f := fs.Lookup(strings.TrimLeft(name, "-"))
	if f == nil {
		return false
	}
	if bf, ok := f.Value.(interface{ IsBoolFlag() bool }); ok && bf.IsBoolFlag() {
		return false
	}
	return true
}

func printDiags(res *compile.Result) {
	if res == nil {
		return
	}
	for _, d := range res.Diags {
		fmt.Fprintln(os.Stderr, d.String())
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "flowc: "+err.Error())
	os.Exit(1)
}
