package main

import (
	"agentworld/internal/world"
	"context"
	"encoding/json"
	"flag"
	"fmt"
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
	flag.Parse()
	report, err := world.RunSurvival(context.Background(), world.SurvivalOptions{Actors: *actors, Seed: *seed, Hours: *hours, Workers: *workers, EatWeight: *eatWeight}, cpuTime)
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
