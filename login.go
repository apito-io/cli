package main

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Login to Apito CLI",
	Long:  `Login to Apito CLI using OAuth.`,
	Run: func(cmd *cobra.Command, args []string) {
		startLoginServer()
	},
}

func startLoginServer() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("token")
		if token != "" {
			saveToken(token)
			fmt.Fprintln(w, "Login successful. You can close this window.")
		} else {
			fmt.Fprintln(w, "Invalid login attempt.")
		}
	})

	go func() {
		fmt.Println("Starting login server on http://localhost:5555")
		if err := http.ListenAndServe(":5555", nil); err != nil {
			fmt.Println("Error starting server:", err)
		}
	}()

	// Open the login URL in the default browser
	loginURL := "https://example.com/oauth/login"
	if err := exec.Command("open", loginURL).Start(); err != nil {
		fmt.Println("Error opening browser:", err)
	}
}

func saveToken(token string) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Println("Error finding home directory:", err)
		return
	}
	configDir := filepath.Join(homeDir, ".apito")
	configFile := filepath.Join(configDir, ".config")

	if err := os.MkdirAll(configDir, 0755); err != nil {
		fmt.Println("Error creating config directory:", err)
		return
	}

	file, err := os.Create(configFile)
	if err != nil {
		fmt.Println("Error creating config file:", err)
		return
	}
	defer file.Close()

	if _, err := file.WriteString(token); err != nil {
		fmt.Println("Error writing token to config file:", err)
	}
	fmt.Println("Token saved successfully.")
}
