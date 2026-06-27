// Command engine runs the Migration Safety Engine control API and the
// state-machine runner. On startup it resumes any in-flight migrations.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/iamyadavvikas/migration-safety-engine/frontend"
	"github.com/iamyadavvikas/migration-safety-engine/internal/plan"
	"github.com/iamyadavvikas/migration-safety-engine/internal/statemachine"
	"github.com/iamyadavvikas/migration-safety-engine/internal/store"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	dsn := envOr("DB_DSN", "postgres://mse:mse@localhost:5499/mse?sslmode=disable")
	// TARGET_DSN is the application database the engine migrates. It defaults to the
	// control DSN so the demo runs on one Postgres; in production it is a separate DB.
	targetDSN := envOr("TARGET_DSN", dsn)
	addr := envOr("ENGINE_ADDR", ":8080")

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	st, err := store.New(ctx, dsn)
	if err != nil {
		log.Error("connect store", "err", err)
		os.Exit(1)
	}
	defer st.Close()

	target, err := pgxpool.New(ctx, targetDSN)
	if err != nil {
		log.Error("connect target", "err", err)
		os.Exit(1)
	}
	defer target.Close()

	runner := statemachine.NewRunner(st, target, log)
	srv := &server{store: st, runner: runner, log: log}

	// Resume any in-flight migrations from their last persisted state.
	srv.resumeAll(ctx)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", srv.healthz)
	mux.Handle("GET /metrics", promhttp.Handler())
	mux.HandleFunc("POST /plans", srv.applyPlan)
	mux.HandleFunc("POST /drift-scan", srv.driftScan)
	mux.HandleFunc("GET /migrations", srv.listMigrations)
	mux.HandleFunc("GET /migrations/{id}", srv.getMigration)

	distFS, err := fs.Sub(frontend.Assets, "dist")
	if err != nil {
		log.Error("subdist", "err", err)
		os.Exit(1)
	}
	fileServer := http.FileServer(http.FS(distFS))

	// Catch-all route to serve the React UI and support client-side SPA routing fallback.
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}

		f, err := distFS.Open(path)
		if err == nil {
			stat, err := f.Stat()
			if err == nil && !stat.IsDir() {
				f.Close()
				fileServer.ServeHTTP(w, r)
				return
			}
			if err == nil {
				f.Close()
			}
		}

		// Fallback to index.html for React Router paths
		http.ServeFileFS(w, r, distFS, "index.html")
	}))

	httpSrv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}

	go func() {
		log.Info("engine listening", "addr", addr)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("http server", "err", err)
			stop()
		}
	}()

	<-ctx.Done()
	log.Info("shutting down")
	shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(shutCtx)
	srv.wg.Wait()
}

type server struct {
	store  *store.Store
	runner *statemachine.Runner
	log    *slog.Logger
	wg     sync.WaitGroup
}

func (s *server) healthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

// applyPlan accepts a MigrationPlan as JSON, creates the migration, and starts
// driving it asynchronously.
func (s *server) applyPlan(w http.ResponseWriter, r *http.Request) {
	var p plan.MigrationPlan
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "bad json: "+err.Error(), http.StatusBadRequest)
		return
	}
	if err := p.Validate(); err != nil {
		http.Error(w, "invalid plan: "+err.Error(), http.StatusUnprocessableEntity)
		return
	}
	id, err := s.store.CreateMigration(r.Context(), &p, string(statemachine.StatePending))
	if err != nil {
		http.Error(w, "create: "+err.Error(), http.StatusInternalServerError)
		return
	}
	s.drive(id)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]string{"migration_id": id.String()})
}

// driftScan accepts a MigrationPlan as JSON and returns a read-only report of how
// far the target table has drifted from what the plan's backfill would produce.
func (s *server) driftScan(w http.ResponseWriter, r *http.Request) {
	var p plan.MigrationPlan
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "bad json: "+err.Error(), http.StatusBadRequest)
		return
	}
	if err := p.Validate(); err != nil {
		http.Error(w, "invalid plan: "+err.Error(), http.StatusUnprocessableEntity)
		return
	}
	rep, err := s.runner.DriftScan(r.Context(), &p)
	if err != nil {
		http.Error(w, "drift scan: "+err.Error(), http.StatusUnprocessableEntity)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(rep)
}

func (s *server) listMigrations(w http.ResponseWriter, r *http.Request) {
	summaries, err := s.store.List(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_ = json.NewEncoder(w).Encode(summaries)
}

func (s *server) getMigration(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	rec, err := s.store.Load(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"migration_id": rec.ID.String(),
		"plan_id":      rec.Plan.ID,
		"state":        rec.State,
		"terminal":     rec.Terminal,
		"updated_at":   rec.UpdatedAt,
	})
}

// drive runs a migration to completion in the background.
func (s *server) drive(id uuid.UUID) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		if err := s.runner.Run(context.Background(), id); err != nil {
			s.log.Error("run migration", "migration", id, "err", err)
		}
	}()
}

// resumeAll continues any non-terminal migrations found at startup.
func (s *server) resumeAll(ctx context.Context) {
	ids, err := s.store.FindResumable(ctx)
	if err != nil {
		s.log.Error("find resumable", "err", err)
		return
	}
	for _, id := range ids {
		s.log.Info("resuming migration", "migration", id)
		s.drive(id)
	}
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
