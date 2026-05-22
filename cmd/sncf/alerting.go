package main

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var alertingCmd = &cobra.Command{
	Use:   "alerting",
	Short: "Manage your price and availability alerts",
}

var alertingListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all your alerts (low-price, sales-opening, full-train)",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := authedClient()
		if err != nil {
			return err
		}

		lp, err := client.GetLowPriceAlerts()
		if err != nil {
			return fmt.Errorf("low-price alerts: %w", err)
		}
		so, err := client.GetSalesOpeningAlerts()
		if err != nil {
			return fmt.Errorf("sales-opening alerts: %w", err)
		}
		ft, err := client.GetFullTrainAlerts()
		if err != nil {
			return fmt.Errorf("full-train alerts: %w", err)
		}

		if jsonOutput {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(map[string]any{
				"lowPrice":     lp,
				"salesOpening": so,
				"fullTrain":    ft,
			})
		}

		total := len(lp) + len(so) + len(ft)
		if total == 0 {
			fmt.Println("No alerts.")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		if len(lp) > 0 {
			fmt.Println("Low-price alerts:")
			fmt.Fprintln(w, "  FROM\tTO\tDATE\tMAX PRICE")
			for _, a := range lp {
				fmt.Fprintf(w, "  %s\t%s\t%s\t%s\n", a.Origin, a.Destination, a.OutwardDate, a.MaxPrice)
			}
			if err := w.Flush(); err != nil {
				return err
			}
		}
		if len(so) > 0 {
			fmt.Println("Sales-opening alerts:")
			fmt.Fprintln(w, "  FROM\tTO\tDATE")
			for _, a := range so {
				fmt.Fprintf(w, "  %s\t%s\t%s\n", a.Origin, a.Destination, a.Date)
			}
			if err := w.Flush(); err != nil {
				return err
			}
		}
		if len(ft) > 0 {
			fmt.Println("Full-train alerts:")
			fmt.Fprintln(w, "  FROM\tTO\tDEPARTURE")
			for _, a := range ft {
				fmt.Fprintf(w, "  %s\t%s\t%s\n", a.Origin, a.Destination, a.DepartureDate)
			}
			if err := w.Flush(); err != nil {
				return err
			}
		}
		return nil
	},
}

var alertingDeleteCmd = &cobra.Command{
	Use:   "delete <alertID>",
	Short: "Delete an alert by ID and type",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		alertType, _ := cmd.Flags().GetString("type")
		switch alertType {
		case "low-price", "sales-opening", "full-train":
		default:
			return fmt.Errorf("--type must be low-price, sales-opening, or full-train (got %q)", alertType)
		}

		if !confirmAction(fmt.Sprintf("Delete %s alert %s?", alertType, args[0])) {
			fmt.Println("Aborted.")
			return nil
		}

		client, err := authedClient()
		if err != nil {
			return err
		}
		if err := client.DeleteAlert(alertType, args[0]); err != nil {
			return fmt.Errorf("failed to delete alert: %w", err)
		}
		fmt.Println("Alert deleted.")
		return nil
	},
}

var alertingCreateLowPriceCmd = &cobra.Command{
	Use:   "create-low-price",
	Short: "Create a low-price alert",
	Example: `  sncf alerting create-low-price --from Lyon --to Paris --start 2026-06-01 --end 2026-06-15 --max-price 50
  sncf alerting create-low-price --from "Marseille" --to "Paris" --start 2026-07-01 --end 2026-07-31 --max-price 35`,
	RunE: func(cmd *cobra.Command, args []string) error {
		from, _ := cmd.Flags().GetString("from")
		to, _ := cmd.Flags().GetString("to")
		start, _ := cmd.Flags().GetString("start")
		end, _ := cmd.Flags().GetString("end")
		maxPrice, _ := cmd.Flags().GetInt("max-price")
		if from == "" || to == "" || start == "" || end == "" || maxPrice == 0 {
			return fmt.Errorf("--from, --to, --start, --end, and --max-price are all required")
		}

		client, err := authedClient()
		if err != nil {
			return err
		}
		acct, err := client.GetAccountDisplay()
		if err != nil {
			return fmt.Errorf("failed to fetch account: %w", err)
		}
		email := acct.PersonalData.Email

		if !confirmAction(fmt.Sprintf("Create low-price alert %s→%s (%s to %s, max %d€) for %s?", from, to, start, end, maxPrice, email)) {
			fmt.Println("Cancelled.")
			return nil
		}
		if err := client.CreateLowPriceAlert(from, to, start, end, email, maxPrice); err != nil {
			return fmt.Errorf("failed to create alert: %w", err)
		}
		fmt.Println("Low-price alert created.")
		return nil
	},
}

