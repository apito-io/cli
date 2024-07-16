package main

import (
	"database/sql"
	"fmt"
	"github.com/dgraph-io/badger/v3"
	"github.com/joho/godotenv"
	"os"
	"path/filepath"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

var changePassCmd = &cobra.Command{
	Use:   "change-pass",
	Short: "Change password for a user",
	Long:  `Change the password for a specified user in the Apito CLI.`,
	Run: func(cmd *cobra.Command, args []string) {
		project, _ := cmd.Flags().GetString("project")
		user, _ := cmd.Flags().GetString("user")
		if project == "" || user == "" {
			fmt.Println("Error: --project and --user are required")
			return
		}

		// Prompt for new password
		prompt := promptui.Prompt{
			Label: "New Password",
			Mask:  '*',
			Validate: func(input string) error {
				if len(input) < 6 {
					return fmt.Errorf("Password must be at least 6 characters")
				}
				return nil
			},
		}
		password, err := prompt.Run()
		if err != nil {
			fmt.Println("Prompt failed:", err)
			return
		}

		// Prompt for confirm password
		confirmPrompt := promptui.Prompt{
			Label: "Confirm Password",
			Mask:  '*',
			Validate: func(input string) error {
				if input != password {
					return fmt.Errorf("Passwords do not match")
				}
				return nil
			},
		}
		confirmPassword, err := confirmPrompt.Run()
		if err != nil {
			fmt.Println("Prompt failed:", err)
			return
		}

		if password == confirmPassword {
			// Read the active config and connect with the database
			if err := changePassword(project, user, password); err != nil {
				fmt.Println("Error changing password:", err)
				return
			}
			fmt.Println("Password changed successfully.")
		} else {
			fmt.Println("Passwords do not match.")
		}
	},
}

func init() {
	changePassCmd.Flags().StringP("project", "p", "", "Project name")
	changePassCmd.Flags().StringP("user", "u", "", "Username")
}

func changePassword(project, user, password string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("error finding home directory: %w", err)
	}
	configFile := filepath.Join(homeDir, ".apito", project, ".config")

	// Load the config file
	envMap, err := godotenv.Read(configFile)
	if err != nil {
		return fmt.Errorf("error reading config file: %w", err)
	}

	dbEngine := envMap["DB_ENGINE"]
	adminUser := envMap["ADMIN_USER"]

	switch dbEngine {
	case "memorydb":
		err = changePasswordMemoryDB(project, adminUser, password)
	case "postgres":
		connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
			envMap["DB_HOST"], envMap["DB_PORT"], envMap["DB_USER"], envMap["DB_PASS"], envMap["DB_NAME"])
		err = changePasswordSQL("postgres", connStr, adminUser, password)
	case "mysql":
		connStr := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s",
			envMap["DB_USER"], envMap["DB_PASS"], envMap["DB_HOST"], envMap["DB_PORT"], envMap["DB_NAME"])
		err = changePasswordSQL("mysql", connStr, adminUser, password)
	default:
		err = fmt.Errorf("unsupported DB_ENGINE: %s", dbEngine)
	}

	if err != nil {
		return fmt.Errorf("error changing password: %w", err)
	}

	fmt.Printf("Password for user %s in project %s changed successfully.\n", user, project)
	return nil
}

func changePasswordMemoryDB(project, user, password string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("error finding home directory: %w", err)
	}
	dbDir := filepath.Join(homeDir, ".apito", project, "memorydb")
	opts := badger.DefaultOptions(dbDir).WithLoggingLevel(badger.ERROR)
	db, err := badger.Open(opts)
	if err != nil {
		return fmt.Errorf("error opening memorydb: %w", err)
	}
	defer db.Close()

	err = db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(user), []byte(password))
	})
	if err != nil {
		return fmt.Errorf("error updating password in memorydb: %w", err)
	}

	return nil
}

func changePasswordSQL(driver, connStr, user, password string) error {
	db, err := sql.Open(driver, connStr)
	if err != nil {
		return fmt.Errorf("error connecting to database: %w", err)
	}
	defer db.Close()

	query := "UPDATE users SET password = $1 WHERE username = $2"
	if driver == "mysql" {
		query = "UPDATE users SET password = ? WHERE username = ?"
	}
	_, err = db.Exec(query, password, user)
	if err != nil {
		return fmt.Errorf("error updating password in database: %w", err)
	}

	return nil
}
