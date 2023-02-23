package main

import (
	"container/heap"
	"time"
)

type message struct {
	arrival     time.Duration
	destination int
	payload     any
}

type simulator struct {
	time time.Duration
	mq   priorityQueue
}

func (s *simulator) drained() bool {
	return len(s.mq) == 0
}

func (s *simulator) queueMessage(delay time.Duration, dest int, msg any) {
	m := &message{s.time + delay, dest, msg}
	heap.Push(&s.mq, m)
}

func (s *simulator) nextMessage() (int, any) {
	m := heap.Pop(&s.mq).(*message)
	if m.arrival < s.time {
		panic("time reversal")
	}
	s.time = m.arrival
	return m.destination, m.payload
}

type priorityQueue []*message

func (pq priorityQueue) Len() int { return len(pq) }

func (pq priorityQueue) Less(i, j int) bool {
	return pq[i].arrival < pq[j].arrival
}

func (pq priorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	return
}

func (pq *priorityQueue) Push(x any) {
	msg := x.(*message)
	*pq = append(*pq, msg)
}

func (pq *priorityQueue) Pop() any {
	idx := len(*pq) - 1
	res := (*pq)[idx]
	(*pq)[idx] = nil
	*pq = (*pq)[0:idx]
	return res
}
