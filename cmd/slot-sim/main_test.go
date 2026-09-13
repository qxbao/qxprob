package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLI_Validation(t *testing.T) {
	var stdout, stderr bytes.Buffer

	t.Run("unknown game", func(t *testing.T) {
		err := run([]string{"--game", "nonexistent"}, &stdout, &stderr)
		if err == nil || !strings.Contains(err.Error(), "unknown game") {
			t.Errorf("expected unknown game error, got %v", err)
		}
	})

	t.Run("zero spins", func(t *testing.T) {
		err := run([]string{"--spins", "0"}, &stdout, &stderr)
		if err == nil || !strings.Contains(err.Error(), "spins must be greater than zero") {
			t.Errorf("expected zero spins error, got %v", err)
		}
	})

	t.Run("invalid bet", func(t *testing.T) {
		err := run([]string{"--bet", "-5"}, &stdout, &stderr)
		if err == nil || !strings.Contains(err.Error(), "bet must be greater than zero") {
			t.Errorf("expected invalid bet error, got %v", err)
		}
	})

	t.Run("empty seeds", func(t *testing.T) {
		err := run([]string{"--server-seed", ""}, &stdout, &stderr)
		if err == nil || !strings.Contains(err.Error(), "server-seed must not be empty") {
			t.Errorf("expected empty server seed error, got %v", err)
		}
	})

	t.Run("invalid format", func(t *testing.T) {
		err := run([]string{"--format", "xml"}, &stdout, &stderr)
		if err == nil || !strings.Contains(err.Error(), "unsupported output format") {
			t.Errorf("expected unsupported format error, got %v", err)
		}
	})
}

func TestCLI_DeterministicSmallRun_Formats(t *testing.T) {
	args := []string{
		"--game", "reference-ways-slot",
		"--version", "1.0.0",
		"--spins", "20",
		"--bet", "100",
		"--server-seed", "test-server-seed",
		"--client-seed", "test-client-seed",
	}

	t.Run("text format", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		textArgs := append(args, "--format", "text")
		if err := run(textArgs, &stdout, &stderr); err != nil {
			t.Fatalf("text format run failed: %v", err)
		}
		if !strings.Contains(stdout.String(), "SLOT SIMULATION REPORT") {
			t.Errorf("missing text report header")
		}
	})

	t.Run("json format", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		jsonArgs := append(args, "--format", "json")
		if err := run(jsonArgs, &stdout, &stderr); err != nil {
			t.Fatalf("json format run failed: %v", err)
		}
		if !strings.Contains(stdout.String(), `"game_id": "reference-ways-slot"`) {
			t.Errorf("missing game_id in json output")
		}
	})

	t.Run("csv format", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		csvArgs := append(args, "--format", "csv")
		if err := run(csvArgs, &stdout, &stderr); err != nil {
			t.Fatalf("csv format run failed: %v", err)
		}
		if !strings.Contains(stdout.String(), "Total RTP (%)") {
			t.Errorf("missing csv header in output")
		}
	})
}

func TestCLI_FileOutput(t *testing.T) {
	tmpDir := t.TempDir()
	outFile := filepath.Join(tmpDir, "report.json")

	args := []string{
		"--game", "reference-ways-slot",
		"--version", "1.0.0",
		"--spins", "10",
		"--bet", "100",
		"--format", "json",
		"--output", outFile,
	}

	var stdout, stderr bytes.Buffer
	if err := run(args, &stdout, &stderr); err != nil {
		t.Fatalf("file output run failed: %v", err)
	}

	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}
	if !strings.Contains(string(data), `"total_spins": 10`) {
		t.Errorf("file content missing total_spins")
	}
}

func TestCLI_DeterministicPinnedReport(t *testing.T) {
	args := []string{
		"--game", "reference-ways-slot",
		"--version", "1.0.0",
		"--spins", "100",
		"--bet", "100",
		"--server-seed", "simulation-public-seed",
		"--client-seed", "reference-tuning",
		"--format", "json",
	}

	var stdout, stderr bytes.Buffer
	if err := run(args, &stdout, &stderr); err != nil {
		t.Fatalf("pinned run failed: %v", err)
	}

	var report struct {
		GameID        string  `json:"game_id"`
		Version       string  `json:"version"`
		TotalSpins    uint64  `json:"total_spins"`
		TotalBet      int64   `json:"total_bet"`
		TotalPayout   int64   `json:"total_payout"`
		BasePayout    int64   `json:"base_payout"`
		FeaturePayout int64   `json:"feature_payout"`
		TotalRTP      float64 `json:"total_rtp"`
		HitCount      uint64  `json:"hit_count"`
		FeatureCount  uint64  `json:"feature_count"`
		MaxMultiplier string  `json:"max_multiplier"`
		MaxPayout     int64   `json:"max_payout"`
	}

	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("unmarshal json report: %v", err)
	}

	if report.GameID != "reference-ways-slot" {
		t.Errorf("GameID = %q, want reference-ways-slot", report.GameID)
	}
	if report.TotalSpins != 100 {
		t.Errorf("TotalSpins = %d, want 100", report.TotalSpins)
	}
	if report.TotalBet != 10000 {
		t.Errorf("TotalBet = %d, want 10000", report.TotalBet)
	}
	if report.TotalPayout != 5654 {
		t.Errorf("TotalPayout = %d, want 5654", report.TotalPayout)
	}
	if report.BasePayout != 4458 {
		t.Errorf("BasePayout = %d, want 4458", report.BasePayout)
	}
	if report.FeaturePayout != 1194 {
		t.Errorf("FeaturePayout = %d, want 1194", report.FeaturePayout)
	}
	if report.HitCount != 92 {
		t.Errorf("HitCount = %d, want 92", report.HitCount)
	}
	if report.FeatureCount != 2 {
		t.Errorf("FeatureCount = %d, want 2", report.FeatureCount)
	}
	if report.MaxMultiplier != "11.7053" {
		t.Errorf("MaxMultiplier = %s, want 11.7053", report.MaxMultiplier)
	}
	if report.MaxPayout != 1170 {
		t.Errorf("MaxPayout = %d, want 1170", report.MaxPayout)
	}
}
