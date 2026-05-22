package main

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var proofsCmd = &cobra.Command{
	Use:   "proofs",
	Short: "Manage purchase justificatifs",
}

var proofsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List trips eligible for a justificatif",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := authedClient()
		if err != nil {
			return err
		}
		proofs, err := client.GetProofs()
		if err != nil {
			return fmt.Errorf("failed to list proofs: %w", err)
		}
		if len(proofs) == 0 {
			fmt.Println("No trips eligible for a justificatif.")
			return nil
		}

		if jsonOutput {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(proofs)
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "MOIS\tDATE\tDE\tA\tSENS\tREF")
		for _, p := range proofs {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
				p.Month, p.Date, p.From, p.To, p.Route, p.Reference)
		}
		if err := w.Flush(); err != nil {
			return err
		}
		fmt.Printf("\n%d voyage(s).\n", len(proofs))
		return nil
	},
}

var proofsGenerateCmd = &cobra.Command{
	Use:   "generate <tripID> [tripID...]",
	Short: "Send a justificatif PDF by email for the given trip(s)",
	Example: `  sncfcli proofs generate 550c27fa-c1ff-484c-9d06-3d6e3ddb6dea
  sncfcli proofs generate --all --name "Jane Doe" --email user@example.com`,
	Args: cobra.MinimumNArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		all, _ := cmd.Flags().GetBool("all")
		email, _ := cmd.Flags().GetString("email")
		name, _ := cmd.Flags().GetString("name")

		client, err := authedClient()
		if err != nil {
			return err
		}

		var tripIDs []string
		if all {
			trips, err := client.GetTrips(true)
			if err != nil {
				return fmt.Errorf("failed to fetch trips: %w", err)
			}
			for _, t := range trips {
				if t.BookingRef != "" {
					tripIDs = append(tripIDs, t.BookingRef)
				}
			}
		} else {
			tripIDs = args
		}
		if len(tripIDs) == 0 {
			return fmt.Errorf("provide trip IDs as arguments, or use --all")
		}

		if email == "" || name == "" {
			acct, err := client.GetAccountDisplay()
			if err != nil {
				return fmt.Errorf("failed to fetch account (needed for email/name): %w", err)
			}
			if email == "" {
				email = acct.PersonalData.Email
			}
			if name == "" {
				name = acct.PersonalData.FirstName + " " + acct.PersonalData.LastName
			}
		}

		if !confirmAction(fmt.Sprintf("Send justificatif for %d trip(s) to %s?", len(tripIDs), email)) {
			fmt.Println("Cancelled.")
			return nil
		}

		if err := client.GenerateProof(email, name, tripIDs); err != nil {
			return fmt.Errorf("failed to generate proof: %w", err)
		}
		fmt.Printf("Justificatif sent to %s for %d trip(s).\n", email, len(tripIDs))
		return nil
	},
}

func init() {
	proofsGenerateCmd.Flags().Bool("all", false, "Generate for all past trips")
	proofsGenerateCmd.Flags().String("email", "", "Recipient email (default: account email)")
	proofsGenerateCmd.Flags().String("name", "", "Recipient name (default: account name)")
	proofsCmd.AddCommand(proofsListCmd)
	proofsCmd.AddCommand(proofsGenerateCmd)
}
