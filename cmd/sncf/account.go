package main

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var accountCmd = &cobra.Command{
	Use:   "account",
	Short: "View your SNCF Connect account",
}

var accountDisplayCmd = &cobra.Command{
	Use:   "display",
	Short: "Show your account profile (name, email, discount cards)",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := authedClient()
		if err != nil {
			return err
		}
		acct, err := client.GetAccountDisplay()
		if err != nil {
			return fmt.Errorf("failed to fetch account: %w", err)
		}
		if jsonOutput {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(acct)
		}
		p := acct.PersonalData
		fmt.Printf("%s %s %s\n", p.Civility, p.FirstName, p.LastName)
		fmt.Printf("Email:    %s\n", p.Email)
		fmt.Printf("Birth:    %s\n", p.BirthDate)
		fmt.Printf("Account:  %s\n", acct.AccountID)
		if acct.CompanyName != "" {
			fmt.Printf("Company:  %s\n", acct.CompanyName)
		}
		if len(acct.DiscountCards) > 0 {
			fmt.Println("\nDiscount cards:")
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "  CODE\tLABEL\tNUMBER\tEXPIRES")
			for _, c := range acct.DiscountCards {
				fmt.Fprintf(w, "  %s\t%s\t%s\t%s\n", c.Code, c.Label, c.Number, c.ExpiresAt)
			}
			if err := w.Flush(); err != nil {
				return err
			}
		}
		return nil
	},
}

var accountCardCodeCmd = &cobra.Command{
	Use:   "card-code",
	Short: "Show your loyalty card number and QR code",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := authedClient()
		if err != nil {
			return err
		}
		cc, err := client.GetCardCode()
		if err != nil {
			return fmt.Errorf("failed to fetch card-code: %w", err)
		}
		if jsonOutput {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(cc)
		}
		if cc.QRCode != "" {
			fmt.Printf("QR code: base64 PNG (%d bytes)\n", len(cc.QRCode))
		} else {
			fmt.Println("No QR code available.")
		}
		return nil
	},
}

var accountCompanionsCmd = &cobra.Command{
	Use:   "companions",
	Short: "Manage your travel companions",
}

var accountCompanionsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List your travel companions",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := authedClient()
		if err != nil {
			return err
		}
		companions, err := client.GetCompanions()
		if err != nil {
			return fmt.Errorf("failed to fetch companions: %w", err)
		}
		if jsonOutput {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(companions)
		}
		if len(companions) == 0 {
			fmt.Println("No companions.")
			return nil
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tNAME\tBIRTH\tTYPE\tCARDS")
		for _, c := range companions {
			cards := ""
			for i, d := range c.DiscountCards {
				if i > 0 {
					cards += ", "
				}
				cards += d.Label
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
				c.ID,
				c.PersonalData.FirstName+" "+c.PersonalData.LastName,
				c.PersonalData.BirthDate, c.TravellerKind, cards)
		}
		return w.Flush()
	},
}

var accountCompanionsAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a travel companion",
	RunE: func(cmd *cobra.Command, args []string) error {
		first, _ := cmd.Flags().GetString("first")
		last, _ := cmd.Flags().GetString("last")
		birth, _ := cmd.Flags().GetString("birth")
		email, _ := cmd.Flags().GetString("email")
		civility, _ := cmd.Flags().GetString("civility")

		client, err := authedClient()
		if err != nil {
			return err
		}
		if err := client.AddCompanion(first, last, birth, email, civility); err != nil {
			return fmt.Errorf("failed to add companion: %w", err)
		}
		fmt.Printf("Companion %s %s added.\n", first, last)
		return nil
	},
}

var accountCompanionsDeleteCmd = &cobra.Command{
	Use:   "delete <companionID>",
	Short: "Delete a travel companion",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if !confirmAction("Delete companion " + args[0] + "?") {
			fmt.Println("Aborted.")
			return nil
		}
		client, err := authedClient()
		if err != nil {
			return err
		}
		if err := client.DeleteCompanion(args[0]); err != nil {
			return fmt.Errorf("failed to delete companion: %w", err)
		}
		fmt.Println("Companion deleted.")
		return nil
	},
}

var accountPetsCmd = &cobra.Command{
	Use:   "pets",
	Short: "Manage your pets",
}

var accountPetsAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a pet",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		petType, _ := cmd.Flags().GetString("type")

		client, err := authedClient()
		if err != nil {
			return err
		}
		if err := client.AddPet(name, petType); err != nil {
			return fmt.Errorf("failed to add pet: %w", err)
		}
		fmt.Printf("Pet %q added.\n", name)
		return nil
	},
}

var accountPetsDeleteCmd = &cobra.Command{
	Use:   "delete <petID>",
	Short: "Delete a pet",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if !confirmAction("Delete pet " + args[0] + "?") {
			fmt.Println("Aborted.")
			return nil
		}
		client, err := authedClient()
		if err != nil {
			return err
		}
		if err := client.DeletePet(args[0]); err != nil {
			return fmt.Errorf("failed to delete pet: %w", err)
		}
		fmt.Println("Pet deleted.")
		return nil
	},
}

var accountPaymentCardsCmd = &cobra.Command{
	Use:   "payment-cards",
	Short: "List your saved payment cards",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := authedClient()
		if err != nil {
			return err
		}
		acct, err := client.GetAccountDisplay()
		if err != nil {
			return fmt.Errorf("failed to fetch account: %w", err)
		}
		cards := acct.PaymentCards
		if jsonOutput {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(cards)
		}
		if len(cards) == 0 {
			fmt.Println("No saved payment cards.")
			return nil
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "TYPE\tNUMBER\tLABEL\tEXPIRES\tMAIN")
		for _, c := range cards {
			main := ""
			if c.IsMain {
				main = "*"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", c.Type, c.MaskedNumber, c.Label, c.ExpirationDate, main)
		}
		return w.Flush()
	},
}

var accountPictureCmd = &cobra.Command{
	Use:   "picture",
	Short: "Show or save your profile picture",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := authedClient()
		if err != nil {
			return err
		}
		pic, err := client.GetProfilePicture()
		if err != nil {
			return fmt.Errorf("failed to fetch picture: %w", err)
		}
		if pic == "" {
			fmt.Println("No profile picture set.")
			return nil
		}
		if jsonOutput {
			enc := json.NewEncoder(os.Stdout)
			return enc.Encode(map[string]string{"picture": pic})
		}
		fmt.Printf("Profile picture: base64 JPEG (%d bytes)\n", len(pic))
		fmt.Println("Use --json to get the raw base64 data.")
		return nil
	},
}

var accountOptInsCmd = &cobra.Command{
	Use:   "notifications",
	Short: "Show your notification preferences",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := authedClient()
		if err != nil {
			return err
		}
		page, err := client.GetOptIns()
		if err != nil {
			return fmt.Errorf("failed to fetch notifications: %w", err)
		}
		if jsonOutput {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(page)
		}
		for _, block := range page.OptInBlocks {
			fmt.Printf("%s:\n", block.Title)
			for _, o := range block.OptIns {
				status := "OFF"
				if o.IsActive {
					status = "ON "
				}
				fmt.Printf("  [%s] %s\n", status, o.Title)
			}
			fmt.Println()
		}
		return nil
	},
}

var accountOffersCmd = &cobra.Command{
	Use:   "offers",
	Short: "Show your offer/alert settings by city",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := authedClient()
		if err != nil {
			return err
		}
		cats, err := client.GetOffersSettings()
		if err != nil {
			return fmt.Errorf("failed to fetch offer settings: %w", err)
		}
		if jsonOutput {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(cats)
		}
		for _, cat := range cats {
			enabled := 0
			for _, it := range cat.Items {
				if it.IsEnabled {
					enabled++
				}
			}
			fmt.Printf("%s (%d/%d enabled):\n", cat.Title, enabled, len(cat.Items))
			for _, it := range cat.Items {
				status := "  "
				if it.IsEnabled {
					status = "* "
				}
				fmt.Printf("  %s%s\n", status, it.Title)
			}
			fmt.Println()
		}
		return nil
	},
}

