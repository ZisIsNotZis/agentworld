package main

import (
	"context"
	"testing"
)

func TestFoodFlowCLIFullHorizonFrozenGates(t *testing.T) {
	for _, q := range []int64{8, 3, 0} {
		t.Run(string(rune('0'+q)), func(t *testing.T) {
			r, err := run(context.Background(), options{yield: q, seed: 7, workers: 4, hours: 168})
			if err != nil {
				t.Fatal(err)
			}
			if r.CompletedHour != 168 || r.Traces[len(r.Traces)-1].Hour != 168 || r.TokenCost != 0 {
				t.Fatalf("wrong horizon: %+v", r)
			}
			switch q {
			case 8:
				if r.Traces[len(r.Traces)-1].Alive < 15 || r.Rejected.Capacity != 0 {
					t.Fatal("abundant gate failed")
				}
			case 3:
				if r.Rejected.Gather != 170 || r.Rejected.Capacity != 170 {
					t.Fatalf("scarce gather rejections: %+v", r.Rejected)
				}
				deathHunger := int64(-1)
				for _, point := range r.Traces {
					if point.Hour == 17 {
						deathHunger = point.Actors[2].Hunger // actor 16 stops at h17
					}
					if point.Hour == 72 && point.Alive > 8 {
						t.Fatal("scarce survival gate failed")
					}
					if point.Hour == 1 && (point.PriorRejection.Gather != 10 || point.PriorRejection.Capacity != 10 || point.PriorHour.Gather != 6) {
						t.Fatalf("scarce h0 attempts: accepted %+v rejected %+v", point.PriorHour, point.PriorRejection)
					}
				}
				if deathHunger != 11 || r.Traces[len(r.Traces)-1].Actors[2].Hunger != deathHunger {
					t.Fatal("scarce dead actor hunger changed after h17")
				}
				for i, successes := range r.FirstEightGather {
					if successes != 3 || r.FirstEightClaims[i] != 8 {
						t.Fatalf("scarce actor %d: %d/%d", i+1, successes, r.FirstEightClaims[i])
					}
				}
			case 0:
				if len(r.ExtinctionHours) != 1 || r.ExtinctionHours[0] != 11 || r.Accepted.Gather != 0 || r.Accepted.Eat != 0 {
					t.Fatal("zero gate failed")
				}
				for _, point := range r.Traces {
					if point.Hour >= 11 {
						for _, actor := range point.Actors {
							if actor.Energy != 0 || actor.Hunger != 11 {
								t.Fatalf("zero-flow dead actor changed after h11: h%d %+v", point.Hour, actor)
							}
						}
					}
				}
			}
		})
	}
}
