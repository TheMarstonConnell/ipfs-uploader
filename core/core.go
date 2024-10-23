package core

import (
	cosmoWallet "github.com/desmos-labs/cosmos-go-wallet/wallet"
	"github.com/ipfs/boxo/ipld/merkledag"
	"github.com/ipfs/go-cid"
	"ipfsUploader/jackal/uploader"
	"os"
	"path"
	"strings"
	"sync"
)

import (
	"github.com/ipfs/boxo/ipld/unixfs"

	ipldFormat "github.com/ipfs/go-ipld-format"
	"github.com/rs/zerolog/log"
)

func PostFile(filePath string, q *uploader.Queue, w *cosmoWallet.Wallet) (string, []byte, error) {
	file, err := os.ReadFile(filePath)
	if err != nil {
		return "", nil, err
	}

	log.Printf("Posting: %s", filePath)
	c, r, err := uploader.PostFile(filePath, file, q, w, false)
	if err != nil {
		return "", nil, err
	}

	log.Print(c)

	return c[0], r, err

}

func PostDir(dirPath string, q *uploader.Queue, w *cosmoWallet.Wallet) (string, []byte, error) {
	directory, err := os.ReadDir(dirPath)
	if err != nil {
		return "", nil, err
	}

	files := make(map[string]string)

	var wg sync.WaitGroup

	for _, entry := range directory {
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		if entry.IsDir() {
			wg.Add(1)
			go func() {
				log.Printf("Entering Dir: %s", entry.Name())
				newDir := path.Join(dirPath, entry.Name())
				folderCID, _, err := PostDir(newDir, q, w)
				if err != nil {
					panic(err)
				}
				files[entry.Name()] = folderCID
				wg.Done()
			}()
			continue
		}

		fileName := entry.Name()

		log.Printf("Entry: %s", fileName)
		toRead := path.Join(dirPath, fileName)
		data, err := os.ReadFile(toRead)
		if err != nil {
			return "", nil, err
		}
		log.Printf("Successfully opened: %s", toRead)

		wg.Add(1)

		go func() {
			l := 0
			for l == 0 {

				log.Printf("Posting: %s", fileName)
				c, _, err := uploader.PostFile(fileName, data, q, w, false)
				if err != nil {
					panic(err)
				}

				l = len(c)

				if l > 0 {
					files[fileName] = c[0]
				}
			}

			wg.Done()
		}()

	}

	wg.Wait()

	childCIDs := make(map[string]cid.Cid)
	for key, s := range files {
		c, err := cid.Parse(s)
		if err != nil {
			return "", nil, err
		}
		childCIDs[key] = c
	}

	n, err := GenIPFSFolderData(childCIDs)
	if err != nil {
		return n.Cid().String(), nil, err
	}

	rawData, err := n.MarshalJSON()
	if err != nil {
		return n.Cid().String(), nil, err
	}

	log.Printf("CID: %s", n.Cid().String())

	log.Printf("%x\n\n%s", rawData, string(rawData))

	c, merkle, err := uploader.PostFile(dirPath, rawData, q, w, true)
	if err != nil {
		return n.Cid().String(), nil, err
	}

	log.Printf("%v, %x", c, merkle)

	return n.Cid().String(), merkle, err

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
