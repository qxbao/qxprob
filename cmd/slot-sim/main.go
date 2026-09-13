// Package main implements the CLI for running Monte Carlo simulations of slot engines.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/qxbao/qxprob/core/entropy"
	coreslot "github.com/qxbao/qxprob/core/slot"
	engineslot "github.com/qxbao/qxprob/engine/slot"
	"github.com/qxbao/qxprob/engine/slotgame/reference"
	"github.com/qxbao/qxprob/slotsim"
)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("slot-sim", flag.ContinueOnError)
	fs.SetOutput(stderr)

	gameFlag := fs.String("game", reference.GameID, "Game ID to simulate")
	versionFlag := fs.String("version", reference.ConfigVersion, "Game config version")
	spinsFlag := fs.Uint64("spins", 100_000, "Number of base spins to simulate")
	betFlag := fs.Int64("bet", 100, "Bet amount in minor units per spin")
	serverSeedFlag := fs.String("server-seed", "simulation-public-seed", "Server seed for deterministic entropy")
	clientSeedFlag := fs.String("client-seed", "simulation-client-seed", "Client seed for deterministic entropy")
	formatFlag := fs.String("format", "text", "Output format: text, json, or csv")
	outputFlag := fs.String("output", "", "Optional file path for output (default: stdout)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if *gameFlag != reference.GameID || *versionFlag != reference.ConfigVersion {
		return fmt.Errorf("unknown game %q or version %q", *gameFlag, *versionFlag)
	}
	if *spinsFlag == 0 {
		return fmt.Errorf("spins must be greater than zero")
	}
	if *betFlag <= 0 {
		return fmt.Errorf("bet must be greater than zero")
	}
	if *serverSeedFlag == "" {
		return fmt.Errorf("server-seed must not be empty")
	}
	if *clientSeedFlag == "" {
		return fmt.Errorf("client-seed must not be empty")
	}

	format := strings.ToLower(strings.TrimSpace(*formatFlag))
	switch format {
	case "text", "json", "csv":
	default:
		return fmt.Errorf("unsupported output format %q (allowed: text, json, csv)", *formatFlag)
	}

	def := reference.Definition()
	vdef, err := coreslot.ValidateDefinition(def)
	if err != nil {
		return fmt.Errorf("failed to validate game definition: %w", err)
	}
	features := reference.Features(vdef)

	game, err := engineslot.Compile(def, features, engineslot.Options{
		Mode: coreslot.PlayModeSimulation,
	})
	if err != nil {
		return fmt.Errorf("failed to compile game definition: %w", err)
	}

	serverSeed := *serverSeedFlag
	clientSeed := *clientSeedFlag
	src := entropy.NewProvablyFairSource(serverSeed, clientSeed, 1)

	runner := func(nonce uint64, req coreslot.SpinRequest) (coreslot.SpinResult, error) {
		src.ResetNonce(nonce)
		return game.Spin(req, src)
	}

	report, err := slotsim.RunWithRunner(context.Background(), slotsim.Options{
		Spins: *spinsFlag,
		Bet:   coreslot.Amount(*betFlag),
	}, runner)
	if err != nil {
		return fmt.Errorf("simulation failed: %w", err)
	}

	var w io.Writer = stdout
	if *outputFlag != "" {
		f, err := os.Create(*outputFlag)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer f.Close()
		w = f
	}

	switch format {
	case "json":
		return slotsim.WriteJSON(w, report)
	case "csv":
		return slotsim.WriteCSV(w, report)
	case "text":
		return slotsim.WriteText(w, report)
	}

	return nil
}
