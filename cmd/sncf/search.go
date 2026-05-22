package main

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/thomasmarcelin/sncf-cli/internal/api"
)

func printProposals(results []api.Proposal) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "DEPART\tARRIVEE\tDUREE\tTRAIN\tPRIX\tDE\tA")
	for _, p := range results {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			p.Departure, p.Arrival, p.Duration, p.Transporter, p.BestPrice, p.Origin, p.Destination)
	}
	_ = w.Flush()
	if len(results) > 0 {
		fmt.Fprintf(os.Stderr, "\nItinerary ID: %s\n", results[0].ItineraryID)
		fmt.Fprintf(os.Stderr, "Use `sncfcli search more %s` for later trains\n", results[0].ItineraryID)
	}
}

var searchCmd = &cobra.Command{
	Use:   "search",
	Short: "Search trains (SNCF Connect BFF)",
	Example: `  sncfcli search --from "Paris" --to "Lyon" --date 2026-06-01
  sncfcli search --from "Paris" --to "Marseille" --date 2026-06-01 --time 08:00 --json
  sncfcli search more <itineraryID>
  sncfcli search more --previous <itineraryID>`,
	RunE: func(cmd *cobra.Command, args []string) error {
		from, _ := cmd.Flags().GetString("from")
		to, _ := cmd.Flags().GetString("to")
		date, _ := cmd.Flags().GetString("date")
		hhmm, _ := cmd.Flags().GetString("time")

		if from == "" || to == "" || date == "" {
			return fmt.Errorf("--from, --to, and --date are required")
		}
		if hhmm == "" {
			hhmm = "08:00"
		}
		when, err := time.ParseInLocation("2006-01-02 15:04", date+" "+hhmm, time.Local)
		if err != nil {
			return fmt.Errorf("bad --date/--time (want YYYY-MM-DD / HH:MM): %w", err)
		}

		client, err := authedClient()
		if err != nil {
			return err
		}
		rawMode, _ := cmd.Flags().GetBool("raw")
		if rawMode {
			b, err := client.SearchWithCardsRaw(from, to, when)
			if err != nil {
				return err
			}
			os.Stdout.Write(b)
			fmt.Fprintln(os.Stdout)
			return nil
		}
		results, err := client.SearchWithCards(from, to, when)
		if err != nil {
			return fmt.Errorf("search failed: %w", err)
		}
		if len(results) == 0 {
			fmt.Println("No itineraries found.")
			return nil
		}

		if jsonOutput {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(results)
		}

		printProposals(results)
		return nil
	},
}

var searchMoreCmd = &cobra.Command{
	Use:   "more <itineraryID>",
	Short: "Load next/previous page of search results",
	Example: `  sncfcli search more <itineraryID>
  sncfcli search more --previous <itineraryID>`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		previous, _ := cmd.Flags().GetBool("previous")

		client, err := authedClient()
		if err != nil {
			return err
		}
		results, err := client.MoreItineraries(args[0], !previous)
		if err != nil {
			return fmt.Errorf("more itineraries: %w", err)
		}
		if len(results) == 0 {
			fmt.Println("No more itineraries.")
			return nil
		}

		if jsonOutput {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(results)
		}

		printProposals(results)
		return nil
	},
}

func init() {
	searchCmd.Flags().StringP("from", "f", "", "Departure (free text, e.g. \"Paris\")")
	searchCmd.Flags().StringP("to", "t", "", "Arrival (free text)")
	searchCmd.Flags().StringP("date", "d", "", "Travel date (YYYY-MM-DD)")
	searchCmd.Flags().String("time", "", "Preferred departure time (HH:MM, default 08:00)")

	searchCmd.Flags().Bool("raw", false, "Dump raw BFF JSON (debug)")
	searchMoreCmd.Flags().Bool("previous", false, "Load previous results instead of next")
	searchCmd.AddCommand(searchMoreCmd)
}
