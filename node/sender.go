package main

import (
	"encoding/gob"
	"github.com/yangl1996/rateless-set-reconcile/ldpc"
	"log"
	"time"
)

type sender struct {
	peerId string
	tx                   *gob.Encoder
	encoder              *ldpc.Encoder
	cwRate               float64 // codeword sending rate in s^-1
	rateIncreaseConstant float64
	rateDecreaseConstant float64
	minRate				float64
	nextCodeword         Codeword

	sendTimer      *time.Timer
	peerLoss       <-chan int
	ourLoss        <-chan int
	newTransaction <-chan *ldpc.Transaction
}

func (s *sender) loop() error {
	ticker := time.NewTicker(1 * time.Second)
	log.Println("sender started")
	for {
		select {
		case l := <-s.peerLoss:
			s.cwRate += s.rateIncreaseConstant * float64(l)
		case l := <-s.ourLoss:
			s.nextCodeword.Loss += l
		case tx := <-s.newTransaction:
			s.encoder.AddTransaction(tx)
		case <-s.sendTimer.C:
			// schedule the next event
			s.cwRate -= s.rateDecreaseConstant
			if s.cwRate < s.minRate {
				s.cwRate = s.minRate
			}
			s.sendTimer.Reset(time.Duration(1.0 / s.cwRate * float64(time.Second)))
			// send the codeword
			s.nextCodeword.Codeword = s.encoder.ProduceCodeword()
			err := s.tx.Encode(s.nextCodeword)
			if err != nil {
				return err
			}
			s.nextCodeword.Loss = 0
		case <-ticker.C:
			log.Printf("peer %s codeword rate %.2f\n", s.peerId, s.cwRate)
		}
	}
	panic("unreachable")
}
