package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

const defaultEngineURL = "http://localhost:5050"

var adminCmd = &cobra.Command{
	Use:   "admin",
	Short: "Admin operations (password reset, etc.)",
	Long:  `Admin commands for protected operations.`,
}

var adminPasswordCmd = &cobra.Command{
	Use:   "password",
	Short: "Password-related admin operations",
	Long:  `Reset or manage user passwords.`,
}

var adminPasswordResetCmd = &cobra.Command{
	Use:   "reset [email]",
	Short: "Reset admin password by email",
	Long:  `Reset the password for a system user by email. Prompts for engine URL (default ` + defaultEngineURL + `), reset token, new password and confirmation. Engine must be running.`,
	Args:  cobra.MaximumNArgs(1),
	Run:   runAdminPasswordReset,
}

func init() {
	adminPasswordCmd.AddCommand(adminPasswordResetCmd)
	adminCmd.AddCommand(adminPasswordCmd)

	adminPasswordResetCmd.Flags().StringP("url", "u", "", "Engine/API URL (default "+defaultEngineURL+")")
}

func runAdminPasswordReset(cmd *cobra.Command, args []string) {
	engineURL, _ := cmd.Flags().GetString("url")
	engineURL = strings.TrimSpace(engineURL)
	if engineURL == "" {
		prompt := promptui.Prompt{
			Label:   "Engine URL (default " + defaultEngineURL + ")",
			Default: defaultEngineURL,
		}
		var errPrompt error
		engineURL, errPrompt = prompt.Run()
		if errPrompt != nil {
			print_error("Failed to get engine URL: " + errPrompt.Error())
			return
		}
		engineURL = strings.TrimSpace(engineURL)
		if engineURL == "" {
			engineURL = defaultEngineURL
		}
	}
	engineURL = strings.TrimSuffix(engineURL, "/")

	var email string
	if len(args) >= 1 {
		email = strings.TrimSpace(args[0])
	}
	if email == "" {
		prompt := promptui.Prompt{
			Label: "Email",
			Validate: func(s string) error {
				if strings.TrimSpace(s) == "" {
					return fmt.Errorf("email is required")
				}
				return nil
			},
		}
		var errPrompt error
		email, errPrompt = prompt.Run()
		if errPrompt != nil {
			print_error("Failed to get email: " + errPrompt.Error())
			return
		}
		email = strings.TrimSpace(email)
	}

	// Ask user to paste reset token (admin secret)
	promptToken := promptui.Prompt{
		Label: "Paste reset token (admin_secret)",
		Validate: func(s string) error {
			if strings.TrimSpace(s) == "" {
				return fmt.Errorf("reset token is required")
			}
			return nil
		},
	}
	resetToken, errToken := promptToken.Run()
	if errToken != nil {
		print_error("Reset token is required.")
		return
	}
	resetToken = strings.TrimSpace(resetToken)

	// Confirmation: "Reset password for <email>? [y/N]"
	promptConfirm := promptui.Select{
		Label:     fmt.Sprintf("Reset password for %s? [y/N]", email),
		Items:     []string{"No", "Yes"},
		CursorPos: 0,
	}
	_, confirm, errConfirm := promptConfirm.Run()
	if errConfirm != nil || confirm != "Yes" {
		print_status("Cancelled.")
		return
	}

	// New password (masked)
	promptPwd := promptui.Prompt{
		Label: "New password",
		Mask:  '*',
		Validate: func(s string) error {
			if len(s) == 0 {
				return fmt.Errorf("password cannot be empty")
			}
			return nil
		},
	}
	newPassword, errPwd := promptPwd.Run()
	if errPwd != nil {
		print_error("Failed to get new password: " + errPwd.Error())
		return
	}

	// Confirm password
	promptPwd2 := promptui.Prompt{
		Label: "Confirm new password",
		Mask:  '*',
		Validate: func(s string) error {
			if s != newPassword {
				return fmt.Errorf("passwords do not match")
			}
			return nil
		},
	}
	_, errPwd2 := promptPwd2.Run()
	if errPwd2 != nil {
		print_error("Passwords do not match.")
		return
	}

	body := map[string]string{
		"email":        email,
		"new_password": newPassword,
		"admin_secret": resetToken,
	}
	bodyJSON, _ := json.Marshal(body)

	url := engineURL + "/admin/reset-password"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(bodyJSON))
	if err != nil {
		print_error("Failed to create request: " + err.Error())
		return
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		print_error("Engine not reachable at " + engineURL + ". Start it with: apito start")
		return
	}
	defer resp.Body.Close()

	var result struct {
		Message string `json:"message"`
		Code    int    `json:"code"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&result)

	if resp.StatusCode != http.StatusOK {
		msg := result.Message
		if msg == "" {
			msg = fmt.Sprintf("HTTP %d", resp.StatusCode)
		}
		print_error("Reset failed: " + msg)
		return
	}

	print_success("Password reset successfully for " + email)
}
