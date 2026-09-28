package component

import (
	"agentworld/internal/sim"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
)

var (
	ErrInvalidDescriptor    = errors.New("invalid component descriptor")
	ErrDuplicateDescriptor  = errors.New("duplicate component descriptor")
	ErrUnknownComponentType = errors.New("unknown component type")
	ErrUnknownField         = errors.New("unknown component field")
	ErrUnknownProjection    = errors.New("unknown component projection")
	ErrDependencyCycle      = errors.New("component dependency cycle")
	ErrValueOutsideSchema   = errors.New("value does not conform to field schema")
)

type RegistryBuilder struct {
	byID   map[sim.ComponentTypeID]ComponentDescriptor
	byName map[string]sim.ComponentTypeID
}

func NewRegistryBuilder() *RegistryBuilder {
	return &RegistryBuilder{byID: make(map[sim.ComponentTypeID]ComponentDescriptor), byName: make(map[string]sim.ComponentTypeID)}
}

func (b *RegistryBuilder) Register(descriptor ComponentDescriptor) error {
	if err := validateDescriptor(descriptor); err != nil {
		return err
	}
	if _, ok := b.byID[descriptor.TypeID]; ok {
		return fmt.Errorf("%w: type ID %d", ErrDuplicateDescriptor, descriptor.TypeID)
	}
	if _, ok := b.byName[descriptor.Name]; ok {
		return fmt.Errorf("%w: name %q", ErrDuplicateDescriptor, descriptor.Name)
	}
	copy := cloneDescriptor(descriptor)
	b.byID[copy.TypeID] = copy
	b.byName[copy.Name] = copy.TypeID
	return nil
}

