package kernel

import (
	"agentworld/internal/component"
	"agentworld/internal/sim"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
)

var ErrEventIntegrity = errors.New("event integrity failure")

const maxEventBytes = 1 << 20

type Cause struct {
	Actor sim.EntityID // zero only for an authorized world source
	World bool
}

func (c Cause) valid() bool { return (c.World && c.Actor == 0) || (!c.World && c.Actor != 0) }

// Event kinds. A component-patch event replaces values in rows that already
// exist; an entity-create event additionally allocates complete newborn rows,
// carried as absent-before deltas ahead of the transition's patches.
const (
	KindComponentPatch = "component-patch"
	KindEntityCreate   = "entity-create"
)

type Event struct {
	ID                          sim.EventID
	Time                        sim.SimTime
	BeforeVersion, AfterVersion sim.WorldVersion
	Kind                        string
	Key                         string
	Cause                       Cause
	Rule                        sim.RuleID
	RuleVersion                 uint32
	Deltas                      []component.FieldDelta
	Metrics                     []component.MetricDelta
	PreviousHash, Hash          [32]byte
}

func cloneEvent(e Event) Event {
	e.Deltas = append([]component.FieldDelta(nil), e.Deltas...)
	e.Metrics = append([]component.MetricDelta(nil), e.Metrics...)
	return e
}

// Bytes is the canonical event body; the hash commits the prior hash and body.
func (e Event) Bytes() ([]byte, error) {
	var b bytes.Buffer
	put := func(v any) { _ = binary.Write(&b, binary.BigEndian, v) }
	text := func(s string) error {
		if len(s) > maxEventBytes-b.Len()-4 {
			return ErrEventIntegrity
		}
		put(uint32(len(s)))
		b.WriteString(s)
		return nil
	}
	value := func(v sim.Value) error {
		data, err := sim.EncodeValue(v)
		if err != nil {
			return err
		}
		if len(data) > maxEventBytes-b.Len()-4 {
			return ErrEventIntegrity
		}
		put(uint32(len(data)))
		b.Write(data)
		return nil
	}
	metric := func(m component.Metric) error {
		p := m.Provenance()
		put(uint64(p.Entity))
		put(uint32(p.Component))
		put(uint64(p.WorldVersion))
		put(uint32(p.SchemaVersion))
		put(uint32(m.ProjectionID()))
		put(uint32(m.ProjectionVersion()))
		if err := text(m.Unit()); err != nil {
			return err
		}
		put(uint32(len(p.SourceFields)))
		for _, id := range p.SourceFields {
			put(uint32(id))
		}
		if err := value(m.Value()); err != nil {
			return err
		}
		return value(m.Uncertainty())
	}
	put(uint64(e.ID))
	put(int64(e.Time))
	put(uint64(e.BeforeVersion))
	put(uint64(e.AfterVersion))
	if err := text(e.Kind); err != nil {
		return nil, err
	}
	if err := text(e.Key); err != nil {
		return nil, err
	}
	if e.Cause.World {
		b.WriteByte(1)
	} else {
		b.WriteByte(0)
	}
	put(uint64(e.Cause.Actor))
	put(uint32(e.Rule))
	put(e.RuleVersion)
	if len(e.Deltas) > 4096 || len(e.Metrics) > 4096 {
		return nil, ErrEventIntegrity
	}
	put(uint32(len(e.Deltas)))
	for _, d := range e.Deltas {
		put(uint64(d.Entity))
		put(uint32(d.Component))
		put(uint32(d.SchemaVersion))
		put(uint32(d.Field))
		if err := value(d.Before); err != nil {
			return nil, err
		}
		if err := value(d.After); err != nil {
			return nil, err
		}
	}
	put(uint32(len(e.Metrics)))
	for _, d := range e.Metrics {
		if err := metric(d.Before); err != nil {
			return nil, err
		}
		if err := metric(d.After); err != nil {
			return nil, err
		}
	}
	if b.Len() > maxEventBytes {
		return nil, ErrEventIntegrity
	}
	return b.Bytes(), nil
}
func hashEvent(e Event) ([32]byte, error) {
	data, err := e.Bytes()
	if err != nil {
		return [32]byte{}, err
	}
	var b bytes.Buffer
	b.Write(e.PreviousHash[:])
	b.Write(data)
	return sha256.Sum256(b.Bytes()), nil
}
