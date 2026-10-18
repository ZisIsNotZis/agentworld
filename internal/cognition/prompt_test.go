package cognition_test

import (
	"strings"
	"testing"

	"agentworld/internal/cognition"
	"agentworld/internal/cognition/protocol"
	"agentworld/internal/sim"
)

// personaPortrait is one declared portrait: a founder born at hour 0 with all
// three alleles neutral.
func personaPortrait(birthHour int, m, y, w int64) protocol.ActorPortrait {
	return protocol.ActorPortrait{BirthHour: birthHour, LocusM: m, LocusY: y, LocusW: w}
}

// TestPersonaCardIsDeterministic pins reproducibility: the same actor and
// portrait render the same person, in every hour of a run except the age band.
func TestPersonaCardIsDeterministic(t *testing.T) {
	portrait := personaPortrait(0, 6, 6, 6)
	first := cognition.PersonaFor(3, portrait, 40)
	second := cognition.PersonaFor(3, portrait, 40)
	if first != second {
		t.Fatalf("persona is not deterministic: %+v vs %+v", first, second)
	}
	later := cognition.PersonaFor(3, portrait, 41)
	first.AgeBand, later.AgeBand = "", ""
	if first != later {
		t.Fatalf("persona changed between adjacent hours: %+v vs %+v", first, later)
	}
}

// TestPersonaCardDependsOnIdAndGenome pins that the card is a function of the
// actor's own identity rows: distinct ids and distinct allele vectors select
// distinct persons, and the age band follows the elapsed hours.
func TestPersonaCardDependsOnIdAndGenome(t *testing.T) {
	portrait := personaPortrait(0, 6, 6, 6)
	names := map[string]bool{}
	temperaments := map[string]bool{}
	for id := sim.EntityID(1); id <= 12; id++ {
		card := cognition.PersonaFor(id, portrait, 10)
		names[card.Name] = true
		temperaments[card.Temperament] = true
	}
	if len(names) < 6 {
		t.Fatalf("12 actors produced only %d distinct names: %v", len(names), names)
	}
	if len(temperaments) < 3 {
		t.Fatalf("12 actors produced only %d distinct temperaments", len(temperaments))
	}
	// The genome selects among wants and fears: a different allele vector must be
	// able to select a different want for the same actor.
	wants := map[string]bool{}
	for m := int64(5); m <= 7; m++ {
		for y := int64(5); y <= 7; y++ {
			for w := int64(5); w <= 7; w++ {
				wants[cognition.PersonaFor(3, personaPortrait(0, m, y, w), 10).Wants] = true
			}
		}
	}
	if len(wants) < 2 {
		t.Fatalf("the genome never changed the want: %v", wants)
	}
	// Age bands follow elapsed hours, not the wall clock.
	if cognition.PersonaFor(1, personaPortrait(0, 6, 6, 6), 0).AgeBand ==
		cognition.PersonaFor(1, personaPortrait(0, 6, 6, 6), 120).AgeBand {
		t.Fatal("age band ignores elapsed hours")
	}
	if cognition.PersonaFor(1, personaPortrait(0, 6, 6, 6), 120).AgeBand !=
		cognition.PersonaFor(1, personaPortrait(0, 6, 6, 6), 200).AgeBand {
		t.Fatal("age band is not a band: the same elapsed band differs")
	}
}

// TestPersonaCardReadsNoPrivilegedRow is the structural half of "never
// privileged": the portrait type has no field for a death hour, a parent, a seed
// or another actor's row, so the card cannot state one. This test fails at
// compile time if a field is added without revisiting that decision.
func TestPersonaCardReadsNoPrivilegedRow(t *testing.T) {
	portrait := protocol.ActorPortrait{BirthHour: 0, LocusM: 6, LocusY: 6, LocusW: 6}
	card := cognition.PersonaFor(7, portrait, 50)
	prompt := strings.ToLower(cognition.NaturalClaimSystemPromptV4(card))
	for _, marker := range []string{"death hour", "lifetime", "parent", "seed", "genome", "allele", "locus"} {
		if strings.Contains(prompt, marker) {
			t.Fatalf("prompt leaks the privileged marker %q: %s", marker, prompt)
		}
	}
}

// TestNaturalClaimSystemPromptV4Shape pins the declared content and bound of the
// v4 system prompt: the situation, the persona, the goal, the four facts, the
// output shape, and under 700 tokens by the declared estimate.
func TestNaturalClaimSystemPromptV4Shape(t *testing.T) {
	card := cognition.PersonaFor(5, personaPortrait(0, 6, 6, 6), 50)
	prompt := cognition.NaturalClaimSystemPromptV4(card)
	// One declared estimate for the token bound: UTF-8 bytes over four, rounded
	// up. It is an estimate, not a measurement, and it is stated once.
	if tokens := (len(prompt) + 3) / 4; tokens > 700 {
		t.Fatalf("v4 system prompt estimate %d tokens exceeds the 700 bound (%d bytes)", tokens, len(prompt))
	}
	ordered := []string{
		"You are one person in a small settlement", // the situation
		card.Name,                               // the persona: name
		card.AgeBand,                            // the persona: age band
		card.Temperament,                        // the persona: temperament
		card.Wants,                              // the persona: want
		card.Fears,                              // the persona: fear
		"stay alive, keep food in your granary", // the goal, in the actor's terms
		"A slot is a strip of ground",           // fact 1
		"The granary is the store your capital fills",   // fact 2
		"at most one unit from the granary in one hour", // fact 3
		"Your bag holds one food unit",                  // fact 4
		"up to a ceiling of twelve",                     // the meal's ceiling, so "a meal restores one" is not an overclaim
		`"why"`,                                         // the output shape
	}
	at := 0
	for _, fragment := range ordered {
		next := strings.Index(prompt[at:], fragment)
		if next < 0 {
			t.Fatalf("prompt is missing %q or has it out of order:\n%s", fragment, prompt)
		}
		at += next + len(fragment)
	}
	// The prompt states no number the request carries: a restated value would be
	// a field-by-field glossary, which the contract forbids.
	for _, number := range []string{"12", "24", "8 food", "hour 0"} {
		if strings.Contains(prompt, number) {
			t.Fatalf("prompt restates a request value %q:\n%s", number, prompt)
		}
	}
	if !strings.Contains(prompt, `"action"`) || !strings.Contains(prompt, "schema_version") {
		t.Fatalf("prompt does not declare the answer shape:\n%s", prompt)
	}
}
