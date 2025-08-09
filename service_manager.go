package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type ManagedService struct {
	Name        string
	CommandPath string
	Arguments   []string
	WorkingDir  string
	PidFilePath string
	LogFilePath string
}

func newManagedService(name, commandPath string, args []string) (*ManagedService, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("cannot get home dir: %w", err)
	}
	apitoDir := filepath.Join(homeDir, ".apito")
	runDir := filepath.Join(apitoDir, "run")
	logsDir := filepath.Join(apitoDir, "logs")
	if err := os.MkdirAll(runDir, 0755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return nil, err
	}
	return &ManagedService{
		Name:        name,
		CommandPath: commandPath,
		Arguments:   args,
		WorkingDir:  apitoDir,
		PidFilePath: filepath.Join(runDir, name+".pid"),
		LogFilePath: filepath.Join(logsDir, name+".log"),
	}, nil
}

func (s *ManagedService) IsRunning() (bool, int) {
	pid, err := s.readPid()
	if err != nil || pid == 0 {
		return false, 0
	}
	if process, err := os.FindProcess(pid); err == nil {
		// signal 0 works on Unix; on Windows this always returns an error, so fallback
		if runtime.GOOS != "windows" {
			if err := process.Signal(syscall.Signal(0)); err == nil {
				return true, pid
			}
		} else {
			// On Windows, try to query tasklist
			out, _ := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid)).Output()
			if strings.Contains(string(out), strconv.Itoa(pid)) {
				return true, pid
			}
		}
	}
	return false, 0
}

func (s *ManagedService) Start() error {
	if running, _ := s.IsRunning(); running {
		return nil
	}

	// Open log file for append
	logFile, err := os.OpenFile(s.LogFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}

	cmd := exec.Command(s.CommandPath, s.Arguments...)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if s.WorkingDir != "" {
		cmd.Dir = s.WorkingDir
	}
	if runtime.GOOS != "windows" {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	}

	if err := cmd.Start(); err != nil {
		logFile.Close()
		return fmt.Errorf("start %s: %w", s.Name, err)
	}

	if err := s.writePid(cmd.Process.Pid); err != nil {
		logFile.Close()
		return err
	}

	// Small delay to allow process to initialize and write logs
	time.Sleep(200 * time.Millisecond)
	return nil
}

func (s *ManagedService) Stop(timeout time.Duration) error {
	pid, err := s.readPid()
	if err != nil || pid == 0 {
		return nil
	}

	// Try graceful stop
	if runtime.GOOS == "windows" {
		_ = exec.Command("taskkill", "/PID", strconv.Itoa(pid), "/T", "/F").Run()
	} else {
		if process, err := os.FindProcess(pid); err == nil {
			_ = process.Signal(syscall.SIGTERM)
		}
	}

	// Wait up to timeout
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		running, _ := s.IsRunning()
		if !running {
			_ = os.Remove(s.PidFilePath)
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}

	// Force kill
	if runtime.GOOS != "windows" {
		if process, err := os.FindProcess(pid); err == nil {
			_ = process.Signal(syscall.SIGKILL)
		}
	}
	_ = os.Remove(s.PidFilePath)
	return nil
}

func (s *ManagedService) Restart() error {
	_ = s.Stop(3 * time.Second)
	return s.Start()
}

func (s *ManagedService) writePid(pid int) error {
	f, err := os.OpenFile(s.PidFilePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(strconv.Itoa(pid))
	return err
}

func (s *ManagedService) readPid() (int, error) {
	content, err := os.ReadFile(s.PidFilePath)
	if err != nil {
		return 0, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(content)))
	if err != nil {
		return 0, err
	}
	return pid, nil
}

// ReadLastLines reads the last n lines from file at path.
func ReadLastLines(path string, n int) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// If file is small, read all
	stat, _ := f.Stat()
	const maxRead = 1 << 20 // 1 MB
	var r io.Reader = f
	if stat != nil && stat.Size() > maxRead {
		// Seek near end
		_, _ = f.Seek(-maxRead, io.SeekEnd)
	}

	scanner := bufio.NewScanner(r)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
		if len(lines) > n {
			lines = lines[1:]
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(lines) == 0 {
		return []string{}, nil
	}
	return lines, nil
}

// Build service definitions for engine and console (caddy)
func getEngineService() (*ManagedService, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	enginePath := filepath.Join(homeDir, ".apito", "bin", "engine")
	return newManagedService("engine", enginePath, nil)
}

func getConsoleService() (*ManagedService, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	caddyPath := resolveCaddyPath()
	caddyfilePath := filepath.Join(homeDir, ".apito", "Caddyfile")
	return newManagedService("console", caddyPath, []string{"run", "--config", caddyfilePath})
}

// Helpers to expose simple Start/Stop/Status API
func startManagedService(name string) error {
	var svc *ManagedService
	var err error
	switch name {
	case "engine":
		svc, err = getEngineService()
	case "console":
		svc, err = getConsoleService()
	default:
		return errors.New("unknown service: " + name)
	}
	if err != nil {
		return err
	}
	return svc.Start()
}

func stopManagedService(name string) error {
	var svc *ManagedService
	var err error
	switch name {
	case "engine":
		svc, err = getEngineService()
	case "console":
		svc, err = getConsoleService()
	default:
		return errors.New("unknown service: " + name)
	}
	if err != nil {
		return err
	}
	return svc.Stop(3 * time.Second)
}

func serviceRunning(name string) (bool, int, string) {
	var svc *ManagedService
	var err error
	switch name {
	case "engine":
		svc, err = getEngineService()
	case "console":
		svc, err = getConsoleService()
	default:
		return false, 0, ""
	}
	if err != nil {
		return false, 0, ""
	}
	running, pid := svc.IsRunning()
	return running, pid, svc.LogFilePath
}
