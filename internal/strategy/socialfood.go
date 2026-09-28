package strategy

import (
	"agentworld/internal/sim"
	"errors"
	"sort"
	"sync"
)

// Social-food v2 has its own policy content and bindings. Choices are intentions:
// the world must check both actor eligibility and donor consent at admission.
const (
	SocialFoodPolicyFormatV2 uint32 = 2
	SocialFoodMaxActors             = 16
	SocialFoodMaxCandidates         = 16
	socialFoodHorizonHours          = 168
)

var (
	ErrInvalidSocialFoodPolicy      = errors.New("invalid social-food policy")
	ErrUnknownSocialFoodPolicy      = errors.New("unknown social-food policy reference")
	ErrInvalidSocialFoodBinding     = errors.New("invalid social-food actor binding")
	ErrInvalidSocialFoodObservation = errors.New("invalid social-food observation")
)

type SocialFoodRef struct {
	ID      string
	Version uint32
}

type SocialFoodTie struct {
	Target   sim.EntityID
	Affinity int64
}

type SocialFoodWeights struct {
	RequestEnergyCeiling int64
	ReportedUrgency      int64
	ReserveFloor         int64
	UrgencyBonus         int64
	LostMealPenalty      int64
	LowEnergyCeiling     int64
	LowEnergyPenalty     int64
	HighHungerFloor      int64
	HighHungerPenalty    int64
	AcceptThreshold      int64
}

type SocialFoodBudget struct{ Candidates, Evaluations int }

type SocialFoodPolicy struct {
	FormatVersion uint32
	Ref           SocialFoodRef
	Budget        SocialFoodBudget
	Weights       SocialFoodWeights
	Ties          [SocialFoodMaxActors]SocialFoodTie // outgoing directed known dyad per actor
}

// FrozenSocialFoodPolicy constructs the declared experiment, not a learned or
// genealogical affinity. The world must compare this exact policy at restore.
func FrozenSocialFoodPolicy(ref SocialFoodRef) SocialFoodPolicy {
	p := SocialFoodPolicy{FormatVersion: SocialFoodPolicyFormatV2, Ref: ref,
		Budget: SocialFoodBudget{SocialFoodMaxCandidates, SocialFoodMaxCandidates},
		Weights: SocialFoodWeights{RequestEnergyCeiling: 8, ReportedUrgency: 2, ReserveFloor: 4,
			UrgencyBonus: 2, LostMealPenalty: 1, LowEnergyCeiling: 6,
			LowEnergyPenalty: 1, HighHungerFloor: 10, HighHungerPenalty: 1, AcceptThreshold: 3}}
	for i := range p.Ties {
		actor := sim.EntityID(i + 1)
		target := actor + 1
		if actor == 8 || actor == 16 {
			target = actor - 7
		}
		affinity := int64(1)
		if actor == 4 || actor == 8 || actor == 12 || actor == 16 {
			affinity = 3
		}
		p.Ties[i] = SocialFoodTie{Target: target, Affinity: affinity}
	}
	return p
}

func validSocialFoodPolicy(p SocialFoodPolicy) bool {
	if p.FormatVersion != SocialFoodPolicyFormatV2 || p.Ref.ID == "" || len(p.Ref.ID) > MaxRefIDBytes || p.Ref.Version == 0 ||
		p.Budget.Candidates < 1 || p.Budget.Candidates > SocialFoodMaxCandidates ||
		p.Budget.Evaluations < 1 || p.Budget.Evaluations > SocialFoodMaxCandidates {
		return false
	}
	frozen := FrozenSocialFoodPolicy(p.Ref)
	return p.Weights == frozen.Weights && p.Ties == frozen.Ties
}

type SocialFoodBinding struct {
	Actor sim.EntityID
	Ref   SocialFoodRef
}

type SocialFoodRegistry struct {
	mu       sync.RWMutex
	policies map[SocialFoodRef]SocialFoodPolicy
}

func NewSocialFoodRegistry() *SocialFoodRegistry {
	return &SocialFoodRegistry{policies: make(map[SocialFoodRef]SocialFoodPolicy)}
}

