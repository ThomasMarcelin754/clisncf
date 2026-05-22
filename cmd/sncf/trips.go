package main

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var tripsCmd = &cobra.Command{
	Use:   "trips",
	Short: "View your trips and reservations",
}

var tripsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List your trips (upcoming by default, --past for history)",
	RunE: func(cmd *cobra.Command, args []string) error {
		past, _ := cmd.Flags().GetBool("past")

		client, err := authedClient()
		if err != nil {
			return err
		}

		trips, err := client.GetTrips(past)
		if err != nil {
			return fmt.Errorf("failed to fetch trips: %w", err)
		}

		if len(trips) == 0 {
			fmt.Println("No trips found.")
			return nil
		}

		if jsonOutput {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(trips)
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "DATE\tFROM\tTO\tTRAIN\tREF")
		for _, t := range trips {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
				t.Date, t.From, t.To, t.TrainNumber, t.BookingRef)
		}
		return w.Flush()
	},
}

var tripsShowCmd = &cobra.Command{
	Use:   "show <tripID>",
	Short: "Show details for a single trip",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := authedClient()
		if err != nil {
			return err
		}
		trips, err := client.GetTripByID(args[0])
		if err != nil {
			return fmt.Errorf("failed to fetch trip: %w", err)
		}
		if jsonOutput {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(trips)
		}
		if len(trips) == 0 {
			fmt.Println("Trip not found.")
			return nil
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "DATE\tFROM\tTO\tTRAIN\tREF")
		for _, t := range trips {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
				t.Date, t.From, t.To, t.TrainNumber, t.BookingRef)
		}
		return w.Flush()
	},
}

var tripsFindCmd = &cobra.Command{
	Use:   "find",
	Short: "Find a trip by PNR reference + train number + date",
	Example: "  sncf trips find --ref DH73QD --train 6616 --date 2026-04-19",
	RunE: func(cmd *cobra.Command, args []string) error {
		ref, _ := cmd.Flags().GetString("ref")
		train, _ := cmd.Flags().GetString("train")
		date, _ := cmd.Flags().GetString("date")
		if ref == "" || train == "" || date == "" {
			return fmt.Errorf("--ref, --train, and --date are required")
		}
		client, err := authedClient()
		if err != nil {
			return err
		}
		trips, err := client.FindTripByInventory(ref, train, date)
		if err != nil {
			return fmt.Errorf("failed to find trip: %w", err)
		}
		if jsonOutput {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(trips)
		}
		if len(trips) == 0 {
			fmt.Println("No trip found for this reference.")
			return nil
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "DATE\tFROM\tTO\tTRAIN\tREF")
		for _, t := range trips {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
				t.Date, t.From, t.To, t.TrainNumber, t.BookingRef)
		}
		return w.Flush()
	},
}

var tripsSetMotiveCmd = &cobra.Command{
	Use:   "set-motive <tripID>",
	Short: "Set trip motive (business or personal)",
	Example: `  sncf trips set-motive 550c27fa-... --business
  sncf trips set-motive 550c27fa-... --personal`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		business, _ := cmd.Flags().GetBool("business")
		personal, _ := cmd.Flags().GetBool("personal")
		if !business && !personal {
			return fmt.Errorf("specify --business or --personal")
		}
		client, err := authedClient()
		if err != nil {
			return err
		}
		if err := client.SetTripMotive(args[0], business); err != nil {
			return fmt.Errorf("failed to set motive: %w", err)
		}
		motive := "personal"
		if business {
			motive = "business"
		}
		fmt.Printf("Trip motive set to %s.\n", motive)
		return nil
	},
}

var tripsFiltersCmd = &cobra.Command{
	Use:   "filters",
	Short: "Show available trip filter options (ALL/BUSINESS/PERSONAL)",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := authedClient()
		if err != nil {
			return err
		}
		f, err := client.GetTripsFilter()
		if err != nil {
			return fmt.Errorf("failed to fetch trip filters: %w", err)
		}
		if jsonOutput {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(f)
		}
		fmt.Printf("%s (default: %s)\n", f.Page.Title, f.Page.RadioGroup.DefaultValue)
		for _, rb := range f.Page.RadioGroup.RadioButtons {
			fmt.Printf("  %s (%s)\n", rb.Label, rb.Value)
		}
		return nil
	},
}

