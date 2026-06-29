// Command engine runs the Migration Safety Engine control API and the
// state-machine runner. On startup it resumes any in-flight migrations.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	"github.com/iamyadavvikas/migration-safety-engine/internal/auth"
	"github.com/iamyadavvikas/migration-safety-engine/internal/chaos"
	"github.com/iamyadavvikas/migration-safety-engine/internal/logging"
	"github.com/iamyadavvikas/migration-safety-engine/internal/plan"
	"github.com/iamyadavvikas/migration-safety-engine/internal/ratelimit"
	"github.com/iamyadavvikas/migration-safety-engine/internal/statemachine"
	"github.com/iamyadavvikas/migration-safety-engine/internal/store"
	"github.com/iamyadavvikas/migration-safety-engine/internal/worker"
)

func main() {
	// Structured logging from env
	logCfg := logging.DefaultConfig()
	if lvl := os.Getenv("LOG_LEVEL"); lvl != "" {
		logCfg.Level = lvl
	}
	if fmt := os.Getenv("LOG_FORMAT"); fmt != "" {
		logCfg.Format = fmt
	}
	log := logging.NewLogger(logCfg)

	dsn := envOr("DB_DSN", "postgres://mse:mse@localhost:5499/mse?sslmode=disable")
	// TARGET_DSN is the application database the engine migrates. It defaults to the
	// control DSN so the demo runs on one Postgres; in production it is a separate DB.
	targetDSN := envOr("TARGET_DSN", dsn)
	// REPLICA_DSN is an optional read replica for SLO checks (reduces load on primary)
	replicaDSN := envOr("REPLICA_DSN", "")
	addr := envOr("ENGINE_ADDR", ":8080")
	jwtSecret := envOr("JWT_SECRET", "")
	jwtExpiry := 24 * time.Hour

	// Production auth guards
	if jwtSecret == "" {
		log.Warn("JWT_SECRET not set — tokens are invalidated on every restart. Set JWT_SECRET for production use.")
	}

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

	// Connect to read replica if configured
	var replica *pgxpool.Pool
	if replicaDSN != "" {
		replica, err = pgxpool.New(ctx, replicaDSN)
		if err != nil {
			log.Warn("failed to connect to replica, using primary for SLO checks", "err", err)
			replica = nil
		} else {
			log.Info("connected to read replica for SLO checks", "dsn", replicaDSN)
		}
	}

	runner := statemachine.NewRunner(st, target, log)
	authService := auth.NewAuth(jwtSecret, jwtExpiry)

	// Initialize worker pool for parallel migrations
	workerPool := worker.NewPool(runner, 4, log)
	workerPool.Start(ctx)

	// Rate limiter — 10 req/s per client, burst of 20
	limiter := ratelimit.NewLimiter(ratelimit.DefaultConfig())

	srv := &server{store: st, runner: runner, log: log, auth: authService, replica: replica, workerPool: workerPool}

	// Resume any in-flight migrations from their last persisted state.
	srv.resumeAll(ctx)

	mux := http.NewServeMux()

	// Public endpoints (no auth required)
	mux.HandleFunc("GET /healthz", srv.healthz)
	mux.HandleFunc("GET /status", srv.status)
	mux.Handle("GET /metrics", promhttp.Handler())
	mux.HandleFunc("POST /auth/login", srv.login)

	// Protected endpoints (auth required)
	mux.HandleFunc("POST /plans", srv.auth.RequireAuth(http.HandlerFunc(srv.applyPlan)).ServeHTTP)
	mux.HandleFunc("POST /drift-scan", srv.auth.RequireAuth(http.HandlerFunc(srv.driftScan)).ServeHTTP)
	mux.HandleFunc("GET /migrations", srv.auth.RequireAuth(http.HandlerFunc(srv.listMigrations)).ServeHTTP)
	mux.HandleFunc("GET /migrations/{id}", srv.auth.RequireAuth(http.HandlerFunc(srv.getMigration)).ServeHTTP)
	mux.HandleFunc("POST /reset-demo", srv.auth.RequireAuth(srv.auth.RequirePermission(auth.PermissionManageSettings, http.HandlerFunc(srv.resetDemo))).ServeHTTP)
	mux.HandleFunc("POST /migrations/{id}/abort", srv.auth.RequireAuth(srv.auth.RequirePermission(auth.PermissionWriteMigrations, http.HandlerFunc(srv.abortMigration))).ServeHTTP)
	mux.HandleFunc("DELETE /migrations", srv.auth.RequireAuth(srv.auth.RequirePermission(auth.PermissionDeleteMigrations, http.HandlerFunc(srv.deleteMigrations))).ServeHTTP)
	mux.HandleFunc("GET /schema/{table}", srv.auth.RequireAuth(http.HandlerFunc(srv.getSchema)).ServeHTTP)
	mux.HandleFunc("GET /migrations/{id}/safety", srv.auth.RequireAuth(srv.auth.RequirePermission(auth.PermissionReadSafety, http.HandlerFunc(srv.getSafetyMetrics))).ServeHTTP)
	mux.HandleFunc("GET /migrations/{id}/backfill", srv.auth.RequireAuth(srv.auth.RequirePermission(auth.PermissionReadSafety, http.HandlerFunc(srv.getBackfillProgress))).ServeHTTP)
	mux.HandleFunc("GET /migrations/{id}/canary", srv.auth.RequireAuth(srv.auth.RequirePermission(auth.PermissionReadSafety, http.HandlerFunc(srv.getCanaryObservations))).ServeHTTP)
	mux.HandleFunc("POST /migrations/{id}/services", srv.auth.RequireAuth(srv.auth.RequirePermission(auth.PermissionWriteMigrations, http.HandlerFunc(srv.registerService))).ServeHTTP)
	mux.HandleFunc("PUT /migrations/{id}/services/{service}", srv.auth.RequireAuth(srv.auth.RequirePermission(auth.PermissionWriteMigrations, http.HandlerFunc(srv.updateServiceCompat))).ServeHTTP)
	mux.HandleFunc("GET /migrations/{id}/services", srv.auth.RequireAuth(srv.auth.RequirePermission(auth.PermissionReadSafety, http.HandlerFunc(srv.getServices))).ServeHTTP)

	// Chaos testing endpoint
	mux.HandleFunc("POST /chaos/run", srv.auth.RequireAuth(srv.auth.RequirePermission(auth.PermissionManageSettings, http.HandlerFunc(srv.runChaosScenario))).ServeHTTP)
	mux.HandleFunc("GET /chaos/scenarios", srv.auth.RequireAuth(srv.auth.RequirePermission(auth.PermissionReadSafety, http.HandlerFunc(srv.listChaosScenarios))).ServeHTTP)

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

	// SPA middleware: intercept browser navigation requests (Accept: text/html)
	// so that page reloads on client-side routes serve index.html instead of 401.
	spaHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		accept := r.Header.Get("Accept")
		isBrowserNav := strings.Contains(accept, "text/html") && !strings.Contains(accept, "application/json")
		isAPIRequest := r.Header.Get("X-Requested-With") == "XMLHttpRequest" ||
			strings.Contains(accept, "application/json") ||
			r.Method == "POST" || r.Method == "PUT" || r.Method == "DELETE"

		if isBrowserNav && !isAPIRequest {
			// Check if path matches a static asset (has file extension)
			path := strings.TrimPrefix(r.URL.Path, "/")
			if path == "" || !strings.Contains(path, ".") {
				// SPA route — serve index.html
				http.ServeFileFS(w, r, distFS, "index.html")
				return
			}
		}

		mux.ServeHTTP(w, r)
	})

	// Apply rate limiting
	finalHandler := limiter.Middleware(spaHandler)

	httpSrv := &http.Server{Addr: addr, Handler: finalHandler, ReadHeaderTimeout: 5 * time.Second}

	go func() {
		log.Info("engine listening", "addr", addr)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("http server", "err", err)
			stop()
		}
	}()

	<-ctx.Done()
	log.Info("shutting down")
	srv.workerPool.Stop()
	shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(shutCtx)
	srv.wg.Wait()
	if srv.replica != nil {
		srv.replica.Close()
	}
}

