// SPDX-License-Identifier: GPL-3.0-or-later

// Package plan computes voucher chain plans for event presales: a fixed number
// of vouchers circulates in a group, each voucher buys one ticket per day and is
// handed to the next person in the evening. The package decides who holds
// which voucher on which day and who hands it to whom.
package plan

import (
	"fmt"
	"strings"
)

// Defaults and bounds. The bounds keep the cubic assignment, the grid and the
// share link small enough for a browser tab.
const (
	DefaultPlusMax  = 2
	DefaultHandover = "23:00"
	MaxVouchers     = 200
	MaxPeople       = 2000
	MaxPlus         = 200
	MaxDays         = 400
	MaxEvery        = 14
)

// Tier is the membership status of a named person. Lower tiers are served
// first.
type Tier int

const (
	TierMember Tier = iota
	TierGuest
	TierAcquaintance
)

var tierNames = [...]string{"member", "guest", "acquaintance"}

// tierAliases maps lower-cased spellings (German and English) to a tier.
var tierAliases = map[string]Tier{
	"member": TierMember, "mitglied": TierMember,
	"guest": TierGuest, "gast": TierGuest, "häufiger gast": TierGuest,
	"acquaintance": TierAcquaintance, "bekanntschaft": TierAcquaintance,
}

// String returns the canonical English name.
func (t Tier) String() string {
	if t < 0 || int(t) >= len(tierNames) {
		return fmt.Sprintf("tier(%d)", int(t))
	}
	return tierNames[t]
}

// ParseTier accepts German and English spellings, case-insensitive.
func ParseTier(s string) (Tier, bool) {
	t, ok := tierAliases[strings.ToLower(strings.TrimSpace(s))]
	return t, ok
}

// MarshalText makes tiers appear as strings in JSON.
func (t Tier) MarshalText() ([]byte, error) { return []byte(t.String()), nil }

// UnmarshalText accepts any alias ParseTier knows.
func (t *Tier) UnmarshalText(b []byte) error {
	v, ok := ParseTier(string(b))
	if !ok {
		return fmt.Errorf("unknown tier %q", string(b))
	}
	*t = v
	return nil
}

// Person is one named participant. IDs are handles and must be unique; they
// must not contain '+' (slot label suffix), ';' and ',' (roster delimiters)
// or '|' (Markdown tables).
type Person struct {
	ID    string `json:"id"`
	Email string `json:"email,omitempty"`
	Tier  Tier   `json:"tier"`
	Plus  int    `json:"plus,omitempty"`  // additional people brought along (+N)
	Group bool   `json:"group,omitempty"` // represents a larger group: Plus may exceed PlusMax, the group keeps its voucher
	Order int    `json:"order,omitempty"` // manual position within the tier; 0 = drawn by lot
	Out   bool   `json:"out,omitempty"`   // left the plan: still drawn (nobody else moves), but only pinned slots remain
	Note  string `json:"note,omitempty"`  // free text from the sign-up form; never in viewer links
}

// Extra adds vouchers that become available later: Count more chains from
// the first plan day on or after Date.
type Extra struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
}

// Event is the per-event configuration plus the roster. It is the canonical
// file format (events/<name>.json).
type Event struct {
	Name         string   `json:"name"`
	Vouchers     int      `json:"vouchers"`               // chains available from day 1
	Extra        []Extra  `json:"extra,omitempty"`        // chains that start later
	Start        string   `json:"start"`                  // date of day 1, YYYY-MM-DD
	End          string   `json:"end,omitempty"`          // last day of the voucher phase, YYYY-MM-DD; "" = until everyone is placed
	Weekends     bool     `json:"weekends,omitempty"`     // plan on Saturdays and Sundays too
	OneChain     bool     `json:"oneChain,omitempty"`     // a group keeps one voucher for all its slots, even if the plan grows; false = as few vouchers as the shortest plan allows
	MembersFirst bool     `json:"membersFirst,omitempty"` // groups start their chains after the last day of a member's own slot, so no member waits for a group; false = from the group's first day
	Every        int      `json:"every,omitempty"`        // plan days from one buying day of a chain to the next; 0 = 1
	PlusMax      int      `json:"plusMax,omitempty"`      // 0 = DefaultPlusMax
	Seed         string   `json:"seed,omitempty"`         // "" = Name; a different seed is a new draw
	Handover     string   `json:"handover,omitempty"`     // display only, "" = DefaultHandover
	People       []Person `json:"people"`

	index map[string]*Person // built by Normalize; nil on copies
}

