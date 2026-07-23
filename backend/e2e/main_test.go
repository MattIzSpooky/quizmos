package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/cucumber/godog"
)

// TestFeatures boots one shared Postgres + Keycloak testcontainer pair and
// a real quizmos server, then runs every *.feature file in ../features
// against it. Each scenario gets a truncated database and a fresh World
// (see steps.go's Before/After hooks) but reuses the same containers and
// server, since spinning up Keycloak per-scenario would be far too slow.
//
// Requires a reachable Docker daemon (respects DOCKER_HOST, e.g. Docker
// Desktop's socket) — see the "test-e2e" Makefile target.
func TestFeatures(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	env, err := startEnvironment(ctx)
	if err != nil {
		t.Fatalf("start test environment: %v", err)
	}
	defer env.shutdown(context.Background())

	suite := godog.TestSuite{
		Name: "quizmos",
		ScenarioInitializer: func(sc *godog.ScenarioContext) {
			InitializeScenario(sc, env)
		},
		Options: &godog.Options{
			Format:   "pretty",
			Paths:    []string{"../features"},
			TestingT: t,
			// Fail loudly on undefined/pending/ambiguous steps instead of
			// silently skipping them and reporting a false-positive pass.
			Strict: true,
		},
	}

	if suite.Run() != 0 {
		t.Fatal("non-zero status returned, failed to run feature tests")
	}
}
