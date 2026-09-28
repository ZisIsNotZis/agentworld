package main

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestFoodFlowCLIReportAndCheckpointResume(t *testing.T) {
	original := options{yield: 3, seed: 7, workers: 1, hours: 8}
	full, err := run(context.Background(), original)
	if err != nil {
		t.Fatal(err)
	}
	for id, n := range full.FirstEightGather {
		if n != 3 || full.FirstEightClaims[id] != 8 {
			t.Fatalf("actor %d got %d gathers", id+1, n)
		}
	}
	if full.AcceptedLinksChecked == 0 || full.UnlinkedRejections == 0 || full.HistoryBytes == 0 || full.JournalBytes == 0 || full.CheckpointBytes == 0 || full.PeakRSSBytes == 0 || full.TokenCost != 0 {
		t.Fatalf("missing evidence: %+v", full)
	}
	if full.Traces[0].Hour != 0 || full.Traces[len(full.Traces)-1].Hour != 8 || len(full.Traces[len(full.Traces)-1].Actors[0].PriorHour) == 0 {
		t.Fatalf("missing trajectory: %+v", full.Traces)
	}
	path := filepath.Join(t.TempDir(), "pilot.bundle")
	prefix := original
	prefix.workers, prefix.checkpointHour, prefix.checkpointPath = 4, 4, path
	stopped, err := run(context.Background(), prefix)
	if err != nil || stopped.CompletedHour != 4 {
		t.Fatalf("checkpoint: %+v %v", stopped, err)
	}
	if info, err := os.Stat(path); err != nil || info.Size() != stopped.CheckpointBytes {
		t.Fatalf("checkpoint size: %v", err)
	}
	continued := original
	continued.resume = path
	resumed, err := run(context.Background(), continued)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(full.Traces, resumed.Traces) || full.CheckpointDigest != resumed.CheckpointDigest || full.Events != resumed.Events || full.Accepted != resumed.Accepted || full.Rejected != resumed.Rejected {
		t.Fatal("checkpoint continuation changed report evidence")
	}
	if _, err := run(context.Background(), prefix); err == nil {
		t.Fatal("overwrote immutable checkpoint")
	}
}

func TestFoodFlowCLIZeroExtinctionAndFlags(t *testing.T) {
	r, err := run(context.Background(), options{yield: 0, seed: 1, workers: 4, hours: 12})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(r.ExtinctionHours, []int{11}) || r.Accepted.Gather != 0 || r.Accepted.Eat != 0 || r.Traces[len(r.Traces)-1].Alive != 0 {
		t.Fatalf("wrong zero-flow extinction: %+v", r)
	}
	for _, o := range []options{{yield: 9, workers: 1, hours: 168}, {yield: 3, workers: 0, hours: 168}, {yield: 3, workers: 1, hours: 169}, {yield: 3, workers: 1, hours: 8, checkpointHour: 4}} {
		if _, err := run(context.Background(), o); err == nil {
			t.Fatalf("accepted invalid options %+v", o)
		}
	}
}