// PlusMaxOrDefault returns the +N limit for people without the group flag.
func (e *Event) PlusMaxOrDefault() int {
	if e.PlusMax <= 0 {
		return DefaultPlusMax
	}
	return e.PlusMax
}

// SeedOrDefault returns the seed string of the draw.
func (e *Event) SeedOrDefault() string {
	if e.Seed == "" {
		return e.Name
	}
	return e.Seed
}

// EveryOrDefault returns the number of plan days between two buying days.
func (e *Event) EveryOrDefault() int {
	if e.Every <= 0 {
		return 1
	}
	return e.Every
}

// HandoverOrDefault returns the displayed handover time.
func (e *Event) HandoverOrDefault() string {
	if e.Handover == "" {
		return DefaultHandover
	}
	return e.Handover
}

// Person returns the roster entry with the given ID, or nil.
func (e *Event) Person(id string) *Person {
	if e.index != nil {
		return e.index[id]
	}
	for i := range e.People {
		if e.People[i].ID == id {
			return &e.People[i]
		}
	}
	return nil
}

// Pin fixes a slot to a cell of the plan: day and voucher are 1-based.
type Pin struct {
	Label   string `json:"label"`
	Day     int    `json:"day"`
	Voucher int    `json:"voucher"`
}

// Slot is one ticket opportunity: a named person (K == 0) or the k-th extra
// person brought by PersonID (K == k).
type Slot struct {
	Label    string
	PersonID string
	K        int
	Day      int // 1-based, 0 until placed
	Voucher  int // 1-based, 0 until placed
	Pinned   bool
}

// SlotLabel builds the label of the k-th slot of a person: "alpha", "alpha+1".
func SlotLabel(id string, k int) string {
	if k == 0 {
		return id
	}
	return fmt.Sprintf("%s+%d", id, k)
}

// Handover is one evening transfer of a voucher: once From has bought on Day,
// the new voucher is expected on the evening Every-1 plan days later and goes to
// To, who holds it next in the same chain — on the next buying day, or later
// if the voucher idles in between.
type Handover struct {
	Day       int
	Voucher   int
	From, To  *Slot
	Kept      bool // the voucher stays with a group: from one of its slots to the next
	Forbidden bool // sponsor→own +k without group flag; only used when unavoidable
}

// Stats summarises a plan.
type Stats struct {
	Days      int `json:"days"`
	Slots     int `json:"slots"`
	Handovers int `json:"handovers"`
	Kept      int `json:"kept"` // handovers within a group
	Forbidden int `json:"forbidden"`
	Pinned    int `json:"pinned"`
	Free      int `json:"free"` // voucher days nobody is placed on: until End, or within the grid
}

// Plan is the computed result. Grid[d-1][v-1] is the slot of voucher v on day
// d, or nil when the voucher is not yet available, idles, or its chain has
// ended. From[v-1] is the first day voucher v exists. Slots with Day 0 did
// not fit before the end of the voucher phase.
type Plan struct {
	Event     *Event
	PhaseDays int      // plan days up to and including Event.End; 0 without an end
	From      []int    // start day per voucher, 1-based; len(From) is the total voucher count
	Dates     []string // YYYY-MM-DD per day
	Slots     []*Slot  // in priority order
	Grid      [][]*Slot
	Handovers []Handover // sorted by day, then voucher
	Warnings  []string
	Stats     Stats
}
