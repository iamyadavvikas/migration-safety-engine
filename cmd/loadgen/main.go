// Command loadgen generates concurrent write traffic against the target table so
// the engine's safety claim can be measured, not just asserted: run it WHILE a
// migration backfills and confirm production write latency stays within SLO. It
// updates a free-text column (dims), never the column the backfill derives from,
// so it cannot itself introduce drift.
//
//	go run ./cmd/loadgen -workers 16 -duration 20s
package main

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	var (
		dsn      = flag.String("dsn", envOr("TARGET_DSN", envOr("DB_DSN", "postgres://mse:mse@localhost:5499/mse?sslmode=disable")), "target database DSN")
		table    = flag.String("table", "catalog_product", "table to write to")
		workers  = flag.Int("workers", 16, "concurrent writers")
		duration = flag.Duration("duration", 20*time.Second, "how long to generate load")
	)
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), *duration)
	defer cancel()

	cfg, err := pgxpool.ParseConfig(*dsn)
	if err != nil {
		fail("parse dsn: %v", err)
	}
	cfg.MaxConns = int32(*workers) + 2
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		fail("connect: %v", err)
	}
	defer pool.Close()

	var maxID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf("SELECT coalesce(max(id), 0) FROM %s", *table)).Scan(&maxID); err != nil {
		fail("max id: %v", err)
	}
	if maxID == 0 {
		fail("table %s is empty; run `make migrate` first", *table)
	}

	updSQL := fmt.Sprintf("UPDATE %s SET dims = $2 WHERE id = $1", *table)

	var (
		wg      sync.WaitGroup
		errs    atomic.Int64
		mu      sync.Mutex
		samples = make([]time.Duration, 0, 1<<16)
	)

	fmt.Printf("loadgen: %d workers, %s, table=%s (max id=%d)\n", *workers, *duration, *table, maxID)
	start := time.Now()

	for w := 0; w < *workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rng := rand.New(rand.NewSource(time.Now().UnixNano() + int64(w)))
			local := make([]time.Duration, 0, 4096)
			for ctx.Err() == nil {
				id := rng.Int63n(maxID) + 1
				val := fmt.Sprintf("lx-%d", rng.Int63())
				t0 := time.Now()
				_, err := pool.Exec(ctx, updSQL, id, val)
				lat := time.Since(t0)
				if err != nil {
					if ctx.Err() != nil {
						break // shutting down
					}
					errs.Add(1)
					continue
				}
				local = append(local, lat)
			}
			mu.Lock()
			samples = append(samples, local...)
			mu.Unlock()
		}()
	}
	wg.Wait()
	elapsed := time.Since(start)

	report(samples, errs.Load(), elapsed)
}

func report(samples []time.Duration, errs int64, elapsed time.Duration) {
	n := len(samples)
	if n == 0 {
		fail("no successful writes (errors=%d)", errs)
	}
	sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })

	pct := func(p float64) time.Duration {
		idx := int(p / 100 * float64(n))
		if idx >= n {
			idx = n - 1
		}
		return samples[idx]
	}
	var total time.Duration
	for _, s := range samples {
		total += s
	}
	tput := float64(n) / elapsed.Seconds()

	fmt.Println("---------------- loadgen results ----------------")
	fmt.Printf("writes ok      : %d\n", n)
	fmt.Printf("errors         : %d\n", errs)
	fmt.Printf("elapsed        : %s\n", elapsed.Round(time.Millisecond))
	fmt.Printf("throughput     : %.0f writes/s\n", tput)
	fmt.Printf("latency mean   : %s\n", (total / time.Duration(n)).Round(10*time.Microsecond))
	fmt.Printf("latency p50    : %s\n", pct(50).Round(10*time.Microsecond))
	fmt.Printf("latency p95    : %s\n", pct(95).Round(10*time.Microsecond))
	fmt.Printf("latency p99    : %s\n", pct(99).Round(10*time.Microsecond))
	fmt.Printf("latency max    : %s\n", samples[n-1].Round(10*time.Microsecond))
	fmt.Println("-------------------------------------------------")
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "loadgen: "+format+"\n", args...)
	os.Exit(1)
}
