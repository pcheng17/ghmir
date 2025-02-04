package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
    "strings"

    "github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type GitLabConfig struct {
	TokenName  string `yaml:"token_name"`
	Token      string `yaml:"token"`
	GroupName  string `yaml:"group_name"`
}

type EntityConfig struct {
	GitHubToken string       `yaml:"github_token"`
    Type        string       `yaml:"type"`
	GitLab      GitLabConfig `yaml:"gitlab"`
}

type Config struct {
	BackupDir        string                  `yaml:"backup_dir"`
	DiscordWebhook   string                  `yaml:"discord_webhook"`
	Entities         map[string]EntityConfig `yaml:"entities"`
}

func loadConfig(configFile string, requestedEntities []string) (*Config, error) {
	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Validate requested entities exist in config
	for _, entity := range requestedEntities {
		entityConfig, exists := config.Entities[entity]
		if !exists {
			return nil, fmt.Errorf("configuration for %s not found", entity)
		}

		// Validate entity configuration
		if entityConfig.GitHubToken == "" {
			return nil, fmt.Errorf("github_token not set for %s", entity)
		}
		if entityConfig.GitLab.Token == "" || entityConfig.GitLab.TokenName == "" || entityConfig.GitLab.GroupName == "" {
			return nil, fmt.Errorf("incomplete gitlab configuration for %s", entity)
		}
		if entityConfig.Type != "user" && entityConfig.Type != "org" {
			return nil, fmt.Errorf("invalid type for %s: must be 'user' or 'org'", entity)
		}
	}

	return &config, nil
}

func sendDiscordMessage(message, webhookURL string) error {
	if webhookURL == "" {
		return nil
	}

	payload := fmt.Sprintf(`{"content":"%s", "username":"GitHub Mirror Bot"}`, message)
	cmd := exec.Command("curl",
		"-H", "Content-Type: application/json",
		"-d", payload,
		"-X", "POST",
		webhookURL)

	return cmd.Run()
}

func mirrorToGitlab(repoPath, gitlabUser, gitlabToken, gitlabGroup string) error {
    repoName := filepath.Base(repoPath)
	log.Printf("Mirroring %s to GitLab under the group %s...", repoName, gitlabGroup)

	if err := os.Chdir(repoPath); err != nil {
		return fmt.Errorf("failed to change to repo directory: %w", err)
	}

	remoteURL := fmt.Sprintf("https://%s:%s@gitlab.com/%s/%s.git",
		gitlabUser, gitlabToken, gitlabGroup, repoName)

	addRemote := exec.Command("git", "remote", "add", "gitlab", remoteURL)
	if err := addRemote.Run(); err != nil {
		setRemote := exec.Command("git", "remote", "set-url", "gitlab", remoteURL)
		if err := setRemote.Run(); err != nil {
			return fmt.Errorf("failed to set remote: %w", err)
		}
	}

	configs := [][]string{
		{"config", "--local", "--replace", "remote.origin.fetch", "+refs/heads/*:refs/heads/*"},
		{"config", "--local", "--add", "remote.origin.fetch", "+refs/tags/*:refs/tags/*"},
		{"config", "--local", "--add", "remote.origin.fetch", "+refs/change/*:refs/change/*"},
		{"config", "--local", "--replace", "remote.gitlab.push", "+refs/heads/*:refs/heads/*"},
		{"config", "--local", "--add", "remote.gitlab.push", "+refs/tags/*:refs/tags/*"},
		{"config", "--local", "--add", "remote.gitlab.push", "+refs/change/*:refs/change/*"},
	}

	for _, args := range configs {
		cmd := exec.Command("git", args...)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to configure git: %w", err)
		}
	}

	push := exec.Command("git", "push", "--mirror", "gitlab")
	if err := push.Run(); err != nil {
		return fmt.Errorf("failed to push to gitlab: %w", err)
	}

	log.Printf("Successfully mirrored %s to GitLab", repoName)
	return nil
}

