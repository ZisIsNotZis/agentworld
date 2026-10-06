package strategy

import (
	"agentworld/internal/sim"
	"sort"
)

// Natural v9 policy lane (ticket 20 L2). The frozen design in
// .scratch/20-exchange-design/spec.md is binding: policy natural/9 (format 9,
// ref {natural,9}, budget 32/32) is the natural/7b cascade VERBATIM plus
// exactly three wage-contract additions, everything else inherited untouched —
// the same shared observation, view, and choice types with the same field
// sets, the same structural validation, memoryless evaluation, the same
// budget semantics, and the same frozen runner phase placement:
//
//	(a) rule-516 labour-offer filing (h+40m+5µs): EvaluateLabourOffer9, the
//	    birth-request cascade's structure with the offer row's preconditions
//	    (alive, bag = 0, no open offer, last-offer-hour < hour) and no claim
//	    fields anywhere — the terms are the derived fixed 1:1 rate, there is
//	    nothing to negotiate or report;
//	(b) rule-518 delivery (the bag decision): Evaluate9 emits NaturalDeliver
//	    instead of Build when the actor's own accepted offer sits inside its
//	    single delivery hour (accept hour + 1) and the bag holds exactly one
//	    unit — the delivery IS the hour's unit conversion. A gather denied
//	    earlier leaves the bag empty, the frozen denial fallback runs, and
//	    the settlement voids on the world's typed path; the policy adds
//	    nothing to that path;
//	(c) rule-517 labour-decision (h+40m+6µs): EvaluateLabourDecision9, the
//	    birth-reply cascade's structure (ascending worker order, first
//	    acceptable accepted, otherwise the lowest refused with its typed
//	    reason, one decision per actor-hour) accepting iff the derived floor
//	    holds: own granary stock >= 4 = R*+2 — the frozen TProvision/TFile
//	    derivation reused verbatim, "pay only from reserve plus the full
//	    price; >= 3 = R*+1 remains" — AND own k+wip < 8 (frozen KMax).
//
// Everything else stays the natural/7b cascade exactly: the bag 3/3/4/4
// floors, the claim cascade (banking floors, no provision affordance), the
// denied-gather fallback, the birth margins at own energy >= 4 (TFile), the
// consent conjunction, the addressee rotation, and the budgets. The birth
// twins below reuse the frozen natural/7 shared cascades unchanged, so their
// verbatim identity is structural, and the differential grid against 7b in
// natural9_test.go proves flips occur exactly in the declared trade cells.
//
// # Privacy (frozen design)
//
// A trader never sees the counterparty's private state. The filing view
// observes only own state and the same-patch roster and publishes nothing
// beyond the addressee — the offer row carries hour/addressee/status/
// last-offer-hour/last-reply-hour and NO claim fields (the birth-request
// field set minus the self-reported claims). The employer's pending view
// carries the worker's public identity and the filing hour only. The
// embedded NaturalObservation in the hourly view keeps the frozen field set
// pinned at compile time; natural9_test.go reflects every new type and scans
// the counterparty-facing views for foreign-state field names.
//
// # The enum ruling (the 515 precedent)
//
// NaturalDeliver is one new action constant appended to the frozen
// NaturalAction enum. As ruled for NaturalProvision there is no String
// method and no textual rendering of the new action values anywhere — the
// constants are the only spelling.
const (
	NaturalPolicyFormatV9 uint32 = 9 // natural/9: the wage-contract arm

	// The natural/9 accept floor reuses the frozen derivation verbatim:
	// stock >= naturalPolicyTProvisionV7 = R*+2 = 4 ("pay only from reserve
	// plus the full price") and k+wip < naturalKMax = 8 (wages buy wip only
	// where capital formation is legal). No new constant is declared: the
	// derived floor is the frozen TProvision/TFile value and the frozen KMax.
)

