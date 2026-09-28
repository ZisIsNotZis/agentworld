package main

import (
	"agentworld/internal/world"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"syscall"
	"time"
)

func cpuTime() (time.Duration, error) {
	var usage syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &usage); err != nil {
		return 0, err
	}
	return time.Duration(usage.Utime.Sec+usage.Stime.Sec)*time.Second + time.Duration(usage.Utime.Usec+usage.Stime.Usec)*time.Microsecond, nil
}

func main() {
	actors := flag.Int("actors", 48, "number of persistent actors (1..256)")
	seed := flag.Uint64("seed", 7, "resource and energy seed")
	hours := flag.Int("hours", 12, "finite simulation horizon in hours (1..240)")
	workers := flag.Int("workers", 4, "scheduler workers")
	eatWeight := flag.Float64("eat-cost", 1, "actor hunger utility weight for eating (0..100)")
	resume := flag.String("resume", "", "restore this immutable checkpoint")
	checkpointPath := flag.String("checkpoint", "", "write a new immutable checkpoint at this path")
	checkpointHour := flag.Int("checkpoint-hour", 0, "stop after this hourly step and write checkpoint (0 runs through horizon)")
	branchWeight := flag.Float64("branch-eat-cost", math.NaN(), "fork from -resume with this future Eat weight; requires -checkpoint")
	flag.Parse()
	branch := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "branch-eat-cost" {
			branch = true
		}
	})
	opts := world.SurvivalOptions{Actors: *actors, Seed: *seed, Hours: *hours, Workers: *workers, EatWeight: *eatWeight}
	if *checkpointHour < 0 || *checkpointHour > *hours || (*checkpointHour != 0 && *checkpointPath == "") || (branch && (math.IsNaN(*branchWeight) || math.IsInf(*branchWeight, 0) || *resume == "" || *checkpointPath == "" || *checkpointHour != 0)) {
		fmt.Fprintln(os.Stderr, "invalid checkpoint flags")
		os.Exit(2)
	}
	var report world.SurvivalReport
	var err error
	if *resume == "" && *checkpointPath == "" {
		report, err = world.RunSurvival(context.Background(), opts, cpuTime)
	} else {
		var s *world.Survival
		if branch {
			s, _, err = world.BranchSurvivalCheckpoint(*resume, *checkpointPath, opts, *branchWeight)
		} else if *resume != "" {
			s, _, err = world.RestoreSurvivalCheckpoint(*resume, opts)
		} else {
			s, err = world.NewSurvival(opts)
		}
		if err == nil {
			for i := 0; *checkpointHour == 0 || i < *checkpointHour; i++ {
				var processed bool
				processed, err = s.Step(context.Background())
				if err != nil || !processed {
					break
				}
			}
		}
		if err == nil && *checkpointPath != "" && !branch {
			_, err = s.SaveCheckpoint(*checkpointPath)
		}
		if err == nil {
			report, err = s.Report(0, 0)
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if !report.ReplayOK {
		fmt.Fprintln(os.Stderr, "accepted-state replay mismatch")
		os.Exit(1)
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
