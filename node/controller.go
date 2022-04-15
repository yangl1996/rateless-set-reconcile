package main

import (
	"encoding/gob"
	"github.com/yangl1996/rateless-set-reconcile/ldpc"
	"github.com/yangl1996/soliton"
	"io"
	"log"
	"math/rand"
	"time"
	"github.com/DataDog/sketches-go/ddsketch"
)

type peer struct {
	newTxToSender chan<- *ldpc.Transaction
	newTxToReceiver chan<- *ldpc.Transaction
}

func (h *peer) notifyNewTransaction(t *ldpc.Transaction) {
	h.newTxToSender <- t
	h.newTxToReceiver <- t
}

func newPeer(id string, conn io.ReadWriter, decoded chan<- *ldpc.Transaction, importTx []*ldpc.Transaction, K, M uint64, solitonC, solitonDelta, initRate, minRate, incConstant, targetLoss float64, decodeTimeout time.Duration, encoderKey [ldpc.SaltSize]byte, decoderKey [ldpc.SaltSize]byte) *peer {
	peerLoss := make(chan int, 100)
	ourLoss := make(chan int, 100)
	senderNewTx := make(chan *ldpc.Transaction, 100)
	receiverNewTx := make(chan *ldpc.Transaction, 100)

	dist := soliton.NewRobustSoliton(rand.New(rand.NewSource(time.Now().Unix())), K, solitonC, solitonDelta)
	s := sender{
		peerId: id,
		tx:                   gob.NewEncoder(conn),
		encoder:              ldpc.NewEncoder(encoderKey, dist, int(K)),
		cwRate:               initRate,
		rateIncreaseConstant: incConstant,
		rateDecreaseConstant: incConstant*targetLoss,
		minRate: minRate,
		sendTimer:            time.NewTimer(time.Duration(1.0 / initRate * float64(time.Second))),
		peerLoss:             peerLoss,
		ourLoss: ourLoss,
		newTransaction:       senderNewTx,
	}

	sketch, err := ddsketch.NewDefaultDDSketchWithExactSummaryStatistics(0.001)
	if err != nil {
		log.Fatalln(err)
	}

	r := receiver{
		peerId: id,
		rx:          gob.NewDecoder(conn),
		decoder:     ldpc.NewDecoder(decoderKey, int(M)),
		peerLoss: peerLoss,
		ourLoss: ourLoss,
		decodedTransaction: decoded,
		newTransaction: receiverNewTx,
		timeout:     decodeTimeout,
		delaySketch: sketch,
	}
	for _, existingTx := range importTx {
		r.decoder.AddTransaction(existingTx)
	}

	txCwCh := make(chan Codeword, 1000)
	go func() {
		err := s.loop(txCwCh)
		if err != nil {
			panic(err)
		}
	}()
	go func() {
		err := s.sendCodewords(txCwCh)
		if err != nil {
			panic(err)
		}
	}()
	cwCh := make(chan Codeword, 1000)
	go func() {
		err := r.receive(cwCh)
		if err != nil {
			panic(err)
		}
	}()
	go func() {
		err := r.decode(cwCh)
		if err != nil {
			panic(err)
		}
	}()
	return &peer{senderNewTx, receiverNewTx}
}

type controller struct {
	peers            []*peer
	newPeer          chan *peer
	decodedTransaction chan *ldpc.Transaction
	localTransaction chan *ldpc.Transaction

	K uint64
	M uint64
	solitonC float64
	solitonDelta float64
	initRate float64
	minRate float64
	incConstant float64
	targetLoss float64
	decodeTimeout time.Duration

	delaySketch *ddsketch.DDSketchWithExactSummaryStatistics
	warmupTime time.Duration
}

func (c *controller) loop() error {
	txcnt := 0
	start := time.Now()
	warmupFinished := false
	ticker := time.NewTicker(1 * time.Second)
	log.Println("controller started")
	for {
		select {
		case tx := <-c.localTransaction:
			for _, peer := range c.peers {
				peer.notifyNewTransaction(tx)
			}
		case tx := <-c.decodedTransaction:
			txcnt += 1
			for _, peer := range c.peers {
				peer.notifyNewTransaction(tx)
			}
			if warmupFinished {
				delay := getDelayUs(tx)
				err := c.delaySketch.Add(delay/1000.0)
				if err != nil {
					log.Println("error inserting delay into sketch:", err)
				}
			}
		case p := <-c.newPeer:
			log.Println("new peer")
			c.peers = append(c.peers, p)
		case <-ticker.C:
			log.Printf("total tx %d\n", txcnt)
			if !warmupFinished {
				if time.Since(start) > c.warmupTime {
					warmupFinished = true
					log.Println("data logging warmup completed")
				} else {
					break
				}
			}
			qts, err := c.delaySketch.GetValuesAtQuantiles([]float64{0.05, 0.50, 0.95})
			if err != nil {
				log.Println("error getting quantiles:", err)
				break
			}
			cnt := c.delaySketch.GetCount()
			sum := c.delaySketch.GetSum()
			log.Printf("since warm up tx=%d, p5_latency_ms=%.2f, p95_latency_ms=%.2f, p50_latency_ms=%.2f, mean_latency_ms=%.2f\n", int(cnt), qts[0], qts[2], qts[1], sum/cnt)
		}
	}
}

func (c *controller) handleConn(id string, conn io.ReadWriter) error {
	var encoderKey [ldpc.SaltSize]byte
	var decoderKey [ldpc.SaltSize]byte
	rand.Read(encoderKey[:])

	// send our key
	_, err := conn.Write(encoderKey[:])
	if err != nil {
		return err
	}
	_, err = conn.Read(decoderKey[:])
	if err != nil {
		return err
	}
	log.Printf("key exchanged with peer %s, our key %x, peer key %x\n", id, encoderKey[:], decoderKey[:])

	p := newPeer(id, conn, c.decodedTransaction, nil, c.K, c.M, c.solitonC, c.solitonDelta, c.initRate, c.minRate, c.incConstant, c.targetLoss, c.decodeTimeout, encoderKey, decoderKey)

	c.newPeer <- p
	return nil
}