var tripsSendHistoryCmd = &cobra.Command{
	Use:   "send-history",
	Short: "Send trip history summary by email",
	Example: `  sncf trips send-history --date 2026-01-01 --email user@example.com`,
	RunE: func(cmd *cobra.Command, args []string) error {
		date, _ := cmd.Flags().GetString("date")
		email, _ := cmd.Flags().GetString("email")
		if date == "" || email == "" {
			return fmt.Errorf("--date and --email are required")
		}
		if !confirmAction(fmt.Sprintf("Send trip history (from %s) to %s?", date, email)) {
			fmt.Println("Cancelled.")
			return nil
		}
		client, err := authedClient()
		if err != nil {
			return err
		}
		if err := client.SendTripsHistory(date, email); err != nil {
			return fmt.Errorf("failed to send trip history: %w", err)
		}
		fmt.Printf("Trip history sent to %s.\n", email)
		return nil
	},
}

var tripsCheckinCmd = &cobra.Command{
	Use:   "checkin <tripID>",
	Short: "Online check-in for a trip (OUIGO)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		email, _ := cmd.Flags().GetString("email")
		if !confirmAction(fmt.Sprintf("Check in trip %s?", args[0])) {
			fmt.Println("Cancelled.")
			return nil
		}
		client, err := authedClient()
		if err != nil {
			return err
		}
		travelers := []map[string]string{{"email": email, "rank": "0"}}
		if err := client.Checkin(args[0], travelers); err != nil {
			return fmt.Errorf("checkin: %w", err)
		}
		fmt.Println("Check-in successful.")
		return nil
	},
}

var tripsWalletCmd = &cobra.Command{
	Use:   "wallet <tripID>",
	Short: "Get Apple/Google Wallet passes for a trip",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		google, _ := cmd.Flags().GetBool("google")
		client, err := authedClient()
		if err != nil {
			return err
		}
		var raw []byte
		if google {
			raw, err = client.GetGoogleWalletPass(args[0])
		} else {
			raw, err = client.GetAppleWalletPasses(args[0])
		}
		if err != nil {
			return err
		}
		out, err := json.MarshalIndent(json.RawMessage(raw), "", "  ")
		if err != nil {
			return fmt.Errorf("format response: %w", err)
		}
		fmt.Println(string(out))
		return nil
	},
}

var tripsCancelCmd = &cobra.Command{
	Use:   "cancel <tripID> <reference>",
	Short: "Start cancellation flow for a trip",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := authedClient()
		if err != nil {
			return err
		}
		raw, err := client.InitCancellation(args[0], args[1])
		if err != nil {
			return fmt.Errorf("init cancellation: %w", err)
		}
		out, err := json.MarshalIndent(json.RawMessage(raw), "", "  ")
		if err != nil {
			return fmt.Errorf("format response: %w", err)
		}
		fmt.Println(string(out))
		return nil
	},
}

var tripsExchangeCmd = &cobra.Command{
	Use:   "exchange <tripID> <reference>",
	Short: "Start exchange flow for a trip",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := authedClient()
		if err != nil {
			return err
		}
		raw, err := client.InitExchange(args[0], args[1])
		if err != nil {
			return fmt.Errorf("init exchange: %w", err)
		}
		out, err := json.MarshalIndent(json.RawMessage(raw), "", "  ")
		if err != nil {
			return fmt.Errorf("format response: %w", err)
		}
		fmt.Println(string(out))
		return nil
	},
}

func init() {
	tripsListCmd.Flags().Bool("past", false, "Show past trips instead of upcoming")
	tripsFindCmd.Flags().String("ref", "", "PNR booking reference (e.g. DH73QD)")
	tripsFindCmd.Flags().String("train", "", "Train number (e.g. 6616)")
	tripsFindCmd.Flags().String("date", "", "Departure date (YYYY-MM-DD)")
	tripsSetMotiveCmd.Flags().Bool("business", false, "Set motive to business")
	tripsSetMotiveCmd.Flags().Bool("personal", false, "Set motive to personal")
	tripsSendHistoryCmd.Flags().String("date", "", "Start date for history (YYYY-MM-DD, required)")
	tripsSendHistoryCmd.Flags().String("email", "", "Recipient email (required)")
	tripsCheckinCmd.Flags().String("email", "", "Traveler email for check-in")
	tripsWalletCmd.Flags().Bool("google", false, "Get Google Wallet pass instead of Apple")
	tripsCmd.AddCommand(tripsListCmd)
	tripsCmd.AddCommand(tripsShowCmd)
	tripsCmd.AddCommand(tripsFindCmd)
	tripsCmd.AddCommand(tripsFiltersCmd)
	tripsCmd.AddCommand(tripsSetMotiveCmd)
	tripsCmd.AddCommand(tripsSendHistoryCmd)
	tripsCmd.AddCommand(tripsCheckinCmd)
	tripsCmd.AddCommand(tripsWalletCmd)
	tripsCmd.AddCommand(tripsCancelCmd)
	tripsCmd.AddCommand(tripsExchangeCmd)
}
