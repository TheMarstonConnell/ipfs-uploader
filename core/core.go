package core

import (
	"encoding/json"
	"fmt"
	cosmoWallet "github.com/desmos-labs/cosmos-go-wallet/wallet"
	ipfslite "github.com/hsanjuan/ipfs-lite"
	"github.com/ipfs/boxo/ipld/merkledag"
	"github.com/ipfs/go-cid"
	"ipfsUploader/jackal/uploader"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"time"
)

import (
	"github.com/ipfs/boxo/ipld/unixfs"

	ipldFormat "github.com/ipfs/go-ipld-format"
	"github.com/rs/zerolog/log"
)

func PostFile(filePath string, q *uploader.Queue, w *cosmoWallet.Wallet, peer *ipfslite.Peer) (string, []byte, error) {
	file, err := os.ReadFile(filePath)
	if err != nil {
		return "", nil, err
	}

	log.Printf("Posting: %s", filePath)
	c, r, err := uploader.PostFile(filePath, file, q, w, false, peer)
	if err != nil {
		return "", nil, err
	}

	log.Print(c)

	return c[0], r, err
}

func PostDir(dirPath string, q *uploader.Queue, w *cosmoWallet.Wallet, peer *ipfslite.Peer) (string, []byte, error) {

	directory, err := os.ReadDir(dirPath)
	if err != nil {
		return "", nil, err
	}

	var files sync.Map

	var wg sync.WaitGroup

	k := 0

	for _, entry := range directory {
		time.Sleep(time.Millisecond * 50)
		for k > 40 {
			time.Sleep(time.Second * 6)
		}

		if strings.HasPrefix(entry.Name(), ".") {

			continue
		}

		if entry.IsDir() {
			wg.Add(1)
			go func() {
				log.Printf("Entering Dir: %s", entry.Name())
				newDir := path.Join(dirPath, entry.Name())
				folderCID, _, err := PostDir(newDir, q, w, peer)
				if err != nil {
					panic(err)
				}
				files.Store(entry.Name(), folderCID)
				wg.Done()
			}()
			continue
		}

		fileName := entry.Name()
		toRead := path.Join(dirPath, fileName)

		wg.Add(1)

		k++

		go func() {
			defer wg.Done()
			l := 0
			for l == 0 {

				data, err := os.ReadFile(toRead)
				if err != nil {
					return
				}

				success := false

				for !success {
					log.Printf("Posting: %s", fileName)
					c, _, err := uploader.PostFile(fileName, data, q, w, false, peer)
					if err != nil {
						log.Print(err)
						continue
					}

					l = len(c)

					if l > 0 {
						files.Store(fileName, c[0])
					}

					success = true
				}

				break

			}

			k--

		}()

	}

	f := false

	go func() {
		for !f {
			log.Printf("Still waiting...")
			time.Sleep(time.Second * 10)
		}
	}()

	wg.Wait()
	f = true

	log.Printf("Done waiting!")

	childCIDs := make(map[string]cid.Cid)
	fs := make(map[string]string)
	files.Range(func(key, s any) bool {
		fs[key.(string)] = s.(string)
		c, err := cid.Parse(s)
		if err != nil {
			return true
		}

		childCIDs[key.(string)] = c
		return true
	})

	log.Printf("Folder CID Generation")

	fileMap, err := json.MarshalIndent(fs, "", "    ")
	if err != nil {
		return "", nil, err
	}
	parent := filepath.Base(dirPath)
	err = os.WriteFile(fmt.Sprintf("%s.json", parent), fileMap, os.ModePerm)
	if err != nil {
		return "", nil, err
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

	c, merkle, err := uploader.PostFile(dirPath, rawData, q, w, true, peer)
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
