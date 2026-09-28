package component

import "agentworld/internal/sim"

func evaluateProjection(descriptor ComponentDescriptor, projection ProjectionDescriptor, entity sim.EntityID, worldVersion sim.WorldVersion, fields []fieldValue) (Metric, error) {
	state := sim.Present
	value := projection.Intercept
	for i, source := range projection.SourceFields {
		var sourceValue sim.Value
		found := false
		for _, field := range fields {
			if field.id == source {
				sourceValue = field.value
				found = true
				break
			}
		}
		if !found {
			return Metric{}, ErrUnknownField
		}
		if !sourceValue.IsPresent() {
			if state == sim.Present || sourceValue.State() > state {
				state = sourceValue.State()
			}
			continue
		}
		numeric, err := numericValue(sourceValue)
		if err != nil {
			return Metric{}, err
		}
		value += projection.Coefficients[i] * numeric
	}
	var result sim.Value
	var err error
	if state == sim.Present {
		if (projection.Bounds.HasMinimum && value < projection.Bounds.Minimum) || (projection.Bounds.HasMaximum && value > projection.Bounds.Maximum) {
			return Metric{}, ErrValueOutsideSchema
		}
		result, err = sim.ScalarValue(value)
	} else {
		result, err = sim.AbsentValue(sim.ScalarKind, state)
	}
	if err != nil {
		return Metric{}, err
	}
	uncertainty, _ := sim.AbsentValue(sim.ScalarKind, sim.Unknown)
	return Metric{
		value: result, uncertainty: uncertainty, projection: projection.ID, projectionVersion: projection.Version, unit: projection.Unit,
		provenance: Provenance{Entity: entity, Component: descriptor.TypeID, SourceFields: append([]sim.FieldID(nil), projection.SourceFields...), WorldVersion: worldVersion, SchemaVersion: descriptor.SchemaVersion},
	}, nil
}

func numericValue(value sim.Value) (float64, error) {
	switch value.Kind() {
	case sim.ScalarKind, sim.ProbabilityKind:
		return value.Scalar()
	case sim.IntegerKind:
		integer, err := value.Integer()
		return float64(integer), err
	default:
		return 0, sim.ErrWrongKind
	}
}