func processRepositories(entityName string, entityConfig EntityConfig, backupDir, webhookURL string) error {
	directory := filepath.Join(backupDir, entityName+"_backup")
    absDirectory, err := filepath.Abs(directory)
    if err != nil {
        return fmt.Errorf("failed to get absolute path: %w", err)
    }

	log.Printf("Processing %s repositories in %s...", entityName, directory)

	if _, err := os.Stat(directory); os.IsNotExist(err) {
		return fmt.Errorf("directory %s not found", directory)
	}

	entries, err := os.ReadDir(directory)
	if err != nil {
		return fmt.Errorf("failed to read directory: %w", err)
	}

	var totalRepos, failedRepos int
	var failedList []string

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		totalRepos++
        repoPath := filepath.Join(absDirectory, entry.Name())
		repoName := entry.Name()

		if err := mirrorToGitlab(repoPath,
			entityConfig.GitLab.TokenName,
			entityConfig.GitLab.Token,
			entityConfig.GitLab.GroupName); err != nil {
			failedRepos++
			failedList = append(failedList, repoName)
			log.Printf("Error mirroring %s: %v", repoName, err)
		}

        os.Chdir("..")
	}

	summary := fmt.Sprintf("[%s] %d repositories mirrored, %d error(s) occurred.",
		entityName, totalRepos, failedRepos)
	if len(failedList) > 0 {
		summary += fmt.Sprintf("\nFailed repos: %v", failedList)
	}

    if webhookURL != "" {
        if err := sendDiscordMessage(summary, webhookURL); err != nil {
            log.Printf("Failed to send Discord message: %v", err)
        }
    }
	return nil
}

var (
    configFile string
    backupDir string
    entities string
    doPush bool
    version = "dev"
)

var rootCmd = &cobra.Command{
    Use: "ghmir",
    Short: "Mirror GitHub repositories to GitLab",
    RunE: func(cmd *cobra.Command, args []string) error {
        // Validate required flags
        if backupDir == "" {
            return fmt.Errorf("--path flag is required")
        }
        if entities == "" {
            return fmt.Errorf("--entities flag is required")
        }

        entityList := strings.Split(entities, ",")
        for i, entity := range entityList {
            entityList[i] = strings.TrimSpace(entity)
            if entityList[i] == "" {
                return fmt.Errorf("empty entity name in entities list")
            }
        }

        expandedBackupDir := os.ExpandEnv(backupDir)
        if err := os.MkdirAll(expandedBackupDir, 0755); err != nil {
            return fmt.Errorf("failed to create backup directory: %w", err)
        }

        expandedConfigFile := os.ExpandEnv(configFile)
        config, err := loadConfig(expandedConfigFile, entityList)
        if err != nil {
            return fmt.Errorf("failed to load config: %w", err)
        }

        for _, entity := range entityList {
            entityConfig := config.Entities[entity]

            // Clone repositories using ghorg
            cmd := exec.Command("ghorg", "clone", entity,
                "--clone-type=" + entityConfig.Type,
                "--token=" + entityConfig.GitHubToken,
                "--backup",
                "--include-submodules",
                "--path=" + expandedBackupDir)

            // Capture both stdout and stderr
            cmd.Stdout = os.Stdout
            cmd.Stderr = os.Stderr

            if err := cmd.Run(); err != nil {
                log.Printf("Warning: Failed to clone %s repositories: %v", entity, err)
                continue
            }

            if doPush {
                // Process repositories
                if err := processRepositories(
                    entity,
                    entityConfig,
                    expandedBackupDir,
                    config.DiscordWebhook); err != nil {
                    log.Printf("Error processing %s repositories: %v", entity, err)
                }
            }
        }

        return nil
    },
}

func init() {
    rootCmd.Version = version
    rootCmd.Flags().StringVarP(&configFile, "config", "", "$HOME/.config/ghmir/config.yaml", "File containing configurations and secrets")
    rootCmd.Flags().StringVarP(&backupDir, "path", "", "", "Backup directory path")
    rootCmd.Flags().StringVarP(&entities, "entities", "", "", "Comma-separated list of entities to mirror")
    rootCmd.Flags().BoolVarP(&doPush, "push", "", false, "Push to GitLab")

    // Mark required flags
    rootCmd.MarkFlagRequired("path")
    rootCmd.MarkFlagRequired("entities")
}

func main() {
    if err := rootCmd.Execute(); err != nil {
        log.Fatalf("Error: %v", err)
        os.Exit(1)
    }
}
