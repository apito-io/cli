package main

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/manifoldco/promptui"
)

const (
	portChoiceStop = "stop"
	portChoiceKill = "kill"
	portChoiceSkip = "skip"
)

type studioPortHold struct {
	Kind  string
	Name  string
	Ports []int
	PIDs  []string
}

var dockerPublishedPort = regexp.MustCompile(`(?:0\.0\.0\.0|\[::\]|127\.0\.0\.1|\*):(\d+)->`)

func ensureStudioPortsFree() error {
	holds, err := collectStudioPortHolds([]int{atoiPort(studioPublishPort), atoiPort(studioEnginePort)})
	if err != nil {
		return err
	}
	return freeStudioPortHolds(holds, askStudioPortConflict, stopStudioPortHold)
}

func atoiPort(value string) int {
	port, _ := strconv.Atoi(value)
	return port
}

func tcpPortBusy(port int) bool {
	listener, err := net.Listen("tcp", ":"+strconv.Itoa(port))
	if err != nil {
		return true
	}
	_ = listener.Close()
	return false
}

func collectStudioPortHolds(ports []int) ([]studioPortHold, error) {
	var busy []int
	for _, port := range ports {
		if tcpPortBusy(port) {
			busy = append(busy, port)
			continue
		}
		print_status(fmt.Sprintf("Port %d is free", port))
	}
	if len(busy) == 0 {
		return nil, nil
	}

	containers := publishedContainersByPort()
	grouped := map[string]*studioPortHold{}
	var order []string
	claimed := map[int]bool{}
	for _, port := range busy {
		containerName, ok := containers[port]
		if !ok {
			continue
		}
		hold, exists := grouped[containerName]
		if !exists {
			hold = &studioPortHold{Kind: "container", Name: containerName}
			grouped[containerName] = hold
			order = append(order, containerName)
		}
		hold.Ports = append(hold.Ports, port)
		claimed[port] = true
	}

	holds := make([]studioPortHold, 0, len(order)+1)
	for _, name := range order {
		holds = append(holds, *grouped[name])
	}

	var processPorts []int
	var pids []string
	seenPID := map[string]bool{}
	for _, port := range busy {
		if claimed[port] {
			continue
		}
		processPorts = append(processPorts, port)
		found, _ := pidsForPort(port)
		for _, pid := range found {
			pid = strings.TrimSpace(pid)
			if pid == "" || seenPID[pid] {
				continue
			}
			seenPID[pid] = true
			pids = append(pids, pid)
		}
	}
	if len(processPorts) > 0 {
		name := "another process"
		if len(pids) > 0 {
			name = "pid " + strings.Join(pids, ", ")
		}
		holds = append(holds, studioPortHold{Kind: "process", Name: name, Ports: processPorts, PIDs: pids})
	}
	return holds, nil
}

func publishedContainersByPort() map[int]string {
	cmd := exec.Command("docker", "ps", "--format", "{{.Names}}\t{{.Ports}}")
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	found := map[int]string{}
	for _, line := range strings.Split(string(out), "\n") {
		parts := strings.SplitN(strings.TrimSpace(line), "\t", 2)
		if len(parts) != 2 || parts[0] == "" {
			continue
		}
		for _, port := range parsePublishedPorts(parts[1]) {
			if _, exists := found[port]; !exists {
				found[port] = parts[0]
			}
		}
	}
	return found
}

func parsePublishedPorts(field string) []int {
	seen := map[int]bool{}
	var ports []int
	for _, match := range dockerPublishedPort.FindAllStringSubmatch(field, -1) {
		port, err := strconv.Atoi(match[1])
		if err != nil || seen[port] {
			continue
		}
		seen[port] = true
		ports = append(ports, port)
	}
	sort.Ints(ports)
	return ports
}

func studioPortWarning(hold studioPortHold) string {
	ports := make([]string, 0, len(hold.Ports))
	for _, port := range hold.Ports {
		ports = append(ports, strconv.Itoa(port))
	}
	target := hold.Name
	if hold.Kind == "container" {
		target = "container " + hold.Name
	}
	joined := strings.Join(ports, " and ")
	message := fmt.Sprintf("Port %s is busy (%s).", joined, target)
	if len(ports) > 1 {
		message = fmt.Sprintf("Ports %s are busy (%s).", joined, target)
	}
	if containsPort(hold.Ports, atoiPort(studioEnginePort)) {
		message += " The engine API on " + studioEnginePort + " will not run."
	}
	if containsPort(hold.Ports, atoiPort(studioPublishPort)) {
		message += " The console on " + studioPublishPort + " will not open."
	}
	return message
}

func containsPort(ports []int, want int) bool {
	for _, port := range ports {
		if port == want {
			return true
		}
	}
	return false
}

func askStudioPortConflict(hold studioPortHold) (string, error) {
	print_warning(studioPortWarning(hold))
	prompt := promptui.Select{
		Label: "Stop the old service, kill it, or skip start",
		Items: []string{
			"Stop the old service and continue",
			"Kill whatever is using the port",
			"Skip start",
		},
	}
	_, choice, err := prompt.Run()
	if err != nil {
		print_status("Port prompt unavailable. Start skipped.")
		return portChoiceSkip, err
	}
	switch choice {
	case "Stop the old service and continue":
		return portChoiceStop, nil
	case "Kill whatever is using the port":
		return portChoiceKill, nil
	default:
		return portChoiceSkip, nil
	}
}

