package main

import (
	"log"
	"math/rand"
	"sync/atomic"
	"time"
)

// Event is the message send event between two processes
type Event struct {
	from    *Process
	to      *Process
	message string
	time    uint32
}

// Process represents a distributed node that can send
// and receive messages between other nodes
type Process struct {
	name   string
	time   uint32
	events chan *Event
}

// Print the process name on formatting
func (p *Process) String() string {
	return p.name
}

// Step increments the logical timestamp of the process
// using an atomic counter to ensure thread safety
func (p *Process) Step() {
	atomic.AddUint32(&p.time, 1)
}

// Send a message to the given process
func (p *Process) Send(to *Process, message string) {
	// increment local timestamp
	p.Step()
	// skip if step is internal i.e. both to and from
	// are the same processes
	if p.name != to.name {
		e := &Event{
			from:    p,
			to:      to,
			message: message,
			time:    p.time,
		}
		to.events <- e
	}
}

// receive listens to messages sent to this process
// it runs as a goroutine
func (p *Process) receive() {
	for {
		select {
		case e := <-p.events:
			// set maximum of local and event timestamps to be
			// the current timestamp
			if e.time > p.time {
				p.time = e.time
			}
			// increment local timestamp
			p.Step()
			// set event timestamp to the updated local timestamp
			e.time = p.time
			log.Printf("%v: %v => %v: %v", e.time, e.from, e.to, e.message)
		}
	}
}

// NewProcess returns a new process
// it triggers the receive goroutine
func NewProcess(name string) *Process {
	p := &Process{
		name:   name,
		events: make(chan *Event),
	}
	go p.receive()
	return p
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

func main() {

	// create three processes
	p1 := NewProcess("Alice")
	p2 := NewProcess("Bob")
	p3 := NewProcess("Charlie")

	processes := [3]*Process{p1, p2, p3}
	for {
		time.Sleep(time.Second)
		// pick 2 at random and send a message
		a := processes[rand.Int()%len(processes)]
		b := processes[rand.Int()%len(processes)]
		a.Send(b, "hi")
	}
}
