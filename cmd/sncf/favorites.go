package main

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var favoritesCmd = &cobra.Command{
	Use:   "favorites",
	Short: "View your favorite places and itineraries",
}

var favoritesPlacesCmd = &cobra.Command{
	Use:   "places",
	Short: "List your favorite places",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := authedClient()
		if err != nil {
			return err
		}
		places, err := client.GetFavoritePlaces()
		if err != nil {
			return fmt.Errorf("failed to fetch favorite places: %w", err)
		}
		if jsonOutput {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(places)
		}
		if len(places) == 0 {
			fmt.Println("No favorite places.")
			return nil
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "LABEL\tTYPE")
		for _, p := range places {
			fmt.Fprintf(w, "%s\t%s\n", p.Label, p.Type)
		}
		return w.Flush()
	},
}

func init() {
	favoritesCmd.AddCommand(favoritesPlacesCmd)
}
