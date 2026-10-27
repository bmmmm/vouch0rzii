// SPDX-License-Identifier: GPL-3.0-or-later

// vouch0rzii plans voucher chains for event presales: a few vouchers circulate
// in a group, each buys one ticket per day and is handed on in the evening.
// The browser build in cmd/vouch0rzii-wasm shares internal/plan with this CLI.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime/debug"

	"github.com/bmmmm/vouch0rzii/internal/plan"
)

var version = "dev"

// DefaultBase is where the page lives; -base overrides it.
const DefaultBase = "https://bmmmm.github.io/vouch0rzii/"

const usage = `vouch0rzii — voucher chain planner

Usage:
  vouch0rzii -event events/<name>.json [flags]

Flags:
  -event FILE     event file (JSON: settings + people)
  -roster FILE    roster paste (TSV/CSV) replacing the event's people
  -pins FILE      JSON array of pins, e.g. [{"label":"alpha","day":1,"voucher":2}]
  -format F       md (default), csv, json, svg, event (the normalised event file)
  -seed S         override the seed (a new draw)
  -share MODE     print a share link instead of the plan: admin, view, board, collect
  -base URL       page URL for -share (default ` + DefaultBase + `)
  -to EMAIL       contact address for -share collect
  -version        print the version

Exit status 2 means the input was rejected; the reasons are on stderr.
`

func resolvedVersion() string {
	if version != "dev" {
		return version
	}
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		return bi.Main.Version
	}
	return version
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("vouch0rzii", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { fmt.Fprint(stderr, usage) }
	var (
		eventPath  = fs.String("event", "", "")
		rosterPath = fs.String("roster", "", "")
		pinsPath   = fs.String("pins", "", "")
		format     = fs.String("format", "md", "")
		seed       = fs.String("seed", "", "")
		share      = fs.String("share", "", "")
		base       = fs.String("base", DefaultBase, "")
		to         = fs.String("to", "", "")
		showVer    = fs.Bool("version", false, "")
	)
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if *showVer {
		fmt.Fprintln(stdout, "vouch0rzii", resolvedVersion())
		return 0
	}
	if *eventPath == "" {
		fmt.Fprint(stderr, usage)
		return 2
	}
	reject := func(err error) int {
		fmt.Fprintln(stderr, err)
		return 2
	}
	e, err := plan.LoadEvent(*eventPath)
	if err != nil {
		return reject(err)
	}
	if *rosterPath != "" {
		b, err := os.ReadFile(*rosterPath)
		if err != nil {
			return reject(err)
		}
		people, warnings, err := plan.ParseRoster(string(b))
		for _, w := range warnings {
			fmt.Fprintln(stderr, "warning:", w)
		}
		if err != nil {
			return reject(err)
		}
		e.People = people
	}
	if *seed != "" {
		e.Seed = *seed
	}
	var pins []plan.Pin
	if *pinsPath != "" {
		b, err := os.ReadFile(*pinsPath)
		if err != nil {
			return reject(err)
		}
		if err := json.Unmarshal(b, &pins); err != nil {
			return reject(fmt.Errorf("pins: %w", err))
		}
	}
	p, err := plan.Compute(e, pins)
	if err != nil {
		return reject(err)
	}

	if *share != "" {
		st := &plan.State{Mode: *share, Event: e, Pins: pins}
		switch *share {
		case plan.ModeView, plan.ModeBoard:
			st.Pins = plan.Freeze(p)
		case plan.ModeCollect:
			if *to == "" {
				return reject(errors.New("-share collect needs -to EMAIL"))
			}
			st.Collect = &plan.Collect{To: *to}
		}
		enc, err := plan.EncodeState(st)
		if err != nil {
			return reject(err)
		}
		fmt.Fprintln(stdout, *base+"#s="+enc)
		return 0
	}

	var out []byte
	switch *format {
	case "md":
		out = []byte(p.Markdown())
	case "csv":
		out = []byte(p.CSV())
	case "json":
		out, err = p.JSON()
	case "svg":
		out = []byte(p.SVG())
	case "event":
		out, err = plan.MarshalEvent(e)
	default:
		return reject(fmt.Errorf("unknown format %q", *format))
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	stdout.Write(out)
	return 0
}
