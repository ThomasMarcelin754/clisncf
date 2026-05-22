package main

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var cartCmd = &cobra.Command{
	Use:   "cart",
	Short: "Show your current SNCF Connect basket (read-only)",
	RunE: func(cmd *cobra.Command, args []string) error {
		raw, _ := cmd.Flags().GetBool("raw")
		client, err := authedClient()
		if err != nil {
			return err
		}
		if raw {
			b, err := client.GetCartRaw()
			if err != nil {
				return err
			}
			os.Stdout.Write(b)
			fmt.Fprintln(os.Stdout)
			return nil
		}
		cart, err := client.GetCart()
		if err != nil {
			return fmt.Errorf("failed to read cart: %w", err)
		}

		if jsonOutput {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(cart)
		}

		if len(cart.Items) == 0 {
			fmt.Printf("Basket empty (nextStep=%s).\n", cart.NextStep)
			return nil
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "TITRE\tDE\tA\tQUAND\tPRIX")
		for _, it := range cart.Items {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
				it.Title, it.From, it.To, it.When, it.PriceLabel)
		}
		if err := w.Flush(); err != nil {
			return err
		}
		fmt.Printf("\nTotal: %s  (next: %s)\n", cart.TotalLabel, cart.NextStep)
		return nil
	},
}

var cartClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Remove all items from your cart",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := authedClient()
		if err != nil {
			return err
		}
		raw, err := client.GetCartRaw()
		if err != nil {
			return err
		}
		var r struct {
			Items []struct {
				ID string `json:"id"`
			} `json:"items"`
		}
		if err := json.Unmarshal(raw, &r); err != nil {
			return err
		}
		if len(r.Items) == 0 {
			fmt.Println("Cart already empty.")
			return nil
		}
		for _, it := range r.Items {
			fmt.Fprintf(os.Stderr, "Removing %s...\n", it.ID)
			if err := client.DeleteCartTravel(it.ID); err != nil {
				return fmt.Errorf("delete %s: %w", it.ID, err)
			}
		}
		fmt.Fprintf(os.Stderr, "Removed %d item(s).\n", len(r.Items))
		return nil
	},
}

func init() {
	cartCmd.Flags().Bool("raw", false, "Dump raw BFF response")
	cartCmd.AddCommand(cartClearCmd)
}
