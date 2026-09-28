package sim

import (
	"errors"
	"math"
)

var (
	ErrInvalidID = errors.New("ID must be non-zero")
	ErrOverflow  = errors.New("numeric overflow")
)

type EntityID uint64
type EventID uint64
type ComponentTypeID uint32
type FieldID uint32
type ProjectionID uint32
type SchemaVersion uint32
type ProjectionVersion uint32
type RuleID uint32
type EnumID uint32
type StreamID uint64

func ValidateEntityID(id EntityID) error                  { return validateNonzero(uint64(id)) }
func ValidateEventID(id EventID) error                    { return validateNonzero(uint64(id)) }
func ValidateComponentTypeID(id ComponentTypeID) error    { return validateNonzero(uint64(id)) }
func ValidateFieldID(id FieldID) error                    { return validateNonzero(uint64(id)) }
func ValidateProjectionID(id ProjectionID) error          { return validateNonzero(uint64(id)) }
func ValidateSchemaVersion(v SchemaVersion) error         { return validateNonzero(uint64(v)) }
func ValidateProjectionVersion(v ProjectionVersion) error { return validateNonzero(uint64(v)) }
func ValidateRuleID(id RuleID) error                      { return validateNonzero(uint64(id)) }
func ValidateEnumID(id EnumID) error                      { return validateNonzero(uint64(id)) }

func NewEntityID(v uint64) (EntityID, error) {
	if err := validateNonzero(v); err != nil {
		return 0, err
	}
	return EntityID(v), nil
}
func NewEventID(v uint64) (EventID, error) {
	if err := validateNonzero(v); err != nil {
		return 0, err
	}
	return EventID(v), nil
}
func NewComponentTypeID(v uint32) (ComponentTypeID, error) {
	if err := validateNonzero(uint64(v)); err != nil {
		return 0, err
	}
	return ComponentTypeID(v), nil
}
func NewFieldID(v uint32) (FieldID, error) {
	if err := validateNonzero(uint64(v)); err != nil {
		return 0, err
	}
	return FieldID(v), nil
}
func NewProjectionID(v uint32) (ProjectionID, error) {
	if err := validateNonzero(uint64(v)); err != nil {
		return 0, err
	}
	return ProjectionID(v), nil
}
func NewSchemaVersion(v uint32) (SchemaVersion, error) {
	if err := validateNonzero(uint64(v)); err != nil {
		return 0, err
	}
	return SchemaVersion(v), nil
}
func NewProjectionVersion(v uint32) (ProjectionVersion, error) {
	if err := validateNonzero(uint64(v)); err != nil {
		return 0, err
	}
	return ProjectionVersion(v), nil
}
func NewRuleID(v uint32) (RuleID, error) {
	if err := validateNonzero(uint64(v)); err != nil {
		return 0, err
	}
	return RuleID(v), nil
}
func NewEnumID(v uint32) (EnumID, error) {
	if err := validateNonzero(uint64(v)); err != nil {
		return 0, err
	}
	return EnumID(v), nil
}

func validateNonzero(v uint64) error {
	if v == 0 {
		return ErrInvalidID
	}
	return nil
}

// EntityAllocator deterministically allocates monotonically increasing IDs.
// It is caller-owned and is not safe for concurrent use.
type EntityAllocator struct{ last EntityID }

func NewEntityAllocator(last EntityID) EntityAllocator { return EntityAllocator{last: last} }

func (a *EntityAllocator) Next() (EntityID, error) {
	if uint64(a.last) == math.MaxUint64 {
		return 0, ErrOverflow
	}
	a.last++
	return a.last, nil
}

func (a EntityAllocator) Last() EntityID { return a.last }
