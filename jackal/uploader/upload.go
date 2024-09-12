package uploader

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
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
	cli.Timeout = time.Second * 20
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

		err := json.NewDecoder(res.Body).Decode(&errRes)
		if err != nil {
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

func PostFile(fileName string, fileData []byte, queue *Queue, w *wallet.Wallet, isFolder bool) ([]string, []byte, error) {
	buf := bytes.NewBuffer(fileData)
	treeBuffer := bytes.NewBuffer(buf.Bytes())

	abci, err := w.Client.RPCClient.ABCIInfo(context.Background())
	if err != nil {
		return nil, nil, err
	}

	cl := types.NewQueryClient(w.Client.GRPCConn)

	params, err := cl.Params(context.Background(), &types.QueryParams{})
	if err != nil {
		return nil, nil, err
	}

	root, _, _, size, err := utils.BuildTree(treeBuffer, params.Params.ChunkSize)
	if err != nil {
		return nil, root, err
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
		40,
		isFolderVal,
		3,
		"{\"memo\":\"Uploaded with jackalIPFS\"}",
	)
	msg.Expires = abci.Response.LastBlockHeight + ((100 * 365 * 24 * 60 * 60) / 6)

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

	log.Printf(res.TxHash)

	c := make([]string, 0)

	provReq := types.QueryActiveProviders{}

	provRes, err := cl.ActiveProviders(context.Background(), &provReq)
	if err != nil {
		return c, root, err
	}
	ps := provRes.Providers

	parsedProvs := make([]types.Providers, len(ps))

	for i, provider := range ps {
		qp := types.QueryProvider{
			Address: provider.Address,
		}

		providerResponse, err := cl.Provider(context.Background(), &qp)
		if err != nil {
			return c, root, err
		}

		parsedProvs[i] = providerResponse.Provider
	}

	log.Printf("There are %d providers available for %s", len(parsedProvs), fileName)

	for i := range parsedProvs {
		j := rand.Intn(i + 1)
		parsedProvs[i], parsedProvs[j] = parsedProvs[j], parsedProvs[i]
	}

	log.Printf("Attempting to upload %s", fileName)

	var k int
	for _, provider := range parsedProvs {
		if k >= 3 {
			continue
		}

		uploadBuffer := bytes.NewBuffer(buf.Bytes())

		cid, err := uploadFile(provider.Ip, uploadBuffer, root, postRes.StartBlock, address, isFolderVal)
		if len(cid) == 0 && err == nil {
			err = fmt.Errorf("CID does not exist")
		}
		if err != nil {
			if strings.Contains(err.Error(), "I cannot claim") {
				break
			}
			log.Err(err)
			continue
		}
		log.Printf("Upload of %s successful to %s with cid: %s", fileName, provider.Ip, cid)
		c = append(c, cid)

		k++
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