func (b *RegistryBuilder) Freeze() (Registry, error) {
	if err := validateDependencies(b.byID); err != nil {
		return Registry{}, err
	}
	byID := make(map[sim.ComponentTypeID]ComponentDescriptor, len(b.byID))
	byName := make(map[string]sim.ComponentTypeID, len(b.byName))
	ids := make([]sim.ComponentTypeID, 0, len(b.byID))
	for id, descriptor := range b.byID {
		byID[id] = cloneDescriptor(descriptor)
		ids = append(ids, id)
	}
	for name, id := range b.byName {
		byName[name] = id
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return Registry{byID: byID, byName: byName, ids: ids}, nil
}

type Registry struct {
	byID   map[sim.ComponentTypeID]ComponentDescriptor
	byName map[string]sim.ComponentTypeID
	ids    []sim.ComponentTypeID
}

func (r Registry) Describe(id sim.ComponentTypeID) (ComponentDescriptor, error) {
	descriptor, ok := r.byID[id]
	if !ok {
		return ComponentDescriptor{}, ErrUnknownComponentType
	}
	return cloneDescriptor(descriptor), nil
}

func (r Registry) TypeIDs() []sim.ComponentTypeID {
	return append([]sim.ComponentTypeID(nil), r.ids...)
}

func validateDescriptor(d ComponentDescriptor) error {
	if sim.ValidateComponentTypeID(d.TypeID) != nil || sim.ValidateSchemaVersion(d.SchemaVersion) != nil || strings.TrimSpace(d.Name) == "" {
		return ErrInvalidDescriptor
	}
	if d.AccessPolicy != AccessAuthorizedReadProject || (d.StorageClass != BuiltinStorage && d.StorageClass != DynamicStorage) || d.MigrationPolicy == MigrationUnspecified {
		return ErrInvalidDescriptor
	}
	if d.ComplexityCost == 0 {
		return ErrInvalidDescriptor
	}
	if len(d.Fields) == 0 {
		return ErrInvalidDescriptor
	}
	fieldIDs := make(map[sim.FieldID]struct{}, len(d.Fields))
	fieldByID := make(map[sim.FieldID]FieldDescriptor, len(d.Fields))
	fieldNames := make(map[string]struct{}, len(d.Fields))
	for _, field := range d.Fields {
		if field.ID == 0 || strings.TrimSpace(field.Name) == "" || strings.TrimSpace(field.Unit) == "" || validateValueType(field.Type) != nil || validateBounds(field.Bounds) != nil {
			return ErrInvalidDescriptor
		}
		if field.Type.Kind == sim.ProbabilityKind && ((field.Bounds.HasMinimum && (field.Bounds.Minimum < 0 || field.Bounds.Minimum > 1)) || (field.Bounds.HasMaximum && (field.Bounds.Maximum < 0 || field.Bounds.Maximum > 1))) {
			return ErrInvalidDescriptor
		}
		if _, ok := fieldIDs[field.ID]; ok {
			return ErrDuplicateDescriptor
		}
		if _, ok := fieldNames[field.Name]; ok {
			return ErrDuplicateDescriptor
		}
		fieldIDs[field.ID] = struct{}{}
		fieldByID[field.ID] = field
		fieldNames[field.Name] = struct{}{}
		if err := validateStates(field.PermittedStates); err != nil {
			return err
		}
		if field.Uncertainty != UncertaintyForbidden && field.Uncertainty != UncertaintyPermitted {
			return ErrInvalidDescriptor
		}
	}
	if len(d.TransitionRules) == 0 || len(d.Projections) == 0 {
		return ErrInvalidDescriptor
	}
	ruleIDs := make(map[sim.RuleID]struct{})
	ruleNames := make(map[string]struct{})
	for _, rule := range d.TransitionRules {
		if rule.ID == 0 || rule.Version == 0 || strings.TrimSpace(rule.Name) == "" {
			return ErrInvalidDescriptor
		}
		if _, ok := ruleIDs[rule.ID]; ok {
			return ErrDuplicateDescriptor
		}
		if _, ok := ruleNames[rule.Name]; ok {
			return ErrDuplicateDescriptor
		}
		ruleIDs[rule.ID] = struct{}{}
		ruleNames[rule.Name] = struct{}{}
	}
	projectionIDs := make(map[sim.ProjectionID]struct{})
	projectionNames := make(map[string]struct{})
	for _, projection := range d.Projections {
		if projection.ID == 0 || projection.Version == 0 || strings.TrimSpace(projection.Name) == "" || strings.TrimSpace(projection.Unit) == "" || len(projection.SourceFields) == 0 || len(projection.SourceFields) != len(projection.Coefficients) || projection.MissingBehavior != PreserveSourceState || validateBounds(projection.Bounds) != nil || !finite(projection.Intercept) {
			return ErrInvalidDescriptor
		}
		if _, ok := projectionIDs[projection.ID]; ok {
			return ErrDuplicateDescriptor
		}
		if _, ok := projectionNames[projection.Name]; ok {
			return ErrDuplicateDescriptor
		}
		projectionIDs[projection.ID] = struct{}{}
		projectionNames[projection.Name] = struct{}{}
		sources := make(map[sim.FieldID]struct{})
		for i, fieldID := range projection.SourceFields {
			field, ok := fieldByID[fieldID]
			if !ok {
				return ErrUnknownField
			}
			if (field.Type.Kind != sim.IntegerKind && field.Type.Kind != sim.ScalarKind && field.Type.Kind != sim.ProbabilityKind) || !finite(projection.Coefficients[i]) {
				return ErrInvalidDescriptor
			}
			if _, ok := sources[fieldID]; ok {
				return ErrDuplicateDescriptor
			}
			sources[fieldID] = struct{}{}
		}
	}
	dependencyIDs := make(map[sim.ComponentTypeID]struct{})
	for _, dependency := range d.Dependencies {
		if dependency == 0 || dependency == d.TypeID {
			return ErrDependencyCycle
		}
		if _, ok := dependencyIDs[dependency]; ok {
			return ErrDuplicateDescriptor
		}
		dependencyIDs[dependency] = struct{}{}
	}
	if d.MigrationPolicy == MigrationRejectUnlisted && len(d.MigrationPaths) != 0 {
		return ErrInvalidDescriptor
	}
	if d.MigrationPolicy == MigrationExplicitPaths && len(d.MigrationPaths) == 0 {
		return ErrInvalidDescriptor
	}
	paths := make(map[[2]sim.SchemaVersion]struct{})
	for _, path := range d.MigrationPaths {
		if path.From == 0 || path.To == 0 || path.From >= path.To || path.To != d.SchemaVersion {
			return ErrInvalidDescriptor
		}
		key := [2]sim.SchemaVersion{path.From, path.To}
		if _, ok := paths[key]; ok {
			return ErrDuplicateDescriptor
		}
		paths[key] = struct{}{}
	}
	return nil
}

func validateValueType(valueType ValueType) error {
	if valueType.Kind < sim.BoolKind || valueType.Kind > sim.SparseMapKind {
		return ErrInvalidDescriptor
	}
	switch valueType.Kind {
	case sim.OptionalKind, sim.ListKind, sim.SetKind:
		if valueType.Element == nil || validateValueType(*valueType.Element) != nil {
			return ErrInvalidDescriptor
		}
	case sim.SparseMapKind:
		if valueType.Key == nil || valueType.Element == nil || validateValueType(*valueType.Key) != nil || validateValueType(*valueType.Element) != nil {
			return ErrInvalidDescriptor
		}
	case sim.RecordKind:
		if len(valueType.Fields) == 0 {
			return ErrInvalidDescriptor
		}
		fieldIDs := make(map[sim.FieldID]struct{})
		fieldNames := make(map[string]struct{})
		for _, field := range valueType.Fields {
			if field.ID == 0 || strings.TrimSpace(field.Name) == "" || validateValueType(field.Type) != nil {
				return ErrInvalidDescriptor
			}
			if _, ok := fieldIDs[field.ID]; ok {
				return ErrDuplicateDescriptor
			}
			if _, ok := fieldNames[field.Name]; ok {
				return ErrDuplicateDescriptor
			}
			fieldIDs[field.ID] = struct{}{}
			fieldNames[field.Name] = struct{}{}
		}
	default:
		if valueType.Element != nil || valueType.Key != nil || len(valueType.Fields) != 0 {
			return ErrInvalidDescriptor
		}
	}
	return nil
}

func validateBounds(bounds Bounds) error {
	if (bounds.HasMinimum && !finite(bounds.Minimum)) || (bounds.HasMaximum && !finite(bounds.Maximum)) || (bounds.HasMinimum && bounds.HasMaximum && bounds.Minimum > bounds.Maximum) {
		return ErrInvalidDescriptor
	}
	return nil
}
func validateStates(states []sim.ValueState) error {
	if len(states) == 0 {
		return ErrInvalidDescriptor
	}
	seen := make(map[sim.ValueState]struct{})
	for _, state := range states {
		if state > sim.Inapplicable {
			return ErrInvalidDescriptor
		}
		if _, ok := seen[state]; ok {
			return ErrDuplicateDescriptor
		}
		seen[state] = struct{}{}
	}
	return nil
}
func validateDependencies(descriptors map[sim.ComponentTypeID]ComponentDescriptor) error {
	const (
		visiting = 1
		visited  = 2
	)
	state := make(map[sim.ComponentTypeID]uint8)
	var visit func(sim.ComponentTypeID) error
	visit = func(id sim.ComponentTypeID) error {
		if state[id] == visiting {
			return ErrDependencyCycle
		}
		if state[id] == visited {
			return nil
		}
		state[id] = visiting
		for _, dependency := range descriptors[id].Dependencies {
			if _, ok := descriptors[dependency]; !ok {
				return ErrUnknownComponentType
			}
			if err := visit(dependency); err != nil {
				return err
			}
		}
		state[id] = visited
		return nil
	}
	for id := range descriptors {
		if err := visit(id); err != nil {
			return err
		}
	}
	return nil
}

func cloneDescriptor(d ComponentDescriptor) ComponentDescriptor {
	d.Fields = append([]FieldDescriptor(nil), d.Fields...)
	for i := range d.Fields {
		d.Fields[i].Type = cloneValueType(d.Fields[i].Type)
		d.Fields[i].PermittedStates = append([]sim.ValueState(nil), d.Fields[i].PermittedStates...)
	}
	d.TransitionRules = append([]TransitionRuleDescriptor(nil), d.TransitionRules...)
	d.Projections = append([]ProjectionDescriptor(nil), d.Projections...)
	for i := range d.Projections {
		d.Projections[i].SourceFields = append([]sim.FieldID(nil), d.Projections[i].SourceFields...)
		d.Projections[i].Coefficients = append([]float64(nil), d.Projections[i].Coefficients...)
	}
	d.Dependencies = append([]sim.ComponentTypeID(nil), d.Dependencies...)
	d.MigrationPaths = append([]MigrationPath(nil), d.MigrationPaths...)
	return d
}
func cloneValueType(t ValueType) ValueType {
	if t.Element != nil {
		value := cloneValueType(*t.Element)
		t.Element = &value
	}
	if t.Key != nil {
		value := cloneValueType(*t.Key)
		t.Key = &value
	}
	t.Fields = append([]ValueFieldType(nil), t.Fields...)
	for i := range t.Fields {
		t.Fields[i].Type = cloneValueType(t.Fields[i].Type)
	}
	return t
}
func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
