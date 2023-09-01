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

type Simulator struct {
	time time.Duration
	mq   priorityQueue
	seq int
}

func (s *Simulator) EventsQueued() int {
	return len(s.mq)
}

func (s *Simulator) EventsDelivered() int {
	return s.seq - len(s.mq)
}

func (s *Simulator) Drained() bool {
	return len(s.mq) == 0
}

func (s *Simulator) Time() time.Duration {
	return s.time
}

func (s *Simulator) RunUntil(t time.Duration) {
	for !s.Drained() && s.time <= t {
		s.deliverNextMessage()
	}
}

func (s *Simulator) Run() {
	for !s.Drained() {
		s.deliverNextMessage()
	}
}

func (s *Simulator) ScheduleMessage(msg OutgoingMessage, from Module) {
	if msg.To == nil {
		m := queuedMessage{s.time + msg.Delay, s.seq, from, from, msg.Payload}
		heap.Push(&s.mq, m)
	} else {
		m := queuedMessage{s.time + msg.Delay, s.seq, from, msg.To, msg.Payload}
		heap.Push(&s.mq, m)
	}
	s.seq += 1
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

type queuedMessage struct {
	arrival     time.Duration
	seq int
	from Module
	to Module
	payload     any
}

type priorityQueue []queuedMessage

func (pq priorityQueue) Len() int { return len(pq) }

func (pq priorityQueue) Less(i, j int) bool {
	if pq[i].arrival < pq[j].arrival {
		return true
	} else if pq[i].arrival == pq[j].arrival {
		return pq[i].seq < pq[j].seq
	}
	return false
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