// NaturalDeliver is the natural/9 bag-decision action appended to the frozen
// NaturalAction enum (one past NaturalProvision; pinned by test). The
// pending-contract worker delivers its held gathered unit into the employer's
// worksite — rule 518 settles it, the enum value is only the intention's
// spelling. Per the recorded enum ruling (natural/7's NaturalProvision, the
// 515 precedent) there is no String method and no textual rendering: the
// constant is the only name of the value.
const NaturalDeliver NaturalAction = 8

// FrozenNaturalPolicy9 constructs the declared natural/9 policy, not a
// tunable family: format 9, ref {natural,9}, budget 32/32. As in every
// frozen natural ref, every actor of the cap-free roster binds the single
// frozen ref its world configured; the genome, not the policy ref, carries
// heredity.
func FrozenNaturalPolicy9() NaturalPolicy {
	return NaturalPolicy{
		FormatVersion: NaturalPolicyFormatV9,
		Ref:           NaturalRef{ID: "natural", Version: 9},
		Budget:        NaturalBudget{Candidates: NaturalMaxCandidates, Evaluations: NaturalMaxCandidates},
	}
}

// validNaturalPolicy9 mirrors validNaturalPolicy for the natural/9 ref: the
// declared format and ref with the same budget-ceiling semantics, so a
// same-ref budget variant stays registrable and fails closed only at
// restore — VerifyFrozenNaturalPolicy compares the complete wire content
// against the executable's frozen policy.
func validNaturalPolicy9(p NaturalPolicy) bool {
	frozen := FrozenNaturalPolicy9()
	return p.FormatVersion == frozen.FormatVersion && p.Ref == frozen.Ref &&
		p.Budget.Candidates >= 1 && p.Budget.Candidates <= frozen.Budget.Candidates &&
		p.Budget.Evaluations >= 1 && p.Budget.Evaluations <= frozen.Budget.Evaluations
}

// frozenNatural9Bound reports whether the bound carries the frozen natural/9
// identity. The 9 cascade refuses to evaluate under any other bound content:
// a bound 7b policy must never take the 9 path. As in v6/v7, the frozen v5/v6
// methods cannot carry the converse guard (their bytes are frozen), so the
// seam must call the matching cascade for the bound ref.
func frozenNatural9Bound(b *NaturalBound) bool {
	return b != nil && b.policy.FormatVersion == NaturalPolicyFormatV9 &&
		b.policy.Ref == FrozenNaturalPolicy9().Ref
}

// NaturalLabourOfferGuard is the actor's own offer-row guard, the rule-516
// row's policy-relevant fields: the status lifecycle value, the last filing
// hour, and the last decision hour. It deliberately carries no addressee (no
// policy decision reads it) and no claim fields (none exist). Status mirrors
// the frozen request statuses; an accepted row's LastReplyHour is the accept
// hour, whose +1 is the contract's single delivery hour.
type NaturalLabourOfferGuard struct {
	Status        NaturalRequestStatus
	LastOfferHour int64
	LastReplyHour int64
}

// validNaturalLabourOfferGuard mirrors the world's frozen offer-row shape at
// one observation hour: the status bounded, the guard hours within [never,
// hour], a filed row behind every non-idle status (idle-with-audit is legal —
// rule 519 clears finished contracts to idle while keeping the filing as the
// audit), and an accepted row carrying its decision hour (the delivery
// window's base).
func validNaturalLabourOfferGuard(g NaturalLabourOfferGuard, hour int) bool {
	if g.Status > NaturalRequestExpired ||
		g.LastOfferHour < naturalNeverHour || g.LastOfferHour > int64(hour) ||
		g.LastReplyHour < naturalNeverHour || g.LastReplyHour > int64(hour) {
		return false
	}
	if g.Status != NaturalRequestIdle && g.LastOfferHour < 0 {
		return false
	}
	if g.Status == NaturalRequestAccepted && g.LastReplyHour < 0 {
		return false
	}
	return true
}

