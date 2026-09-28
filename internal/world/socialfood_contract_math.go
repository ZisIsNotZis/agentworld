package world

import "agentworld/internal/sim"

type SocialFoodRequestStatus int64

const (
	SocialFoodIdle SocialFoodRequestStatus = iota
	SocialFoodPending
	SocialFoodAccepted
	SocialFoodRefused
	SocialFoodExpired
	SocialFoodWithdrawn
)

type SocialFoodBagState struct {
	Units                       int64
	Source, Gatherer, LastDonor sim.EntityID // zero iff not applicable; gift retains original source/gatherer
}
type SocialFoodActorState struct {
	Bag                                                           SocialFoodBagState
	Energy, Hunger, BasalSpent, CapLost, Consumed, LastGatherHour int64
}
type SocialFoodRequestState struct {
	ID                        int64 // (hour*16 + requester ID), zero only for idle
	Hour                      int64
	Addressee                 sim.EntityID
	Claim                     int64 // reported urgency only, never private requester energy
	Status                    SocialFoodRequestStatus
	Requests, Gifts, Refusals int64 // witnessed outgoing directed edge to the fixed addressee
}
type SocialFoodLedgerState struct {
	LastReplyHour, Received, Given, Net int64 // Net is received - given
}
type SocialFoodConsent struct {
	RequestID int64
	Donor     sim.EntityID
	Hour      int
	Accept    bool
}
type SocialFoodPatchState struct{ Yield, Pulses, Produced, Unrealized int64 }
type SocialFoodSlotState struct{ Stock, Gathered int64 }
type SocialFoodBalance struct {
	Produced, Consumed, Stock, Held, Gathered, Unrealized int64
	InitialEnergy, Energy, BasalSpent, CapLost            int64
}
type SocialFoodAssistance struct {
	Requester, Addressee      sim.EntityID
	Requests, Gifts, Refusals int64
	Fraction                  float64 // numerical (gifts+1)/(requests+2), not moral trust
	SchemaVersion             sim.SchemaVersion
	ProjectionVersion         sim.ProjectionVersion
	SourceFields              [2]sim.FieldID // witnessed requests and gifts on directed dyad
	WorldVersion              sim.WorldVersion
	Unit                      string
}

func socialFoodRequestID(hour int, requester sim.EntityID) int64 {
	return int64(hour)*SocialFoodActorCount + int64(requester)
}
func validSocialFoodBag(b SocialFoodBagState, holder sim.EntityID) bool {
	if b.Units == 0 {
		return b.Source == 0 && b.Gatherer == 0 && b.LastDonor == 0
	}
	patch, err := SocialFoodActorPatchID(holder)
	if err != nil || b.Units != 1 || b.Source != patch {
		return false
	}
	gatherPatch, gerr := SocialFoodActorPatchID(b.Gatherer)
	if gerr != nil || gatherPatch != patch {
		return false
	}
	if b.LastDonor != 0 {
		donorPatch, derr := SocialFoodActorPatchID(b.LastDonor)
		if derr != nil || donorPatch != patch || b.LastDonor == holder {
			return false
		}
	} else if b.Gatherer != holder {
		return false
	}
	return true
}
func validSocialFoodActor(actor sim.EntityID, a SocialFoodActorState) bool {
	if _, err := SocialFoodActorPatchID(actor); err != nil || !validSocialFoodBag(a.Bag, actor) || a.Energy < 0 || a.Energy > 12 || a.Hunger < 0 || a.Hunger > 24 || a.BasalSpent < 0 || a.BasalSpent > 168 || a.CapLost < 0 || a.CapLost > 168 || a.Consumed < 0 || a.Consumed > 168 || a.LastGatherHour < SocialFoodNeverGatheredHour || a.LastGatherHour >= 168 {
		return false
	}
	return 11+a.Consumed == a.Energy+a.BasalSpent+a.CapLost
}
func validSocialFoodRequest(owner sim.EntityID, r SocialFoodRequestState) bool {
	target, _, err := SocialFoodKnownAddressee(owner)
	if err != nil || r.Requests < 0 || r.Requests > 168 || r.Gifts < 0 || r.Refusals < 0 || r.Gifts+r.Refusals > r.Requests {
		return false
	}
	if r.Status == SocialFoodIdle {
		return r.ID == 0 && r.Hour == -1 && r.Addressee == 0 && r.Claim == 0 && r.Requests == 0 && r.Gifts == 0 && r.Refusals == 0
	}
	return r.Status >= SocialFoodPending && r.Status <= SocialFoodWithdrawn && r.Hour >= 0 && r.Hour < 168 && r.ID == socialFoodRequestID(int(r.Hour), owner) && r.Addressee == target && r.Claim >= 0 && r.Claim <= 2 && r.Requests > 0 && (r.Status != SocialFoodPending || r.Gifts+r.Refusals < r.Requests) && (r.Status != SocialFoodAccepted || r.Gifts > 0) && (r.Status != SocialFoodRefused || r.Refusals > 0)
}
func validSocialFoodLedger(l SocialFoodLedgerState) bool {
	return l.LastReplyHour >= -1 && l.LastReplyHour < 168 && l.Received >= 0 && l.Received <= 168 && l.Given >= 0 && l.Given <= 168 && l.Net == l.Received-l.Given
}

