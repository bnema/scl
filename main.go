package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/spf13/cobra"
)

type Result struct {
	ContainerID   string
	ContainerName string
	Line          string
	LineNum       int
}

var (
	pattern string
	rootCmd = &cobra.Command{
		Use:   "sacl [pattern]",
		Short: "Search All Container Logs - search through all running Docker container logs",
		Long: `Search All Container Logs (sacl) allows you to search through the logs of all running Docker containers 
for a specific pattern.`,
		Example: `  sacl "error in database"
  sacl "connection refused"`,
		Args: cobra.ExactArgs(1),
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
	// Get list of running containers
	cmd := exec.Command("docker", "ps", "--format", "{{.ID}}:{{.Names}}")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("error listing containers: %v", err)
	}

	containers := strings.Split(strings.TrimSpace(string(output)), "\n")
	results := make(chan Result)
	var wg sync.WaitGroup

	// Search logs for each container concurrently
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

	// Close results channel when all searches are done
	go func() {
		wg.Wait()
		close(results)
	}()

	// Print results as they come in
	for result := range results {
		fmt.Printf("[%s:%s] Line %d: %s\n",
			result.ContainerID[:12],
			result.ContainerName,
			result.LineNum,
			result.Line)
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
