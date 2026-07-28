// Package e2e's step definitions are split across steps_*.go files, one
// per feature area (roughly one per *.feature file in ../features) —
// each contributing its own registerXSteps(sc) function. This file is
// just the entry point: per-scenario lifecycle hooks, plus the call to
// every registerXSteps.
package e2e

import (
	"context"
	"fmt"
	"time"

	"github.com/cucumber/godog"
)

type worldKeyType struct{}

var worldKey = worldKeyType{}

func worldFromContext(ctx context.Context) *World {
	w, ok := ctx.Value(worldKey).(*World)
	if !ok {
		panic("no World in context — step registered without the Before hook running?")
	}
	return w
}

const defaultWaitTimeout = 10 * time.Second

// InitializeScenario registers every step definition and the per-scenario
// lifecycle hooks (fresh World, clean database) against a shared,
// already-running environment.
func InitializeScenario(sc *godog.ScenarioContext, env *environment) {
	sc.Before(func(ctx context.Context, _ *godog.Scenario) (context.Context, error) {
		if err := env.truncateAll(ctx); err != nil {
			return ctx, fmt.Errorf("truncate tables before scenario: %w", err)
		}
		return context.WithValue(ctx, worldKey, newWorld(env)), nil
	})
	sc.After(func(ctx context.Context, _ *godog.Scenario, err error) (context.Context, error) {
		worldFromContext(ctx).closeAllSockets()
		return ctx, err
	})

	registerAdminAuthSteps(sc)
	registerAdminGameSocketSteps(sc)
	registerWebsocketConnectionSteps(sc)
	registerQuizAuthoringSteps(sc)
	registerQuestionMediaSteps(sc)
	registerGameLifecycleSteps(sc)
	registerPlayerColorSteps(sc)
	registerGameControlSteps(sc)
	registerQuestionNavigationSteps(sc)
	registerResetAnswersSteps(sc)
	registerLiveGameplaySteps(sc)
	registerFreeTextQuestionSteps(sc)
	registerPublicLookupSteps(sc)
}
