package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/madddtone/flowc/internal/compile"
	"github.com/madddtone/flowc/internal/store"
)

func cmdWatch(args []string) {
	fs := flag.NewFlagSet("watch", flag.ExitOnError)
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

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		fatal(err)
	}
	defer watcher.Close()

	watched := map[string]bool{}
	addWatch := func(dir string) {
		if dir == "" || watched[dir] {
			return
		}
		if err := watcher.Add(dir); err == nil {
			watched[dir] = true
		}
	}
	addWatch(filepath.Dir(t.flowFile))

	compileOnce := func() {
		res, cerr := compile.Write(t.flowFile, t.jsonFile)
		printDiags(res)
		if cerr != nil && res == nil {
			fmt.Fprintf(os.Stderr, "flowc: compile failed: %v\n", cerr)
			_ = writeActive(t, res)
			return
		}
		if t.project && res != nil {
			_ = store.UpdateStats(t.id, res.Doc.Meta.Stats.Nodes, res.Doc.Meta.Stats.Edges)
		}
		_ = writeActive(t, res)
		if res != nil {
			fmt.Printf("[%s] compiled %s (%d nodes, %d edges)\n",
				time.Now().Format("15:04:05"), filepath.Base(t.flowFile),
				res.Doc.Meta.Stats.Nodes, res.Doc.Meta.Stats.Edges)
			for _, n := range res.Doc.Nodes {
				if n.Subflow == "" {
					continue
				}
				r := n.Subflow
				if !filepath.IsAbs(r) {
					r = filepath.Join(filepath.Dir(t.flowFile), r)
				}
				addWatch(filepath.Dir(r))
			}
		}
	}

	compileOnce()
	fmt.Printf("watching %s (ctrl-c to stop)\n", t.flowFile)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	var timer *time.Timer
	trigger := func() {
		if timer != nil {
			timer.Stop()
		}
		timer = time.AfterFunc(150*time.Millisecond, compileOnce)
	}

	for {
		select {
		case <-sigs:
			fmt.Println("\nstopped")
			return
		case ev, ok := <-watcher.Events:
			if !ok {
				return
			}
			if strings.HasSuffix(ev.Name, ".md") {
				if ev.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename|fsnotify.Remove) != 0 {
					trigger()
				}
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			fmt.Fprintf(os.Stderr, "flowc: watch error: %v\n", err)
		}
	}
}