// version must be the authoritative snapshot version used to read r.
func SocialFoodProjectAssistance(requester sim.EntityID, version sim.WorldVersion, r SocialFoodRequestState) (SocialFoodAssistance, error) {
	if !validSocialFoodRequest(requester, r) {
		return SocialFoodAssistance{}, ErrSocialFoodContract
	}
	target, _, _ := SocialFoodKnownAddressee(requester)
	return SocialFoodAssistance{Requester: requester, Addressee: target, Requests: r.Requests, Gifts: r.Gifts, Refusals: r.Refusals, Fraction: float64(r.Gifts+1) / float64(r.Requests+2), SchemaVersion: SocialFoodSchemaVersion, ProjectionVersion: SocialFoodProjectionVersion, SourceFields: [2]sim.FieldID{SocialFoodRequestsWitnessedField, SocialFoodGiftsWitnessedField}, WorldVersion: version, Unit: "fraction"}, nil
}

// Request admission uses privileged eligibility only in the trusted harness;
// this energy must never be included in donor-facing observations or events.
func SocialFoodRequest(hour int, requester, addressee sim.EntityID, deniedGather bool, claim int64, before SocialFoodRequestState, actor SocialFoodActorState) (SocialFoodRequestState, error) {
	target, _, err := SocialFoodKnownAddressee(requester)
	if _, e := SocialFoodPhaseTime(hour, 1); e != nil || err != nil || target != addressee || !deniedGather || claim < 0 || claim > 2 || !validSocialFoodActor(requester, actor) || actor.Energy == 0 || actor.Energy > 8 || actor.Bag.Units != 0 || !validSocialFoodRequest(requester, before) || before.Requests >= 168 || before.Status == SocialFoodPending || (before.Status != SocialFoodIdle && before.Hour >= int64(hour)) {
		return SocialFoodRequestState{}, ErrSocialFoodContract
	}
	before.ID = socialFoodRequestID(hour, requester)
	before.Hour = int64(hour)
	before.Addressee = addressee
	before.Claim = claim
	before.Status = SocialFoodPending
	before.Requests++
	return before, nil
}
func socialFoodPending(hour int, donor, recipient sim.EntityID, r SocialFoodRequestState, l SocialFoodLedgerState) bool {
	target, _, err := SocialFoodKnownAddressee(recipient)
	return err == nil && target == donor && validSocialFoodRequest(recipient, r) && r.Status == SocialFoodPending && r.Hour == int64(hour) && validSocialFoodLedger(l) && l.LastReplyHour < int64(hour)
}

