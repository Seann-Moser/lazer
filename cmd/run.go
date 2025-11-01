/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/Seann-Moser/lazer/pkg/controller"
	"github.com/spf13/cobra"
)

// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:   "run",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
		c, err := controller.New(false)
		if err != nil {
			return
		}
		defer c.Close()
		ctx, cancel := context.WithCancel(cmd.Context())
		go func() {
			<-sigs
			cancel()
		}()
		c.Run(ctx)

		fmt.Println("lazer command finished")
	},
}

func init() {
	rootCmd.AddCommand(runCmd)
}
