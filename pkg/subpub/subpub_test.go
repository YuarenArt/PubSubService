package subpub

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

// SubPubTestSuite is a test suite for the SubPub system.
type SubPubTestSuite struct {
	suite.Suite
	sp SubPub
}

// SetupTest initializes a new SubPub instance for each test.
func (s *SubPubTestSuite) SetupTest() {
	s.sp = NewSubPub()
}

// TearDownTest closes the SubPub instance after each test.
func (s *SubPubTestSuite) TearDownTest() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := s.sp.Close(ctx)
	s.NoError(err)
}

// TestSubPub runs the SubPub test suite.
func TestSubPub(t *testing.T) {
	suite.Run(t, new(SubPubTestSuite))
}

// TestSubscribeSuccess verifies that a subscriber receives a published message.
func (s *SubPubTestSuite) TestSubscribeSuccess() {
	subject := "test.subject"
	var receivedMsg interface{}
	cb := func(msg interface{}) {
		receivedMsg = msg
	}

	sub, err := s.sp.Subscribe(subject, cb)
	s.NoError(err)
	s.NotNil(sub)

	err = s.sp.Publish(subject, "test message")
	s.NoError(err)
	time.Sleep(100 * time.Millisecond) // Wait for async delivery.
	s.Equal("test message", receivedMsg)

	sub.Unsubscribe()
}

// TestSubscribeInvalid checks that invalid subscription attempts return errors.
func (s *SubPubTestSuite) TestSubscribeInvalid() {
	sub, err := s.sp.Subscribe("", func(msg interface{}) {})
	s.Error(err)
	s.Equal("subject cannot be empty", err.Error())
	s.Nil(sub)

	sub, err = s.sp.Subscribe("test.subject", nil)
	s.Error(err)
	s.Equal("callback cannot be nil", err.Error())
	s.Nil(sub)
}

// TestMultipleSubscribersSameSubject tests that multiple subscribers to the same subject receive all messages.
func (s *SubPubTestSuite) TestMultipleSubscribersSameSubject() {
	subject := "test.subject"
	var receivedMsgs1, receivedMsgs2 []interface{}

	cb1 := func(msg interface{}) {
		receivedMsgs1 = append(receivedMsgs1, msg)
	}
	cb2 := func(msg interface{}) {
		receivedMsgs2 = append(receivedMsgs2, msg)
	}

	sub1, err := s.sp.Subscribe(subject, cb1)
	s.NoError(err)
	sub2, err := s.sp.Subscribe(subject, cb2)
	s.NoError(err)

	err = s.sp.Publish(subject, "message1")
	s.NoError(err)
	err = s.sp.Publish(subject, "message2")
	s.NoError(err)

	time.Sleep(200 * time.Millisecond)

	s.Equal([]interface{}{"message1", "message2"}, receivedMsgs1)
	s.Equal([]interface{}{"message1", "message2"}, receivedMsgs2)

	sub1.Unsubscribe()
	sub2.Unsubscribe()
}

// TestSubscribersDifferentSubjects tests that subscribers to different subjects receive only their messages.
func (s *SubPubTestSuite) TestSubscribersDifferentSubjects() {
	subject1 := "subject1"
	subject2 := "subject2"
	var receivedMsgs1, receivedMsgs2 []interface{}

	cb1 := func(msg interface{}) {
		receivedMsgs1 = append(receivedMsgs1, msg)
	}
	cb2 := func(msg interface{}) {
		receivedMsgs2 = append(receivedMsgs2, msg)
	}

	sub1, err := s.sp.Subscribe(subject1, cb1)
	s.NoError(err)
	sub2, err := s.sp.Subscribe(subject2, cb2)
	s.NoError(err)

	err = s.sp.Publish(subject1, "message1")
	s.NoError(err)
	err = s.sp.Publish(subject2, "message2")
	s.NoError(err)

	time.Sleep(100 * time.Millisecond)

	s.Equal([]interface{}{"message1"}, receivedMsgs1)
	s.Equal([]interface{}{"message2"}, receivedMsgs2)

	sub1.Unsubscribe()
	sub2.Unsubscribe()
}

