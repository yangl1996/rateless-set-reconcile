package main

import (
	"github.com/yangl1996/rateless-set-reconcile/ldpc"
	"encoding/gob"
	"time"
)

type receivedCodeword struct {
	*ldpc.PendingCodeword
	receivedTime time.Time
}

type sender struct {
	tx *gob.Encoder
	encoder *ldpc.Encoder
	txRate float64	// codeword sending rate in s^-1
	rateIncreaseConstant float64
	rateDecreaseConstant float64
	nextCodeword Codeword

	sendTimer *time.Timer
	peerLoss chan int
	ourLoss chan int
	shutdown chan struct{}
}

func (s *sender) loop() error {
	for {
		select {
		case <-s.shutdown:
			return nil
		case l := <-s.peerLoss:
			s.txRate += s.rateIncreaseConstant * float64(l)
		case l := <-s.ourLoss:
			s.nextCodeword.Loss += l
		case <-s.sendTimer.C:
			// schedule the next event
			s.sendTimer.Reset(time.Duration(1.0 / s.txRate * float64(time.Second)))
			// send the codeword
			s.nextCodeword.Codeword = s.encoder.ProduceCodeword()
			err := s.tx.Encode(s.nextCodeword)
			if err != nil {
				return err
			}
			s.nextCodeword.Loss = 0
		}
	}
	panic("unreachable")
}

type peer struct {
	decoder *ldpc.Decoder
	rx *gob.Decoder
	rxLoss int
	rxWindow []receivedCodeword
}

func (p *peer) receiveCodeword(c *ldpc.Codeword) []*ldpc.Transaction {
	return nil
}

type controller struct {
	peers []*peer
}


