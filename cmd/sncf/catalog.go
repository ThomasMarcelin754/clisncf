package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var catalogCmd = &cobra.Command{
	Use:   "catalog",
	Short: "Browse the SNCF product catalog (discount cards, TER, etc.)",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := authedClient()
		if err != nil {
			return err
		}
		nodes, err := client.GetCatalog()
		if err != nil {
			return fmt.Errorf("failed to fetch catalog: %w", err)
		}
		if jsonOutput {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(nodes)
		}
		for _, n := range nodes {
			title := n.Header.Title
			if title == "" {
				continue
			}
			fmt.Printf("[%s] %s\n", n.Type, title)
			if n.Header.Description != "" {
				fmt.Printf("  %s\n", n.Header.Description)
			}
			for _, c := range n.Body.Cards {
				price := ""
				if c.Price != "" {
					price = " — " + c.Price
				}
				fmt.Printf("    • %s%s\n", c.Title, price)
			}
		}
		return nil
	},
}
