package cognition_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"reflect"
	"strings"
	"testing"

	"agentworld/internal/cognition"
	"agentworld/internal/strategy"
)

// The pins below were captured by running these encoders against the fixtures
// in this file on the pre-change revision (0a28a07) and then re-verified here;
// they are literal golden wire bytes, never fabricated. The natural/7
// fingerprint pin was captured from a detached worktree at 0a28a07 by
// evaluating strategy.FrozenNaturalPolicyFingerprint(strategy.FrozenNaturalPolicy7()).
const (
	goldenCallWire         = "7b22736571223a312c226b696e64223a2263616c6c222c22726f6c65223a226163746f722d6465636973696f6e222c22726571756573745f68617368223a2231633331353435623333633030316266616364373565386266613762386264663363646436303735646265636132303139323236333134626464326137653063222c22726571756573745f6279746573223a2265794a7a5932686c62574666646d567963326c76626949364d537769636d39735a534936496d466a644739794c57526c59326c7a61573975496977696332566862534936496d35686448567959577775593278686157306966513d3d222c22726573706f6e73655f626f6479223a2265794a7a5932686c62574666646d567963326c76626949364d53776959574e3061573975496a6f695a3246306147567949697769636d4630615739755957786c496a6f695a3239735a47567549697769593239755a6d6c6b5a57356a5a5349364d43343166513d3d222c22636f6e74656e74223a2265794a7a5932686c62574666646d567963326c76626949364d53776959574e3061573975496a6f695a3246306147567949697769636d4630615739755957786c496a6f695a3239735a47567549697769593239755a6d6c6b5a57356a5a5349364d43343166513d3d222c226d6f64656c5f6964223a227a61692f676c6d2d352e332d666c617368222c22656e64706f696e74223a22676174657761792e6578616d706c652f76312f636861742f636f6d706c6574696f6e73222c2263726564656e7469616c5f70726573656e74223a747275652c2273616d706c696e67223a7b2274656d7065726174757265223a302e372c22746f705f70223a312c226d61785f746f6b656e73223a3132382c2273656564223a6e756c6c7d2c227573616765223a7b2270726f6d70745f746f6b656e73223a34312c22636f6d706c6574696f6e5f746f6b656e73223a32377d2c22636f73745f6d6963726f73223a313233342c2270726963655f76657273696f6e223a227673312d70726963652f31222c22617474656d7074223a312c22737461747573223a226f6b222c22737461727465645f61745f756e69785f6d6963726f73223a313730303030303030303030303030302c226c6174656e63795f6d6963726f73223a3235303030302c22707265765f68617368223a2230303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030222c2268617368223a2234343461313066366638656633386663376331306331303961626666353339393033353838306462326535653966366639323630666137633730356362656637227d"
	goldenInterventionWire = "7b22736571223a312c226b696e64223a2270617463682d70726f706f7365222c226361757365223a7b2277616b655f706f6c696379223a22706572696f646963283629222c22686f7572223a32342c2274726967676572223a225433222c2265766964656e63655f646967657374223a2261626162616261626162616261626162616261626162616261626162616261626162616261626162616261626162616261626162616261626162616261626162227d2c2263616c6c5f72656673223a5b302c315d2c22636f7374223a7b2263616c6c73223a322c2270726f6d70745f746f6b656e73223a3130302c22636f6d706c6574696f6e5f746f6b656e73223a34302c22636f73745f6d6963726f73223a3138302c22656e67696e655f6d6963726f73223a393030307d2c227061746368223a7b22706172656e745f726566223a7b226964223a226e61747572616c222c2276657273696f6e223a377d2c226368696c645f726566223a7b226964223a226e61747572616c222c2276657273696f6e223a387d2c22706172656e745f66696e6765727072696e74223a2265653161663536636439333732306336326363656532326632653232343739393731323038636136306461386430383261386561613033613538343937306562222c226368696c645f66696e6765727072696e74223a2263646364636463646364636463646364636463646364636463646364636463646364636463646364636463646364636463646364636463646364636463646364222c2264656c746173223a5b7b22706172616d223a22636c61696d2e70726f766973696f6e5f656e657267795f666c6f6f72222c2276616c7565223a337d5d2c22696e74656e74223a226c6f776572207468652070726f766973696f6e20666c6f6f72222c2270617463685f646967657374223a2265666566656665666566656665666566656665666566656665666566656665666566656665666566656665666566656665666566656665666566656665666566222c2273636f7065223a2273747261746567792d7468726573686f6c6473222c2261666665637465645f6163746f72735f646967657374223a2231323132313231323132313231323132313231323132313231323132313231323132313231323132313231323132313231323132313231323132313231323132227d2c2276616c69646174696f6e223a7b2276657264696374223a2261646d6974222c22726561736f6e223a22696e2d646f6d61696e222c2273616e64626f785f6272616e63685f646967657374223a2233343334333433343334333433343334333433343334333433343334333433343334333433343334333433343334333433343334333433343334333433343334222c2277696e646f775f686f757273223a32347d2c22726573756c74223a7b22737461747573223a226170706c696564222c2277696e646f775f686f757273223a32342c22696e64696361746f725f64656c746173223a5b7b22696e64696361746f72223a226361706974616c5f7065616b222c2264656c7461223a327d5d7d2c22707265765f68617368223a2230303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030303030222c2268617368223a2232346537306161653535323937303936383337613831623231336638643437616364346161666330643733663161313566613133646333383236373565393237227d"
	// Captured pre-change (commit 0a28a07): natural/7 canonical wire and
	// fingerprint. The wire is AWNP, version 7, {natural,7}, budget 32/32.
	frozenNatural7Wire        = "41574e5000000007076e61747572616c000000072020"
	frozenNatural7Fingerprint = "ee1af56cd93720c62ccee22f2e22479971208ca60da8d082a8eaa03a584970eb"
)