// naturalLabourDeliveryDue reports whether the actor's own accepted offer
// sits inside its single delivery hour — the accept hour + 1, exactly the
// matched-contract window the world's rule-518 revalidation compares
// (engaged = hour-1). The lifecycle is the frozen shape: a pending offer
// lives only inside its filing hour, accept/refuse are same-hour, and the
// pulse after the delivery hour clears the accepted row to idle-with-audit,
// so exactly one hour can ever be due.
func naturalLabourDeliveryDue(g NaturalLabourOfferGuard, hour int) bool {
	return g.Status == NaturalRequestAccepted && g.LastReplyHour == int64(hour)-1
}

// NaturalLabourObservation is the natural/9 hourly claim view: the frozen v5
// observation embedded whole — the complete shared field set pinned by the
// type system, no field added, removed, or retyped — plus the actor's own
// labour-offer guard, the one own-row addition the delivery decision needs.
type NaturalLabourObservation struct {
	NaturalObservation
	OwnOffer NaturalLabourOfferGuard
}

// Evaluate9 is the frozen natural/9 hourly decision cascade: the natural/7b
// Evaluate verbatim — structural validation, InvalidSelf degradation, the
// bag decision (3/3/4/4), the denied-gather fallback, at most one stored
// meal per actor-hour, budget semantics, no provision affordance — except
// the bag decision's positive investment slot, which becomes the rule-518
// delivery when the actor's own contract is inside its single delivery hour
// and the bag holds exactly one unit. The frozen build-denial leg precedes
// the substitution verbatim: a BuildDenied hour was never a delivery hour
// (a pending contract never attempts a build), and both cascades eat there.
// The bound must carry the frozen natural/9 identity or the evaluation fails
// closed with ErrInvalidNaturalPolicy.
func (b *NaturalBound) Evaluate9(obs NaturalLabourObservation) (NaturalChoice, error) {
	if !frozenNatural9Bound(b) {
		return NaturalChoice{}, ErrInvalidNaturalPolicy
	}
	return natural9Claim(b, obs)
}

