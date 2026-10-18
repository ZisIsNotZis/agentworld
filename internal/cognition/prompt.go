package cognition

import (
	"fmt"
	"strings"

	"agentworld/internal/cognition/protocol"
	"agentworld/internal/sim"
)

// The natural-claim/v4 system prompt (ticket 36, deliverable B).
//
// The v1-v3 contracts send the canonical request as the whole user message
// under a fixed gateway system line, so the model receives a bare data dump: no
// task, no goal, no world glossary, and action names ("gather", "provision")
// that do not describe what the world does with them. This file owns the v4
// system message and the persona it carries.
//
// The prompt carries, in order: the situation (three sentences); who the actor
// is; the goal in the actor's own words; and only the four facts that renaming
// a request field could not carry (a slot, the granary, the one-granary-draw-
// per-hour rule, and the bag's single unit plus energy as the death channel).
// Everything else the actor reads is in the request, named for what it is, so
// no field is restated here as a glossary entry.
//
// The prompt is a pure function of the declared persona card and nothing else,
// and the card is a pure function of the actor's own identity rows: the entity
// id, the elapsed hours since birth, and the three inherited alleles. It never
// reads a death hour, a parent, the world seed, another actor's row or any
// world-private state, so the same actor renders the same prompt on every replay
// and the card cannot state a fact the actor could not know.

// PersonaCard is the declared persona of one actor: the "who you are" block of
// the v4 prompt. AgeBand is a band of elapsed hours rather than a fabricated age
// in years, because the world's time unit is the hour and the fixture defines no
// year.
type PersonaCard struct {
	Name        string
	AgeBand     string
	Temperament string
	Wants       string
	Fears       string
}

// The declared persona tables. They are narrative, not measurements: no entry
// states a number, a rule or a fact about the actor's body, so nothing in the
// card can be mistaken for state.
var (
	personaNames = [...]string{
		"Tamsin", "Odo", "Briar", "Hale", "Wren", "Corin", "Maud", "Silas",
		"Enna", "Tobiah", "Rill", "Perrin", "Odile", "Bram", "Nessa", "Gareth",
	}
	personaTemperaments = [...]string{
		"You are steady and unhurried, even in a bad hour.",
		"You are quick to act and slow to forgive a slight.",
		"You are careful with everything you own and loath to spend it.",
		"You are generous with what you have, and it has cost you before.",
		"You are watchful and say little, and you notice what others let slip.",
		"You are stubborn: once you have decided, that hour is spent that way.",
	}
	personaWants = [...]string{
		"enough put by that one bad week cannot touch you",
		"to see your store grow, and to be known for it",
		"something built in your own place that outlasts you",
		"a child of your own, grown and standing",
		"to never again depend on another person's goodwill",
		"to finish what you start, however long it takes",
	}
	personaFears = [...]string{
		"an empty granary in a cold hour",
		"being the one with nothing while others have food",
		"dying with your work half-built",
		"leaving nobody behind you",
		"being refused help you had counted on",
		"having your store taken while you sleep",
	}
)

// Declared age-band edges in elapsed hours. The world's horizon is 168 hours and
// a founder's lifetime is a few hundred, so the bands are coarse on purpose.
const (
	personaAgeNewcomerHours   = 24
	personaAgePrimeHours      = 96
	personaAgeNewcomerBand    = "new to this place"
	personaAgePrimeBand       = "in your prime"
	personaAgeLongLivedBand   = "long-lived here"
	personaCardUnsetBirthHour = -1
)

// PersonaFor derives one actor's persona card. It is deterministic in (actor,
// portrait) alone and its only slow input is the elapsed-hour age band, so the
// actor is the same person in every hour of a run.
func PersonaFor(actor sim.EntityID, portrait protocol.ActorPortrait, hour int) PersonaCard {
	base := personaHash(uint64(actor))
	alleles := uint64(portrait.LocusM*7+portrait.LocusY*11+portrait.LocusW*13) % 97
	return PersonaCard{
		Name:        personaNames[base%uint64(len(personaNames))],
		AgeBand:     personaAgeBand(portrait.BirthHour, hour),
		Temperament: personaTemperaments[(base+alleles)%uint64(len(personaTemperaments))],
		Wants:       personaWants[alleles%uint64(len(personaWants))],
		Fears:       personaFears[(alleles+base/7)%uint64(len(personaFears))],
	}
}

// personaAgeBand maps the actor's elapsed hours onto a declared band. A birth
// hour the world never set (the zero portrait of a contract that does not carry
// one) reads as the newest band rather than as a fabricated age.
func personaAgeBand(birthHour, hour int) string {
	if birthHour < 0 {
		birthHour = personaCardUnsetBirthHour
	}
	elapsed := hour - birthHour
	if elapsed < personaAgeNewcomerHours {
		return personaAgeNewcomerBand
	}
	if elapsed < personaAgePrimeHours {
		return personaAgePrimeBand
	}
	return personaAgeLongLivedBand
}

// personaHash is the FNV-1a hash of the actor's entity id: the declared
// selection function of the persona tables. It is written here rather than taken
// from hash/fnv so its value is pinned by a test rather than by a library
// version.
func personaHash(v uint64) uint64 {
	const (
		offset = 14695981039346656037
		prime  = 1099511628211
	)
	h := uint64(offset)
	for i := 0; i < 8; i++ {
		h ^= (v >> (8 * i)) & 0xff
		h *= prime
	}
	return h
}

// NaturalClaimSystemPromptV4 renders the declared v4 system prompt for one
// persona card. The text is fixed apart from the card, so two actors differ only
// in the persona they are given.
func NaturalClaimSystemPromptV4(card PersonaCard) string {
	var b strings.Builder
	b.WriteString("You are one person in a small settlement, and you live or die by your own store of food. " +
		"The world moves one hour at a time and asks you, once an hour, what you will do with that hour. " +
		"Whatever you answer is what you attempt; if it is not possible for you, the world quietly does what you would have done anyway.\n\n")
	fmt.Fprintf(&b, "You are %s, %s. %s What you want most is %s. What you fear is %s.\n\n",
		card.Name, card.AgeBand, card.Temperament, card.Wants, card.Fears)
	b.WriteString("Your life, in your own words: stay alive, keep food in your granary, build up your capital, and raise a child if you can.\n\n")
	b.WriteString("Four things the numbers cannot tell you:\n" +
		"- A slot is a strip of ground beside your home. Food appears in your slots on its own, hour after hour, whether or not you act; each slot holds one unit, and a unit waiting there is yours to pick up.\n" +
		"- The granary is the store your capital fills, one unit an hour for each point you hold. Standing food there slowly rots, but it is the only store a child can be fed from.\n" +
		"- You can take at most one unit from the granary in one hour, and both routes use up that hour's one draw: eating it on the spot, or putting it in your bag.\n" +
		"- Your bag holds one food unit and no more; a meal or a piece of capital is made from exactly that unit. Energy is what keeps you alive: it drains as the hours pass, a meal restores one unit of it, and at zero you die.\n\n")
	b.WriteString("Answer with exactly one JSON object and nothing else:\n" +
		"{\"schema_version\": 4, \"action\": \"<one action, spelled exactly as the request lists it>\", \"why\": \"<one sentence on why this action, this hour>\"}\n")
	return b.String()
}

// personaPromptTokens is the declared token estimate of the v4 system prompt:
// UTF-8 bytes over four, rounded up. It is an estimate, not a measurement, and
// it exists so the "under 700 tokens" bound has a check that fails when the
// prompt grows rather than a claim that it did not.
func personaPromptTokens(prompt string) int {
	return (len(prompt) + 3) / 4
}
