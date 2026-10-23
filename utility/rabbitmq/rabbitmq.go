package rabbitmq

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	amqp "github.com/rabbitmq/amqp091-go"
)

var (
	// ErrNacked: the broker refused the message (for example it could not
	// write it). Retry later with the same message id.
	ErrNacked = errors.New("broker nacked the message")
	// ErrUnroutable: no queue is bound for the routing key. Retrying will
	// not help until the topology is declared.
	ErrUnroutable = errors.New("no queue bound for the routing key")
)

// LoadURL reads rabbitmq.url, or the RABBITMQ_URL environment variable.
func LoadURL(ctx context.Context) (string, error) {
	v, err := g.Cfg().Get(ctx, "rabbitmq.url")
	if err != nil {
		return "", gerror.Wrap(err, "read rabbitmq.url")
	}
	url := v.String()
	if url == "" {
		url = os.Getenv("RABBITMQ_URL")
	}
	if url == "" {
		return "", gerror.New("rabbitmq.url (or RABBITMQ_URL) is required")
	}
	return url, nil
}

// Message is one event to publish.
type Message struct {
	RoutingKey string
	// MessageID is the event id: the same on every retry, so consumers can
	// recognise a repeat.
	MessageID string
	Body      []byte
	// TTL, if set, makes the message expire (and be dead-lettered) after
	// this long. Zero means no expiry; a negative value expires at once.
	TTL time.Duration
	// Delayed: the consumer should start a new trace linked to this one
	// rather than continue it (see headerDelayed).
	Delayed bool
}

// Publisher publishes persistent, mandatory messages on a confirm-mode
// channel and waits for the broker's answer. It reconnects on the next call
// after the connection drops. Calls are serialised, which is what lets a
// returned (unroutable) message be matched to the publish it belongs to.
type Publisher struct {
	url     string
	timeout time.Duration

	mu      sync.Mutex
	conn    *amqp.Connection
	ch      *amqp.Channel
	returns chan amqp.Return
}

// NewPublisher does not connect yet. timeout bounds each publish (design:
// 5s); running out is an unknown outcome, to be retried with the same id.
func NewPublisher(url string, timeout time.Duration) *Publisher {
	return &Publisher{url: url, timeout: timeout}
}

func (p *Publisher) Publish(ctx context.Context, m Message) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.connect(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	msg := amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		MessageId:    m.MessageID,
		Timestamp:    time.Now(),
		Body:         m.Body,
		Headers:      amqp.Table{},
	}
	inject(ctx, msg.Headers)
	if m.Delayed {
		msg.Headers[headerDelayed] = true
	}
	if m.TTL != 0 {
		ms := m.TTL.Milliseconds()
		if ms < 0 {
			ms = 0
		}
		msg.Expiration = strconv.FormatInt(ms, 10)
	}
	confirm, err := p.ch.PublishWithDeferredConfirmWithContext(ctx, Exchange, m.RoutingKey, true, false, msg)
	if err != nil {
		p.reset()
		return err
	}
	acked, err := confirm.WaitContext(ctx)
	if err != nil {
		p.reset() // the confirm may still arrive on this channel; start clean
		return err
	}
	if !acked {
		return ErrNacked
	}
	// The broker sends basic.return before the ack of the same message, and
	// the client queues it before handling the ack.
	select {
	case r := <-p.returns:
		if r.MessageId == m.MessageID {
			return fmt.Errorf("%w: %s", ErrUnroutable, m.RoutingKey)
		}
	default:
	}
	return nil
}

func (p *Publisher) connect() error {
	if p.ch != nil && !p.ch.IsClosed() && !p.conn.IsClosed() {
		return nil
	}
	p.reset()
	conn, err := amqp.Dial(p.url)
	if err != nil {
		return err
	}
	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return err
	}
	if err := ch.Confirm(false); err != nil {
		conn.Close()
		return err
	}
	if err := Declare(ch); err != nil {
		conn.Close()
		return err
	}
	p.conn, p.ch = conn, ch
	p.returns = ch.NotifyReturn(make(chan amqp.Return, 16))
	return nil
}

func (p *Publisher) reset() {
	if p.conn != nil {
		p.conn.Close()
	}
	p.conn, p.ch, p.returns = nil, nil, nil
}

// Close closes the connection.
func (p *Publisher) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.reset()
}

// Handler processes one delivery. nil acknowledges it; an error leaves it to
// be delivered again (after a pause), because the work was not recorded.
type Handler func(ctx context.Context, d amqp.Delivery) error

// Consume runs handler for every message of queue until ctx ends,
// reconnecting with backoff when the connection drops. prefetch bounds how
// many unacknowledged messages this consumer holds.
func Consume(ctx context.Context, url, queue string, prefetch int, handler Handler) {
	backoff := time.Second
	for ctx.Err() == nil {
		connected, err := consumeOnce(ctx, url, queue, prefetch, handler)
		if ctx.Err() != nil {
			return
		}
		if connected {
			backoff = time.Second // it worked for a while: start over
		}
		g.Log().Warningf(ctx, "consumer %s: %v; reconnecting in %s", queue, err, backoff)
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		if backoff < 30*time.Second {
			backoff *= 2
		}
	}
}

// consumeOnce reports whether it got as far as consuming, so a connection
// that worked for a while resets the backoff.
func consumeOnce(ctx context.Context, url, queue string, prefetch int, handler Handler) (bool, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return false, err
	}
	defer conn.Close()
	ch, err := conn.Channel()
	if err != nil {
		return false, err
	}
	if err := Declare(ch); err != nil {
		return false, err
	}
	if err := ch.Qos(prefetch, 0, false); err != nil {
		return false, err
	}
	deliveries, err := ch.ConsumeWithContext(ctx, queue, "", false, false, false, false, nil)
	if err != nil {
		return false, err
	}
	g.Log().Infof(ctx, "consumer %s: connected", queue)
	for {
		select {
		case <-ctx.Done():
			return true, ctx.Err()
		case d, ok := <-deliveries:
			if !ok {
				return true, errors.New("delivery channel closed")
			}
			dctx, span := startConsume(ctx, queue, d)
			err := handler(dctx, d)
			endSpan(span, err)
			if err != nil {
				g.Log().Warningf(ctx, "consumer %s: message %s not processed, will be redelivered: %v", queue, d.MessageId, err)
				// A short pause so a failing dependency is not hammered.
				select {
				case <-ctx.Done():
				case <-time.After(time.Second):
				}
				if err := d.Nack(false, true); err != nil {
					return true, err
				}
				continue
			}
			if err := d.Ack(false); err != nil {
				return true, err
			}
		}
	}
}