// natural9Claim is the frozen natural/9 claim cascade: the natural/7b body
// verbatim (Gather | EatStored at the survival floor | Wait; the denied-
// gather fallback; the v6 bag decision 3/3/4/4) with the guard validation
// and the delivery substitution, and nothing else.
func natural9Claim(b *NaturalBound, obs NaturalLabourObservation) (NaturalChoice, error) {
	if obs.Actor != b.actor || obs.Ref != b.policy.Ref ||
		obs.Hour < 0 || obs.Hour >= naturalHorizonHours || obs.LastDenial > NaturalBuildDenied ||
		len(obs.HomeSlots) > naturalSlotsPerPatch ||
		!validNaturalLabourOfferGuard(obs.OwnOffer, obs.Hour) {
		return NaturalChoice{}, ErrInvalidNaturalObservation
	}
	choice := NaturalChoice{Kind: NaturalWait, ObservedVersion: obs.WorldVersion, Ref: b.policy.Ref, Fallback: NaturalNoCandidate}
	energy, energyOK := capacityInteger(obs.Energy, 0, naturalEnergyCapacity)
	_, hungerOK := capacityInteger(obs.Hunger, 0, naturalHungerCapacity)
	bag, bagOK := capacityInteger(obs.BagUnits, 0, naturalBagCapacity)
	capital, capitalOK := capacityInteger(obs.Capital, 0, naturalKMax)
	wip, wipOK := capacityInteger(obs.Wip, 0, naturalWipMax)
	granary, granaryOK := capacityInteger(obs.GranaryStock, 0, naturalGranaryStockMax)
	_, investedOK := capacityInteger(obs.InvestedUnits, 0, naturalHorizonHours)
	lastBuild, lastBuildOK := capacityInteger(obs.LastBuildHour, naturalNeverHour, naturalHorizonHours-1)
	lastMeal, lastMealOK := capacityInteger(obs.LastStoredMealHour, naturalNeverHour, naturalHorizonHours-1)
	if !energyOK || !hungerOK || !bagOK || !capitalOK || !wipOK || !granaryOK || !investedOK || !lastBuildOK || !lastMealOK ||
		lastBuild > int64(obs.Hour) || lastMeal > int64(obs.Hour) {
		choice.Fallback = NaturalInvalidSelf
		return choice, nil
	}
	// The bag names its holder's home patch whether empty or held; a founder
	// must name its ID-placed patch, a newborn its proposer-placed patch.
	home, err := obs.BagSource.EntityRef()
	if err != nil || obs.BagSource.Kind() != sim.EntityRefKind || obs.BagSource.State() != sim.Present || !naturalPatchEntity(b.patches, home) {
		choice.Fallback = NaturalInvalidSelf
		return choice, nil
	}
	if expected, ok := naturalFounderHomePatch(b.patches, obs.Actor); ok && expected != home {
		choice.Fallback = NaturalInvalidSelf
		return choice, nil
	}
	// A recorded build denial implies the refused unit is still held.
	if obs.LastDenial == NaturalBuildDenied && bag != 1 {
		return NaturalChoice{}, ErrInvalidNaturalObservation
	}
	slots := append([]NaturalSlotView(nil), obs.HomeSlots...)
	sort.Slice(slots, func(i, j int) bool { return slots[i].ID < slots[j].ID })
	first := naturalFirstSlot(home)
	for i, view := range slots {
		if view.ID < first || view.ID >= first+naturalSlotsPerPatch || (i > 0 && slots[i-1].ID == view.ID) {
			return NaturalChoice{}, ErrInvalidNaturalObservation
		}
		if _, ok := capacityInteger(view.Stock, 0, 1); !ok {
			return NaturalChoice{}, ErrInvalidNaturalObservation
		}
	}
	if energy == 0 {
		return choice, nil // a dead actor never acts
	}
	if bag == 1 { // frozen v6 bag decision, with the natural/9 substitution
		if b.policy.Budget.Candidates < 1 || b.policy.Budget.Evaluations < 1 {
			choice.Fallback = NaturalBudgetExhausted
			return choice, nil
		}
		if obs.LastDenial == NaturalBuildDenied {
			// The world refused this hour's build (no free wild slot with
			// stock 0); report Build unavailable and fall back to the meal.
			// Verbatim 7b, and ahead of the delivery window: a pending
			// contract never attempts a build, so this leg keeps the flip
			// set exactly on the reachable delivery cells.
			choice.Kind, choice.Evaluated, choice.Fallback = NaturalEat, 1, NaturalNoFallback
			return choice, nil
		}
		// The natural/9 substitution: the delivery IS the hour's unit
		// conversion. The gathered unit belongs to the matched contract,
		// so the delivery replaces Build (and the margin-failed Eat) in
		// every due cell; the world's rule-518 fresh-snapshot revalidation
		// owns every refusal (typed void, zero mutation, the bag intact —
		// a denied gather never reaches here with the unit held).
		if naturalLabourDeliveryDue(obs.OwnOffer, obs.Hour) {
			choice.Kind, choice.Evaluated, choice.Fallback = NaturalDeliver, 1, NaturalNoFallback
			return choice, nil
		}
		if capital+wip < naturalKMax && energy >= naturalPolicyTFloorV6 &&
			(granary >= 1 || (wip >= 1 && energy >= naturalPolicyTFinishV6) || energy >= naturalPolicyTStartV6) {
			choice.Kind, choice.Evaluated, choice.Fallback = NaturalBuild, 1, NaturalNoFallback
		} else {
			choice.Kind, choice.Evaluated, choice.Fallback = NaturalEat, 1, NaturalNoFallback
		}
		return choice, nil
	}
	if obs.LastDenial == NaturalGatherDenied { // denied-gather fallback, verbatim 7b
		if granary >= 1 && lastMeal < int64(obs.Hour) {
			choice.Kind, choice.Evaluated, choice.Fallback = NaturalEatStored, 1, NaturalNoFallback
		} else {
			choice.Kind, choice.RestDuration, choice.Evaluated, choice.Fallback = NaturalRest, NaturalRestDuration, 1, NaturalNoFallback
		}
		return choice, nil
	}
	candidates := make([]sim.EntityID, 0, len(slots))
	for _, view := range slots {
		if stock, ok := capacityInteger(view.Stock, 0, 1); ok && stock == 1 {
			candidates = append(candidates, view.ID)
		}
	}
	if len(candidates) > b.policy.Budget.Candidates || len(candidates) > b.policy.Budget.Evaluations {
		choice.Fallback = NaturalBudgetExhausted // all-or-nothing: never a sorted prefix
		return choice, nil
	}
	if len(candidates) > 0 {
		choice.Kind, choice.TargetSlot, choice.Evaluated, choice.Fallback = NaturalGather, candidates[0], len(candidates), NaturalNoFallback
		return choice, nil
	}
	// The 7b bank floor verbatim: the bank is food only at the survival
	// floor, and the ablation's conversion stays disabled — the wage, not
	// provisioning, is natural/9's granary-inflow channel.
	if granary >= 1 && energy <= naturalPolicyTBankV7 && lastMeal < int64(obs.Hour) { // one stored meal per actor-hour at most
		choice.Kind, choice.Evaluated, choice.Fallback = NaturalEatStored, 1, NaturalNoFallback
	}
	return choice, nil
}

