package slotsim

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
)

// WriteJSON writes the simulation Report formatted as indented JSON.
func WriteJSON(w io.Writer, r Report) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

// WriteCSV writes the simulation Report and distribution buckets in CSV format.
func WriteCSV(w io.Writer, r Report) error {
	cw := csv.NewWriter(w)

	records := [][]string{
		{"Metric", "Value"},
		{"Game ID", r.GameID},
		{"Version", r.Version},
		{"Total Spins", strconv.FormatUint(r.TotalSpins, 10)},
		{"Bet Per Spin", strconv.FormatInt(int64(r.BetPerSpin), 10)},
		{"Total Bet", strconv.FormatInt(int64(r.TotalBet), 10)},
		{"Total Payout", strconv.FormatInt(int64(r.TotalPayout), 10)},
		{"Base Payout", strconv.FormatInt(int64(r.BasePayout), 10)},
		{"Feature Payout", strconv.FormatInt(int64(r.FeaturePayout), 10)},
		{"Total RTP (%)", fmt.Sprintf("%.4f", r.TotalRTP*100)},
		{"Base RTP (%)", fmt.Sprintf("%.4f", r.BaseRTP*100)},
		{"Feature RTP (%)", fmt.Sprintf("%.4f", r.FeatureRTP*100)},
		{"Hit Count", strconv.FormatUint(r.HitCount, 10)},
		{"Hit Frequency (%)", fmt.Sprintf("%.4f", r.HitFrequency*100)},
		{"Feature Count", strconv.FormatUint(r.FeatureCount, 10)},
		{"Feature Frequency (%)", fmt.Sprintf("%.4f", r.FeatureFrequency*100)},
		{"Average Win", fmt.Sprintf("%.4f", r.AverageWin)},
		{"Average Feature Win", fmt.Sprintf("%.4f", r.AverageFeatureWin)},
		{"Max Multiplier", r.MaxMultiplier.String()},
		{"Max Payout", strconv.FormatInt(int64(r.MaxPayout), 10)},
		{"Variance", fmt.Sprintf("%.6f", r.Variance)},
		{"Standard Deviation", fmt.Sprintf("%.6f", r.StandardDeviation)},
		{"Duration (s)", fmt.Sprintf("%.3f", r.DurationSeconds)},
		{},
		{"Bucket Label", "Min Multiplier", "Max Multiplier", "Count", "Percentage (%)"},
	}

	for _, b := range r.Buckets {
		records = append(records, []string{
			b.Label,
			fmt.Sprintf("%.1f", b.Min),
			fmt.Sprintf("%.1f", b.Max),
			strconv.FormatUint(b.Count, 10),
			fmt.Sprintf("%.4f", b.Percent),
		})
	}

	if err := cw.WriteAll(records); err != nil {
		return err
	}
	cw.Flush()
	return cw.Error()
}

// WriteText writes the simulation Report formatted as human-readable text.
func WriteText(w io.Writer, r Report) error {
	var err error
	write := func(text string) {
		if err != nil {
			return
		}
		_, err = io.WriteString(w, text)
	}

	write("================================================================================\n")
	write("SLOT SIMULATION REPORT\n")
	write("================================================================================\n")
	write(fmt.Sprintf("Game:              %s (v%s)\n", r.GameID, r.Version))
	write(fmt.Sprintf("Total Spins:       %d\n", r.TotalSpins))
	write(fmt.Sprintf("Bet Per Spin:      %d\n", r.BetPerSpin))
	write(fmt.Sprintf("Total Bet:         %d\n", r.TotalBet))
	write(fmt.Sprintf("Total Payout:      %d\n", r.TotalPayout))
	write("--------------------------------------------------------------------------------\n")
	write(fmt.Sprintf("Total RTP:         %.4f%%\n", r.TotalRTP*100))
	write(fmt.Sprintf("Base RTP:          %.4f%%\n", r.BaseRTP*100))
	write(fmt.Sprintf("Feature RTP:       %.4f%%\n", r.FeatureRTP*100))
	write("--------------------------------------------------------------------------------\n")
	write(fmt.Sprintf("Hit Frequency:     %.4f%% (%d hits)\n", r.HitFrequency*100, r.HitCount))
	write(fmt.Sprintf("Feature Frequency: %.4f%% (%d features)\n", r.FeatureFrequency*100, r.FeatureCount))
	write(fmt.Sprintf("Average Win:       %.2f\n", r.AverageWin))
	write(fmt.Sprintf("Avg Feature Win:   %.2f\n", r.AverageFeatureWin))
	write(fmt.Sprintf("Max Multiplier:    %sx (Payout: %d)\n", r.MaxMultiplier.String(), r.MaxPayout))
	write(fmt.Sprintf("Standard Dev:      %.6f (Variance: %.6f)\n", r.StandardDeviation, r.Variance))
	write(fmt.Sprintf("Duration:          %.3fs\n", r.DurationSeconds))
	write("--------------------------------------------------------------------------------\n")
	write(fmt.Sprintf("%-18s %12s %12s\n", "Bucket", "Count", "Percent"))
	write("--------------------------------------------------------------------------------\n")
	for _, b := range r.Buckets {
		write(fmt.Sprintf("%-18s %12d %11.4f%%\n", b.Label, b.Count, b.Percent))
	}
	write("================================================================================\n")

	return err
}
