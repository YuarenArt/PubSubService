package subpub

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// MessageHandler is a callback function that processes messages.
type MessageHandler func(msg interface{})

// Subscription is an interface for managing subscriptions.
type Subscription interface {
	// Unsubscribe wilL remove interest in the current
	Unsubscribe()
}

// SubPub is the interface for the Publisher-Subscriber system.
type SubPub interface {
	// Subscribe creates an asynchronous queue subscribe
	Subscribe(subject string, cb MessageHandler, opts ...SubscribeOption) (Subscription, error)

	// Publish publishes the msg argument to the given
	Publish(subject string, msg interface{}) error

	// Close will shutdown sub-pub system.
	// May be blocked by data delivery until the context is canceled
	Close(ctx context.Context) error
}

// SubscribeOption is an option for configuring subscriptions.
type SubscribeOption func(*subscriber)

// WithBufferSize sets the buffer size for the subscriber's channel.
func WithBufferSize(size int) SubscribeOption {
	return func(s *subscriber) {
		if size >= 0 {
			s.ch = make(chan interface{}, size)
		}
	}
}

// WithTimeout sets the timeout for message delivery to the subscriber.
func WithTimeout(timeout time.Duration) SubscribeOption {
	return func(s *subscriber) {
		if timeout > 0 {
			s.timeout = timeout
		}
	}
}

// subscriber represents a single subscriber with its own message queue.
type subscriber struct {
	id      uint64
	ch      chan interface{}
	timeout time.Duration
	cancel  context.CancelFunc
	cb      MessageHandler
}

// subscription represents a subscription to a subject.
type subscription struct {
	sp      *SubPubImpl
	subject string
	id      uint64
}

// SubPubImpl is the main implementation of the Publisher-Subscriber system.
type SubPubImpl struct {
	mu        sync.RWMutex
	subjects  map[string]map[uint64]*subscriber
	nextID    uint64
	closed    bool
	closeOnce sync.Once
	wg        sync.WaitGroup
}

// NewSubPub creates a new SubPub instance.
func NewSubPub() SubPub {
	return &SubPubImpl{
		subjects: make(map[string]map[uint64]*subscriber),
	}
}

// Subscribe registers a handler for the specified subject with optional configurations.
func (sp *SubPubImpl) Subscribe(subject string, cb MessageHandler, opts ...SubscribeOption) (Subscription, error) {
	if subject == "" {
		return nil, errors.New("subject cannot be empty")
	}
	if cb == nil {
		return nil, errors.New("callback cannot be nil")
	}

	sp.mu.Lock()
	defer sp.mu.Unlock()

	if sp.closed {
		return nil, errors.New("subscribe on closed SubPub")
	}

	ctx, cancel := context.WithCancel(context.Background())
	id := sp.nextID
	sp.nextID++

	// The default buffer size is 100, can be overridden by options.
	sub := &subscriber{
		id:     id,
		ch:     make(chan interface{}, 100),
		cancel: cancel,
		cb:     cb,
	}

	// Apply subscription options.
	for _, opt := range opts {
		opt(sub)
	}

	if sp.subjects[subject] == nil {
		sp.subjects[subject] = make(map[uint64]*subscriber)
	}
	sp.subjects[subject][id] = sub

	sp.wg.Add(1)
	go func() {
		defer sp.wg.Done()
		for {
			select {
			case msg, ok := <-sub.ch:
				if !ok {
					return
				}
				sub.cb(msg)
			case <-ctx.Done():
				return
			}
		}
	}()

	return &subscription{sp: sp, subject: subject, id: id}, nil
}

// Publish sends a message to all subscribers of the given subject, respecting timeouts.
func (sp *SubPubImpl) Publish(subject string, msg interface{}) error {
	if subject == "" {
		return errors.New("subject cannot be empty")
	}

	sp.mu.RLock()
	defer sp.mu.RUnlock()

	if sp.closed {
		return errors.New("publish on closed SubPub")
	}

	subs, ok := sp.subjects[subject]
	if !ok {
		return nil
	}

	var errs []error
	for id, sub := range subs {
		timeout := sub.timeout
		if timeout == 0 {
			timeout = 50 * time.Millisecond
		}

		ctx, cancel := context.WithTimeout(context.Background(), timeout)

		select {
		case sub.ch <- msg:

		case <-ctx.Done():
			errs = append(errs, fmt.Errorf("dropped message for slow subscriber ID %d", id))
		}

		cancel()
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// Close shuts down the SubPub system, respecting the provided context.
func (sp *SubPubImpl) Close(ctx context.Context) error {
	var err error
	sp.closeOnce.Do(func() {
		sp.mu.Lock()
		sp.closed = true

		for subject, subs := range sp.subjects {
			for id, sub := range subs {
				select {
				case <-ctx.Done():
					sp.mu.Unlock()
					err = ctx.Err()
					return
				default:
					sub.cancel()
					close(sub.ch)
					delete(subs, id)
				}
			}
			if len(subs) == 0 {
				delete(sp.subjects, subject)
			}
		}
		sp.mu.Unlock()

		done := make(chan struct{})
		go func() {
			sp.wg.Wait()
			close(done)
		}()

		select {
		case <-done:
		case <-ctx.Done():
			err = ctx.Err()
		}
	})
	return err
}

// Unsubscribe removes the subscription from the SubPub.
func (s *subscription) Unsubscribe() {
	s.sp.mu.Lock()
	defer s.sp.mu.Unlock()

	if subs, ok := s.sp.subjects[s.subject]; ok {
		if sub, ok := subs[s.id]; ok {
			sub.cancel()
			close(sub.ch)
			delete(subs, s.id)
			if len(subs) == 0 {
				delete(s.sp.subjects, s.subject)
			}
		}
	}
}
