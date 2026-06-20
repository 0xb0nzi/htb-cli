package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/0xb0nzi/htb-cli/config"
	"github.com/0xb0nzi/htb-cli/lib/challenge"
	"github.com/0xb0nzi/htb-cli/lib/utils"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var challengeCmd = &cobra.Command{
	Use:   "challenge",
	Short: "Download challenge files",
	Long:  "Resolves a challenge by name and downloads its files (zip, password 'hackthebox').",
	Run: func(cmd *cobra.Command, args []string) {
		name, err := cmd.Flags().GetString("name")
		if err != nil {
			config.GlobalConfig.Logger.Error("", zap.Error(err))
			os.Exit(1)
		}
		if strings.TrimSpace(name) == "" {
			fmt.Println("required flag 'name' not set")
			os.Exit(1)
		}
		out, err := cmd.Flags().GetString("output")
		if err != nil {
			config.GlobalConfig.Logger.Error("", zap.Error(err))
			os.Exit(1)
		}

		id, err := utils.SearchChallengeByName(name)
		if err != nil {
			config.GlobalConfig.Logger.Error("", zap.Error(err))
			os.Exit(1)
		}

		dest := out
		if strings.TrimSpace(dest) == "" {
			dest = filepath.Join(".", strings.ReplaceAll(id.Name, " ", "_")+".zip")
		}

		saved, err := challenge.Download(id.ID, dest)
		if err != nil {
			config.GlobalConfig.Logger.Error("", zap.Error(err))
			os.Exit(1)
		}
		fmt.Printf("Downloaded %s to %s\n", id.Name, saved)
	},
}

func init() {
	rootCmd.AddCommand(challengeCmd)
	challengeCmd.Flags().StringP("name", "c", "", "Challenge name")
	challengeCmd.Flags().StringP("output", "o", "", "Output path (default: ./<name>.zip)")
}
