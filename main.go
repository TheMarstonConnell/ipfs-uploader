package main

import (
	"github.com/desmos-labs/cosmos-go-wallet/types"
	"github.com/joho/godotenv"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"ipfsUploader/cmd"
	"ipfsUploader/jackal/uploader"
	"ipfsUploader/jackal/wallet"
	"os"
)

func init() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	log.Logger = log.With().Caller().Logger()
	log.Logger = log.Level(zerolog.TraceLevel)
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal().Msg("Error loading .env file")
	}

	seed := os.Getenv("SEED")
	w, err := wallet.CreateWallet(seed, "m/44'/118'/0'/0/0", types.ChainConfig{
		Bech32Prefix:  "jkl",
		RPCAddr:       os.Getenv("RPC"),
		GRPCAddr:      os.Getenv("GRPC"),
		GasPrice:      "0.02ujkl",
		GasAdjustment: 1.5,
	})
	if err != nil {
		panic(err)
	}

	log.Printf(w.AccAddress())

	q := uploader.NewQueue(w)
	q.Listen()

	root := cmd.RootCMD(q, w)

	err = root.Execute()
	if err != nil {
		panic(err)
	}

}
