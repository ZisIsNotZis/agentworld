package component

import (
	"agentworld/internal/sim"
	"sort"
)

func validateSeed(descriptor ComponentDescriptor, fields []FieldSeed) error {
	fieldByID := make(map[sim.FieldID]FieldDescriptor, len(descriptor.Fields))
	for _, field := range descriptor.Fields {
		fieldByID[field.ID] = field
	}
	seen := make(map[sim.FieldID]struct{}, len(fields))
	for _, seeded := range fields {
		if _, ok := seen[seeded.Field]; ok {
			return ErrInvalidRequest
		}
		seen[seeded.Field] = struct{}{}
		field, ok := fieldByID[seeded.Field]
		if !ok {
			return ErrUnknownField
		}
		if err := validateValue(field, seeded.Value); err != nil {
			return err
		}
	}
	for _, field := range descriptor.Fields {
		if _, ok := seen[field.ID]; ok {
			continue
		}
		missingAllowed := false
		for _, state := range field.PermittedStates {
			if state == sim.Missing {
				missingAllowed = true
				break
			}
		}
		if !missingAllowed {
			return ErrValueOutsideSchema
		}
	}
	return nil
}
func validateValue(field FieldDescriptor, value sim.Value) error {
	if value.Kind() != field.Type.Kind {
		return ErrValueOutsideSchema
	}
	allowed := false
	for _, state := range field.PermittedStates {
		if value.State() == state {
			allowed = true
			break
		}
	}
	if !allowed {
		return ErrValueOutsideSchema
	}
	if !value.IsPresent() {
		return nil
	}
	if err := validateNested(field.Type, value); err != nil {
		return err
	}
	if field.Bounds.HasMinimum || field.Bounds.HasMaximum {
		numeric, err := numericValue(value)
		if err != nil {
			return ErrValueOutsideSchema
		}
		if (field.Bounds.HasMinimum && numeric < field.Bounds.Minimum) || (field.Bounds.HasMaximum && numeric > field.Bounds.Maximum) {
			return ErrValueOutsideSchema
		}
	}
	return nil
}
func validateNested(valueType ValueType, value sim.Value) error {
	if value.Kind() != valueType.Kind {
		return ErrValueOutsideSchema
	}
	if !value.IsPresent() {
		return nil
	}
	switch valueType.Kind {
	case sim.OptionalKind:
		item, err := value.Optional()
		if err != nil {
			return err
		}
		return validateNested(*valueType.Element, item)
	case sim.ListKind:
		items, err := value.List()
		if err != nil {
			return err
		}
		for _, item := range items {
			if err := validateNested(*valueType.Element, item); err != nil {
				return err
			}
		}
	case sim.SetKind:
		items, err := value.Set()
		if err != nil {
			return err
		}
		for _, item := range items {
			if err := validateNested(*valueType.Element, item); err != nil {
				return err
			}
		}
	case sim.SparseMapKind:
		items, err := value.SparseMap()
		if err != nil {
			return err
		}
		for _, item := range items {
			if err := validateNested(*valueType.Key, item.Key); err != nil {
				return err
			}
			if err := validateNested(*valueType.Element, item.Value); err != nil {
				return err
			}
		}
	case sim.RecordKind:
		fields, err := value.Record()
		if err != nil {
			return err
		}
		if len(fields) != len(valueType.Fields) {
			return ErrValueOutsideSchema
		}
		types := append([]ValueFieldType(nil), valueType.Fields...)
		sort.Slice(types, func(i, j int) bool { return types[i].ID < types[j].ID })
		for i := range fields {
			if fields[i].ID != types[i].ID {
				return ErrValueOutsideSchema
			}
			if err := validateNested(types[i].Type, fields[i].Value); err != nil {
				return err
			}
		}
	}
	return nil
}
func findField(descriptor ComponentDescriptor, id sim.FieldID) (FieldDescriptor, error) {
	for _, field := range descriptor.Fields {
		if field.ID == id {
			return field, nil
		}
	}
	return FieldDescriptor{}, ErrUnknownField
}
func findProjection(descriptor ComponentDescriptor, id sim.ProjectionID) (ProjectionDescriptor, error) {
	for _, projection := range descriptor.Projections {
		if projection.ID == id {
			return projection, nil
		}
	}
	return ProjectionDescriptor{}, ErrUnknownProjection
}
