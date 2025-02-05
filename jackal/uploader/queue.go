package uploader

import (
	"github.com/rs/zerolog/log"
	"strings"
	"sync"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	walletTypes "github.com/desmos-labs/cosmos-go-wallet/types"
	"github.com/desmos-labs/cosmos-go-wallet/wallet"
)

type MsgHolder struct {
	m   sdk.Msg
	r   *sdk.TxResponse
	wg  *sync.WaitGroup
	err error
}

type Queue struct {
	messages []*MsgHolder
	w        *wallet.Wallet
	stopped  bool
}

func NewQueue(w *wallet.Wallet) *Queue {
	q := Queue{
		messages: make([]*MsgHolder, 0),
		w:        w,
	}
	return &q
}

func (q *Queue) Stop() {
	q.stopped = true
}

func (q *Queue) TooBusy() bool {
	return len(q.messages) > 100
}

func (q *Queue) Listen() {
	go func() {
		for !q.stopped {
			time.Sleep(time.Second * 8)
			q.popAndPost()
		}
	}()
}

func (q *Queue) popAndPost() {
	// log.Printf("Checking queue for new messages...")
	if len(q.messages) == 0 {
		return
	}
	// log.Printf("Found one!")

	size := 10
	if len(q.messages) < size {
		size = len(q.messages)
	}

	newMessages := q.messages[0:size]
	q.messages = q.messages[size:]

	var msgs []sdk.Msg

	for _, message := range newMessages {
		msgs = append(msgs, message.m)
	}

	gas := 2000000

	data := walletTypes.NewTransactionData(
		msgs...,
	).WithGasLimit(uint64(gas)).WithFeeAuto()
	//.WithFeeAmount(sdk.NewCoins(sdk.NewInt64Coin("ujkl", int64(float64(gas)*0.02)+1)))

	e := "timed out waiting for tx to be included in a block"
	for strings.Contains(e, "timed out waiting for tx to be included in a block") {
		res, err := q.w.BroadcastTxCommit(data)
		e = ""
		if err != nil {
			e = err.Error()
			log.Print(err)
			log.Error().Err(err)
			continue
		}
		if res == nil {
			log.Printf("response is for sure empty")
			continue
		}

		for _, m := range newMessages {
			m.err = err
			m.r = res

			m.wg.Done()
		}
	}
}

func (q *Queue) Post(msg sdk.Msg) (*sdk.TxResponse, error) {
	var wg sync.WaitGroup
	m := MsgHolder{
		m:  msg,
		wg: &wg,
	}
	wg.Add(1)

	q.messages = append(q.messages, &m)

	wg.Wait()

	return m.r, nil
}
