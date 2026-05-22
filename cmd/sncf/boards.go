package main

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var boardsCmd = &cobra.Command{
	Use:   "boards <station>",
	Short: "Real-time departure/arrival boards for a station",
	Example: `  sncfcli boards "Paris Gare de Lyon"
  sncfcli boards "Marseille" --arrivals`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		arrivals, _ := cmd.Flags().GetBool("arrivals")

		client, err := authedClient()
		if err != nil {
			return err
		}
		stationID, stationLabel, err := client.AutocompletePlace(args[0])
		if err != nil {
			return fmt.Errorf("station lookup: %w", err)
		}
		boards, err := client.GetBoards(stationID)
		if err != nil {
			return fmt.Errorf("failed to fetch boards: %w", err)
		}
		if jsonOutput {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(boards)
		}

		fmt.Printf("%s\n\n", stationLabel)
		for lineType, board := range boards.BoardsByLineID {
			if board.MainLineBoards == nil {
				continue
			}
			items := board.MainLineBoards.DeparturesBoard.Items
			label := "Départs"
			if arrivals {
				items = board.MainLineBoards.ArrivalsBoard.Items
				label = "Arrivées"
			}
			if len(items) == 0 {
				continue
			}
			fmt.Printf("%s — %s (%d):\n", lineType, label, len(items))
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "  HEURE\tTRAIN\tDESTINATION\tVOIE\tRETARD")
			for _, it := range items {
				delay := ""
				if it.TimeLabelDisrupted != "" {
					delay = it.TimeLabelDisrupted
				}
				dest := it.DestinationLabel
				if arrivals {
					dest = it.OriginLabel
				}
				fmt.Fprintf(w, "  %s\t%s\t%s\t%s\t%s\n",
					it.TimeLabel, it.VehicleInfo.Label, dest, it.PlatformLabel, delay)
			}
			if err := w.Flush(); err != nil {
				return err
			}
			fmt.Println()
		}
		return nil
	},
}

func init() {
	boardsCmd.Flags().Bool("arrivals", false, "Show arrivals instead of departures")
}
