package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

type Result struct {
	ContainerID   string
	ContainerName string
	Line          string
	LineNum       int
}

var (
	// Styling
	pattern string
	styles  = struct {
		containerHeader lipgloss.Style
		resultLine      lipgloss.Style
		separator       lipgloss.Style
	}{
		containerHeader: lipgloss.NewStyle().
			Bold(true).
			// Green
			Foreground(lipgloss.Color("#00FF00")).Align(lipgloss.Center).
			Padding(0, 1),
		resultLine: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")),
		separator: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666666")),
	}
	// Cobra root command
	rootCmd = &cobra.Command{
		Use:     "scl [pattern]",
		Short:   "Search Container Logs - search through all running Docker container logs",
		Long:    `Search Container Logs (scl) allows you to search through the logs of all running Docker containers for a specific pattern.`,
		Example: `scl "error in database"`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pattern = args[0]
			return runSearch()
		},
	}
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runSearch() error {
	cmd := exec.Command("docker", "ps", "--format", "{{.ID}}:{{.Names}}")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("error listing containers: %v", err)
	}

	containers := strings.Split(strings.TrimSpace(string(output)), "\n")
	results := make(chan Result)
	var wg sync.WaitGroup

	for _, container := range containers {
		if container == "" {
			continue
		}
		parts := strings.Split(container, ":")
		containerID, containerName := parts[0], parts[1]

		wg.Add(1)
		go func(id, name string) {
			defer wg.Done()
			searchContainerLogs(id, name, pattern, results)
		}(containerID, containerName)
	}

	// Create a map to group results by container
	containerResults := make(map[string][]Result)

	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect and group results
	for result := range results {
		containerResults[result.ContainerName] = append(
			containerResults[result.ContainerName],
			result,
		)
	}

	// Print grouped results
	for containerName, results := range containerResults {
		header := fmt.Sprintf("%s (%d matches)",
			containerName, len(results))
		fmt.Println(styles.containerHeader.Render(header))

		for _, result := range results {
			line := fmt.Sprintf("[%s] Line %d: %s",
				result.ContainerID[:12],
				result.LineNum,
				result.Line)
			fmt.Println(styles.resultLine.Render(line))
		}
		fmt.Println(styles.separator.Render(strings.Repeat("-", 60)))
	}

	return nil
}

func searchContainerLogs(containerID, containerName, pattern string, results chan<- Result) {
	cmd := exec.Command("docker", "logs", containerID)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Printf("Error getting logs for container %s: %v\n", containerID[:12], err)
		return
	}

	if err := cmd.Start(); err != nil {
		fmt.Printf("Error starting logs command for container %s: %v\n", containerID[:12], err)
		return
	}

	scanner := bufio.NewScanner(stdout)
	lineNum := 1

	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, pattern) {
			results <- Result{
				ContainerID:   containerID,
				ContainerName: containerName,
				Line:          line,
				LineNum:       lineNum,
			}
		}
		lineNum++
	}

	if err := cmd.Wait(); err != nil {
		fmt.Printf("Error waiting for logs command for container %s: %v\n", containerID[:12], err)
	}
}
