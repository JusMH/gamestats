package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gamestats/simulator/internal/simulation"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	os.Exit(run(ctx, os.Args[1:], os.Stdout, os.Stderr))
}
func run(ctx context.Context, args []string, out, errout io.Writer) int {
	flags := flag.NewFlagSet("simulator", flag.ContinueOnError)
	flags.SetOutput(errout)
	base := flags.String("go-url", "http://127.0.0.1:8080", "Go service base URL")
	id := flags.String("run-id", "demo", "deterministic ID prefix; use a new one for each run on the same Go process")
	mode := flags.String("mode", "demo", "demo (six concurrent matches) or negative (separate invalid cases)")
	verify := flags.Bool("verify", true, "verify independent expected analytics; false only observes")
	concurrency := flags.Int("concurrency", 6, "maximum active matches, 2..16; batches synchronize checkpoints")
	timeout := flags.Duration("timeout", 10*time.Second, "timeout per HTTP request, no retries")
	duration := flags.Duration("max-duration", 2*time.Minute, "maximum total run duration")
	reportPath := flags.String("report", "", "optional JSON evidence file; keeps the current report and two previous reports")
	flags.Usage = func() {
		fmt.Fprintln(errout, "Usage: simulator [options]\nRequires running Go and Scala services. No games, servers or profiles are started or modified.")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(errout, "unexpected positional arguments")
		return 2
	}
	if *duration <= 0 || *concurrency < 2 || *concurrency > 16 || (*mode != "demo" && *mode != "negative") || (*mode == "negative" && !*verify) {
		fmt.Fprintln(errout, "invalid duration, concurrency, mode or verification disabled for negative checks")
		return 2
	}
	scenarios, err := simulation.Scenarios(*id)
	if err != nil {
		fmt.Fprintln(errout, err)
		return 2
	}
	client, err := simulation.NewClient(*base, *timeout)
	if err != nil {
		fmt.Fprintln(errout, err)
		return 2
	}
	ctx, cancel := context.WithTimeout(ctx, *duration)
	defer cancel()
	log := func(message string) { fmt.Fprintln(out, message) }
	var evidence any
	if *mode == "negative" {
		var report simulation.NegativeReport
		report, err = simulation.CheckNegative(ctx, client, *id, log)
		evidence = report
	} else {
		runner := simulation.Runner{Client: client, Concurrency: *concurrency, Verify: *verify, Log: log}
		var report simulation.Report
		report, err = runner.Run(ctx, scenarios)
		evidence = report
		fmt.Fprintln(out, "\nSUMMARY (ordered by game/match)")
		for _, m := range report.Matches {
			fmt.Fprintf(out, "%s/%s accepted=%d queries=%d verified=%v error=%q\n", m.GameID, m.MatchID, m.Accepted, m.Queries, m.Verified, m.Error)
		}
		fmt.Fprintf(out, "Peak simultaneous HTTP requests: %d\n", report.PeakRequests)
	}
	if *reportPath != "" {
		if writeErr := writeReport(*reportPath, evidence); writeErr != nil {
			fmt.Fprintln(errout, "write report:", writeErr)
			return 1
		}
	}
	if err != nil {
		fmt.Fprintln(errout, "FAILED:", err)
		return 1
	}
	if !*verify {
		fmt.Fprintln(out, "COMPLETED: observed only; analytics were NOT verified")
	} else {
		fmt.Fprintln(out, "PASS: all selected scenario expectations verified")
	}
	return 0
}