// The frozen score uses only donor state, directed affinity and the reported
// claim. Requester reserve is never part of a donor-facing decision.
func socialFoodConsentScore(recipient sim.EntityID, r SocialFoodRequestState, giver SocialFoodActorState) int64 {
	_, affinity, _ := SocialFoodKnownAddressee(recipient)
	score := affinity - 1 // donor loses its own held meal
	if r.Claim == 2 {
		score += 2
	}
	if giver.Energy <= 6 {
		score--
	}
	if giver.Hunger >= 10 {
		score--
	}
	return score
}

// Consent is an independent donor intention bound to the exact pending ID.
// The harness must obtain it from the donor's policy callback, not manufacture
// it from a request. The score check prevents even a forged affirmative token
// from changing the frozen decision for the observed donor state/claim.
func SocialFoodAccept(hour int, donor, recipient sim.EntityID, c SocialFoodConsent, r SocialFoodRequestState, giver, receiver SocialFoodActorState, given, received SocialFoodLedgerState) (SocialFoodRequestState, SocialFoodActorState, SocialFoodActorState, SocialFoodLedgerState, SocialFoodLedgerState, error) {
	fail := func() (SocialFoodRequestState, SocialFoodActorState, SocialFoodActorState, SocialFoodLedgerState, SocialFoodLedgerState, error) {
		return SocialFoodRequestState{}, SocialFoodActorState{}, SocialFoodActorState{}, SocialFoodLedgerState{}, SocialFoodLedgerState{}, ErrSocialFoodContract
	}
	if _, err := SocialFoodPhaseTime(hour, 2); err != nil || !socialFoodPending(hour, donor, recipient, r, given) || !validSocialFoodLedger(received) || c.RequestID != r.ID || c.Donor != donor || c.Hour != hour || !c.Accept || !validSocialFoodActor(donor, giver) || !validSocialFoodActor(recipient, receiver) || giver.Energy < 4 || giver.Bag.Units != 1 || socialFoodConsentScore(recipient, r, giver) < 3 || receiver.Energy == 0 || receiver.Bag.Units != 0 || given.Given >= 168 || received.Received >= 168 || r.Gifts >= 168 {
		return fail()
	}
	patch, _ := SocialFoodActorPatchID(donor)
	other, _ := SocialFoodActorPatchID(recipient)
	if patch != other || giver.Bag.Source != patch {
		return fail()
	}
	bag := giver.Bag
	giver.Bag = SocialFoodBagState{}
	receiver.Bag = bag
	receiver.Bag.LastDonor = donor
	r.Status = SocialFoodAccepted
	r.Gifts++
	given.LastReplyHour = int64(hour)
	given.Given++
	given.Net--
	received.Received++
	received.Net++
	if !validSocialFoodRequest(recipient, r) || !validSocialFoodActor(donor, giver) || !validSocialFoodActor(recipient, receiver) || !validSocialFoodLedger(given) || !validSocialFoodLedger(received) {
		return fail()
	}
	return r, giver, receiver, given, received, nil
}
func SocialFoodRefuse(hour int, donor, recipient sim.EntityID, c SocialFoodConsent, r SocialFoodRequestState, giver SocialFoodActorState, l SocialFoodLedgerState) (SocialFoodRequestState, SocialFoodLedgerState, error) {
	if _, err := SocialFoodPhaseTime(hour, 2); err != nil || !socialFoodPending(hour, donor, recipient, r, l) || c.RequestID != r.ID || c.Donor != donor || c.Hour != hour || c.Accept || !validSocialFoodActor(donor, giver) || giver.Energy == 0 || (giver.Energy >= 4 && giver.Bag.Units == 1 && socialFoodConsentScore(recipient, r, giver) >= 3) || r.Refusals >= 168 {
		return SocialFoodRequestState{}, SocialFoodLedgerState{}, ErrSocialFoodContract
	}
	r.Status = SocialFoodRefused
	r.Refusals++
	l.LastReplyHour = int64(hour)
	if !validSocialFoodRequest(recipient, r) || !validSocialFoodLedger(l) {
		return SocialFoodRequestState{}, SocialFoodLedgerState{}, ErrSocialFoodContract
	}
	return r, l, nil
}

