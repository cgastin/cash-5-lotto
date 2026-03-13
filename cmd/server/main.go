// Command server is the Lambda entry point for the Cash Five API.
// It wraps the HTTP router with the AWS Lambda adapter.
//
// For local development, set LOCAL_DEV=true to run as a plain HTTP server on :8080.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/cgastin/cash-five-lotto/internal/api"
	"github.com/cgastin/cash-five-lotto/internal/auth"
	"github.com/cgastin/cash-five-lotto/internal/ingestion"
	"github.com/cgastin/cash-five-lotto/internal/store"
)

const csvURL = "https://www.texaslottery.com/export/sites/lottery/Games/Cash_Five/Winning_Numbers/cashfive.csv"

func main() {
	deps := buildDeps()
	router := api.NewRouter(deps)

	if os.Getenv("LOCAL_DEV") == "true" {
		addr := ":8080"
		log.Printf("Starting local dev server on %s", addr)
		if err := http.ListenAndServe(addr, router); err != nil {
			log.Fatalf("server error: %v", err)
		}
		return
	}

	// Production: use Lambda adapter
	startLambda(router)
}

// buildDeps wires all dependencies from environment variables.
func buildDeps() api.Dependencies {
	// Storage backend: DynamoDB in production, local files for LOCAL_DEV.
	var drawRepo store.DrawRepository
	var syncRepo store.SyncStateRepository
	var userRepo store.UserRepository
	var subRepo store.SubscriptionRepository
	var predRepo store.PredictionRepository

	if os.Getenv("LOCAL_DEV") == "true" {
		storeDir := envOr("STORE_DIR", ".cash5-data")
		local, err := store.NewLocalDrawRepository(storeDir)
		if err != nil {
			log.Fatalf("open local draw repo: %v", err)
		}
		localSync, err := store.NewLocalSyncStateRepository(storeDir)
		if err != nil {
			log.Fatalf("open local sync repo: %v", err)
		}
		localPred, err := store.NewLocalPredictionRepository(storeDir)
		if err != nil {
			log.Fatalf("open local prediction repo: %v", err)
		}
		drawRepo = local
		syncRepo = localSync
		predRepo = localPred
		// Stub implementations for non-critical repos in local dev
		userRepo = &stubUserRepo{}
		subRepo = &stubSubRepo{}
	} else {
		// Production: DynamoDB repositories (implemented in store/dynamo.go — Phase 4+)
		log.Fatal("DynamoDB store not yet implemented; set LOCAL_DEV=true for development")
	}

	// Auth middleware
	var verifier auth.JWTVerifier
	if os.Getenv("LOCAL_DEV") == "true" {
		// Use mock verifier in dev — all requests with "Bearer dev-token" get admin claims
		verifier = &auth.MockVerifier{
			Tokens: map[string]*auth.Claims{
				"dev-token": {
					UserID:  "dev-user-001",
					Email:   "dev@example.com",
					Groups:  []string{"users"},
					IsAdmin: false,
				},
				"admin-token": {
					UserID:  "admin-001",
					Email:   "admin@example.com",
					Groups:  []string{"users", "admins"},
					IsAdmin: true,
				},
			},
		}
	} else {
		region := mustEnv("AWS_REGION")
		poolID := mustEnv("COGNITO_USER_POOL_ID")
		verifier = auth.NewCognitoVerifier(region, poolID)
	}

	middleware := &auth.Middleware{
		Verifier: verifier,
		SubRepo:  subRepo,
	}

	var syncFunc func() (int, error)
	if os.Getenv("LOCAL_DEV") == "true" {
		dr := drawRepo
		sr := syncRepo
		syncFunc = func() (int, error) {
			ctx := context.Background()
			data, err := ingestion.DownloadCSV(ctx, csvURL)
			if err != nil {
				return 0, fmt.Errorf("download: %w", err)
			}
			draws, _, err := ingestion.ParseCSV(data, csvURL)
			if err != nil {
				return 0, fmt.Errorf("parse: %w", err)
			}
			before, _ := dr.GetDrawCount(ctx)
			if err := dr.BatchUpsertDraws(ctx, draws); err != nil {
				return 0, fmt.Errorf("upsert: %w", err)
			}
			after, _ := dr.GetDrawCount(ctx)
			added := after - before
			latest, _ := dr.GetLatestDraw(ctx)
			state := store.SyncState{
				LastSuccessfulSync: time.Now().UTC(),
				TotalDrawsStored:   after,
				LastSyncStrategy:   "full",
			}
			if latest != nil {
				state.LatestDrawDate = latest.DrawDate
			}
			_ = sr.UpdateSyncState(ctx, state)
			return added, nil
		}
	}

	return api.Dependencies{
		DrawRepo:       drawRepo,
		SubRepo:        subRepo,
		UserRepo:       userRepo,
		PredRepo:       predRepo,
		SyncRepo:       syncRepo,
		AuthMiddleware: middleware,
		SyncFunc:       syncFunc,
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("required environment variable %s is not set", key)
	}
	return v
}

// startLambda starts the Lambda handler. This import is conditional on build tags
// to keep the CLI binary slim. In the actual build, aws-lambda-go is added.
func startLambda(handler http.Handler) {
	// Lambda handler using net/http adapter
	// Import: github.com/aws/aws-lambda-go/lambda
	// Import: github.com/awslabs/aws-lambda-go-api-proxy/httpadapter
	// These are added when the Lambda build tag is set.
	fmt.Fprintln(os.Stderr, "Lambda mode: compile with lambda build tag")
	os.Exit(1)
}

// Stub implementations for local development

type stubUserRepo struct{}

func (r *stubUserRepo) GetUser(_ context.Context, userID string) (*store.User, error) {
	return &store.User{UserID: userID, Email: userID + "@dev.local", EmailVerified: true}, nil
}
func (r *stubUserRepo) UpsertUser(_ context.Context, _ store.User) error { return nil }

type stubSubRepo struct{}

func (r *stubSubRepo) GetSubscription(_ context.Context, _ string) (*store.Subscription, error) {
	// Local dev: grant Pro to all users so all features are accessible.
	return &store.Subscription{
		Plan:       store.PlanPro,
		Status:     store.StatusActive,
		PlanSource: store.SourceAdminGranted,
	}, nil
}
func (r *stubSubRepo) UpsertSubscription(_ context.Context, _ store.Subscription) error {
	return nil
}