// NaturalLabourOfferObservation is the actor-visible rule-516 filing view at
// h+40m+5µs: the actor's own alive/bag margins, its own offer guard, the
// public home patch, and the same-patch roster. It observes nothing else:
// no counterparty state of any kind (the roster is bare IDs), no claim
// fields (the terms are the derived fixed 1:1 rate — there is nothing to
// negotiate or report), and no slot, tenure, ledger, or history data.
type NaturalLabourOfferObservation struct {
	Actor        sim.EntityID
	Ref          NaturalRef
	Hour         int
	WorldVersion sim.WorldVersion
	HomePatch    sim.EntityID // the actor's home patch (its bag source row); public
	Energy       sim.Value
	BagUnits     sim.Value
	OwnOffer     NaturalLabourOfferGuard
	PatchMates   []sim.EntityID // same-patch roster excluding self; runner-supplied public data
}

type NaturalLabourOfferAction uint8

const (
	NaturalLabourOfferWait NaturalLabourOfferAction = iota
	NaturalLabourOfferFile
)

// NaturalLabourOfferChoice files one addressed offer. The published content
// is the addressee and nothing else: the offer terms are frozen (one
// gathered wild unit delivered for exactly one granary stock unit), so the
// choice struct has no claim fields, no price field, and no terms field.
type NaturalLabourOfferChoice struct {
	Kind            NaturalLabourOfferAction
	Addressee       sim.EntityID
	ObservedVersion sim.WorldVersion
	Ref             NaturalRef
	Evaluated       int
	Fallback        NaturalFallback
}