// TestPublishNoSubscribers tests that publishing to a subject with no subscribers does not cause an error.
func (s *SubPubTestSuite) TestPublishNoSubscribers() {
	err := s.sp.Publish("non.existent.subject", "message")
	s.NoError(err)
}

// TestPublishInvalidSubject tests that publishing to an empty subject returns an error.
func (s *SubPubTestSuite) TestPublishInvalidSubject() {
	err := s.sp.Publish("", "message")
	s.Error(err)
	s.Equal("subject cannot be empty", err.Error())
}

// TestPublishTimeout tests message dropping when a subscriber is slow.
func (s *SubPubTestSuite) TestPublishTimeout() {
	subject := "test.subject"
	var receivedMsgs []interface{}

	cb := func(msg interface{}) {
		time.Sleep(100 * time.Millisecond)
		receivedMsgs = append(receivedMsgs, msg)
	}

	sub, err := s.sp.Subscribe(subject, cb, WithBufferSize(1), WithTimeout(50*time.Millisecond))
	s.NoError(err)

	err = s.sp.Publish(subject, "message1")
	s.NoError(err)
	err = s.sp.Publish(subject, "message2")
	if err != nil {
		s.Contains(err.Error(), "dropped message for slow subscriber")
	}

	time.Sleep(200 * time.Millisecond)

	s.True(len(receivedMsgs) >= 1)
	s.True(len(receivedMsgs) <= 2)

	sub.Unsubscribe()
}

// TestBufferSizeOption tests that messages are dropped when the buffer is full.
func (s *SubPubTestSuite) TestBufferSizeOption() {
	subject := "test.subject"
	var receivedMsgs []interface{}

	cb := func(msg interface{}) {
		time.Sleep(50 * time.Millisecond)
		receivedMsgs = append(receivedMsgs, msg)
	}

	sub, err := s.sp.Subscribe(subject, cb, WithBufferSize(2))
	s.NoError(err)

	err = s.sp.Publish(subject, "message1")
	s.NoError(err)
	err = s.sp.Publish(subject, "message2")
	s.NoError(err)
	err = s.sp.Publish(subject, "message3")
	if err != nil {
		s.Contains(err.Error(), "dropped message for slow subscriber")
	}

	time.Sleep(200 * time.Millisecond)

	s.True(len(receivedMsgs) >= 2)
	s.True(len(receivedMsgs) <= 3)

	sub.Unsubscribe()
}

// TestTimeoutOption tests that a slow subscriber causes a timeout error.
func (s *SubPubTestSuite) TestTimeoutOption() {
	subject := "test.subject"
	var receivedMsgs []interface{}

	cb := func(msg interface{}) {
		time.Sleep(100 * time.Millisecond)
		receivedMsgs = append(receivedMsgs, msg)
	}

	sub, err := s.sp.Subscribe(subject, cb, WithBufferSize(0), WithTimeout(10*time.Millisecond))
	s.NoError(err)

	go func() {
		for i := 0; i < 5; i++ {
			s.sp.Publish(subject, fmt.Sprintf("dummy%d", i))
		}
	}()

	time.Sleep(50 * time.Millisecond)

	err = s.sp.Publish(subject, "message1")
	s.Error(err)
	s.Contains(err.Error(), "dropped message for slow subscriber")

	time.Sleep(200 * time.Millisecond)
	s.True(len(receivedMsgs) <= 1)

	sub.Unsubscribe()
}

// TestUnsubscribe tests that a subscriber stops receiving messages after unsubscribing.
func (s *SubPubTestSuite) TestUnsubscribe() {
	subject := "test.subject"
	var receivedMsgs []interface{}

	cb := func(msg interface{}) {
		receivedMsgs = append(receivedMsgs, msg)
	}

	sub, err := s.sp.Subscribe(subject, cb)
	s.NoError(err)

	err = s.sp.Publish(subject, "message1")
	s.NoError(err)

	sub.Unsubscribe()

	err = s.sp.Publish(subject, "message2")
	s.NoError(err)

	time.Sleep(100 * time.Millisecond)

	s.Equal([]interface{}{"message1"}, receivedMsgs)
}

