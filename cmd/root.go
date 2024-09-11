package cmd

import (
	"fmt"
	"github.com/desmos-labs/cosmos-go-wallet/wallet"
	"github.com/spf13/cobra"
	"ipfsUploader/core"
	"ipfsUploader/jackal/uploader"
)

func RootCMD(q *uploader.Queue, w *wallet.Wallet) *cobra.Command {
	c := &cobra.Command{
		Use:   "liftoff",
		Short: "Liftoff is a command line application for posting files to Jackal and hosting them on IPFS.",
		Long:  `Liftoff is a command line application for posting files to Jackal and hosting them on IPFS. These files are organized in folders as they appear on disk using IPFS folder nodes saved as virtual files on Jackal.`,
	}

	c.AddCommand(LaunchCMD(q, w))

	return c

}

func LaunchCMD(q *uploader.Queue, w *wallet.Wallet) *cobra.Command {
	return &cobra.Command{
		Use:   "launch [dir]",
		Short: "Starts the upload process.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {

			dir := args[0]

			cid, _, err := core.PostDir(dir, q, w)
			if err != nil {
				return err
			}

			fmt.Printf("Lift Off! 🚀\n\nYou can now view your files at ipfs://%s\n", cid)

			return nil
		},
	}
}