func hexBytes(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatalf("bad hex %q: %v", s, err)
	}
	return b
}

func goldenCallRecord() cognition.CallRecord {
	request := []byte(`{"schema_version":1,"role":"actor-decision","seam":"natural.claim"}`)
	response := []byte(`{"schema_version":1,"action":"gather","rationale":"golden","confidence":0.5}`)
	return cognition.CallRecord{
		Seq:                 1,
		Kind:                "call",
		Role:                cognition.RoleActorDecision,
		RequestHash:         cognitionHash(request),
		RequestBytes:        request,
		ResponseBody:        response,
		Content:             response,
		ModelID:             cognition.DefaultModelID,
		Endpoint:            "gateway.example/v1/chat/completions",
		CredentialPresent:   true,
		Sampling:            cognition.FrozenSampling(),
		Usage:               cognition.Usage{PromptTokens: 41, CompletionTokens: 27},
		CostMicros:          1234,
		PriceVersion:        "vs1-price/1",
		Attempt:             1,
		Status:              cognition.CallStatusOK,
		StartedAtUnixMicros: 1700000000000000,
		LatencyMicros:       250000,
		PrevHash:            strings.Repeat("0", 64),
	}
}

func cognitionHash(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func sealCall(t *testing.T, r cognition.CallRecord) cognition.CallRecord {
	t.Helper()
	chain, err := cognition.CallRecordChainBytes(r)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(chain)
	r.Hash = hex.EncodeToString(sum[:])
	return r
}

func goldenInterventionRecord() cognition.InterventionRecord {
	fp := frozenNatural7Fingerprint
	return cognition.InterventionRecord{
		Seq:  1,
		Kind: cognition.InterventionPatchPropose,
		Cause: cognition.InterventionCause{
			WakePolicy:     "periodic(6)",
			Hour:           24,
			Trigger:        "T3",
			EvidenceDigest: strings.Repeat("ab", 32),
		},
		CallRefs: []uint64{0, 1},
		Cost: cognition.InterventionCost{
			Calls: 2, PromptTokens: 100, CompletionTokens: 40, CostMicros: 180, EngineMicros: 9000,
		},
		Patch: &cognition.PatchRecord{
			ParentRef:            cognition.NaturalRefRecord{ID: "natural", Version: 7},
			ChildRef:             cognition.NaturalRefRecord{ID: "natural", Version: 8},
			ParentFingerprint:    fp,
			ChildFingerprint:     strings.Repeat("cd", 32),
			Deltas:               []cognition.ParameterDelta{{Param: "claim.provision_energy_floor", Value: 3}},
			Intent:               "lower the provision floor",
			PatchDigest:          strings.Repeat("ef", 32),
			Scope:                "strategy-thresholds",
			AffectedActorsDigest: strings.Repeat("12", 32),
		},
		Validation: &cognition.ValidationRecord{
			Verdict: "admit", Reason: "in-domain", SandboxBranchDigest: strings.Repeat("34", 32), WindowHours: 24,
		},
		Result: cognition.InterventionResult{
			Status: "applied", WindowHours: 24,
			IndicatorDeltas: []cognition.IndicatorDelta{{Indicator: "capital_peak", Delta: 2}},
		},
		PrevHash: strings.Repeat("0", 64),
	}
}

func sealIntervention(t *testing.T, r cognition.InterventionRecord) cognition.InterventionRecord {
	t.Helper()
	chain, err := cognition.InterventionRecordChainBytes(r)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(chain)
	r.Hash = hex.EncodeToString(sum[:])
	return r
}

func TestCallRecordCanonicalRoundTripAndGolden(t *testing.T) {
	rec := sealCall(t, goldenCallRecord())
	wire, err := cognition.EncodeCallRecord(rec)
	if err != nil {
		t.Fatal(err)
	}
	if got := hex.EncodeToString(wire); got != goldenCallWire {
		t.Fatalf("call golden drift:\n got  %s\n want %s", got, goldenCallWire)
	}
	decoded, err := cognition.DecodeCallRecord(wire)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded, rec) {
		t.Fatalf("roundtrip mismatch:\n%+v\n%+v", decoded, rec)
	}
	// Determinism: a second encoding is byte-identical.
	again, err := cognition.EncodeCallRecord(rec)
	if err != nil || !bytes.Equal(wire, again) {
		t.Fatalf("non-deterministic: %v", err)
	}
}