type server struct {
	store      *store.Store
	runner     *statemachine.Runner
	log        *slog.Logger
	auth       *auth.Auth
	replica    *pgxpool.Pool
	workerPool *worker.Pool
	wg         sync.WaitGroup
}

// chaosStoreAdapter adapts pgxpool to the chaos.ChaosStore interface.
type chaosStoreAdapter struct {
	target *pgxpool.Pool
}

func (c *chaosStoreAdapter) CreateTestTable(ctx context.Context, name string, rows int) error {
	_, err := c.target.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id SERIAL PRIMARY KEY,
			data TEXT,
			status TEXT DEFAULT 'pending',
			updated_at TIMESTAMPTZ DEFAULT NOW()
		)
	`, name))
	if err != nil {
		return err
	}
	// Insert test rows
	for i := 0; i < rows; i += 1000 {
		batch := rows - i
		if batch > 1000 {
			batch = 1000
		}
		for j := 0; j < batch; j++ {
			_, err := c.target.Exec(ctx, fmt.Sprintf(
				`INSERT INTO %s (data) VALUES ($1)`, name),
				fmt.Sprintf("test_%d_%d", i, j))
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *chaosStoreAdapter) DropTestTable(ctx context.Context, name string) error {
	_, err := c.target.Exec(ctx, fmt.Sprintf(`DROP TABLE IF EXISTS %s`, name))
	return err
}

func (c *chaosStoreAdapter) GetRowCount(ctx context.Context, name string) (int64, error) {
	var count int64
	err := c.target.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s`, name)).Scan(&count)
	return count, err
}

