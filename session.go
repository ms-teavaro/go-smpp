package smpp

import (
	"context"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"sync"
	"time"

	"github.com/ms-teavaro/go-smpp/pdu"
)

type Session struct {
	parent       net.Conn
	receiveQueue chan any
	pending      map[int32]func(any)
	pendingLock  sync.Mutex
	NextSequence func() int32
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

func NewSession(ctx context.Context, parent net.Conn) (session *Session) {
	random := rand.New(rand.NewSource(time.Now().Unix()))
	session = &Session{
		parent:       parent,
		receiveQueue: make(chan any),
		pending:      make(map[int32]func(any)),
		NextSequence: random.Int31,
		ReadTimeout:  time.Minute * 15,
		WriteTimeout: time.Minute * 15,
	}
	go session.watch(ctx)
	return
}

//goland:noinspection SpellCheckingInspection
func (s *Session) watch(ctx context.Context) {
	var err error
	var packet any
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		if s.ReadTimeout > 0 {
			_ = s.parent.SetReadDeadline(time.Now().Add(s.ReadTimeout))
		}
		if packet, err = pdu.Unmarshal(s.parent); err == io.EOF {
			return
		}
		if packet == nil {
			continue
		}
		if status, ok := err.(pdu.CommandStatus); ok {
			_ = s.Send(&pdu.GenericNACK{
				Header: pdu.Header{CommandStatus: status, Sequence: pdu.ReadSequence(packet)},
				Tags:   pdu.Tags{0xFFFF: []byte(err.Error())},
			})
			continue
		}
		if callback, ok := s.pending[pdu.ReadSequence(packet)]; ok {
			callback(packet)
		} else {
			s.receiveQueue <- packet
		}
	}
}

func (s *Session) Submit(ctx context.Context, packet pdu.Responsable) (resp any, err error) {
	s.pendingLock.Lock()
	defer s.pendingLock.Unlock()
	sequence := s.NextSequence()
	pdu.WriteSequence(packet, sequence)
	if err = s.Send(packet); err != nil {
		return
	}
	returns := make(chan any, 1)
	s.pending[sequence] = func(resp any) {
		returns <- resp
	}
	defer delete(s.pending, sequence)
	select {
	case <-ctx.Done():
		err = ErrContextDone
	case resp = <-returns:
	}
	return
}

func (s *Session) Send(packet any) (err error) {
	sequence := pdu.ReadSequence(packet)
	if sequence == 0 || sequence < 0 {
		err = pdu.ErrInvalidSequence
		return
	}
	if s.WriteTimeout > 0 {
		err = s.parent.SetWriteDeadline(time.Now().Add(s.WriteTimeout))
	}
	if err == nil {
		_, err = pdu.Marshal(s.parent, packet)
	}
	if err == io.EOF {
		err = ErrConnectionClosed
	}
	return
}

func (s *Session) EnquireLink(ctx context.Context, tick time.Duration, timeout time.Duration) (err error) {
	ticker := time.NewTicker(tick)
	defer ticker.Stop()
	for {
		ctx, cancel := context.WithTimeout(ctx, timeout)
		if _, err = s.Submit(ctx, new(pdu.EnquireLink)); err != nil {
			log.Println("enquireLink: submit failed:", err)
			ticker.Stop()
			err = s.Close(ctx)
			if err != nil {
				log.Println("enquireLink: session close failed:", err)
			}
		}
		cancel()
		<-ticker.C
	}
}

func (s *Session) Close(ctx context.Context) (err error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	_, err = s.Submit(ctx, new(pdu.Unbind))
	if err != nil {
		return fmt.Errorf("submit failed in close: %w", err)
	}
	close(s.receiveQueue)
	return s.parent.Close()
}

func (s *Session) PDU() <-chan any {
	return s.receiveQueue
}
