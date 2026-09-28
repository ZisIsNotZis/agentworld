package component

import (
	"agentworld/internal/sim"
	"fmt"
	"testing"
)

var (
	benchmarkView    View
	benchmarkBatch   Batch
	benchmarkMetrics MetricBatch
)

func benchmarkReader(b *testing.B) (Reader, Authority) {
	builder := NewRegistryBuilder()
	if err := builder.Register(EnergyDescriptor()); err != nil {
		b.Fatal(err)
	}
	if err := builder.Register(FatigueDescriptor()); err != nil {
		b.Fatal(err)
	}
	registry, err := builder.Freeze()
	if err != nil {
		b.Fatal(err)
	}
	seeds := make([]ComponentSeed, 0, 2000)
	for entity := 1; entity <= 1000; entity++ {
		energy, _ := sim.ScalarValue(float64(entity))
		fatigue, _ := sim.ScalarValue(float64(entity%1000) / 1000)
		seeds = append(seeds, ComponentSeed{Entity: sim.EntityID(entity), Component: EnergyTypeID, Fields: []FieldSeed{{Field: EnergyReserveField, Value: energy}}}, ComponentSeed{Entity: sim.EntityID(entity), Component: FatigueTypeID, Fields: []FieldSeed{{Field: FatigueLevelField, Value: fatigue}}})
	}
	reader, authority, err := NewReader(registry, 1, seeds)
	if err != nil {
		b.Fatal(err)
	}
	return reader, authority
}

func BenchmarkPointRead(b *testing.B) {
	reader, authority := benchmarkReader(b)
	for _, test := range []struct {
		name   string
		typeID sim.ComponentTypeID
	}{{"energy", EnergyTypeID}, {"fatigue", FatigueTypeID}} {
		b.Run(test.name, func(b *testing.B) {
			b.ReportAllocs()
			request := ReadRequest{Entity: 500, Component: test.typeID, WorldVersion: 1, Authority: authority}
			for i := 0; i < b.N; i++ {
				var err error
				benchmarkView, err = reader.Read(request)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
func BenchmarkScan1000(b *testing.B) {
	reader, authority := benchmarkReader(b)
	for _, test := range []struct {
		name   string
		typeID sim.ComponentTypeID
	}{{"energy", EnergyTypeID}, {"fatigue", FatigueTypeID}} {
		b.Run(test.name, func(b *testing.B) {
			b.ReportAllocs()
			request := ScanRequest{Component: test.typeID, WorldVersion: 1, Authority: authority}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				var err error
				benchmarkBatch, err = reader.Scan(request)
				if err != nil {
					b.Fatal(err)
				}
				if benchmarkBatch.Len() != 1000 {
					b.Fatal(fmt.Errorf("rows = %d", benchmarkBatch.Len()))
				}
			}
		})
	}
}
func BenchmarkProject1000(b *testing.B) {
	reader, authority := benchmarkReader(b)
	for _, test := range []struct {
		name       string
		typeID     sim.ComponentTypeID
		projection sim.ProjectionID
	}{{"energy", EnergyTypeID, EnergyProjection}, {"fatigue", FatigueTypeID, FatigueProjection}} {
		b.Run(test.name, func(b *testing.B) {
			b.ReportAllocs()
			request := ProjectRequest{Component: test.typeID, Projection: test.projection, WorldVersion: 1, Authority: authority}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				var err error
				benchmarkMetrics, err = reader.Project(request)
				if err != nil {
					b.Fatal(err)
				}
				if benchmarkMetrics.Len() != 1000 {
					b.Fatal(fmt.Errorf("metrics = %d", benchmarkMetrics.Len()))
				}
			}
		})
	}
}
