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
	mux.HandleFunc("POST /reset-demo", srv.resetDemo)
	mux.HandleFunc("POST /migrations/{id}/abort", srv.abortMigration)
	mux.HandleFunc("DELETE /migrations", srv.deleteMigrations)
	mux.HandleFunc("GET /schema/{table}", srv.getSchema)

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
	// If this is a browser navigation (no X-Requested-With header), serve the SPA
	if r.Header.Get("X-Requested-With") != "XMLHttpRequest" {
		// Check if it's a browser request (Accept: text/html)
		if strings.Contains(r.Header.Get("Accept"), "text/html") {
			distFS, err := fs.Sub(frontend.Assets, "dist")
			if err == nil {
				http.ServeFileFS(w, r, distFS, "index.html")
				return
			}
		}
	}

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
		"table":        rec.Plan.Table,
		"plan":         rec.Plan,
	})
}

// resetDemo resets the demo table to its pre-migration state.
func (s *server) resetDemo(w http.ResponseWriter, r *http.Request) {
	_, err := s.store.Target().Exec(r.Context(),
		"ALTER TABLE catalog_product ADD COLUMN IF NOT EXISTS legacy_shipping text; "+
			"ALTER TABLE catalog_product DROP COLUMN IF EXISTS shipping_class;")
	if err != nil {
		http.Error(w, "reset demo: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// abortMigration sets a migration to the RolledBack terminal state.
func (s *server) abortMigration(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	if err := s.store.UpdateState(r.Context(), id, string(statemachine.StateRolledBack), true); err != nil {
		http.Error(w, "abort: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// deleteMigrations removes multiple terminal migrations by their IDs.
func (s *server) deleteMigrations(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json: "+err.Error(), http.StatusBadRequest)
		return
	}
	if len(req.IDs) == 0 {
		http.Error(w, "no ids provided", http.StatusBadRequest)
		return
	}

	// Safety: only allow deleting terminal (Done or RolledBack) migrations.
	// Non-terminal migrations must be aborted first.
	uids := make([]uuid.UUID, 0, len(req.IDs))
	for _, raw := range req.IDs {
		id, err := uuid.Parse(raw)
		if err != nil {
			http.Error(w, "bad id: "+raw, http.StatusBadRequest)
			return
		}
		// Check that the migration is terminal before allowing delete.
		rec, err := s.store.Load(r.Context(), id)
		if errors.Is(err, store.ErrNotFound) {
			continue // skip already-deleted
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if !rec.Terminal {
			http.Error(w, "cannot delete non-terminal migration "+id.String()+": abort it first", http.StatusConflict)
			return
		}
		uids = append(uids, id)
	}

	if err := s.store.DeleteMany(r.Context(), uids); err != nil {
		http.Error(w, "delete: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// schemaColumn is a column returned by the schema introspection endpoint.
type schemaColumn struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Nullable bool   `json:"nullable"`
	Default  string `json:"default,omitempty"`
}

// getSchema returns the current column structure of a table in the target database.
func (s *server) getSchema(w http.ResponseWriter, r *http.Request) {
	table := r.PathValue("table")
	if table == "" {
		http.Error(w, "table parameter required", http.StatusBadRequest)
		return
	}

	rows, err := s.store.Target().Query(r.Context(),
		"SELECT column_name, data_type, is_nullable, COALESCE(column_default, '') "+
			"FROM information_schema.columns WHERE table_name = $1 ORDER BY ordinal_position", table)
	if err != nil {
		http.Error(w, "introspect: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var columns []schemaColumn
	for rows.Next() {
		var col schemaColumn
		var nullable string
		if err := rows.Scan(&col.Name, &col.Type, &nullable, &col.Default); err != nil {
			http.Error(w, "scan: "+err.Error(), http.StatusInternalServerError)
			return
		}
		col.Nullable = nullable == "YES"
		columns = append(columns, col)
	}
	if err := rows.Err(); err != nil {
		http.Error(w, "rows: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"table":   table,
		"columns": columns,
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