// TestClose tests that a closed SubPub prevents new subscriptions and publications.
func (s *SubPubTestSuite) TestClose() {
	subject := "test.subject"
	var receivedMsgs []interface{}

	cb := func(msg interface{}) {
		receivedMsgs = append(receivedMsgs, msg)
	}

	_, err := s.sp.Subscribe(subject, cb)
	s.NoError(err)

	err = s.sp.Publish(subject, "message1")
	s.NoError(err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = s.sp.Close(ctx)
	s.NoError(err)

	err = s.sp.Publish(subject, "message2")
	s.Error(err)
	s.Equal("publish on closed SubPub", err.Error())

	_, err = s.sp.Subscribe(subject, cb)
	s.Error(err)
	s.Equal("subscribe on closed SubPub", err.Error())

	time.Sleep(100 * time.Millisecond)

	s.Equal([]interface{}{"message1"}, receivedMsgs)
}

// TestCloseWithSlowSubscribers tests that closing with slow subscribers respects the context timeout.
func (s *SubPubTestSuite) TestCloseWithSlowSubscribers() {
	subject := "test.subject"
	var receivedMsgs []interface{}

	cb := func(msg interface{}) {
		time.Sleep(100 * time.Millisecond)
		receivedMsgs = append(receivedMsgs, msg)
	}

	_, err := s.sp.Subscribe(subject, cb, WithBufferSize(1))
	s.NoError(err)

	err = s.sp.Publish(subject, "message1")
	s.NoError(err)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	err = s.sp.Close(ctx)
	if err != nil {
		s.Equal(context.DeadlineExceeded, err)
	}

	time.Sleep(200 * time.Millisecond)

	s.True(len(receivedMsgs) <= 1)
}

// TestConcurrentOperations tests that multiple subscribers can concurrently receive messages.
func (s *SubPubTestSuite) TestConcurrentOperations() {
	subject := "test.subject"
	var wg sync.WaitGroup
	var subscribeWg sync.WaitGroup
	const numSubs = 5
	const numMsgs = 10

	receivedMsgs := make([][]interface{}, numSubs)
	for i := 0; i < numSubs; i++ {
		subscribeWg.Add(1)
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			receivedMsgs[idx] = []interface{}{}
			cb := func(msg interface{}) {
				receivedMsgs[idx] = append(receivedMsgs[idx], msg)
			}
			sub, err := s.sp.Subscribe(subject, cb)
			s.NoError(err)
			subscribeWg.Done()
			defer sub.Unsubscribe()

			time.Sleep(200 * time.Millisecond)
			s.Equal(numMsgs, len(receivedMsgs[idx]))
		}(i)
	}

	subscribeWg.Wait()

	for i := 0; i < numMsgs; i++ {
		err := s.sp.Publish(subject, fmt.Sprintf("message%d", i))
		s.NoError(err)
	}

	wg.Wait()
}

// TestMultipleSubscribeUnsubscribe tests subscribing and unsubscribing multiple times.
func (s *SubPubTestSuite) TestMultipleSubscribeUnsubscribe() {
	subject := "test.subject"
	var receivedMsgs []interface{}

	cb := func(msg interface{}) {
		receivedMsgs = append(receivedMsgs, msg)
	}

	sub1, err := s.sp.Subscribe(subject, cb)
	s.NoError(err)
	sub1.Unsubscribe()

	sub2, err := s.sp.Subscribe(subject, cb)
	s.NoError(err)

	err = s.sp.Publish(subject, "message1")
	s.NoError(err)

	time.Sleep(200 * time.Millisecond)
	s.Equal([]interface{}{"message1"}, receivedMsgs)

	sub2.Unsubscribe()
}
