package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

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
	// Search parameters
	pattern     string
	sincePeriod string
	tailLines   int

	// Styling
	styles = struct {
		containerHeader lipgloss.Style
		resultLine      lipgloss.Style
		separator       lipgloss.Style
	}{
		containerHeader: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#00FF00")).
			Align(lipgloss.Center),
		resultLine: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")),
		separator: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666666")),
	}

	// Cobra root command
	rootCmd = &cobra.Command{
		Use:   "scl [pattern]",
		Short: "Search Container Logs - search through all running Docker container logs",
		Long:  `Search Container Logs (scl) allows you to search through the logs of all running Docker containers for a specific pattern.`,
		Example: `  scl "App not found"
  scl "App not found" --since 1h
  scl "App not found" --tail 100`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pattern = args[0]
			return runSearch()
		},
	}
)

func init() {
	rootCmd.Flags().StringVarP(&sincePeriod, "since", "s", "", "Show logs since duration (e.g., 1h, 30m, 24h)")
	rootCmd.Flags().IntVarP(&tailLines, "tail", "t", 0, "Number of lines to show from the end of logs (0 for all)")
	rootCmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if sincePeriod != "" {
			if !strings.HasSuffix(sincePeriod, "h") &&
				!strings.HasSuffix(sincePeriod, "m") &&
				!strings.HasSuffix(sincePeriod, "s") {
				return fmt.Errorf("invalid time format for --since flag. Use h for hours, m for minutes, s for seconds (e.g., 1h, 30m, 24h)")
			}
		}
		if tailLines < 0 {
			return fmt.Errorf("tail lines must be >= 0")
		}
		return nil
	}
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runSearch() error {
	start := time.Now()
	containers, err := getContainers()
	if err != nil {
		return err
	}

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

	go func() {
		wg.Wait()
		close(results)
	}()

	containerResults := make(map[string][]Result)
	totalMatches := 0
	for result := range results {
		containerResults[result.ContainerName] = append(
			containerResults[result.ContainerName],
			result,
		)
		totalMatches++
	}

	// Print results
	for containerName, results := range containerResults {
		header := fmt.Sprintf("%s (%d matches)", containerName, len(results))
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

	duration := time.Since(start)
	roundedDuration := duration.Round(time.Millisecond)
	fmt.Printf("\nSearch completed in %v with %d total matches\n", roundedDuration, totalMatches)

	return nil
}

func searchContainerLogs(containerID, containerName, pattern string, results chan<- Result) {
	cmdArgs := getDockerArgs(containerID)
	cmd := exec.Command("docker", cmdArgs...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	if err := cmd.Start(); err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// Let use all our CPU cores
	numWorkers := runtime.NumCPU()
	lines := make(chan string)
	var scanWg sync.WaitGroup

	for i := 0; i < numWorkers; i++ {
		scanWg.Add(1)
		// Goroutines go brrrrr
		go func() {
			defer scanWg.Done()
			lineNum := 1
			for line := range lines {
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
		}()
	}

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		lines <- scanner.Text()
	}
	close(lines)

	scanWg.Wait()
	cmd.Wait()
}

func getDockerArgs(containerID string) []string {
	cmdArgs := []string{"logs"}
	if sincePeriod != "" {
		cmdArgs = append(cmdArgs, "--since", sincePeriod)
	}
	if tailLines > 0 {
		cmdArgs = append(cmdArgs, "--tail", fmt.Sprintf("%d", tailLines))
	}
	cmdArgs = append(cmdArgs, containerID)
	return cmdArgs
}

func getContainers() ([]string, error) {
	cmd := exec.Command("docker", "ps", "--format", "{{.ID}}:{{.Names}}")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("error listing containers: %v", err)
	}
	return strings.Split(strings.TrimSpace(string(output)), "\n"), nil
}
