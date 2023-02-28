package des

import (
	"container/heap"
	"time"
)

type Module interface {
	HandleMessage(payload any, from Module, timestamp time.Duration) []OutgoingMessage
}

type OutgoingMessage struct {
	Payload any
	To Module // nil means sending to the sender itself
	Delay time.Duration
}

type queuedMessage struct {
	arrival     time.Duration
	from Module
	to Module
	payload     any
}

type Simulator struct {
	time time.Duration
	mq   priorityQueue
}

func (s *Simulator) Drained() bool {
	return len(s.mq) == 0
}

func (s *Simulator) Time() time.Duration {
	return s.time
}

func (s *Simulator) Run() {
	for !s.Drained() {
		s.deliverNextMessage()
	}
}

func (s *Simulator) ScheduleMessage(msg OutgoingMessage, from Module) {
	m := queuedMessage{s.time + msg.Delay, from, msg.To, msg.Payload}
	heap.Push(&s.mq, m)
}

func (s *Simulator) deliverNextMessage() {
	m := heap.Pop(&s.mq).(queuedMessage)
	if m.arrival < s.time {
		panic("time reversal")
	}
	s.time = m.arrival
	nm := m.to.HandleMessage(m.payload, m.from, s.time)
	for _, v := range nm {
		s.ScheduleMessage(v, m.to)
	}
}

type priorityQueue []queuedMessage

func (pq priorityQueue) Len() int { return len(pq) }

func (pq priorityQueue) Less(i, j int) bool {
	return pq[i].arrival < pq[j].arrival
}

func (pq priorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	return
}

func (pq *priorityQueue) Push(x any) {
	msg := x.(queuedMessage)
	*pq = append(*pq, msg)
}

func (pq *priorityQueue) Pop() any {
	idx := len(*pq) - 1
	res := (*pq)[idx]
	(*pq)[idx].payload = nil
	*pq = (*pq)[0:idx]
	return res
}
