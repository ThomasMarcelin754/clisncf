package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var trafficCmd = &cobra.Command{
	Use:   "traffic",
	Short: "Show current traffic disruptions (public, no auth needed)",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := authedClient()
		if err != nil {
			return err
		}
		raw, err := client.GetTrafficInfo()
		if err != nil {
			return fmt.Errorf("failed to fetch traffic info: %w", err)
		}
		if jsonOutput {
			_, _ = os.Stdout.Write(raw)
			fmt.Println()
			return nil
		}
		type lineItem struct {
			Name  string `json:"name"`
			Label string `json:"label"`
		}
		type section struct {
			Label string     `json:"label"`
			Items []lineItem `json:"items"`
		}
		type mainLine struct {
			Name         string `json:"name"`
			InfoMessages []struct {
				Title string `json:"title"`
			} `json:"infoMessages"`
		}
		var info struct {
			IDF struct {
				Disruptions section `json:"disruptions"`
				Work        section `json:"work"`
				Next        section `json:"next"`
			} `json:"idf"`
			MainLines []mainLine `json:"mainLines"`
		}
		if err := json.Unmarshal(raw, &info); err != nil {
			fmt.Printf("Traffic info: %d bytes (use --json for full data)\n", len(raw))
			return nil
		}
		count := 0
		if len(info.IDF.Disruptions.Items) > 0 {
			fmt.Printf("IDF — %s (%d):\n", info.IDF.Disruptions.Label, len(info.IDF.Disruptions.Items))
			for _, item := range info.IDF.Disruptions.Items {
				name := item.Name
				if name == "" {
					name = item.Label
				}
				fmt.Printf("  - %s\n", name)
			}
			count += len(info.IDF.Disruptions.Items)
			fmt.Println()
		}
		for _, line := range info.MainLines {
			if len(line.InfoMessages) > 0 {
				fmt.Printf("%s (%d):\n", line.Name, len(line.InfoMessages))
				for _, m := range line.InfoMessages {
					fmt.Printf("  - %s\n", m.Title)
				}
				count += len(line.InfoMessages)
				fmt.Println()
			}
		}
		if count == 0 {
			fmt.Println("No disruptions reported.")
		}
		return nil
	},
}
