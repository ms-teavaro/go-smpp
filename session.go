package smpp

import (
	"context"
	"io"
	"log"
	"math/rand"
	"net"
	"sync"
	"time"

	"github.com/ms-teavaro/go-smpp/pdu"
	"github.com/pkg/errors"
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

// watch gets "stuff" from the tcp connection
// if there is a callback in the sequence / callback map
// which is only created in Submit it's called
// otherwise the pdu is put into the receive queue
//
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

// Submit prepares the packet pdu while adding a new sequence to it.
// Then it calls send and waits for a response if send did not return an error.
// BUG: blocks Submit until a response is received or the context timed out.
// As the async enquireLink uses Submit too, this causes issue.
func (s *Session) Submit(ctx context.Context, packet pdu.Responsable) (resp any, err error) {
	s.pendingLock.Lock()
	defer s.pendingLock.Unlock()
	// get new sequence and write into the paket
	sequence := s.NextSequence() // random sequence is questionable
	pdu.WriteSequence(packet, sequence)
	if err = s.Send(packet); err != nil {
		err = errors.Wrap(err, "send failed in submit")
		return
	}
	// no error in send, make callback for packet
	returns := make(chan any, 1)
	s.pending[sequence] = func(resp any) {
		returns <- resp
	}
	// defer cleanup
	defer delete(s.pending, sequence)
	// select has no default, waits for ctx || resp
	// pending is still locked!!!
	select {
	case <-ctx.Done(): // likely timeout
		err = ErrContextDone
	case resp = <-returns: // we got something back
	}
	return
}

func (s *Session) Send(packet any) (err error) {
	sequence := pdu.ReadSequence(packet)
	if sequence == 0 || sequence < 0 {
		return pdu.ErrInvalidSequence
	}
	if s.WriteTimeout > 0 {
		err = s.parent.SetWriteDeadline(time.Now().Add(s.WriteTimeout))
		return errors.Wrap(err, "setWriteDeadline failed")
	}

	_, err = pdu.Marshal(s.parent, packet)
	if err != nil {
		return errors.Wrap(err, "marshal to tcp connection failed")
	}

	return nil
}

func (s *Session) EnquireLink(ctx context.Context, tick time.Duration, timeout time.Duration) (err error) {
	ticker := time.NewTicker(tick)
	defer ticker.Stop()
	for {
		var enquireLink pdu.EnquireLink
		sequence := s.NextSequence()
		pdu.WriteSequence(enquireLink, sequence)
		if err = s.Send(enquireLink); err != nil {
			log.Println("enquireLink: send failed:", err)
			continue
		}
		select { // wait for some event, no default!
		case <-ctx.Done():
			return ErrContextDone
		case <-ticker.C:
		}
	}
}

func (s *Session) Close(ctx context.Context) (err error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	_, err = s.Submit(ctx, new(pdu.Unbind))
	if err != nil {
		return errors.Wrap(err, "submit failed in unbind/close")
	}
	close(s.receiveQueue)
	return s.parent.Close()
}

func (s *Session) PDU() <-chan any {
	return s.receiveQueue
}
