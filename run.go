package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/eiannone/keyboard"
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the engine for the specified project",
	Long:  `Run the engine binary located at ~/.apito/<project>/<project>`,
	Run: func(cmd *cobra.Command, args []string) {
		project, _ := cmd.Flags().GetString("project")
		if project == "" {
			fmt.Println("Error: --project is required")
			return
		}
		runEngine(project)
	},
}

func runEngine(project string) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Println("Error finding home directory:", err)
		return
	}
	projectDir := filepath.Join(homeDir, ".apito", project)

	ctx := context.Background()

	err = run(ctx, projectDir, project)
	if err != nil {
		fmt.Println("Error starting engine:", err)
		return
	}

	fmt.Println("Engine started with PID:")
}

// #todo better handling the process termination process
func run(ctx context.Context, projectDir, projectName string) error {

	enginePath := filepath.Join(projectDir, projectName)

	ctx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(ctx, "sh", "-c", enginePath)

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	/*cmd.Cancel = func() error {
		return nil
	}*/

	// Set the output of the command
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	fmt.Println("Starting app :", projectName, cmd.String())

	err := cmd.Start()
	if err != nil {
		return fmt.Errorf("failed to start the app: %w", err)
	}

	// Save the PID to the .env file
	pid := cmd.Process.Pid
	err = updateConfig(projectDir, "ENGINE_PID", strconv.Itoa(pid))
	if err != nil {
		return err
	}

	fmt.Println("Press `Ctrl+T` or `q` to stop the engine...")

	// Start listening for keyboard inputs
	if err = keyboard.Open(); err != nil {
		return err // Execution may end after this instruction without calling 'cancel' function
	}
	defer keyboard.Close()

	go func() {
		for {
			char, key, err := keyboard.GetKey()
			if err != nil {
				fmt.Printf("Error reading input: %v\n", err)
				return
			}

			// Check for Ctrl+T (Ctrl+T sends the ASCII code 20)
			if key == keyboard.KeyCtrlT {
				fmt.Println("Received Ctrl+T, stopping the engine...")
				cancel()
				return
			}

			// Optionally handle other keys here
			if char == 'q' {
				fmt.Println("Received 'q', quitting...")
				cancel()
				return
			}
		}
	}()

	select {
	case <-ctx.Done():
		if ctx.Err() != nil {
			fmt.Print(ctx.Err())
		} else {
			fmt.Printf("Server process exited gracefully")
		}
	}

	err = cmd.Wait()
	if err != nil {
		return err
	}

	return nil
}