// EvaluateLabourOffer9 is the frozen natural/9 worker decision at
// h+40m+5µs: the birth-request cascade's structure — living, free offer
// slot, one filing per actor-hour, the deterministic same-patch addressee
// rotation, honest minimal publication — over the rule-516 preconditions:
// the actor alive, its bag empty (the bag decision has consumed the hour's
// unit), no open offer (pending or accepted — one open contract per
// worker), and last-offer-hour strictly below the hour. Unlike the birth
// request there are no own surplus margins and no reported claims: any
// living empty-bagged actor without an open contract is a worker candidate,
// and the wage is the first landless granary-inflow channel. The bound must
// carry the frozen natural/9 identity or the evaluation fails closed.
func (b *NaturalBound) EvaluateLabourOffer9(obs NaturalLabourOfferObservation) (NaturalLabourOfferChoice, error) {
	if !frozenNatural9Bound(b) {
		return NaturalLabourOfferChoice{}, ErrInvalidNaturalPolicy
	}
	if obs.Actor != b.actor || obs.Ref != b.policy.Ref ||
		obs.Hour < 0 || obs.Hour >= naturalHorizonHours ||
		!naturalPatchEntity(b.patches, obs.HomePatch) ||
		!validNaturalLabourOfferGuard(obs.OwnOffer, obs.Hour) {
		return NaturalLabourOfferChoice{}, ErrInvalidNaturalObservation
	}
	choice := NaturalLabourOfferChoice{Kind: NaturalLabourOfferWait, ObservedVersion: obs.WorldVersion, Ref: b.policy.Ref, Fallback: NaturalNoCandidate}
	energy, energyOK := capacityInteger(obs.Energy, 0, naturalEnergyCapacity)
	bag, bagOK := capacityInteger(obs.BagUnits, 0, naturalBagCapacity)
	if !energyOK || !bagOK {
		choice.Fallback = NaturalInvalidSelf
		return choice, nil
	}
	founderHome, isFounder := naturalFounderHomePatch(b.patches, obs.Actor)
	if isFounder && founderHome != obs.HomePatch {
		return NaturalLabourOfferChoice{}, ErrInvalidNaturalObservation
	}
	mates := append([]sim.EntityID(nil), obs.PatchMates...)
	sort.Slice(mates, func(i, j int) bool { return mates[i] < mates[j] })
	for i, mate := range mates {
		if !naturalRosterEntity(b.patches, mate) || mate == obs.Actor || (i > 0 && mates[i-1] == mate) {
			return NaturalLabourOfferChoice{}, ErrInvalidNaturalObservation
		}
		// Founder placement is public fixed geometry: a founder mate on a
		// different patch than the observed home patch is a malformed roster.
		// Newborn placement stays runner-supplied public data; the world
		// re-gates same-patch on fresh truth at the decision.
		if home, ok := naturalFounderHomePatch(b.patches, mate); ok && home != obs.HomePatch {
			return NaturalLabourOfferChoice{}, ErrInvalidNaturalObservation
		}
	}
	if energy == 0 {
		return choice, nil // a dead actor never acts
	}
	if obs.OwnOffer.Status == NaturalRequestPending || obs.OwnOffer.Status == NaturalRequestAccepted {
		return choice, nil // one open contract per worker; a live offer resolves first
	}
	if obs.OwnOffer.LastOfferHour >= int64(obs.Hour) {
		return choice, nil // one filing per actor-hour
	}
	if bag != 0 {
		return choice, nil // the held unit is the hour's own conversion, not a filing hour
	}
	if b.policy.Budget.Candidates < 1 || b.policy.Budget.Evaluations < 1 ||
		len(mates) > b.policy.Budget.Candidates || len(mates) > b.policy.Budget.Evaluations {
		choice.Fallback = NaturalBudgetExhausted
		return choice, nil
	}
	if len(mates) == 0 {
		return choice, nil
	}
	addressee := mates[0] // wraparound successor of the highest same-patch ID
	for _, mate := range mates {
		if mate > obs.Actor {
			addressee = mate
			break
		}
	}
	choice.Kind = NaturalLabourOfferFile
	choice.Addressee = addressee
	choice.Evaluated = len(mates)
	choice.Fallback = NaturalNoFallback
	return choice, nil
}

// NaturalLabourPendingView is one pending labour offer addressed to the
// observing employer: the worker's public identity and the filing hour. The
// offer row carries no claim fields, so this view has none either — and no
// worker energy, granary, capital, bag, loci, tenure, ledger, birth/death
// hour, or history field of any kind: the employer never sees worker truth.
type NaturalLabourPendingView struct {
	Worker    sim.EntityID
	Addressee sim.EntityID
	Hour      int
}