func (c *chaosStoreAdapter) InjectNetworkPartition(_ context.Context, _ time.Duration) error {
	return nil // Simulated — no real partition injection in demo
}

func (c *chaosStoreAdapter) InjectReplicationLag(_ context.Context, _ int) error {
	return nil
}

func (c *chaosStoreAdapter) InjectLockTimeout(_ context.Context, _ int) error {
	return nil
}

func (c *chaosStoreAdapter) InjectConnectionExhaustion(_ context.Context, _ int) error {
	return nil
}

func (c *chaosStoreAdapter) ResetAll(_ context.Context) error {
	return nil
}

func (s *server) healthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

// status returns system health information.
func (s *server) status(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get migration stats
	stats := map[string]any{
		"status": "healthy",
		"time":   time.Now().UTC(),
	}

	// Count migrations by state
	migrations, err := s.store.List(ctx)
	if err == nil {
		stateCount := make(map[string]int)
		for _, m := range migrations {
			stateCount[string(m.State)]++
		}
		stats["migrations"] = map[string]any{
			"total":  len(migrations),
			"states": stateCount,
		}
	}

	// Get database stats
	var dbStats struct {
		ActiveConns int
		IdleConns   int
		MaxConns    int
	}
	err = s.store.Target().QueryRow(ctx, `
		SELECT 
			COUNT(*) FILTER (WHERE state = 'active'),
			COUNT(*) FILTER (WHERE state = 'idle'),
			(SELECT setting::int FROM pg_settings WHERE name = 'max_connections')
		FROM pg_stat_activity
	`).Scan(&dbStats.ActiveConns, &dbStats.IdleConns, &dbStats.MaxConns)
	if err == nil {
		stats["database"] = map[string]any{
			"active_connections": dbStats.ActiveConns,
			"idle_connections":   dbStats.IdleConns,
			"max_connections":    dbStats.MaxConns,
		}
	}

	// Get replication lag
	var lagMs int
	err = s.store.Target().QueryRow(ctx, `
		SELECT COALESCE(
			EXTRACT(EPOCH FROM (now() - pg_last_xact_replay_timestamp())) * 1000,
			0
		)::int
	`).Scan(&lagMs)
	if err == nil {
		stats["replication_lag_ms"] = lagMs
	}

	// Worker pool stats
	if s.workerPool != nil {
		poolStats := s.workerPool.Stats()
		stats["workers"] = map[string]any{
			"running": poolStats.Workers,
			"pending": poolStats.Pending,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(stats)
}

// login authenticates a user and returns a JWT token.
func (s *server) login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json: "+err.Error(), http.StatusBadRequest)
		return
	}

	token, role, err := s.auth.Authenticate(req.Username, req.Password)
	if err != nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"token": token,
		"role":  string(role),
	})
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

