// SPDX-License-Identifier: GPL-3.0-or-later

package plan

import (
	"encoding/json"
	"hash/fnv"
	"strings"
)

// fingerprintEmoji are 64 widely supported single-codepoint emoji; six bits
// of the hash pick one.
var fingerprintEmoji = strings.Fields(
	"🦊 🐢 🐙 🦉 🐝 🦋 🐧 🦀 🐳 🦒 🦔 🐸 🦜 🐌 🦩 🐬 🐼 🐨 🦁 🐯 🐮 🐷 🐵 🐰 " +
		"🍉 🍋 🥝 🍄 🌵 🌻 🍀 🌈 🍩 🍕 🥨 🧀 🍒 🥥 🍇 🥑 " +
		"🚀 🛸 ⛵ 🎈 🎲 🎯 🧩 🧸 🪁 🎁 🔑 🔔 📌 🧲 🎺 🥁 🎸 🔮 🧭 ⏰ 💡 🔦 🧯 🪐")

// Fingerprint names the data version of a plan with four emoji: a hash over
// settings, roster (without notes) and the placed grid. Admin and viewer
// links of the same state share it; any change to roster, settings or
// placement changes it, so two people see at a glance whether they look at
// the same plan.
func (p *Plan) Fingerprint() string {
	ev := *p.Event
	ev.index = nil
	ev.People = append([]Person(nil), ev.People...)
	for i := range ev.People {
		ev.People[i].Note = ""
	}
	h := fnv.New64a()
	b, _ := json.Marshal(&ev)
	h.Write(b)
	for _, row := range p.Grid {
		for _, s := range row {
			if s != nil {
				h.Write([]byte(s.Label))
			}
			h.Write([]byte{0})
		}
	}
	sum := h.Sum64()
	var out strings.Builder
	for i := 0; i < 4; i++ {
		out.WriteString(fingerprintEmoji[(sum>>(6*i))&63])
	}
	return out.String()
}
