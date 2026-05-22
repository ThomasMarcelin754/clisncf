package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/thomasmarcelin/sncf-cli/internal/auth"
	"golang.org/x/term"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage authentication",
}

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Log in headlessly (email + password + OTP)",
	RunE: func(cmd *cobra.Command, args []string) error {
		email, _ := cmd.Flags().GetString("email")
		password, _ := cmd.Flags().GetString("password")
		useChrome, _ := cmd.Flags().GetBool("chrome")

		if useChrome {
			profile, _ := cmd.Flags().GetString("profile")
			e, p, err := auth.ChromeCredentials(profile)
			if err != nil {
				return fmt.Errorf("chrome credential extraction: %w", err)
			}
			email = e
			password = p
			fmt.Fprintf(os.Stderr, "Credentials from Chrome: %s\n", email)
		}

		if email == "" {
			fmt.Print("Email: ")
			fmt.Scanln(&email)
		}
		if password == "" {
			fmt.Print("Password: ")
			pwBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
			fmt.Println()
			if err != nil {
				return fmt.Errorf("read password: %w", err)
			}
			password = string(pwBytes)
		}

		sess, err := auth.HeadlessLogin(email, password, func(maskedEmail string) (string, error) {
			fmt.Printf("\n📧 OTP sent to %s\n", maskedEmail)
			fmt.Print("Enter 6-digit code: ")
			var otp string
			fmt.Scanln(&otp)
			if len(otp) != 6 {
				return "", fmt.Errorf("expected 6-digit code, got %q", otp)
			}
			return otp, nil
		})
		if err != nil {
			return fmt.Errorf("login failed: %w", err)
		}

		if err := sess.Save(); err != nil {
			return fmt.Errorf("save session: %w", err)
		}

		fmt.Fprintf(os.Stderr, "\n")
		fmt.Printf("✓ Logged in to sncf-connect.com as %s\n", sess.Email)
		fmt.Printf("  Token expires in 30 min (auto-refreshed on each command).\n")
		return nil
	},
}

var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show the logged-in account",
	RunE: func(cmd *cobra.Command, args []string) error {
		sess, err := auth.LoadSession()
		if err != nil {
			return err
		}
		if sess == nil {
			return fmt.Errorf("not logged in — run `sncfcli auth login`")
		}
		status := "HOT"
		if sess.Expired() {
			if sess.RefreshToken != "" {
				status = "EXPIRED (will auto-refresh)"
			} else {
				status = "DEAD (no refresh token)"
			}
		}
		fmt.Printf("Email:   %s\n", sess.Email)
		fmt.Printf("Status:  %s\n", status)
		fmt.Printf("Expires: %s\n", sess.ExpiresAt.Format("15:04:05"))
		return nil
	},
}

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Clear the stored session",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := auth.ClearSession(); err != nil {
			return err
		}
		fmt.Println("Session cleared.")
		return nil
	},
}

var authStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Report session state",
	RunE: func(cmd *cobra.Command, args []string) error {
		sess, err := auth.LoadSession()
		if err != nil {
			return err
		}
		if sess == nil {
			fmt.Println("ABSENT — no session. Run `sncfcli auth login`.")
			return nil
		}
		if sess.Expired() {
			if sess.RefreshToken != "" {
				fmt.Printf("COLD — token expired but refresh available (%s).\n", sess.Email)
				fmt.Println("Next command will auto-refresh.")
			} else {
				fmt.Printf("DEAD — token expired, no refresh (%s). Run `sncfcli auth login`.\n", sess.Email)
			}
		} else {
			fmt.Printf("HOT — %s, expires %s.\n", sess.Email, sess.ExpiresAt.Format("15:04:05"))
		}
		return nil
	},
}

func init() {
	loginCmd.Flags().String("email", "", "Account email (prompted if omitted)")
	loginCmd.Flags().String("password", "", "Password (prompted if omitted; prefer prompt for security)")
	loginCmd.Flags().Bool("chrome", false, "Auto-extract email+password from Chrome (only OTP needed)")
	loginCmd.Flags().String("profile", "Default", "Chrome profile for --chrome")
	authCmd.AddCommand(loginCmd)
	authCmd.AddCommand(whoamiCmd)
	authCmd.AddCommand(logoutCmd)
	authCmd.AddCommand(authStatusCmd)
}