// getSafetyMetrics returns DDL execution logs for a migration.
func (s *server) getSafetyMetrics(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}

	logs, err := s.store.GetDDLLogs(r.Context(), id)
	if err != nil {
		http.Error(w, "get ddl logs: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"migration_id": id.String(),
		"ddl_logs":     logs,
	})
}

// getBackfillProgress returns backfill progress entries for a migration.
func (s *server) getBackfillProgress(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}

	progress, err := s.store.GetBackfillProgress(r.Context(), id)
	if err != nil {
		http.Error(w, "get backfill progress: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"migration_id": id.String(),
		"progress":     progress,
	})
}

// getCanaryObservations returns canary observations for a migration.
func (s *server) getCanaryObservations(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}

	observations, err := s.store.GetCanaryObservations(r.Context(), id)
	if err != nil {
		http.Error(w, "get canary observations: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"migration_id": id.String(),
		"observations": observations,
	})
}

// registerService registers a service as dependent on a migration.
func (s *server) registerService(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}

	var req struct {
		ServiceName   string `json:"service_name"`
		SchemaVersion int    `json:"schema_version"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json: "+err.Error(), http.StatusBadRequest)
		return
	}

	if req.ServiceName == "" {
		http.Error(w, "service_name required", http.StatusBadRequest)
		return
	}

	svcID, err := s.store.RegisterService(r.Context(), id, req.ServiceName, req.SchemaVersion)
	if err != nil {
		http.Error(w, "register service: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"service_id": svcID.String(),
	})
}

// updateServiceCompat updates a service's compatibility status.
func (s *server) updateServiceCompat(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}

	service := r.PathValue("service")
	if service == "" {
		http.Error(w, "service name required", http.StatusBadRequest)
		return
	}

	var req struct {
		Compatible bool `json:"compatible"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json: "+err.Error(), http.StatusBadRequest)
		return
	}

	if err := s.store.UpdateServiceCompat(r.Context(), id, service, req.Compatible); err != nil {
		http.Error(w, "update service compat: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// getServices returns all services registered for a migration.
func (s *server) getServices(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}

	services, err := s.store.GetServices(r.Context(), id)
	if err != nil {
		http.Error(w, "get services: "+err.Error(), http.StatusInternalServerError)
		return
	}

	ready, notReady, err := s.store.AllServicesReady(r.Context(), id)
	if err != nil {
		http.Error(w, "check services ready: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"migration_id": id.String(),
		"services":     services,
		"all_ready":    ready,
		"not_ready":    notReady,
	})
}

// drive runs a migration to completion in the background using the worker pool.
func (s *server) drive(id uuid.UUID) {
	s.workerPool.Submit(id)
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

// runChaosScenario runs a chaos engineering test scenario.
func (s *server) runChaosScenario(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Scenario string `json:"scenario"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.Scenario == "" {
		http.Error(w, "scenario name required", http.StatusBadRequest)
		return
	}

	// Get chaos engine from context — we'll use a simple approach
	adapter := &chaosStoreAdapter{target: s.store.Target()}
	ce := chaos.NewChaosEngine(adapter, s.log)

	result, err := ce.RunScenario(r.Context(), req.Scenario)
	if err != nil {
		http.Error(w, "run scenario: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"scenario":           result.Scenario,
		"start_time":         result.StartTime,
		"end_time":           result.EndTime,
		"duration":           result.Duration.String(),
		"final_row_count":    result.FinalRowCount,
		"verification_error": result.VerificationError,
	})
}

// listChaosScenarios returns available chaos test scenarios.
func (s *server) listChaosScenarios(w http.ResponseWriter, r *http.Request) {
	adapter := &chaosStoreAdapter{target: s.store.Target()}
	ce := chaos.NewChaosEngine(adapter, s.log)

	scenarios := ce.GetScenarioNames()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"scenarios": scenarios,
	})
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