// NaturalLabourDecisionObservation is the actor-visible rule-517 view at
// h+40m+6µs: the employer's own floor margins (granary stock, capital, wip),
// its own one-decision-per-actor-hour guard, and the pending addressed
// offers' public identities. No worker private state has a field here.
type NaturalLabourDecisionObservation struct {
	Actor               sim.EntityID
	Ref                 NaturalRef
	Hour                int
	WorldVersion        sim.WorldVersion
	Energy              sim.Value
	GranaryStock        sim.Value
	Capital             sim.Value
	Wip                 sim.Value
	OwnLastDecisionHour int64 // typed world ledger guard; -1 until first decision
	Pending             []NaturalLabourPendingView
}

type NaturalLabourDecisionAction uint8

const (
	NaturalLabourDecisionNone   NaturalLabourDecisionAction = iota // no decision this hour
	NaturalLabourDecisionAccept                                    // engage the worker for next hour's delivery
	// NaturalLabourDecisionRefuse is a typed policy refusal, final at the
	// policy level (the E12 class): no execution, no draws, no candidate.
	NaturalLabourDecisionRefuse
)

// NaturalLabourDecisionReason is the typed refusal taxonomy of the frozen
// derived floor, in the settlement revalidation's clause order.
type NaturalLabourDecisionReason uint8

const (
	NaturalLabourReasonNone NaturalLabourDecisionReason = iota
	// NaturalLabourReasonFloorStock: own granary stock is below the derived
	// wage floor R*+2 = 4 — below it the stock is survival stock, not wage
	// stock ("pay only from reserve plus the full price; >= 3 = R*+1
	// remains").
	NaturalLabourReasonFloorStock
	// NaturalLabourReasonFloorKMax: own k+wip is at the frozen KMax, so
	// wages cannot buy wip there.
	NaturalLabourReasonFloorKMax
)

// NaturalLabourDecisionChoice is one decision intention; the runner places
// it at the frozen instant and the world rechecks everything.
type NaturalLabourDecisionChoice struct {
	Kind            NaturalLabourDecisionAction
	Worker          sim.EntityID
	Reason          NaturalLabourDecisionReason
	ObservedVersion sim.WorldVersion
	Ref             NaturalRef
	Evaluated       int
	Fallback        NaturalFallback
}

