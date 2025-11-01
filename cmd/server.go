package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/Seann-Moser/lazer/pkg/controller"
	"github.com/spf13/cobra"
)

// runCmd represents the run coxm
var serverCmd = &cobra.Command{
	Use:   "serve",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
		c, err := controller.New(true)
		if err != nil {
			return
		}
		defer c.Close()
		ctx, cancel := context.WithCancel(cmd.Context())
		wg := sync.WaitGroup{}
		wg.Add(1)
		go func() {
			<-sigs
			cancel()
			wg.Done()
		}()
		go func() {
			c.StartServer(ctx)
		}()

		wg.Wait()
		fmt.Println("lazer command finished")
	},
}

func init() {
	rootCmd.AddCommand(serverCmd)
}