func freeStudioPortHolds(holds []studioPortHold, ask func(studioPortHold) (string, error), stop func(studioPortHold, string) error) error {
	for _, hold := range holds {
		choice, err := ask(hold)
		if err != nil || choice == portChoiceSkip {
			return fmt.Errorf("start skipped: %s", studioPortWarning(hold))
		}
		if err := stop(hold, choice); err != nil {
			return err
		}
	}
	return nil
}

func stopStudioPortHold(hold studioPortHold, choice string) error {
	switch choice {
	case portChoiceStop:
		if hold.Kind == "container" {
			if err := runDocker("stop", hold.Name); err != nil {
				return fmt.Errorf("stop %s: %w", hold.Name, err)
			}
		} else if err := killHoldProcesses(hold, false); err != nil {
			return err
		}
	case portChoiceKill:
		if hold.Kind == "container" {
			if err := runDocker("kill", hold.Name); err != nil {
				return fmt.Errorf("kill %s: %w", hold.Name, err)
			}
		} else if err := killHoldProcesses(hold, true); err != nil {
			return err
		}
	default:
		return fmt.Errorf("start skipped: %s", studioPortWarning(hold))
	}

	deadline := time.Now().Add(5 * time.Second)
	for {
		if studioPortsFree(hold.Ports) {
			for _, port := range hold.Ports {
				print_success(fmt.Sprintf("Port %d is free", port))
			}
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("port still busy after stopping %s", hold.Name)
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func killHoldProcesses(hold studioPortHold, force bool) error {
	if len(hold.PIDs) == 0 {
		for _, port := range hold.Ports {
			if err := killProcessOnPort(port); err != nil {
				return err
			}
		}
		return nil
	}
	signal := "-TERM"
	if force {
		signal = "-KILL"
	}
	for _, pid := range hold.PIDs {
		if err := exec.Command("kill", signal, pid).Run(); err != nil {
			return fmt.Errorf("kill %s: %w", pid, err)
		}
	}
	return nil
}

func studioPortsFree(ports []int) bool {
	for _, port := range ports {
		if tcpPortBusy(port) {
			return false
		}
	}
	return true
}

type urlProbe struct {
	Status int
	Server string
}

func waitForStudioReady() error {
	client := &http.Client{
		Timeout: 2 * time.Second,
		Transport: &http.Transport{
			Proxy: nil,
		},
	}
	print_status("Checking http://localhost:" + studioPublishPort)
	print_status("Checking http://localhost:" + studioPublishPort + "/admin")
	deadline := time.Now().Add(30 * time.Second)
	var last error
	for {
		last = checkStudioReady(client)
		if last == nil {
			print_success("Console answered at http://localhost:" + studioPublishPort)
			print_success("Admin answered at http://localhost:" + studioPublishPort + "/admin")
			print_success("Engine answered at http://localhost:" + studioEnginePort)
			return nil
		}
		if time.Now().After(deadline) {
			return last
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func checkStudioReady(client *http.Client) error {
	console, consoleErr := probeURL(client, "http://127.0.0.1:"+studioPublishPort+"/")
	admin, adminErr := probeURL(client, "http://127.0.0.1:"+studioPublishPort+"/admin")
	engine, engineErr := probeURL(client, "http://127.0.0.1:"+studioEnginePort+"/")
	return studioReadyError(console, consoleErr, admin, adminErr, engine, engineErr)
}

func probeURL(client *http.Client, rawURL string) (urlProbe, error) {
	response, err := client.Get(rawURL)
	if err != nil {
		return urlProbe{}, err
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, response.Body)
	return urlProbe{Status: response.StatusCode, Server: response.Header.Get("Server")}, nil
}

func studioReadyError(console urlProbe, consoleErr error, admin urlProbe, adminErr error, engine urlProbe, engineErr error) error {
	consoleURL := "http://localhost:" + studioPublishPort
	adminURL := consoleURL + "/admin"
	engineURL := "http://localhost:" + studioEnginePort
	if consoleErr != nil && engineErr == nil && strings.Contains(strings.ToLower(engine.Server), "nginx") {
		return fmt.Errorf("%s did not answer. Port %s is serving nginx, so the engine cannot bind and the API will not run", consoleURL, studioEnginePort)
	}
	if consoleErr != nil {
		return fmt.Errorf("%s did not answer: %w", consoleURL, consoleErr)
	}
	if !acceptableConsoleStatus(console.Status) {
		return fmt.Errorf("%s returned HTTP %d", consoleURL, console.Status)
	}
	if adminErr != nil {
		return fmt.Errorf("%s did not answer: %w", adminURL, adminErr)
	}
	if !acceptableAdminStatus(admin.Status) {
		return fmt.Errorf("%s returned HTTP %d. The engine API behind the console is not ready", adminURL, admin.Status)
	}
	if engineErr != nil {
		return fmt.Errorf("%s did not answer: %w", engineURL, engineErr)
	}
	if engine.Status >= 500 {
		return fmt.Errorf("%s returned HTTP %d", engineURL, engine.Status)
	}
	return nil
}

func acceptableConsoleStatus(status int) bool {
	return status >= 200 && status < 400
}

func acceptableAdminStatus(status int) bool {
	return status > 0 && status != http.StatusBadGateway && status != http.StatusServiceUnavailable && status != http.StatusGatewayTimeout
}
