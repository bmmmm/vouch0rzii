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

// Defaults applied when an Event leaves a field empty.
const (
	DefaultPlusMax  = 2
	DefaultHandover = "23:00"
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
	"member": TierMember, "mitglied": TierMember, "m": TierMember,
	"guest": TierGuest, "gast": TierGuest, "häufiger gast": TierGuest,
	"haeufiger gast": TierGuest, "(häufiger) gast": TierGuest, "g": TierGuest,
	"acquaintance": TierAcquaintance, "bekanntschaft": TierAcquaintance,
	"bekannt": TierAcquaintance, "bekannte": TierAcquaintance,
	"bekannter": TierAcquaintance, "a": TierAcquaintance, "b": TierAcquaintance,
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
// must not contain '+', which separates the handle from the +k suffix in slot
// labels.
type Person struct {
	ID    string   `json:"id"`
	Email string   `json:"email,omitempty"`
	Tier  Tier     `json:"tier"`
	Card  bool     `json:"card,omitempty"`  // credit card available: shown only, never scheduled on
	Plus  int      `json:"plus,omitempty"`  // additional people brought along (+N)
	Group bool     `json:"group,omitempty"` // represents a larger group: Plus may exceed PlusMax, own +k may follow each other
	Order int      `json:"order,omitempty"` // manual position within the tier; 0 = drawn by lot
	Knows []string `json:"knows,omitempty"` // IDs of people this person knows; made symmetric on load
	Note  string   `json:"note,omitempty"`
}

// Event is the per-event configuration plus the roster. It is the canonical
// file format (events/<name>.json).
type Event struct {
	Name     string   `json:"name"`
	Vouchers int      `json:"vouchers"`
	Start    string   `json:"start"`              // date of day 1, YYYY-MM-DD
	Weekends bool     `json:"weekends,omitempty"` // plan on Saturdays and Sundays too
	PlusMax  int      `json:"plusMax,omitempty"`  // 0 = DefaultPlusMax
	Seed     string   `json:"seed,omitempty"`     // "" = Name; a different seed is a new draw
	Handover string   `json:"handover,omitempty"` // display only, "" = DefaultHandover
	People   []Person `json:"people"`
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

// HandoverOrDefault returns the displayed handover time.
func (e *Event) HandoverOrDefault() string {
	if e.Handover == "" {
		return DefaultHandover
	}
	return e.Handover
}

// Person returns the roster entry with the given ID, or nil.
func (e *Event) Person(id string) *Person {
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
	rank     int // position in the priority list
}

// SlotLabel builds the label of the k-th slot of a person: "alpha", "alpha+1".
func SlotLabel(id string, k int) string {
	if k == 0 {
		return id
	}
	return fmt.Sprintf("%s+%d", id, k)
}

// ParseSlotLabel splits "alpha+2" into ("alpha", 2) and "alpha" into
// ("alpha", 0). The suffix must be a positive integer.
func ParseSlotLabel(label string) (id string, k int, err error) {
	label = strings.TrimSpace(label)
	i := strings.LastIndexByte(label, '+')
	if i < 0 {
		return label, 0, nil
	}
	if _, err := fmt.Sscanf(label[i+1:], "%d", &k); err != nil || k <= 0 || fmt.Sprint(k) != label[i+1:] {
		return "", 0, fmt.Errorf("bad slot label %q", label)
	}
	return label[:i], k, nil
}

// Handover is one evening transfer of a voucher from the holder of day d to
// the holder of day d+1 in the same chain.
type Handover struct {
	Day       int // the evening of this day
	Voucher   int
	From, To  *Slot
	Known     bool // the two know each other
	Forbidden bool // sponsor→own +k without group flag; only used when unavoidable
}

// Stats summarises a plan.
type Stats struct {
	Days      int `json:"days"`
	Slots     int `json:"slots"`
	Handovers int `json:"handovers"`
	Known     int `json:"known"`
	Unknown   int `json:"unknown"`
	Forbidden int `json:"forbidden"`
	Pinned    int `json:"pinned"`
}

// Plan is the computed result. Grid[d-1][v-1] is the slot of voucher v on day
// d, or nil when the chain has ended.
type Plan struct {
	Event     *Event
	Dates     []string // YYYY-MM-DD per day
	Slots     []*Slot  // in priority order
	Grid      [][]*Slot
	Handovers []Handover
	Warnings  []string
	Stats     Stats
}