var alertingCreateSalesOpeningCmd = &cobra.Command{
	Use:   "create-sales-opening",
	Short: "Create a sales-opening (booking) alert",
	Example: `  sncf alerting create-sales-opening --from Paris --to Lyon --date 2026-09-01`,
	RunE: func(cmd *cobra.Command, args []string) error {
		from, _ := cmd.Flags().GetString("from")
		to, _ := cmd.Flags().GetString("to")
		date, _ := cmd.Flags().GetString("date")
		if from == "" || to == "" || date == "" {
			return fmt.Errorf("--from, --to, and --date are all required")
		}

		client, err := authedClient()
		if err != nil {
			return err
		}
		acct, err := client.GetAccountDisplay()
		if err != nil {
			return fmt.Errorf("failed to fetch account: %w", err)
		}
		email := acct.PersonalData.Email

		if !confirmAction(fmt.Sprintf("Create sales-opening alert %s→%s (%s) for %s?", from, to, date, email)) {
			fmt.Println("Cancelled.")
			return nil
		}
		if err := client.CreateSalesOpeningAlert(from, to, date, email); err != nil {
			return fmt.Errorf("failed to create alert: %w", err)
		}
		fmt.Println("Sales-opening alert created.")
		return nil
	},
}

var alertingCheckODCmd = &cobra.Command{
	Use:     "check-od",
	Short:   "Check if an OD is eligible for low-price alerts",
	Example: `  sncf alerting check-od --from Paris --to Lyon`,
	RunE: func(cmd *cobra.Command, args []string) error {
		from, _ := cmd.Flags().GetString("from")
		to, _ := cmd.Flags().GetString("to")
		if from == "" || to == "" {
			return fmt.Errorf("--from and --to are required")
		}

		client, err := authedClient()
		if err != nil {
			return err
		}
		r, err := client.CheckLowPriceOD(from, to)
		if err != nil {
			return err
		}

		if jsonOutput {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(r)
		}

		if !r.Eligible {
			fmt.Printf("%s → %s: not eligible for low-price alerts\n", from, to)
			return nil
		}
		fmt.Printf("%s → %s: eligible\n", from, to)
		if len(r.DaysOfWeek) > 0 {
			days := make([]string, len(r.DaysOfWeek))
			for i, d := range r.DaysOfWeek {
				days[i] = d.Title.Label
			}
			fmt.Printf("  Days: %v\n", days)
		}
		if len(r.TimeSlots) > 0 {
			fmt.Printf("  Time slots: %s – %s\n", r.TimeSlots[0].Label, r.TimeSlots[len(r.TimeSlots)-1].Label)
		}
		return nil
	},
}

var alertingCheckScheduleCmd = &cobra.Command{
	Use:   "check-schedule",
	Short: "Check schedule eligibility and get price range (min/max/avg) for an OD",
	Example: `  sncf alerting check-schedule --from Paris --to Lyon --start 2026-06-01 --end 2026-06-30
  sncf alerting check-schedule --from Marseille --to Bordeaux --start 2026-07-01 --end 2026-07-15`,
	RunE: func(cmd *cobra.Command, args []string) error {
		from, _ := cmd.Flags().GetString("from")
		to, _ := cmd.Flags().GetString("to")
		start, _ := cmd.Flags().GetString("start")
		end, _ := cmd.Flags().GetString("end")
		if from == "" || to == "" || start == "" || end == "" {
			return fmt.Errorf("--from, --to, --start, and --end are all required")
		}

		client, err := authedClient()
		if err != nil {
			return err
		}
		r, err := client.CheckLowPriceSchedule(from, to, start, end)
		if err != nil {
			return err
		}

		if jsonOutput {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(r)
		}

		if !r.Eligible {
			msg := "not eligible"
			if r.InvalidDateMessage != "" {
				msg = r.InvalidDateMessage
			}
			fmt.Printf("%s → %s (%s to %s): %s\n", from, to, start, end, msg)
			return nil
		}
		fmt.Printf("%s → %s (%s to %s):\n", from, to, start, end)
		pr := r.PriceRange
		fmt.Printf("  Price range: %.0f%s – %.0f%s (avg %.0f%s)\n",
			pr.Min, pr.CurrencySymbol, pr.Max, pr.CurrencySymbol, pr.Average, pr.CurrencySymbol)
		fmt.Printf("  Bookable: %s to %s\n", r.MinStartDateSelectable, r.MaxDateSelectable)
		return nil
	},
}

