package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/charmbracelet/lipgloss"
	"github.com/esteban-ams/fastship/internal/config"
	"github.com/spf13/cobra"
)

var (
	passStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true)
	failStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true)
	warnStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Bold(true)
	nameStyle = lipgloss.NewStyle().Width(40)
)

func pass(label string) {
	fmt.Printf("  %s %s\n", passStyle.Render("PASS"), nameStyle.Render(label))
}

func fail(label string) {
	fmt.Printf("  %s %s\n", failStyle.Render("FAIL"), nameStyle.Render(label))
}

func warn(label string) {
	fmt.Printf("  %s %s\n", warnStyle.Render("WARN"), nameStyle.Render(label))
}

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check system requirements and configuration",
	RunE:  runDoctor,
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}

func runDoctor(cmd *cobra.Command, args []string) error {
	fmt.Println()
	fmt.Println("FastShip Doctor")
	fmt.Println("===============")
	fmt.Println()

	allOk := true

	// 1. Docker CLI
	if _, err := exec.LookPath("docker"); err != nil {
		fail("Docker CLI available")
		allOk = false
	} else {
		pass("Docker CLI available")
	}

	// 2. Docker daemon running
	if err := exec.Command("docker", "info").Run(); err != nil {
		fail("Docker daemon running")
		allOk = false
	} else {
		pass("Docker daemon running")
	}

	// 3. Docker Compose
	if err := exec.Command("docker", "compose", "version").Run(); err != nil {
		fail("Docker Compose available")
		allOk = false
	} else {
		pass("Docker Compose available")
	}

	// 4. Git (warn only — needed for build mode)
	if _, err := exec.LookPath("git"); err != nil {
		warn("Git available (needed for build mode)")
	} else {
		pass("Git available")
	}

	// 5. Config file
	cfg, cfgErr := config.Load(configPath)
	if cfgErr != nil {
		fail(fmt.Sprintf("Config file (%s)", configPath))
		fmt.Printf("         %s\n", cfgErr)
		allOk = false
	} else {
		pass(fmt.Sprintf("Config file (%s)", configPath))
	}

	// 6. Services configured
	if cfg != nil && len(cfg.Services) > 0 {
		pass(fmt.Sprintf("Services configured (%d)", len(cfg.Services)))

		// 7. Compose files exist
		for name, svc := range cfg.Services {
			if _, err := os.Stat(svc.ComposeFile); err != nil {
				fail(fmt.Sprintf("  %s: compose file (%s)", name, svc.ComposeFile))
				allOk = false
			} else {
				pass(fmt.Sprintf("  %s: compose file exists", name))
			}
		}
	} else if cfg != nil {
		fail("Services configured")
		allOk = false
	}

	fmt.Println()
	if allOk {
		fmt.Println(passStyle.Render("All checks passed!"))
	} else {
		fmt.Println(failStyle.Render("Some checks failed. Fix the issues above."))
	}
	fmt.Println()

	return nil
}
