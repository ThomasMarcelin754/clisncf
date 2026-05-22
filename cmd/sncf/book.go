package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/thomasmarcelin/sncf-cli/internal/api"
)

var bookCmd = &cobra.Command{
	Use:   "book",
	Short: "Book a train (search → select → cart → finalization, stops before payment)",
	Long: `Drives the booking funnel up to the payment step.
The CLI will NOT pay — it stops at finalization/create and shows the payment info.
You must complete payment on sncf-connect.com or the app.`,
	Example: `  sncf book --from "Paris" --to "Lyon" --date 2026-06-15 --select 0
  sncf book --from "Paris" --to "Marseille" --date 2026-07-01 --time 08:00 --select 0
  sncf book --from "Paris" --to "Toulon" --date 2026-07-10 --select 2 --seat FENETRE --deck BAS`,
	RunE: func(cmd *cobra.Command, args []string) error {
		from, _ := cmd.Flags().GetString("from")
		to, _ := cmd.Flags().GetString("to")
		dateStr, _ := cmd.Flags().GetString("date")
		timeStr, _ := cmd.Flags().GetString("time")
		selectIdx, _ := cmd.Flags().GetInt("select")
		offerIdx, _ := cmd.Flags().GetInt("offer")
		seatCode, _ := cmd.Flags().GetString("seat")
		deckCode, _ := cmd.Flags().GetString("deck")

		if from == "" || to == "" || dateStr == "" {
			return fmt.Errorf("--from, --to, and --date are required")
		}
		if timeStr == "" {
			timeStr = "08:00"
		}

		when, err := time.Parse("2006-01-02 15:04", dateStr+" "+timeStr)
		if err != nil {
			return fmt.Errorf("invalid date/time: %w", err)
		}

		client, err := authedClient()
		if err != nil {
			return err
		}

		fmt.Fprintf(os.Stderr, "Searching %s → %s on %s...\n", from, to, dateStr)
		proposals, err := client.SearchWithCards(from, to, when)
		if err != nil {
			return fmt.Errorf("search: %w", err)
		}
		if len(proposals) == 0 {
			return fmt.Errorf("no trains found for %s → %s on %s", from, to, dateStr)
		}

		for i, p := range proposals {
			marker := "  "
			if i == selectIdx {
				marker = "→ "
			}
			bookable := ""
			if !p.IsBookable {
				bookable = " [SOLD OUT]"
			}
			fmt.Fprintf(os.Stderr, "%s[%d] %s → %s  %s  %s  %s%s\n",
				marker, i, p.Departure, p.Arrival, p.Duration, p.Transporter, p.BestPrice, bookable)
			for j, o := range p.Offers {
				fmt.Fprintf(os.Stderr, "       [%d.%d] %s %s — %s\n", i, j, o.Class, o.FareName, o.Price)
			}
		}

		if selectIdx < 0 || selectIdx >= len(proposals) {
			return fmt.Errorf("--select %d out of range (0-%d)", selectIdx, len(proposals)-1)
		}

		selected := proposals[selectIdx]
		if !selected.IsBookable {
			return fmt.Errorf("train %s → %s is not bookable", selected.Departure, selected.Arrival)
		}
		if len(selected.Offers) == 0 {
			return fmt.Errorf("no offers for this train")
		}
		if offerIdx < 0 || offerIdx >= len(selected.Offers) {
			return fmt.Errorf("--offer %d out of range (0-%d)", offerIdx, len(selected.Offers)-1)
		}

		offer := selected.Offers[offerIdx]
		fmt.Fprintf(os.Stderr, "\nSelected: %s → %s (%s) %s %s — %s\n",
			selected.Departure, selected.Arrival, selected.Duration,
			offer.Class, offer.FareName, offer.Price)

		if !confirmAction("Add this train to cart?") {
			fmt.Println("Cancelled.")
			return nil
		}

		fmt.Fprintln(os.Stderr, "Adding to cart...")
		var prefs *api.PlacementPrefs
		if seatCode != "" && offer.SegmentID != "" {
			prefs = &api.PlacementPrefs{
				SegmentID: offer.SegmentID,
				SeatCode:  seatCode,
				DeckCode:  deckCode,
			}
			fmt.Fprintf(os.Stderr, "Placement: seat=%s deck=%s segment=%s\n", seatCode, deckCode, offer.SegmentID)
		}
		if err := client.BookWithPlacement(selected.ItineraryID, offer.ID, prefs); err != nil {
			return fmt.Errorf("book: %w", err)
		}
		fmt.Fprintln(os.Stderr, "Added to cart.")

		if jsonOutput {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(map[string]any{
				"status":      "booked",
				"proposal":    selected,
				"offer":       offer,
				"itineraryId": selected.ItineraryID,
				"next_steps":  "GET /api/v1/carts → BookUpdate → FinalizationCreate → payment on sncf-connect.com",
			})
		}

		fmt.Println("\nTrain added to cart. Next steps:")
		fmt.Println("  1. Complete payment on sncf-connect.com or the app")
		fmt.Println("  2. Use `sncf cart` to view your cart")
		return nil
	},
}

func init() {
	bookCmd.Flags().StringP("from", "f", "", "Departure (free text)")
	bookCmd.Flags().StringP("to", "t", "", "Destination (free text)")
	bookCmd.Flags().StringP("date", "d", "", "Travel date (YYYY-MM-DD)")
	bookCmd.Flags().String("time", "08:00", "Preferred departure time (HH:MM)")
	bookCmd.Flags().Int("select", 0, "Index of the train to select (from search results)")
	bookCmd.Flags().Int("offer", 0, "Index of the offer/fare to select within the train")
	bookCmd.Flags().String("seat", "", "Seat preference: FENETRE, COULOIR, SOLO, DUO_COTE_A_COTE, CLUB_QUATRE")
	bookCmd.Flags().String("deck", "ANY", "Deck preference: BAS, HAUT, ANY")
}