func init() {
	accountCmd.AddCommand(accountDisplayCmd)
	accountCmd.AddCommand(accountCardCodeCmd)
	accountCmd.AddCommand(accountCompanionsCmd)
	accountCmd.AddCommand(accountPaymentCardsCmd)
	accountCmd.AddCommand(accountPictureCmd)
	accountCmd.AddCommand(accountOptInsCmd)
	accountCmd.AddCommand(accountOffersCmd)
	accountCmd.AddCommand(accountPetsCmd)

	// companions subcommands
	accountCompanionsCmd.AddCommand(accountCompanionsListCmd)
	accountCompanionsCmd.AddCommand(accountCompanionsAddCmd)
	accountCompanionsCmd.AddCommand(accountCompanionsDeleteCmd)

	accountCompanionsAddCmd.Flags().String("first", "", "First name (required)")
	accountCompanionsAddCmd.Flags().String("last", "", "Last name (required)")
	accountCompanionsAddCmd.Flags().String("birth", "", "Date of birth YYYY-MM-DD (required)")
	accountCompanionsAddCmd.Flags().String("email", "", "Email address")
	accountCompanionsAddCmd.Flags().String("civility", "MISTER", "Civility (MISTER or MISSIS)")
	_ = accountCompanionsAddCmd.MarkFlagRequired("first")
	_ = accountCompanionsAddCmd.MarkFlagRequired("last")
	_ = accountCompanionsAddCmd.MarkFlagRequired("birth")

	// pets subcommands
	accountPetsCmd.AddCommand(accountPetsAddCmd)
	accountPetsCmd.AddCommand(accountPetsDeleteCmd)

	accountPetsAddCmd.Flags().String("name", "", "Pet name (required)")
	accountPetsAddCmd.Flags().String("type", "", "Pet type: DOG, CAT, or OTHER (required)")
	_ = accountPetsAddCmd.MarkFlagRequired("name")
	_ = accountPetsAddCmd.MarkFlagRequired("type")

	// update subcommands
	accountCmd.AddCommand(accountUpdateInfoCmd)
	accountCmd.AddCommand(accountUpdateAddressCmd)
	accountCmd.AddCommand(accountUpdateProofsCmd)
	accountCmd.AddCommand(accountUpdateEnterpriseCmd)
	accountCmd.AddCommand(accountUpdateInfoProCmd)
	accountCmd.AddCommand(accountUpdateNewsletterCmd)
	accountCmd.AddCommand(accountUpdateNotificationCmd)
	accountCmd.AddCommand(accountUpdatePaymentCardCmd)

	accountUpdateEnterpriseCmd.Flags().String("code", "", "Enterprise code (required)")
	accountUpdateEnterpriseCmd.Flags().Bool("enable", false, "Enable enterprise features")
	accountUpdateEnterpriseCmd.Flags().Bool("disable", false, "Disable enterprise features")

	accountUpdateInfoProCmd.Flags().String("company", "", "Company name")
	accountUpdateInfoProCmd.Flags().String("email-pro", "", "Professional email")
	accountUpdateInfoProCmd.Flags().String("siren", "", "SIREN number")
	accountUpdateInfoProCmd.Flags().String("tva", "", "TVA number")
	accountUpdateInfoProCmd.Flags().String("address", "", "Professional address")

	accountUpdateNewsletterCmd.Flags().Bool("subscribe", false, "Subscribe to newsletter")
	accountUpdateNewsletterCmd.Flags().Bool("unsubscribe", false, "Unsubscribe from newsletter")
	accountUpdateNewsletterCmd.Flags().String("market", "fr_FR", "Market locale")

	accountUpdateNotificationCmd.Flags().Bool("enable", false, "Enable notification")
	accountUpdateNotificationCmd.Flags().Bool("disable", false, "Disable notification")

	accountUpdatePaymentCardCmd.Flags().String("name", "", "Card display name (required)")
	accountUpdatePaymentCardCmd.Flags().Bool("main", false, "Set as main payment card")

	accountUpdateInfoCmd.Flags().String("first", "", "First name")
	accountUpdateInfoCmd.Flags().String("last", "", "Last name")
	accountUpdateInfoCmd.Flags().String("birth", "", "Birth date (YYYY-MM-DD)")
	accountUpdateInfoCmd.Flags().String("civility", "", "MISTER or MISSIS")
	accountUpdateInfoCmd.Flags().String("phone", "", "Phone number")

	accountUpdateAddressCmd.Flags().String("street", "", "Main address")
	accountUpdateAddressCmd.Flags().String("complement", "", "Address complement")
	accountUpdateAddressCmd.Flags().String("city", "", "City")
	accountUpdateAddressCmd.Flags().String("zip", "", "Zip code")
	accountUpdateAddressCmd.Flags().String("country", "FR", "Country code")

	accountUpdateProofsCmd.Flags().String("email-pro", "", "Professional email for justificatifs")
	accountUpdateProofsCmd.Flags().Bool("auto-send", false, "Automatically send justificatifs")
}

