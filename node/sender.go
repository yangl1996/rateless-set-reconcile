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

	peerLoss       <-chan int
	ourLoss        <-chan int
	newTransaction <-chan *ldpc.Transaction
	droppedCodewords int
	credit float64
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
	sendTicker := time.NewTicker(10 * time.Millisecond)
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
		case <-sendTicker.C:
			s.credit += 0.01 * s.cwRate
			for s.credit >= 1.0 {
				s.credit -= 1.0
				// send the codewords
				nc := Codeword{s.encoder.ProduceCodeword(), s.accumLoss, time.Now().UnixMicro()}
				select {
				case ch <- nc:
					s.accumLoss = 0
					s.cwRate -= s.rateDecreaseConstant
					if s.cwRate < s.minRate {
						s.cwRate = s.minRate
					}
				default:
					// do not reset accumLoss now that the codeword is skipped
					s.droppedCodewords += 1
				}
			}
		case <-ticker.C:
			log.Printf("peer %s codeword rate %.2f dropped %d\n", s.peerId, s.cwRate, s.droppedCodewords)
		}
	}
	panic("unreachable")
}
