package main

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/thomasmarcelin/sncf-cli/internal/api"
)

var statusCmd = &cobra.Command{
	Use:   "status [train-number]",
	Short: "Check real-time train status",
	Example: `  sncf status 6231
  sncf status 6231 --json`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		trainNumber := args[0]

		client := api.NewOpenDataClient()
		info, err := client.GetTrainStatus(trainNumber)
		if err != nil {
			return fmt.Errorf("failed to get status: %w", err)
		}

		if jsonOutput {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(info)
		}

		fmt.Printf("Train %s (%s)\n", info.TrainNumber, info.TrainType)
		fmt.Printf("Status: %s\n\n", info.Status)

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "STATION\tARRIVAL\tDEPARTURE\tDELAY")
		for _, stop := range info.Stops {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
				stop.StationName, stop.ArrivalTime, stop.DepartureTime, stop.Delay)
		}
		return w.Flush()
	},
}