// SocialFoodExpire closes an unanswered request at the same hour's finalization
// boundary, after reply/gift. A stale request cannot be expired at a later pulse.
func SocialFoodExpire(hour int, at sim.SimTime, owner sim.EntityID, r SocialFoodRequestState) (SocialFoodRequestState, error) {
	finalize, err := SocialFoodPhaseTime(hour, 3)
	if err != nil || at != finalize || !validSocialFoodRequest(owner, r) || r.Status != SocialFoodPending || r.Hour != int64(hour) {
		return SocialFoodRequestState{}, ErrSocialFoodContract
	}
	r.Status = SocialFoodExpired
	if !validSocialFoodRequest(owner, r) {
		return SocialFoodRequestState{}, ErrSocialFoodContract
	}
	return r, nil
}
func SocialFoodWithdraw(hour int, owner sim.EntityID, r SocialFoodRequestState) (SocialFoodRequestState, error) {
	if !validSocialFoodRequest(owner, r) || r.Status != SocialFoodPending || r.Hour != int64(hour) {
		return SocialFoodRequestState{}, ErrSocialFoodContract
	}
	r.Status = SocialFoodWithdrawn
	return r, nil
}
func SocialFoodProduce(hour int, p SocialFoodPatchState, slots [8]SocialFoodSlotState) (SocialFoodPatchState, [8]SocialFoodSlotState, error) {
	if _, err := SocialFoodPhaseTime(hour, 0); err != nil || p.Pulses != int64(hour) || p.Yield < 0 || p.Yield > 8 || p.Produced < 0 || p.Produced > 1344 || p.Unrealized < 0 || p.Unrealized > 1344 || p.Produced+p.Unrealized != p.Yield*p.Pulses {
		return SocialFoodPatchState{}, slots, ErrSocialFoodContract
	}
	var total int64
	for _, s := range slots {
		if s.Stock < 0 || s.Stock > 1 || s.Gathered < 0 || s.Gathered+s.Stock > p.Pulses {
			return SocialFoodPatchState{}, slots, ErrSocialFoodContract
		}
		total += s.Gathered + s.Stock
	}
	if total != p.Produced {
		return SocialFoodPatchState{}, slots, ErrSocialFoodContract
	}
	out := slots
	var produced int64
	for i := range out {
		if out[i].Stock == 0 && produced < p.Yield {
			out[i].Stock = 1
			produced++
		}
	}
	p.Pulses++
	p.Produced += produced
	p.Unrealized += p.Yield - produced
	return p, out, nil
}
func SocialFoodBasal(actor sim.EntityID, a SocialFoodActorState, elapsed int64) (SocialFoodActorState, error) {
	if !validSocialFoodActor(actor, a) || elapsed < 1 || elapsed > 168 || a.BasalSpent > 168-elapsed {
		return SocialFoodActorState{}, ErrSocialFoodContract
	}
	spent := elapsed
	if spent > a.Energy {
		spent = a.Energy
	}
	a.Energy -= spent
	a.BasalSpent += spent
	if elapsed >= 24-a.Hunger {
		a.Hunger = 24
	} else {
		a.Hunger += elapsed
	}
	if !validSocialFoodActor(actor, a) {
		return SocialFoodActorState{}, ErrSocialFoodContract
	}
	return a, nil
}
func SocialFoodGather(hour int, patch, slot, actor sim.EntityID, s SocialFoodSlotState, a SocialFoodActorState) (SocialFoodSlotState, SocialFoodActorState, error) {
	owner, err := SocialFoodActorPatchID(actor)
	idx := int(patch) - 3001
	if _, e := SocialFoodPhaseTime(hour, 0); e != nil || err != nil || owner != patch || idx < 0 || idx >= 2 || slot < sim.EntityID(4001+idx*8) || slot >= sim.EntityID(4009+idx*8) || s.Stock != 1 || s.Gathered < 0 || s.Gathered > int64(hour) || !validSocialFoodActor(actor, a) || a.Energy == 0 || a.Bag.Units != 0 || a.LastGatherHour >= int64(hour) {
		return SocialFoodSlotState{}, SocialFoodActorState{}, ErrSocialFoodContract
	}
	s.Stock = 0
	s.Gathered++
	a.Bag = SocialFoodBagState{Units: 1, Source: patch, Gatherer: actor}
	a.LastGatherHour = int64(hour)
	return s, a, nil
}
func SocialFoodConsume(actor sim.EntityID, a SocialFoodActorState) (SocialFoodActorState, error) {
	if !validSocialFoodActor(actor, a) || a.Energy == 0 || a.Bag.Units != 1 || a.Consumed >= 168 {
		return SocialFoodActorState{}, ErrSocialFoodContract
	}
	a.Bag = SocialFoodBagState{}
	a.Consumed++
	if a.Energy == 12 {
		a.CapLost++
	} else {
		a.Energy++
	}
	if a.Hunger > 0 {
		a.Hunger--
	}
	if !validSocialFoodActor(actor, a) {
		return SocialFoodActorState{}, ErrSocialFoodContract
	}
	return a, nil
}

