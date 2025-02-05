package uploader

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	ipfslite "github.com/hsanjuan/ipfs-lite"
	"github.com/rs/zerolog/log"
	"io"
	"math/rand"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/desmos-labs/cosmos-go-wallet/wallet"
	canine "github.com/jackalLabs/canine-chain/v3/app"
	"github.com/jackalLabs/canine-chain/v3/x/storage/types"
	"github.com/jackalLabs/canine-chain/v3/x/storage/utils"
)

//var blackList sync.Map

type ErrorResponse struct {
	Error string `json:"error"`
}

type IPFSResponse struct {
	Cid string `json:"cid"`
}

func uploadFile(ip string, r io.Reader, merkle []byte, start int64, address string, postType int64) (string, error) {

	//_, ok := blackList.Load(ip)
	//if ok {
	//	return "", fmt.Errorf("blacklisted")
	//}

	cli := http.DefaultClient
	cli.Timeout = time.Second * 120
	u, err := url.Parse(ip)
	if err != nil {
		return "", err
	}

	u = u.JoinPath("upload")

	var b bytes.Buffer
	writer := multipart.NewWriter(&b)
	defer writer.Close()

	err = writer.WriteField("sender", address)
	if err != nil {
		return "", err
	}

	err = writer.WriteField("merkle", hex.EncodeToString(merkle))
	if err != nil {
		return "", err
	}

	err = writer.WriteField("start", fmt.Sprintf("%d", start))
	if err != nil {
		return "", err
	}

	err = writer.WriteField("type", fmt.Sprintf("%d", postType))
	if err != nil {
		return "", err
	}

	fileWriter, err := writer.CreateFormFile("file", hex.EncodeToString(merkle))
	if err != nil {
		return "", err
	}

	_, err = io.Copy(fileWriter, r)
	if err != nil {
		return "", err
	}
	writer.Close()

	req, _ := http.NewRequest("POST", u.String(), &b)
	req.Header.Add("Content-Type", writer.FormDataContentType())

	res, err := cli.Do(req)
	if err != nil {
		return "", err
	}

	defer res.Body.Close()

	if res.StatusCode != 200 {

		var errRes ErrorResponse
		bodyBytes, err := io.ReadAll(res.Body)
		if err != nil {
			return "", err
		}

		err = json.Unmarshal(bodyBytes, &errRes)
		if err != nil {
			fmt.Printf("NOT JSON FROM %s! BODY: %s", ip, string(bodyBytes))
			return "", err
		}

		return "", fmt.Errorf("upload failed with code %d | %s", res.StatusCode, errRes.Error)
	}

	var ipfsRes IPFSResponse
	err = json.NewDecoder(res.Body).Decode(&ipfsRes)
	if err != nil {
		return "", err
	}

	return ipfsRes.Cid, nil
}