var alertingCalendarCmd = &cobra.Command{
	Use:   "calendar",
	Short: "Show best price per day for a route",
	Example: `  sncf alerting calendar --from Paris --to Lyon --start 2026-06-01 --end 2026-06-30
  sncf alerting calendar --from Marseille --to Lille --start 2026-07-01 --end 2026-07-31`,
	RunE: func(cmd *cobra.Command, args []string) error {
		from, _ := cmd.Flags().GetString("from")
		to, _ := cmd.Flags().GetString("to")
		start, _ := cmd.Flags().GetString("start")
		end, _ := cmd.Flags().GetString("end")
		if from == "" || to == "" || start == "" || end == "" {
			return fmt.Errorf("--from, --to, --start, and --end are all required")
		}

		client, err := authedClient()
		if err != nil {
			return err
		}
		days, err := client.GetCalendarBestPrices(from, to, start, end)
		if err != nil {
			return err
		}

		if jsonOutput {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(days)
		}

		if len(days) == 0 {
			fmt.Println("No prices found for this period.")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "DATE\tPRICE\tBEST?\n")
		for _, d := range days {
			highlight := ""
			if d.Highlight {
				highlight = " ★"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\n", d.Date, d.Price, highlight)
		}
		return w.Flush()
	},
}

func init() {
	alertingCmd.AddCommand(alertingListCmd)
	alertingCmd.AddCommand(alertingDeleteCmd)
	alertingCmd.AddCommand(alertingCreateLowPriceCmd)
	alertingCmd.AddCommand(alertingCreateSalesOpeningCmd)
	alertingCmd.AddCommand(alertingCheckODCmd)
	alertingCmd.AddCommand(alertingCheckScheduleCmd)
	alertingCmd.AddCommand(alertingCalendarCmd)

	alertingDeleteCmd.Flags().String("type", "", "Alert type: low-price, sales-opening, or full-train (required)")
	_ = alertingDeleteCmd.MarkFlagRequired("type")

	alertingCreateLowPriceCmd.Flags().StringP("from", "f", "", "Origin (free text)")
	alertingCreateLowPriceCmd.Flags().StringP("to", "t", "", "Destination (free text)")
	alertingCreateLowPriceCmd.Flags().String("start", "", "Date range start (YYYY-MM-DD)")
	alertingCreateLowPriceCmd.Flags().String("end", "", "Date range end (YYYY-MM-DD)")
	alertingCreateLowPriceCmd.Flags().Int("max-price", 0, "Maximum price in euros")

	alertingCreateSalesOpeningCmd.Flags().StringP("from", "f", "", "Origin (free text)")
	alertingCreateSalesOpeningCmd.Flags().StringP("to", "t", "", "Destination (free text)")
	alertingCreateSalesOpeningCmd.Flags().String("date", "", "Travel date (YYYY-MM-DD)")

	alertingCheckODCmd.Flags().StringP("from", "f", "", "Origin (free text)")
	alertingCheckODCmd.Flags().StringP("to", "t", "", "Destination (free text)")

	alertingCheckScheduleCmd.Flags().StringP("from", "f", "", "Origin (free text)")
	alertingCheckScheduleCmd.Flags().StringP("to", "t", "", "Destination (free text)")
	alertingCheckScheduleCmd.Flags().String("start", "", "Date range start (YYYY-MM-DD)")
	alertingCheckScheduleCmd.Flags().String("end", "", "Date range end (YYYY-MM-DD)")

	alertingCalendarCmd.Flags().StringP("from", "f", "", "Origin (free text)")
	alertingCalendarCmd.Flags().StringP("to", "t", "", "Destination (free text)")
	alertingCalendarCmd.Flags().String("start", "", "First date (YYYY-MM-DD)")
	alertingCalendarCmd.Flags().String("end", "", "Last date (YYYY-MM-DD)")
}
