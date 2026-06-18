// Command mgctl is the operator CLI for the Migration Safety Engine.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/iamyadavvikas/migration-safety-engine/internal/plan"
)

func main() {
	if err := root().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func root() *cobra.Command {
	var addr string
	cmd := &cobra.Command{
		Use:           "mgctl",
		Short:         "Migration Safety Engine CLI",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.PersistentFlags().StringVar(&addr, "addr", "http://localhost:8080", "engine address")

	cmd.AddCommand(planCmd(&addr), statusCmd(&addr), watchCmd(&addr), driftScanCmd(&addr))
	return cmd
}

func planCmd(addr *string) *cobra.Command {
	cmd := &cobra.Command{Use: "plan", Short: "Manage migration plans"}
	var file string
	apply := &cobra.Command{
		Use:   "apply",
		Short: "Submit a migration plan",
		RunE: func(_ *cobra.Command, _ []string) error {
			p, err := plan.Parse(file)
			if err != nil {
				return err
			}
			body, _ := json.Marshal(p)
			resp, err := http.Post(*addr+"/plans", "application/json", bytes.NewReader(body))
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			out, _ := io.ReadAll(resp.Body)
			if resp.StatusCode >= 300 {
				return fmt.Errorf("engine: %s: %s", resp.Status, out)
			}
			fmt.Print(string(out))
			return nil
		},
	}
	apply.Flags().StringVarP(&file, "file", "f", "", "path to migration plan YAML")
	_ = apply.MarkFlagRequired("file")
	cmd.AddCommand(apply)
	return cmd
}

func statusCmd(addr *string) *cobra.Command {
	return &cobra.Command{
		Use:   "status <migration-id>",
		Short: "Show a migration's current state",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return printStatus(*addr, args[0])
		},
	}
}

func watchCmd(addr *string) *cobra.Command {
	return &cobra.Command{
		Use:   "watch <migration-id>",
		Short: "Poll a migration until it reaches a terminal state",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			for {
				state, terminal, err := fetchStatus(*addr, args[0])
				if err != nil {
					return err
				}
				fmt.Printf("%s  state=%s\n", time.Now().Format("15:04:05"), state)
				if terminal {
					return nil
				}
				time.Sleep(500 * time.Millisecond)
			}
		},
	}
}

// driftScanCmd posts a plan to the engine and prints how far the target table
// has drifted from what the plan's backfill would produce.
func driftScanCmd(addr *string) *cobra.Command {
	var file string
	cmd := &cobra.Command{
		Use:   "drift-scan",
		Short: "Report rows in the target table that diverge from the plan's backfill",
		RunE: func(_ *cobra.Command, _ []string) error {
			p, err := plan.Parse(file)
			if err != nil {
				return err
			}
			body, _ := json.Marshal(p)
			resp, err := http.Post(*addr+"/drift-scan", "application/json", bytes.NewReader(body))
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			out, _ := io.ReadAll(resp.Body)
			if resp.StatusCode >= 300 {
				return fmt.Errorf("engine: %s: %s", resp.Status, out)
			}
			var rep struct {
				Table   string  `json:"table"`
				Column  string  `json:"column"`
				Total   int64   `json:"total"`
				Nulls   int64   `json:"nulls"`
				Drifted int64   `json:"drifted"`
				Parity  float64 `json:"parity"`
			}
			if err := json.Unmarshal(out, &rep); err != nil {
				return err
			}
			fmt.Printf("table=%s column=%s total=%d nulls=%d drifted=%d parity=%.5f\n",
				rep.Table, rep.Column, rep.Total, rep.Nulls, rep.Drifted, rep.Parity)
			if rep.Drifted > 0 {
				return fmt.Errorf("drift detected: %d/%d rows diverge", rep.Drifted, rep.Total)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&file, "file", "f", "", "path to migration plan YAML")
	_ = cmd.MarkFlagRequired("file")
	return cmd
}

func fetchStatus(addr, id string) (string, bool, error) {
	resp, err := http.Get(addr + "/migrations/" + id)
	if err != nil {
		return "", false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		out, _ := io.ReadAll(resp.Body)
		return "", false, fmt.Errorf("engine: %s: %s", resp.Status, out)
	}
	var m struct {
		State    string `json:"state"`
		Terminal bool   `json:"terminal"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		return "", false, err
	}
	return m.State, m.Terminal, nil
}

func printStatus(addr, id string) error {
	state, terminal, err := fetchStatus(addr, id)
	if err != nil {
		return err
	}
	fmt.Printf("state=%s terminal=%v\n", state, terminal)
	return nil
}