var accountUpdateEnterpriseCmd = &cobra.Command{
	Use:   "update-enterprise",
	Short: "Set or update your enterprise code",
	Example: `  sncfcli account update-enterprise --code CODE123 --enable
  sncfcli account update-enterprise --code CODE123 --disable`,
	RunE: func(cmd *cobra.Command, args []string) error {
		code, _ := cmd.Flags().GetString("code")
		enable, _ := cmd.Flags().GetBool("enable")
		disable, _ := cmd.Flags().GetBool("disable")
		if code == "" {
			return fmt.Errorf("--code is required")
		}
		if !enable && !disable {
			return fmt.Errorf("specify --enable or --disable")
		}
		state := enable
		action := "enable"
		if disable {
			state = false
			action = "disable"
		}
		if !confirmAction(fmt.Sprintf("Set enterprise code %q (%s)?", code, action)) {
			fmt.Println("Cancelled.")
			return nil
		}
		client, err := authedClient()
		if err != nil {
			return err
		}
		if err := client.UpdateEnterpriseCode(code, state); err != nil {
			return fmt.Errorf("failed to update enterprise code: %w", err)
		}
		fmt.Printf("Enterprise code %q %sd.\n", code, action)
		return nil
	},
}

var accountUpdateInfoProCmd = &cobra.Command{
	Use:   "update-info-pro",
	Short: "Update your professional info (company, SIREN, TVA)",
	Example: `  sncfcli account update-info-pro --company "ACME" --email-pro "pro@acme.fr" --siren "123456789"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		company, _ := cmd.Flags().GetString("company")
		emailPro, _ := cmd.Flags().GetString("email-pro")
		siren, _ := cmd.Flags().GetString("siren")
		tva, _ := cmd.Flags().GetString("tva")
		address, _ := cmd.Flags().GetString("address")
		if !confirmAction(fmt.Sprintf("Update professional info (company=%q)?", company)) {
			fmt.Println("Cancelled.")
			return nil
		}
		client, err := authedClient()
		if err != nil {
			return err
		}
		if err := client.UpdateInfoPro(address, company, emailPro, siren, tva); err != nil {
			return fmt.Errorf("failed to update info pro: %w", err)
		}
		fmt.Println("Professional info updated.")
		return nil
	},
}

var accountUpdateNewsletterCmd = &cobra.Command{
	Use:   "update-newsletter",
	Short: "Subscribe or unsubscribe from the generic newsletter",
	Example: `  sncfcli account update-newsletter --subscribe
  sncfcli account update-newsletter --unsubscribe`,
	RunE: func(cmd *cobra.Command, args []string) error {
		subscribe, _ := cmd.Flags().GetBool("subscribe")
		unsubscribe, _ := cmd.Flags().GetBool("unsubscribe")
		if !subscribe && !unsubscribe {
			return fmt.Errorf("specify --subscribe or --unsubscribe")
		}
		state := subscribe
		action := "subscribe to"
		if unsubscribe {
			state = false
			action = "unsubscribe from"
		}
		if !confirmAction(fmt.Sprintf("Confirm %s newsletter?", action)) {
			fmt.Println("Cancelled.")
			return nil
		}
		client, err := authedClient()
		if err != nil {
			return err
		}
		market, _ := cmd.Flags().GetString("market")
		if err := client.UpdateNewsletter(state, market); err != nil {
			return fmt.Errorf("failed to update newsletter: %w", err)
		}
		fmt.Printf("Newsletter: %sd.\n", action)
		return nil
	},
}

var accountUpdateNotificationCmd = &cobra.Command{
	Use:   "update-notification <optInID>",
	Short: "Enable or disable a notification preference",
	Example: `  sncfcli account update-notification BEFORE_PRE_RESERVED_EXPIRATION --enable
  sncfcli account update-notification BEFORE_PRE_RESERVED_EXPIRATION --disable`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		enable, _ := cmd.Flags().GetBool("enable")
		disable, _ := cmd.Flags().GetBool("disable")
		if !enable && !disable {
			return fmt.Errorf("specify --enable or --disable")
		}
		state := enable
		action := "enable"
		if disable {
			state = false
			action = "disable"
		}
		if !confirmAction(fmt.Sprintf("Confirm %s notification %q?", action, args[0])) {
			fmt.Println("Cancelled.")
			return nil
		}
		client, err := authedClient()
		if err != nil {
			return err
		}
		if err := client.UpdateOptIn(args[0], state); err != nil {
			return fmt.Errorf("failed to update notification: %w", err)
		}
		fmt.Printf("Notification %q %sd.\n", args[0], action)
		return nil
	},
}

var accountUpdatePaymentCardCmd = &cobra.Command{
	Use:   "update-payment-card <cardID>",
	Short: "Update a saved payment card's name or main status",
	Example: `  sncfcli account update-payment-card 100016756526 --name "Ma carte" --main`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		main, _ := cmd.Flags().GetBool("main")
		if name == "" {
			return fmt.Errorf("--name is required")
		}
		if !confirmAction(fmt.Sprintf("Update payment card %s (name=%q, main=%v)?", args[0], name, main)) {
			fmt.Println("Cancelled.")
			return nil
		}
		client, err := authedClient()
		if err != nil {
			return err
		}
		if err := client.UpdatePaymentCard(args[0], name, main); err != nil {
			return fmt.Errorf("failed to update payment card: %w", err)
		}
		fmt.Println("Payment card updated.")
		return nil
	},
}

var accountUpdateInfoCmd = &cobra.Command{
	Use:   "update-info",
	Short: "Update your personal info (name, phone, etc.)",
	Example: `  sncfcli account update-info --phone "+33612345678"
  sncfcli account update-info --first Thomas --last Marcelin`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := authedClient()
		if err != nil {
			return err
		}
		acct, err := client.GetAccountDisplay()
		if err != nil {
			return err
		}
		p := acct.PersonalData
		first, _ := cmd.Flags().GetString("first")
		last, _ := cmd.Flags().GetString("last")
		birth, _ := cmd.Flags().GetString("birth")
		civility, _ := cmd.Flags().GetString("civility")
		phone, _ := cmd.Flags().GetString("phone")
		if first == "" {
			first = p.FirstName
		}
		if last == "" {
			last = p.LastName
		}
		if birth == "" {
			birth = p.BirthDate
		}
		if civility == "" {
			civility = p.Civility
		}

		if !confirmAction(fmt.Sprintf("Update personal info for %s %s?", first, last)) {
			fmt.Println("Cancelled.")
			return nil
		}
		if err := client.UpdatePersonalInfo(first, last, birth, civility, phone); err != nil {
			return fmt.Errorf("failed to update: %w", err)
		}
		fmt.Println("Personal info updated.")
		return nil
	},
}

var accountUpdateAddressCmd = &cobra.Command{
	Use:   "update-address",
	Short: "Update your postal address",
	RunE: func(cmd *cobra.Command, args []string) error {
		street, _ := cmd.Flags().GetString("street")
		complement, _ := cmd.Flags().GetString("complement")
		city, _ := cmd.Flags().GetString("city")
		zip, _ := cmd.Flags().GetString("zip")
		country, _ := cmd.Flags().GetString("country")
		if street == "" || city == "" || zip == "" {
			return fmt.Errorf("--street, --city, and --zip are required")
		}
		if !confirmAction(fmt.Sprintf("Update address to %s, %s %s?", street, zip, city)) {
			fmt.Println("Cancelled.")
			return nil
		}
		client, err := authedClient()
		if err != nil {
			return err
		}
		if err := client.UpdateAddress(street, complement, city, zip, country); err != nil {
			return fmt.Errorf("failed to update address: %w", err)
		}
		fmt.Println("Address updated.")
		return nil
	},
}

var accountUpdateProofsCmd = &cobra.Command{
	Use:   "update-proofs-settings",
	Short: "Update justificatif email settings",
	RunE: func(cmd *cobra.Command, args []string) error {
		emailPro, _ := cmd.Flags().GetString("email-pro")
		autoSend, _ := cmd.Flags().GetBool("auto-send")
		client, err := authedClient()
		if err != nil {
			return err
		}
		if err := client.UpdateProofsSettings(emailPro, autoSend); err != nil {
			return fmt.Errorf("failed to update: %w", err)
		}
		fmt.Println("Proofs settings updated.")
		return nil
	},
}
