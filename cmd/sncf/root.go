package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var (
	jsonOutput bool
	yesFlag    bool
)

// confirmAction prompts the user for confirmation. Returns true if --yes flag
// is set or if the user types "y"/"yes". Defaults to NO on empty input.
func confirmAction(prompt string) bool {
	if yesFlag {
		return true
	}
	fmt.Fprintf(os.Stderr, "%s [y/N] ", prompt)
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
		return answer == "y" || answer == "yes"
	}
	return false
}

var rootCmd = &cobra.Command{
	Use:   "sncfcli",
	Short: "SNCF CLI - Manage your train travel from the terminal",
	Long:  `A command-line interface for searching trains, booking tickets, and managing your SNCF account.`,
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	rootCmd.PersistentFlags().BoolVarP(&yesFlag, "yes", "y", false, "Skip confirmation prompts")

	rootCmd.AddCommand(authCmd)
	rootCmd.AddCommand(searchCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(tripsCmd)
	rootCmd.AddCommand(cartCmd)
	rootCmd.AddCommand(proofsCmd)
	rootCmd.AddCommand(accountCmd)
	rootCmd.AddCommand(alertingCmd)
	rootCmd.AddCommand(favoritesCmd)
	rootCmd.AddCommand(trafficCmd)
	rootCmd.AddCommand(catalogCmd)
	rootCmd.AddCommand(boardsCmd)
	rootCmd.AddCommand(bookCmd)
	rootCmd.AddCommand(vehicleCmd)
	rootCmd.AddCommand(versionCmd)
}
