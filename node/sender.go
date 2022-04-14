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
	accumLoss int

	sendTimer      *time.Timer
	peerLoss       <-chan int
	ourLoss        <-chan int
	newTransaction <-chan *ldpc.Transaction
}

func (s *sender) sendCodewords(ch <-chan Codeword) error {
	for cw := range ch {
		err := s.tx.Encode(cw)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *sender) loop(ch chan<- Codeword) error {
	ticker := time.NewTicker(1 * time.Second)
	log.Println("sender started")
	for {
		select {
		case l := <-s.peerLoss:
			s.cwRate += s.rateIncreaseConstant * float64(l)
		case l := <-s.ourLoss:
			s.accumLoss += l
		case tx := <-s.newTransaction:
			s.encoder.AddTransaction(tx)
		case <-s.sendTimer.C:
			// send the codeword
			nc := Codeword{s.encoder.ProduceCodeword(), s.accumLoss}
			select {
			case ch <- nc:
				s.accumLoss = 0
				s.cwRate -= s.rateDecreaseConstant
				if s.cwRate < s.minRate {
					s.cwRate = s.minRate
				}
			default:
				// do not reset accumLoss now that the codeword is skipped
			}
			// schedule the next event
			s.sendTimer.Reset(time.Duration(1.0 / s.cwRate * float64(time.Second)))
		case <-ticker.C:
			log.Printf("peer %s codeword rate %.2f\n", s.peerId, s.cwRate)
		}
	}
	panic("unreachable")
}
