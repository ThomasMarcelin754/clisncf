package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

var vehicleCmd = &cobra.Command{
	Use:   "vehicle <trainNumber>",
	Short: "Show train composition, stops, and occupancy",
	Example: `  sncf vehicle 6633 --date 2026-05-21
  sncf vehicle 6609 --date 2026-06-15 --from RESARAIL_STA_8768600 --to RESARAIL_STA_8772319
  sncf vehicle 6633 --date 2026-05-21 --json`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		date, _ := cmd.Flags().GetString("date")
		from, _ := cmd.Flags().GetString("from")
		to, _ := cmd.Flags().GetString("to")
		if date == "" {
			return fmt.Errorf("--date is required (YYYY-MM-DD)")
		}

		client, err := authedClient()
		if err != nil {
			return err
		}

		raw, err := client.GetVehicleDetails(args[0], date, from, to)
		if err != nil {
			return err
		}

		if jsonOutput {
			out, err := json.MarshalIndent(json.RawMessage(raw), "", "  ")
			if err != nil {
				return fmt.Errorf("format response: %w", err)
			}
			fmt.Println(string(out))
			return nil
		}

		var data struct {
			ScreenTitle string `json:"screenTitle"`
			Destination string `json:"destination"`
			Occupancy   *struct {
				Status string `json:"status"`
				Title  string `json:"title"`
			} `json:"occupancy"`
			Composition *struct {
				DeparturePositionInformation *struct {
					Description string `json:"description"`
				} `json:"departurePositionInformation"`
			} `json:"composition"`
			Stops []struct {
				Location string `json:"locationLabel"`
				Time     string `json:"timeLabel"`
				Duration string `json:"durationLabel"`
				Platform string `json:"platformLabel"`
				Type     string `json:"informationSegmentType"`
			} `json:"lineDetailStops"`
		}
		if err := json.Unmarshal(raw, &data); err != nil {
			return err
		}

		fmt.Println(data.ScreenTitle)
		if data.Destination != "" {
			fmt.Println(data.Destination)
		}
		if data.Occupancy != nil {
			fmt.Printf("Occupation: %s (%s)\n", data.Occupancy.Title, data.Occupancy.Status)
		}
		if data.Composition != nil && data.Composition.DeparturePositionInformation != nil {
			fmt.Printf("Départ: %s\n", data.Composition.DeparturePositionInformation.Description)
		}

		fmt.Println()
		for _, s := range data.Stops {
			marker := " "
			switch s.Type {
			case "START_ACTIVE_SEGMENT":
				marker = "●"
			case "END_ACTIVE_SEGMENT":
				marker = "●"
			case "STOP_ACTIVE_SEGMENT":
				marker = "○"
			}
			line := fmt.Sprintf("%s %5s  %s", marker, s.Time, s.Location)
			if s.Platform != "" {
				line += "  [" + s.Platform + "]"
			}
			if s.Duration != "" {
				line += "  (" + s.Duration + ")"
			}
			fmt.Println(line)
		}
		return nil
	},
}

func init() {
	vehicleCmd.Flags().StringP("date", "d", "", "Train date (YYYY-MM-DD)")
	vehicleCmd.Flags().StringP("from", "f", "", "Origin station ID (optional)")
	vehicleCmd.Flags().StringP("to", "t", "", "Destination station ID (optional)")
}
