// SPDX-License-Identifier: GPL-3.0-or-later

//go:build js && wasm

// vouch0rzii-wasm is the browser build. It exposes internal/plan to the page
// as globalThis.vouch0rzii (plan, parseRoster, parseSignup, formatRoster,
// encodeState, decodeState, version) and stays resident; web/ holds the page and
// scripts/build-web.sh assembles both into web/dist. Every function returns a
// plain object with ok, errors[] and warnings[], never throws, and passes
// structured data as JSON strings so that the page and the CLI see byte-equal
// output (scripts/web-smoke.mjs proves it).
package main

import (
	"encoding/json"
	"fmt"
	"maps"
	"runtime/debug"
	"strings"
	"syscall/js"

	"github.com/bmmmm/vouch0rzii/internal/plan"
)

var version = "dev"

// guard turns a panic inside f into an ok:false result, so one bad input
// cannot end the resident Go program and break every later call.
func guard(f func(js.Value, []js.Value) any) js.Func {
	return js.FuncOf(func(this js.Value, args []js.Value) (res any) {
		defer func() {
			if r := recover(); r != nil {
				res = fail(fmt.Errorf("internal error: %v", r), nil)
			}
		}()
		return f(this, args)
	})
}

func strArg(args []js.Value, i int) string {
	if i < len(args) && args[i].Type() == js.TypeString {
		return args[i].String()
	}
	return ""
}

func toAny(ss []string) []any {
	out := make([]any, len(ss))
	for i, s := range ss {
		out[i] = s
	}
	return out
}

func fail(err error, warnings []string) any {
	return js.ValueOf(map[string]any{
		"ok":       false,
		"errors":   toAny(strings.Split(strings.TrimSpace(err.Error()), "\n")),
		"warnings": toAny(warnings),
	})
}

func ok(warnings []string, fields map[string]any) any {
	res := map[string]any{"ok": true, "errors": []any{}, "warnings": toAny(warnings)}
	maps.Copy(res, fields)
	return js.ValueOf(res)
}

// planFn(eventJSON, pinsJSON) → {ok, errors, warnings, exportJson, markdown, csv, svg}
func planFn(_ js.Value, args []js.Value) any {
	e, err := plan.ParseEvent([]byte(strArg(args, 0)))
	if err != nil {
		return fail(err, nil)
	}
	var pins []plan.Pin
	if s := strings.TrimSpace(strArg(args, 1)); s != "" {
		if err := json.Unmarshal([]byte(s), &pins); err != nil {
			return fail(err, nil)
		}
	}
	p, err := plan.Compute(e, pins)
	if err != nil {
		return fail(err, nil)
	}
	out, err := p.JSON()
	if err != nil {
		return fail(err, nil)
	}
	return ok(p.Warnings, map[string]any{
		"exportJson": string(out),
		"markdown":   p.Markdown(),
		"csv":        p.CSV(),
		"svg":        p.SVG(),
	})
}

// parseRoster(tsv) → {ok, errors, warnings, peopleJson}
func parseRoster(_ js.Value, args []js.Value) any {
	people, warnings, err := plan.ParseRoster(strArg(args, 0))
	if err != nil {
		return fail(err, warnings)
	}
	b, err := json.Marshal(people)
	if err != nil {
		return fail(err, warnings)
	}
	return ok(warnings, map[string]any{"peopleJson": string(b)})
}

// parseSignup(text) → {ok, errors, warnings, peopleJson}: sign-up lines, by
// position or by content (plan.ParseSignup).
func parseSignup(_ js.Value, args []js.Value) any {
	people, warnings, err := plan.ParseSignup(strArg(args, 0))
	if err != nil {
		return fail(err, warnings)
	}
	b, err := json.Marshal(people)
	if err != nil {
		return fail(err, warnings)
	}
	return ok(warnings, map[string]any{"peopleJson": string(b)})
}

// formatRoster(peopleJson) → {ok, errors, warnings, tsv}
func formatRoster(_ js.Value, args []js.Value) any {
	var people []plan.Person
	if err := json.Unmarshal([]byte(strArg(args, 0)), &people); err != nil {
		return fail(err, nil)
	}
	return ok(nil, map[string]any{"tsv": plan.FormatRoster(people)})
}

// encodeState(stateJson) → {ok, errors, warnings, encoded}
func encodeState(_ js.Value, args []js.Value) any {
	var st plan.State
	if err := json.Unmarshal([]byte(strArg(args, 0)), &st); err != nil {
		return fail(err, nil)
	}
	enc, err := plan.EncodeState(&st)
	if err != nil {
		return fail(err, nil)
	}
	return ok(nil, map[string]any{"encoded": enc})
}

// decodeState(encoded) → {ok, errors, warnings, stateJson}
func decodeState(_ js.Value, args []js.Value) any {
	st, err := plan.DecodeState(strArg(args, 0))
	if err != nil {
		return fail(err, nil)
	}
	b, err := json.Marshal(st)
	if err != nil {
		return fail(err, nil)
	}
	return ok(nil, map[string]any{"stateJson": string(b)})
}

func versionString() string {
	if version != "dev" {
		return version
	}
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		return bi.Main.Version
	}
	return version
}

func main() {
	js.Global().Set("vouch0rzii", js.ValueOf(map[string]any{
		"plan":         guard(planFn),
		"parseRoster":  guard(parseRoster),
		"parseSignup":  guard(parseSignup),
		"formatRoster": guard(formatRoster),
		"encodeState":  guard(encodeState),
		"decodeState":  guard(decodeState),
		"version":      versionString(),
	}))
	select {}
}