func PostFile(fileName string, fileData []byte, queue *Queue, w *wallet.Wallet, isFolder bool, peer *ipfslite.Peer) ([]string, []byte, error) {
	buf := bytes.NewBuffer(fileData)
	treeBuffer := bytes.NewBuffer(buf.Bytes())

	uploadBuffer := bytes.NewBuffer(buf.Bytes())

	n, err := peer.AddFile(context.Background(), uploadBuffer, nil)
	if err != nil {
		return nil, nil, err
	}

	c := []string{n.Cid().String()}

	//abci, err := w.Client.RPCClient.ABCIInfo(context.Background())
	//if err != nil {
	//	return nil, nil, err
	//}
	//_ = abci
	cl := types.NewQueryClient(w.Client.GRPCConn)

	params, err := cl.Params(context.Background(), &types.QueryParams{})
	if err != nil {
		return nil, nil, err
	}

	root, _, _, size, err := utils.BuildTree(treeBuffer, params.Params.ChunkSize)
	if err != nil {
		return nil, root, err
	}

	count := 0
	merkleRes, err := cl.AllFilesByMerkle(context.Background(), &types.QueryAllFilesByMerkle{
		Merkle: root,
	})
	if err == nil {
		count = len(merkleRes.Files)
	}

	if count > 0 {

		if len(merkleRes.Files[0].Proofs) > 0 {
			log.Printf("Skipping %s", fileName)
			return c, root, nil
		}
	}

	address := w.AccAddress()

	var isFolderVal int64
	if isFolder {
		isFolderVal = 1
	}

	msg := types.NewMsgPostFile(
		address,
		root,
		int64(size),
		7200,
		isFolderVal,
		3,
		fmt.Sprintf("{\"memo\":\"Uploaded with jackalIPFS\", \"cid\":\"%s\"}", c[0]),
	)
	//msg.Expires = 10711096 + int64(float64(25*365*24*60*60)/5.75)
	msg.Expires = 143000000

	if err := msg.ValidateBasic(); err != nil {
		return nil, root, err
	}

	res, err := queue.Post(msg)
	if err != nil {
		return nil, root, err
	}

	if res == nil {
		return nil, root, fmt.Errorf("response is empty")
	}
	if res.Code != 0 {
		return nil, root, fmt.Errorf(res.RawLog)
	}

	var postRes types.MsgPostFileResponse
	resData, err := hex.DecodeString(res.Data)
	if err != nil {
		return nil, root, err
	}

	encodingCfg := canine.MakeEncodingConfig()
	var txMsgData sdk.TxMsgData
	err = encodingCfg.Marshaler.Unmarshal(resData, &txMsgData)
	if err != nil {
		return nil, root, err
	}

	if len(txMsgData.Data) == 0 {
		return nil, root, fmt.Errorf("no message data")
	}

	err = postRes.Unmarshal(txMsgData.Data[0].Data)
	if err != nil {
		return nil, root, err
	}

	var parsedProvs = []types.Providers{
		{
			Address: "jkl1esjprqperjzwspaz6er7azzgqkvsa6n5kljv05",
			Ip:      "https://mprov02.jackallabs.io",
		},
		{
			Address: "jkl1t5708690gf9rc3mmtgcjmn9padl8va5g03f9wm",
			Ip:      "https://mprov01.jackallabs.io",
		},
		{
			Address: "jkl1taj9qq2qpr9ya6su6qplajfd39duhkqx4d6r04",
			Ip:      "https://node2.jackalstorageprovider40.com",
		},
		{
			Address: "jkl1dht8meprya6jr7w9g9zcp4p98ccxvckufvu4zc",
			Ip:      "https://jklstorage1.squirrellogic.com",
		},
	}

	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(parsedProvs), func(i, j int) { parsedProvs[i], parsedProvs[j] = parsedProvs[j], parsedProvs[i] })

	failCount := 0
	for {
		success := false
		log.Printf("Attempting to upload %s | %d bytes", fileName, len(buf.Bytes()))
		for _, provider := range parsedProvs {
			uploadBuffer := bytes.NewBuffer(buf.Bytes())
			cid, err := uploadFile(provider.Ip, uploadBuffer, root, postRes.StartBlock, address, isFolderVal)
			if err != nil {
				if strings.Contains(err.Error(), "I cannot claim") {
					log.Printf("I (%s) cannot claim %s", provider.Ip, fileName)
					success = true
					continue
				}
				log.Printf("Error from %s: %v", fileName, err)
				log.Err(err)
				time.Sleep(time.Second * 10)
				continue
			}
			if len(cid) == 0 {
				err := fmt.Errorf("CID does not exist")
				log.Printf("Error from %s: %v", fileName, err)
				log.Err(err)
				time.Sleep(time.Second * 10)
				continue
			}

			c = append(c, cid)
			success = true
			log.Printf("Upload of %s successful to %s with cid: %s", fileName, provider.Ip, cid)
		}
		if !success {
			failCount++
			if failCount >= 10 {
				break
			}

			log.Printf("Trying %s again", fileName)
			time.Sleep(time.Second * 20)

		} else {
			break
		}

	}
	if failCount >= 10 {
		return c, root, fmt.Errorf("did not upload any files for %s", fileName)
	}

	return c, root, nil
}

func filterArray(arr []types.Providers, filter map[string]bool) []types.Providers {
	var result []types.Providers
	for _, item := range arr {
		if !filter[item.Ip] {
			result = append(result, item) // Add to result if not in filter
		}
	}
	return result
}
