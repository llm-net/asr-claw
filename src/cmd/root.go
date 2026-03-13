package cmd

import (
	"github.com/spf13/cobra"
)

// Version is set at build time via ldflags.
var Version = "dev"

var (
	outputMode string
	timeout    int
	verbose    bool
)

var rootCmd = &cobra.Command{
	Use:   "asr-claw",
	Short: "Speech recognition CLI for AI agent automation",
	Long:  "asr-claw — transcribe audio from stdin, files, or URLs with multiple ASR engines.",
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&outputMode, "output", "o", "json", "output format: json | text | quiet")
	rootCmd.PersistentFlags().IntVar(&timeout, "timeout", 60000, "command timeout in milliseconds")
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "enable debug output to stderr")

	rootCmd.AddCommand(versionCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version",
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Println(Version)
	},
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}
