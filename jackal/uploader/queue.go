package uploader

import (
	"github.com/rs/zerolog/log"
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

func (q *Queue) Listen() {
	go func() {
		for !q.stopped {
			time.Sleep(time.Millisecond * 1000)
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

	m := q.messages[0]
	q.messages = q.messages[1:]

	msg := m.m

	gas := 220000

	data := walletTypes.NewTransactionData(
		msg,
	).WithGasLimit(uint64(gas)).WithFeeAuto()
	//.WithFeeAmount(sdk.NewCoins(sdk.NewInt64Coin("ujkl", int64(float64(gas)*0.02)+1)))
	res, err := q.w.BroadcastTxCommit(data)
	if err != nil {
		log.Print(err)
		log.Error().Err(err)
	}
	if res == nil {
		log.Printf("response is for sure empty")
	}
	m.err = err
	m.r = res

	m.wg.Done()
}

func (q *Queue) Post(msg sdk.Msg) (*sdk.TxResponse, error) {
	log.Printf("posting message...")

	var wg sync.WaitGroup
	m := MsgHolder{
		m:  msg,
		wg: &wg,
	}
	wg.Add(1)

	q.messages = append(q.messages, &m)

	log.Printf("waiting...")

	wg.Wait()

	return m.r, nil
}
