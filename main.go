package main

import (
	"github.com/desmos-labs/cosmos-go-wallet/types"
	"github.com/ipfs/boxo/ipld/merkledag"
	"github.com/ipfs/go-cid"
	"github.com/joho/godotenv"
	"github.com/rs/zerolog"
	"ipfsUploader/jackal/uploader"
	"ipfsUploader/jackal/wallet"
	"os"
)

import (
	"github.com/ipfs/boxo/ipld/unixfs"

	ipldFormat "github.com/ipfs/go-ipld-format"
	"github.com/rs/zerolog/log"
)

func init() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	log.Logger = log.With().Caller().Logger()
	log.Logger = log.Level(zerolog.DebugLevel)
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

	files := map[string]string{
		"1": "bafybeihdxnfgloqg26ztcaawxijkbibgyt6uykhhdtyk4spuamub2tgnfm",
		"2": "bafybeia6dhanrmza6pidqt6abieejmytcvf6wtlhqshwb5hv2zvvfrdefy",
		"3": "bafybeieudrr6744fdpybgrxadl2elqcncvfakzqyipake5milgleihldre",
	}
	childCIDs := make(map[string]cid.Cid)
	for key, s := range files {
		c, err := cid.Parse(s)
		if err != nil {
			panic(err)
		}
		childCIDs[key] = c
	}

	n, err := GenIPFSFolderData(childCIDs)
	if err != nil {
		panic(err)
	}

	rawData, err := n.MarshalJSON()
	if err != nil {
		panic(err)
	}

	log.Printf("CID: %s", n.Cid().String())

	log.Printf("%x\n\n%s", rawData, string(rawData))

	q.Listen()

	c, merkle, err := uploader.PostFile(rawData, q, w, true)
	if err != nil {
		panic(err)
	}

	log.Printf("%v, %x", c, merkle)

}

func GenIPFSFolderData(childCIDs map[string]cid.Cid) (node *merkledag.ProtoNode, err error) {
	folderNode := unixfs.EmptyDirNode()

	for key, childCID := range childCIDs {
		// Create a link
		link := &ipldFormat.Link{
			Name: key,
			Cid:  childCID,
		}

		// Add the link to the folder node
		err := folderNode.AddRawLink(key, link)
		if err != nil {
			return nil, err
		}
	}

	return folderNode, nil
}