// EvaluateLabourDecision9 is the frozen natural/9 employer decision at
// h+40m+6µs: the birth-reply cascade's structure — ascending worker order,
// the first acceptable pending accepted, otherwise the lowest-ID pending
// refused with its typed reason, one decision per actor-hour — over the
// derived accept floor: own granary stock >= 4 = R*+2 (the frozen
// TProvision/TFile derivation reused verbatim) AND own k+wip < 8 (frozen
// KMax). The floor is the employer's own state alone, so every pending
// offer faces the same verdict; the world's rule-517 re-checks the row and
// the engagement slot on fresh truth. The bound must carry the frozen
// natural/9 identity or the evaluation fails closed.
func (b *NaturalBound) EvaluateLabourDecision9(obs NaturalLabourDecisionObservation) (NaturalLabourDecisionChoice, error) {
	if !frozenNatural9Bound(b) {
		return NaturalLabourDecisionChoice{}, ErrInvalidNaturalPolicy
	}
	if obs.Actor != b.actor || obs.Ref != b.policy.Ref ||
		obs.Hour < 0 || obs.Hour >= naturalHorizonHours ||
		obs.OwnLastDecisionHour < naturalNeverHour || obs.OwnLastDecisionHour > int64(obs.Hour) {
		return NaturalLabourDecisionChoice{}, ErrInvalidNaturalObservation
	}
	choice := NaturalLabourDecisionChoice{Kind: NaturalLabourDecisionNone, ObservedVersion: obs.WorldVersion, Ref: b.policy.Ref, Fallback: NaturalNoCandidate}
	energy, energyOK := capacityInteger(obs.Energy, 0, naturalEnergyCapacity)
	granary, granaryOK := capacityInteger(obs.GranaryStock, 0, naturalGranaryStockMax)
	capital, capitalOK := capacityInteger(obs.Capital, 0, naturalKMax)
	wip, wipOK := capacityInteger(obs.Wip, 0, naturalWipMax)
	if !energyOK || !granaryOK || !capitalOK || !wipOK {
		choice.Fallback = NaturalInvalidSelf
		return choice, nil
	}
	pending := append([]NaturalLabourPendingView(nil), obs.Pending...)
	sort.Slice(pending, func(i, j int) bool { return pending[i].Worker < pending[j].Worker })
	for i, view := range pending {
		if !naturalRosterEntity(b.patches, view.Worker) || view.Worker == obs.Actor ||
			view.Addressee != obs.Actor || view.Hour != obs.Hour ||
			(i > 0 && pending[i-1].Worker == view.Worker) {
			return NaturalLabourDecisionChoice{}, ErrInvalidNaturalObservation
		}
	}
	if energy == 0 || obs.OwnLastDecisionHour >= int64(obs.Hour) || len(pending) == 0 {
		return choice, nil // a dead employer never acts; one decision per actor-hour
	}
	if b.policy.Budget.Candidates < 1 || b.policy.Budget.Evaluations < 1 ||
		len(pending) > b.policy.Budget.Candidates || len(pending) > b.policy.Budget.Evaluations {
		choice.Fallback = NaturalBudgetExhausted
		return choice, nil
	}
	// The derived accept floor, frozen: the same resource clauses the
	// settlement revalidates, so a policy accept never needs the world's
	// typed stock or KMax void. Refusal reasons follow the revalidation's
	// clause order: stock first, then KMax.
	floorOK := granary >= naturalPolicyTProvisionV7 && capital+wip < naturalKMax
	reason := NaturalLabourReasonNone
	switch {
	case granary < naturalPolicyTProvisionV7:
		reason = NaturalLabourReasonFloorStock
	case capital+wip >= naturalKMax:
		reason = NaturalLabourReasonFloorKMax
	}
	examined := 0
	for _, view := range pending {
		examined++
		if floorOK {
			choice.Kind = NaturalLabourDecisionAccept
			choice.Worker = view.Worker
			choice.Evaluated = examined
			choice.Fallback = NaturalNoFallback
			return choice, nil
		}
	}
	choice.Kind = NaturalLabourDecisionRefuse
	choice.Worker = pending[0].Worker
	choice.Reason = reason
	choice.Evaluated = examined
	choice.Fallback = NaturalNoFallback
	return choice, nil
}

// EvaluateBirthRequest9 is the frozen natural/9 proposer decision at
// h+40m+5µs: the natural/7b birth-request cascade verbatim — the v6 margins
// with the own-energy filing floor TFile = 4, the deterministic same-patch
// addressee rotation, and the honest self-reported claims. The bound must
// carry the frozen natural/9 identity or the evaluation fails closed. The
// frozen design's negotiation budget (one filing per actor-hour across the
// birth request and the labour offer) is the runner's serial duty at the
// shared instant; each cascade here stays exactly its frozen shape.
func (b *NaturalBound) EvaluateBirthRequest9(obs NaturalRequestObservation) (NaturalRequestChoice, error) {
	if !frozenNatural9Bound(b) {
		return NaturalRequestChoice{}, ErrInvalidNaturalPolicy
	}
	return natural7BirthRequest(b, obs)
}

// EvaluateBirthReply9 is the frozen natural/9 consenter decision at
// h+40m+6µs: the natural/7b birth-reply cascade verbatim — ascending
// requester order, the consent conjunction with the own-energy accept floor
// TFile = 4, the reported-claims margins, the kin clause, and the typed
// refusal reasons. The bound must carry the frozen natural/9 identity or
// the evaluation fails closed.
func (b *NaturalBound) EvaluateBirthReply9(obs NaturalReplyObservation) (NaturalReplyChoice, error) {
	if !frozenNatural9Bound(b) {
		return NaturalReplyChoice{}, ErrInvalidNaturalPolicy
	}
	return natural7BirthReply(b, obs)
}