func (r *SocialFoodRegistry) Register(policy SocialFoodPolicy) error {
	if r == nil || !validSocialFoodPolicy(policy) {
		return ErrInvalidSocialFoodPolicy
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.policies == nil {
		return ErrInvalidSocialFoodPolicy
	}
	if _, exists := r.policies[policy.Ref]; exists {
		return ErrInvalidSocialFoodPolicy
	}
	r.policies[policy.Ref] = policy // value-only; no caller-owned slices or maps
	return nil
}

func (r *SocialFoodRegistry) Bind(binding SocialFoodBinding) (*SocialFoodBound, error) {
	if r == nil || binding.Actor < 1 || binding.Actor > SocialFoodMaxActors ||
		binding.Ref.ID == "" || len(binding.Ref.ID) > MaxRefIDBytes || binding.Ref.Version == 0 {
		return nil, ErrInvalidSocialFoodBinding
	}
	r.mu.RLock()
	p, ok := r.policies[binding.Ref]
	r.mu.RUnlock()
	if !ok {
		return nil, ErrUnknownSocialFoodPolicy
	}
	return &SocialFoodBound{binding: binding, policy: p}, nil
}

type SocialFoodBound struct {
	binding SocialFoodBinding
	policy  SocialFoodPolicy
}

func (b *SocialFoodBound) Binding() SocialFoodBinding {
	if b == nil {
		return SocialFoodBinding{}
	}
	return b.binding
}

func (b *SocialFoodBound) Policy() SocialFoodPolicy {
	if b == nil {
		return SocialFoodPolicy{}
	}
	return b.policy
}

// RequestGuard is the requester's own world-visible state. A pending request
// from an earlier hour must be finalized before the next one is proposed.
type SocialFoodRequestStatus uint8

const (
	SocialFoodIdle SocialFoodRequestStatus = iota
	SocialFoodPending
	SocialFoodAccepted
	SocialFoodRefused
	SocialFoodExpired
	SocialFoodWithdrawn
)

type SocialFoodRequestGuard struct {
	Hour   int64 // -1 for idle
	Status SocialFoodRequestStatus
}

type SocialFoodRequestObservation struct {
	Actor, Patch           sim.EntityID
	Ref                    SocialFoodRef
	Hour                   int
	WorldVersion           sim.WorldVersion
	OwnEnergy, OwnBagUnits sim.Value
	DeniedGatherHour       int64 // -1 if never denied; own rejected Gather only
	OwnRequest             SocialFoodRequestGuard
	TestClaimOverride      *int64 // explicit test injection; never changes own eligibility or target
}

type SocialFoodAction uint8

const (
	SocialFoodWait SocialFoodAction = iota
	SocialFoodRequest
	SocialFoodAccept
	SocialFoodRefuse
)

type SocialFoodFallback uint8

const (
	SocialFoodNoFallback SocialFoodFallback = iota
	SocialFoodNoCandidate
	SocialFoodBudgetExhausted
	SocialFoodInvalidSelf
)

type SocialFoodChoice struct {
	Kind            SocialFoodAction
	Target          sim.EntityID // request addressee or reply requester
	RequestID       int64        // reply binds to exactly this pending request
	Claim           int64        // outgoing reported urgency, not private reserve
	Score           int64        // donor decision score, never authority to transfer
	ObservedVersion sim.WorldVersion
	Ref             SocialFoodRef
	Evaluated       int
	Fallback        SocialFoodFallback
}

func socialFoodOwnInteger(v sim.Value, min, max int64) (int64, bool) {
	if v.Kind() != sim.IntegerKind {
		return 0, false
	}
	n, err := v.Integer()
	return n, err == nil && n >= min && n <= max
}

func (b *SocialFoodBound) validObservation(actor sim.EntityID, ref SocialFoodRef, patch sim.EntityID, hour int) bool {
	if b == nil || actor != b.binding.Actor || ref != b.binding.Ref || hour < 0 || hour >= socialFoodHorizonHours {
		return false
	}
	return (actor <= 8 && patch == 3001) || (actor >= 9 && patch == 3002)
}

func (b *SocialFoodBound) EvaluateRequest(obs SocialFoodRequestObservation) (SocialFoodChoice, error) {
	if !b.validObservation(obs.Actor, obs.Ref, obs.Patch, obs.Hour) ||
		obs.DeniedGatherHour < -1 || obs.DeniedGatherHour > int64(obs.Hour) ||
		obs.OwnRequest.Status > SocialFoodWithdrawn || obs.OwnRequest.Hour < -1 || obs.OwnRequest.Hour > int64(obs.Hour) ||
		(obs.OwnRequest.Status == SocialFoodIdle && obs.OwnRequest.Hour != -1) ||
		(obs.OwnRequest.Status != SocialFoodIdle && obs.OwnRequest.Hour < 0) ||
		(obs.TestClaimOverride != nil && (*obs.TestClaimOverride < 0 || *obs.TestClaimOverride > 2)) {
		return SocialFoodChoice{}, ErrInvalidSocialFoodObservation
	}
	choice := SocialFoodChoice{Kind: SocialFoodWait, Ref: b.binding.Ref, ObservedVersion: obs.WorldVersion, Fallback: SocialFoodNoCandidate}
	energy, energyOK := socialFoodOwnInteger(obs.OwnEnergy, 0, 12)
	bag, bagOK := socialFoodOwnInteger(obs.OwnBagUnits, 0, 1)
	if !energyOK || !bagOK {
		choice.Fallback = SocialFoodInvalidSelf
		return choice, nil
	}
	if energy == 0 || energy > b.policy.Weights.RequestEnergyCeiling || bag != 0 ||
		obs.DeniedGatherHour != int64(obs.Hour) || obs.OwnRequest.Status == SocialFoodPending ||
		obs.OwnRequest.Hour == int64(obs.Hour) {
		return choice, nil
	}
	if b.policy.Budget.Candidates < 1 || b.policy.Budget.Evaluations < 1 {
		choice.Fallback = SocialFoodBudgetExhausted
		return choice, nil
	}
	choice.Kind = SocialFoodRequest
	choice.Target = b.policy.Ties[obs.Actor-1].Target
	choice.Claim = b.policy.Weights.ReportedUrgency
	if obs.TestClaimOverride != nil {
		choice.Claim = *obs.TestClaimOverride
	}
	choice.Evaluated, choice.Fallback = 1, SocialFoodNoFallback
	return choice, nil
}

// PendingView contains only the addressed request's public claim and identity.
// It intentionally has no requester energy, bag, history, or denial fields.
type SocialFoodPendingView struct {
	Requester, Addressee sim.EntityID
	RequestID            int64
	Hour                 int
	Claim                int64
}

type SocialFoodReplyObservation struct {
	Actor, Patch         sim.EntityID
	Ref                  SocialFoodRef
	Hour                 int
	WorldVersion         sim.WorldVersion
	OwnEnergy, OwnHunger sim.Value
	OwnBagUnits          sim.Value
	OwnLastReplyHour     int64 // typed world ledger guard; -1 until first reply
	Pending              []SocialFoodPendingView
}

func (b *SocialFoodBound) EvaluateReply(obs SocialFoodReplyObservation) (SocialFoodChoice, error) {
	if !b.validObservation(obs.Actor, obs.Ref, obs.Patch, obs.Hour) || obs.OwnLastReplyHour < -1 || obs.OwnLastReplyHour > int64(obs.Hour) || len(obs.Pending) > SocialFoodMaxCandidates {
		return SocialFoodChoice{}, ErrInvalidSocialFoodObservation
	}
	choice := SocialFoodChoice{Kind: SocialFoodWait, Ref: b.binding.Ref, ObservedVersion: obs.WorldVersion, Fallback: SocialFoodNoCandidate}
	energy, energyOK := socialFoodOwnInteger(obs.OwnEnergy, 0, 12)
	hunger, hungerOK := socialFoodOwnInteger(obs.OwnHunger, 0, 24)
	bag, bagOK := socialFoodOwnInteger(obs.OwnBagUnits, 0, 1)
	if !energyOK || !hungerOK || !bagOK {
		choice.Fallback = SocialFoodInvalidSelf
		return choice, nil
	}
	pending := append([]SocialFoodPendingView(nil), obs.Pending...)
	sort.Slice(pending, func(i, j int) bool { return pending[i].Requester < pending[j].Requester })
	for i, view := range pending {
		if view.Requester < 1 || view.Requester > SocialFoodMaxActors || view.Requester == obs.Actor ||
			view.Addressee != obs.Actor || view.Hour != obs.Hour || view.Claim < 0 || view.Claim > 2 ||
			view.RequestID != int64(obs.Hour)*SocialFoodMaxActors+int64(view.Requester) ||
			((view.Requester <= 8) != (obs.Patch == 3001)) ||
			b.policy.Ties[view.Requester-1].Target != obs.Actor ||
			(i > 0 && pending[i-1].Requester == view.Requester) {
			return SocialFoodChoice{}, ErrInvalidSocialFoodObservation
		}
	}
	if energy == 0 || obs.OwnLastReplyHour == int64(obs.Hour) || len(pending) == 0 {
		return choice, nil
	}
	if len(pending) > b.policy.Budget.Candidates || len(pending) > b.policy.Budget.Evaluations {
		choice.Fallback = SocialFoodBudgetExhausted
		return choice, nil
	}
	// One reply per hour. Equal scores tie on requester ID, not input order.
	for _, view := range pending {
		w := b.policy.Weights
		score := b.policy.Ties[view.Requester-1].Affinity - w.LostMealPenalty
		if view.Claim == w.ReportedUrgency {
			score += w.UrgencyBonus
		}
		if energy <= w.LowEnergyCeiling {
			score -= w.LowEnergyPenalty
		}
		if hunger >= w.HighHungerFloor {
			score -= w.HighHungerPenalty
		}
		if choice.Evaluated == 0 || score > choice.Score {
			choice.Target, choice.RequestID, choice.Score = view.Requester, view.RequestID, score
			choice.Kind = SocialFoodRefuse
			if energy >= w.ReserveFloor && bag == 1 && score >= w.AcceptThreshold {
				choice.Kind = SocialFoodAccept
			}
		}
		choice.Evaluated++
	}
	choice.Fallback = SocialFoodNoFallback
	return choice, nil
}