func TestInterventionRecordCanonicalRoundTripAndGolden(t *testing.T) {
	rec := sealIntervention(t, goldenInterventionRecord())
	wire, err := cognition.EncodeInterventionRecord(rec)
	if err != nil {
		t.Fatal(err)
	}
	if got := hex.EncodeToString(wire); got != goldenInterventionWire {
		t.Fatalf("intervention golden drift:\n got  %s\n want %s", got, goldenInterventionWire)
	}
	decoded, err := cognition.DecodeInterventionRecord(wire)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Result.Status != rec.Result.Status || decoded.Patch.Deltas[0] != rec.Patch.Deltas[0] {
		t.Fatalf("roundtrip mismatch: %+v", decoded)
	}
}

// The golden intervention pins the pre-change natural/7 fingerprint, so this
// test both anchors the L1 codec to the pre-change revision and fails if the
// frozen strategy codec drifts.
func TestPreChangeNatural7FingerprintAnchor(t *testing.T) {
	wire, err := strategy.EncodeFrozenNaturalPolicy(strategy.FrozenNaturalPolicy7())
	if err != nil {
		t.Fatal(err)
	}
	if got := hex.EncodeToString(wire); got != frozenNatural7Wire {
		t.Fatalf("natural/7 wire drifted: %s", got)
	}
	fp, err := strategy.FrozenNaturalPolicyFingerprint(strategy.FrozenNaturalPolicy7())
	if err != nil {
		t.Fatal(err)
	}
	if got := hex.EncodeToString(fp[:]); got != frozenNatural7Fingerprint {
		t.Fatalf("natural/7 fingerprint drifted: %s", got)
	}
	if goldenInterventionRecord().Patch.ParentFingerprint != frozenNatural7Fingerprint {
		t.Fatalf("golden patch no longer pins the pre-change fingerprint")
	}
}

func TestCodecTamperRejects(t *testing.T) {
	call := sealCall(t, goldenCallRecord())
	callWire, err := cognition.EncodeCallRecord(call)
	if err != nil {
		t.Fatal(err)
	}
	intervention := sealIntervention(t, goldenInterventionRecord())
	interventionWire, err := cognition.EncodeInterventionRecord(intervention)
	if err != nil {
		t.Fatal(err)
	}
	for name, tc := range map[string]struct {
		wire   []byte
		decode func([]byte) error
	}{
		"call": {callWire, func(b []byte) error { _, err := cognition.DecodeCallRecord(b); return err }},
		"intervention": {interventionWire, func(b []byte) error {
			_, err := cognition.DecodeInterventionRecord(b)
			return err
		}},
	} {
		if err := tc.decode(tc.wire); err != nil {
			t.Fatalf("%s clean wire rejected: %v", name, err)
		}
		// Every single-byte mutation must be rejected.
		for i := range tc.wire {
			bad := append([]byte(nil), tc.wire...)
			bad[i] ^= 0xff
			if err := tc.decode(bad); err == nil {
				t.Fatalf("%s byte %d mutation accepted", name, i)
			}
		}
		for _, edit := range map[string]func([]byte) []byte{
			"empty":     func([]byte) []byte { return nil },
			"truncated": func(b []byte) []byte { return b[:len(b)-1] },
			"trailing":  func(b []byte) []byte { return append(append([]byte(nil), b...), ' ') },
			"pretty":    func(b []byte) []byte { return append(append([]byte(nil), b...), '\n') },
		} {
			if err := tc.decode(edit(tc.wire)); err == nil {
				t.Fatalf("%s %s edit accepted", name, "trailing/truncated")
			}
		}
	}
}

func TestCodecRejectsWrongKindAndUnknownFields(t *testing.T) {
	rec := sealCall(t, goldenCallRecord())
	// A hand-built wrong-kind wire must fail.
	wire := strings.Replace(string(mustWire(t, rec)), `"kind":"call"`, `"kind":"wrong"`, 1)
	if _, err := cognition.DecodeCallRecord([]byte(wire)); !errors.Is(err, cognition.ErrInvalidRecord) {
		t.Fatalf("wrong kind accepted: %v", err)
	}
	unknown := strings.Replace(string(mustWire(t, rec)), `"role":`, `"unknown_field":1,"role":`, 1)
	if _, err := cognition.DecodeCallRecord([]byte(unknown)); !errors.Is(err, cognition.ErrInvalidRecord) {
		t.Fatalf("unknown field accepted: %v", err)
	}
	badHash := strings.Replace(string(mustWire(t, rec)), rec.Hash, strings.Repeat("0", 64), 1)
	if _, err := cognition.DecodeCallRecord([]byte(badHash)); !errors.Is(err, cognition.ErrRecordHashMismatch) {
		t.Fatalf("hash drift accepted: %v", err)
	}
}

func mustWire(t *testing.T, rec cognition.CallRecord) []byte {
	t.Helper()
	// Re-seal because callers may have altered fields.
	chain, err := cognition.CallRecordChainBytes(rec)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(chain)
	rec.Hash = hex.EncodeToString(sum[:])
	wire, err := cognition.EncodeCallRecord(rec)
	if err != nil {
		t.Fatal(err)
	}
	return wire
}