// SocialFoodCheckWitnesses reconciles directed request outcomes with food
// transfer ledgers. It does not replace replaying events for consent evidence.
func SocialFoodCheckWitnesses(requests [16]SocialFoodRequestState, ledgers [16]SocialFoodLedgerState) error {
	var gifts, received, given, net int64
	for i := range requests {
		if !validSocialFoodRequest(sim.EntityID(i+1), requests[i]) || !validSocialFoodLedger(ledgers[i]) {
			return ErrSocialFoodContract
		}
		gifts += requests[i].Gifts
		received += ledgers[i].Received
		given += ledgers[i].Given
		net += ledgers[i].Net
	}
	if gifts != received || gifts != given || net != 0 {
		return ErrSocialFoodContract
	}
	return nil
}
func SocialFoodCheckConservation(patches [2]SocialFoodPatchState, slots [2][8]SocialFoodSlotState, actors [16]SocialFoodActorState) (SocialFoodBalance, error) {
	var b SocialFoodBalance
	b.InitialEnergy = 11 * 16
	for i, p := range patches {
		if p.Yield < 0 || p.Yield > 8 || p.Pulses < 0 || p.Pulses > 168 || p.Produced < 0 || p.Produced > 1344 || p.Unrealized < 0 || p.Unrealized > 1344 || p.Produced+p.Unrealized != p.Yield*p.Pulses {
			return SocialFoodBalance{}, ErrSocialFoodContract
		}
		var gathered, stock, consumed, held int64
		for _, s := range slots[i] {
			if s.Stock < 0 || s.Stock > 1 || s.Gathered < 0 || s.Gathered > p.Pulses || s.Stock+s.Gathered > p.Pulses {
				return SocialFoodBalance{}, ErrSocialFoodContract
			}
			gathered += s.Gathered
			stock += s.Stock
		}
		for j := i * 8; j < (i+1)*8; j++ {
			a := actors[j]
			if !validSocialFoodActor(sim.EntityID(j+1), a) {
				return SocialFoodBalance{}, ErrSocialFoodContract
			}
			consumed += a.Consumed
			held += a.Bag.Units
			b.Energy += a.Energy
			b.BasalSpent += a.BasalSpent
			b.CapLost += a.CapLost
		}
		if gathered+stock != p.Produced || gathered != consumed+held || p.Produced != consumed+held+stock {
			return SocialFoodBalance{}, ErrSocialFoodContract
		}
		b.Produced += p.Produced
		b.Unrealized += p.Unrealized
		b.Gathered += gathered
		b.Stock += stock
		b.Consumed += consumed
		b.Held += held
	}
	if b.InitialEnergy+b.Consumed != b.Energy+b.BasalSpent+b.CapLost {
		return SocialFoodBalance{}, ErrSocialFoodContract
	}
	return b, nil
}
