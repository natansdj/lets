package frameworks

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/natansdj/lets"
	"github.com/natansdj/lets/rabbitmq"
	"github.com/natansdj/lets/types"

	"github.com/rabbitmq/amqp091-go"
)

// Initialize RabbitMQ Configuration.
var RabbitMQConfig types.IRabbitMQConfig

// RabbitMQ server defirinitions.
type rabbitServer struct {
	dsn           string
	config        amqp091.Config
	connection    *amqp091.Connection        // Legacy: Direct connection (deprecated, use pooledConn)
	channel       *amqp091.Channel           // Legacy: Direct channel (deprecated, use channelMgr)
	pooledConn    *rabbitmq.PooledConnection // New: Pooled connection
	channelMgr    *rabbitmq.ChannelManager   // New: Channel manager
	retryDuration time.Duration
	autoAck       bool
	usePool       bool // Flag to enable pool usage
}

// Initialize RabbitMQ server.
func (r *rabbitServer) init(config types.IRabbitMQServer) {
	r.dsn = fmt.Sprintf("amqp://%s:%s@%s:%s/%s", config.GetUsername(), config.GetPassword(), config.GetHost(), config.GetPort(), config.GetVHost())
	r.config = amqp091.Config{Properties: amqp091.NewConnectionProperties()}
	r.config.Properties.SetClientConnectionName(os.Getenv("SERVICE_ID"))
	r.retryDuration = time.Duration(10) * time.Second
	r.config.Heartbeat = time.Duration(5) * time.Second
	r.autoAck = config.GetAutoAck()
	r.usePool = true // Enable pooling by default
}

// Start consuming.
func (r *rabbitServer) connect() {
	if r.usePool {
		r.connectWithPool()
	} else {
		r.connectLegacy()
	}
}

// connectWithPool uses the new connection pool (recommended)
func (r *rabbitServer) connectWithPool() {
	pool := rabbitmq.GetConnectionPool()

	var err error
	r.pooledConn, err = pool.GetConnection(r.dsn, r.config)
	if err != nil {
		lets.LogE("RabbitMQ Pool: Failed to get pooled connection: %v", err)
		// Fallback to legacy connection
		r.usePool = false
		r.connectLegacy()
		return
	}

	// Set legacy connection reference for backward compatibility
	r.connection = r.pooledConn.GetRawConnection()

	// Initialize channel manager
	r.channelMgr = rabbitmq.NewChannelManager(r.pooledConn)

	lets.LogI("RabbitMQ: Connected via pool (vhost: %s)", extractVHost(r.dsn))
}

// connectLegacy uses the old direct connection method (for backward compatibility)
func (r *rabbitServer) connectLegacy() {
	var err error
	for {
		err = nil
		r.connection, err = amqp091.DialConfig(r.dsn, r.config)
		if err != nil {
			lets.LogERL("rabbitmq-connect-failed", "ERR00 RabbitMQ: %s", err.Error())
			lets.LogERL("rabbitmq-retry-connection", "RabbitMQ: wait retry connection ...")
			<-time.After(r.retryDuration)
			continue
		}
		break
	}

	// Listen for error on connection
	go func() {
		lets.LogERL("rabbitmq-connection-closed", "RabbitMQ: %s", <-r.connection.NotifyClose(make(chan *amqp091.Error)))
		go RabbitMQ() // Retry Connection
	}()

	// Create channel connection.
	if r.channel, err = r.connection.Channel(); err != nil {
		lets.LogERL("rabbitmq-channel-error", "ERR10 RabbitMQ: %s", err.Error())
		return
	}

	lets.LogI("RabbitMQ: connected (legacy mode)")
}

// getChannel returns a channel, using pool if available
func (r *rabbitServer) getChannel() (*amqp091.Channel, error) {
	if r.usePool && r.channelMgr != nil {
		pc, err := r.channelMgr.GetChannel()
		if err != nil {
			return nil, err
		}
		return pc.GetRawChannel(), nil
	}

	// Legacy: return the shared channel
	if r.channel != nil {
		return r.channel, nil
	}

	return nil, fmt.Errorf("no channel available")
}

// extractVHost extracts vhost from DSN for logging
func extractVHost(dsn string) string {
	for i := len(dsn) - 1; i >= 0; i-- {
		if dsn[i] == '/' {
			if i+1 < len(dsn) {
				return dsn[i+1:]
			}
			return "/"
		}
	}
	return "/"
}

func (r *rabbitServer) Disconnect() {
	lets.LogI("RabbitMQ Client Stopping ...")

	// Close channel manager if using pool
	if r.usePool && r.channelMgr != nil {
		if err := r.channelMgr.CloseAll(); err != nil {
			lets.LogE("RabbitMQ: Error closing channel manager: %v", err)
		}
	}

	// Close legacy channel if exists
	if !r.usePool && r.channel != nil {
		if err := r.channel.Close(); err != nil {
			lets.LogE("RabbitMQ: Error closing channel: %v", err)
		}
	}

	// Note: We don't close pooled connections here as they're managed by the pool
	// and may be shared across multiple servers

	lets.LogI("RabbitMQ Client Stopped ...")
}

// RabbitMQ consumer definitions.
type rabbitConsumer struct {
	queue      amqp091.Queue
	deliveries <-chan amqp091.Delivery
	done       chan error
	engine     types.Engine
}

// Start consuming.
func (r *rabbitConsumer) consume(server *rabbitServer, consumer types.IRabbitMQConsumer) {
	lets.LogD("RabbitMQ Consumer: consuming ...")

	var err error
	var channel *amqp091.Channel

	// Get channel from pool or legacy
	if server.usePool && server.channelMgr != nil {
		pc, err := server.channelMgr.GetChannel()
		if err != nil {
			lets.LogERL("rabbitmq-consume-channel-error", "ERR23 RabbitMQ: Failed to get channel: %s", err.Error())
			time.Sleep(10 * time.Second)
			r.reconnect(server, consumer)
			r.consume(server, consumer)
			return
		}
		channel = pc.GetRawChannel()
	} else {
		channel = server.channel
	}

	if channel == nil {
		lets.LogERL("rabbitmq-consume-nil-channel", "ERR23.5 RabbitMQ: Channel is nil")
		time.Sleep(10 * time.Second)
		r.reconnect(server, consumer)
		r.consume(server, consumer)
		return
	}

	// Consume message
	if r.deliveries, err = channel.Consume(
		r.queue.Name,       // name
		consumer.GetName(), // consumerTag,
		server.autoAck,     // autoAck
		false,              // exclusive
		false,              // noLocal
		false,              // noWait
		nil,                // arguments
	); err != nil {
		lets.LogERL("rabbitmq-consume-error", "ERR24 RabbitMQ: %s", err.Error())

		// Retry Connection
		time.Sleep(10 * time.Second)
		r.reconnect(server, consumer)
		r.consume(server, consumer)
		return
	}

	r.listenMessage(server, consumer)
}

func (r *rabbitConsumer) listenMessage(server *rabbitServer, consumer types.IRabbitMQConsumer) {
	cleanup := func() {
		//TODO : handle reconnection
		lets.LogERL("rabbitmq-channel-closed", "RabbitMQ Server DC: %s", "Delivery channel is closed.")
		//r.listenMessage(server, consumer)
		time.Sleep(1 * time.Second)
		r.consume(server, consumer)
		lets.LogERL("rabbitmq-reconsume", "RabbitMQ listenMessage reconsume...")
		//r.done <- nil
	}
	defer cleanup()

	var deliveryCount uint64 = 0
	sem := make(chan struct{}, consumer.GetConcurrentCall()) // limit goroutines

	// Message processing loop
	for delivery := range r.deliveries {
		// CRITICAL: Check if delivery is from a closed channel (zero value)
		// When channel closes, range returns zero values infinitely causing CPU spike
		if delivery.DeliveryTag == 0 && len(delivery.Body) == 0 && delivery.ConsumerTag == "" {
			lets.LogERL("rabbitmq-empty-delivery", "RabbitMQ Server: Empty delivery detected (channel likely closed) for consumer %s", consumer.GetName())
			// Channel is closed, break out of loop to trigger reconnection via cleanup()
			break
		}

		if consumer.GetDebug() {
			deliveryCount++
			lets.LogD("RabbitMQ Server: %s %d Byte delivery: [%v] \n%q", consumer.GetName(), len(delivery.Body), delivery.DeliveryTag, delivery.Body)
		}

		// VALIDATION: Check for empty or whitespace-only messages BEFORE JSON parsing
		// This prevents unnecessary CPU cycles on invalid messages
		trimmedBody := strings.TrimSpace(string(delivery.Body))
		if trimmedBody == "" {
			lets.LogERL("rabbitmq-empty-message", "RabbitMQ Server: Empty message received on queue %s (consumer: %s)", r.queue.Name, consumer.GetName())

			// NACK the message without requeue to move it to DLX or discard it
			if !server.autoAck {
				delivery.Nack(false, false) // multiple=false, requeue=false
			}
			continue
		}

		// Bind body into types.RabbitBody.
		body := consumer.GetBody()
		err := json.Unmarshal(delivery.Body, &body)
		if err != nil {
			lets.LogERL("rabbitmq-parse-error", "RabbitMQ Server: JSON parse error for consumer %s: %s", consumer.GetName(), err.Error())

			// NACK the message - it's malformed and won't be processable on retry
			if !server.autoAck {
				delivery.Nack(false, false) // multiple=false, requeue=false
			}
			continue
		}

		// Read reply to
		var replyTo types.ReplyTo
		if delivery.ReplyTo != "" {
			if err := json.Unmarshal([]byte(delivery.ReplyTo), &replyTo); err != nil {
				lets.LogE("RabbitMQ Server: ReplyTo Error: %s", err.Error())

				if delivery.ReplyTo != "" {
					lets.LogE("RabbitMQ Server: Set routing key / queue: %s", delivery.ReplyTo)

					replyTo.SetRoutingKey(delivery.ReplyTo)
				}
			}
		}

		// Create event data.
		event := types.Event{
			Name:          body.GetEvent(),
			Data:          body.GetData(),
			ReplyTo:       &replyTo,
			CorrelationId: delivery.CorrelationId,
			Exchange:      delivery.Exchange,
			RoutingKey:    delivery.RoutingKey,
		}

		// Call event handler.
		if consumer.GetConcurrentCall() > 1 {
			sem <- struct{}{}
			go func(ev *types.Event) {
				defer func() { <-sem }()
				r.engine.Call(ev.Name, ev)
			}(&event)
		} else {
			r.engine.Call(event.Name, &event)
		}

		// If not auto ack, trigger acknowledge for message
		if !server.autoAck {
			delivery.Ack(false)
		}
	}
}

func (r *rabbitConsumer) reconnect(server *rabbitServer, consumer types.IRabbitMQConsumer) {
	var err error
	var channel *amqp091.Channel

	// Get channel from pool or legacy
	if server.usePool && server.channelMgr != nil {
		pc, err := server.channelMgr.GetChannel()
		if err != nil {
			lets.LogERL("rabbitmq-reconnect-channel-error", "ERR R00 RabbitMQ: Failed to get channel: %s", err.Error())
			return
		}
		channel = pc.GetRawChannel()
	} else {
		// Create channel connection (legacy).
		if channel, err = server.connection.Channel(); err != nil {
			lets.LogERL("rabbitmq-reconnect-error", "ERR R00 RabbitMQ: %s", err.Error())
			return
		}
		server.channel = channel
	}

	r.qos(channel, consumer)
	r.declareWithChannel(channel, server, consumer)
}

func (r *rabbitConsumer) qos(channel *amqp091.Channel, consumer types.IRabbitMQConsumer) {
	// Queue channel qos
	if err := channel.Qos(
		consumer.GetPrefetchCount(),
		consumer.GetPrefetchSize(),
		false,
	); err != nil {
		lets.LogERL("rabbitmq-qos-error", "RabbitMQ QoS error: %s", err.Error())
		return
	}
}

func (r *rabbitConsumer) declare(server *rabbitServer, consumer types.IRabbitMQConsumer) {
	var err error
	var channel *amqp091.Channel

	// Get channel from pool or legacy
	if server.usePool && server.channelMgr != nil {
		pc, err := server.channelMgr.GetChannel()
		if err != nil {
			lets.LogERL("rabbitmq-declare-channel-error", "ERR D00 RabbitMQ: Failed to get channel: %s", err.Error())
			return
		}
		channel = pc.GetRawChannel()
	} else {
		// Create channel connection (legacy)
		if channel, err = server.connection.Channel(); err != nil {
			lets.LogERL("rabbitmq-declare-channel-error", "ERR R00 RabbitMQ: %s", err.Error())
			return
		}
		server.channel = channel
	}

	r.declareWithChannel(channel, server, consumer)
}

func (r *rabbitConsumer) declareWithChannel(channel *amqp091.Channel, server *rabbitServer, consumer types.IRabbitMQConsumer) {
	var err error

	// Declare (or using existing) queue.
	if r.queue, err = channel.QueueDeclare(
		consumer.GetQueue(), // name of the queue
		true,                // durable
		false,               // delete when unused
		false,               // exclusive
		false,               // noWait
		nil,                 // arguments
	); err != nil {
		lets.LogERL("rabbitmq-queue-declare-error", "ERR D01 RabbitMQ: %s", err.Error())
		return
	}

	// Bind queue to exchange.
	if err = channel.QueueBind(
		r.queue.Name,             // name of the queue
		consumer.GetRoutingKey(), // routing key
		consumer.GetExchange(),   // sourceExchange
		false,                    // noWait
		nil,                      // arguments
	); err != nil {
		lets.LogERL("rabbitmq-queue-bind-error", "ERR D02 RabbitMQ: %s", err.Error())

		// Retry Connection
		time.Sleep(10 * time.Second)
		go r.declare(server, consumer)
		return
	}

	lets.LogD("RabbitMQ: declare queue finish ...")
}

// RabbitMQ publisher definitions.
type RabbitPublisher struct {
	channel    *amqp091.Channel         // Legacy: Direct channel
	channelMgr *rabbitmq.ChannelManager // New: Channel manager for pooled access
	name       string
	usePool    bool
}

func (r *RabbitPublisher) init(server *rabbitServer, publisher types.IRabbitMQPublisher) {
	r.name = publisher.GetName()
	r.usePool = server.usePool

	if server.usePool && server.channelMgr != nil {
		r.channelMgr = server.channelMgr
	} else {
		r.channel = server.channel
	}
}

func (r *RabbitPublisher) Publish(event types.IEvent) (err error) {
	// Get channel (pooled or legacy)
	var channel *amqp091.Channel

	if r.usePool && r.channelMgr != nil {
		pc, err := r.channelMgr.GetChannel()
		if err != nil {
			lets.LogERL("rabbitmq-publisher-channel-error", "RabbitMQ Publisher: Failed to get channel: %s", err.Error())
			return err
		}
		channel = pc.GetRawChannel()
	} else {
		channel = r.channel
	}

	if channel == nil {
		lets.LogERL("rabbitmq-publisher-nil-channel", "RabbitMQ Publisher: Channel is nil or publisher not configured.")
		return fmt.Errorf("channel is nil")
	}

	var body = event.GetBody()
	var replyTo = event.GetReplyTo()

	// Encode object to json string
	if event.GetDebug() {
		seqNo := channel.GetNextPublishSeqNo()
		lets.LogD("RabbitMQ Publisher: to: exchange '%s'; routing key/queue: '%s'", event.GetExchange(), event.GetRoutingKey())
		lets.LogD("RabbitMQ Publisher: sequence no: %d; %d Bytes; Body: \n%s", seqNo, len(body), string(body))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	publishing := amqp091.Publishing{
		Headers:         amqp091.Table{},
		ContentType:     "application/json",
		ContentEncoding: "",
		Body:            []byte(body),
		DeliveryMode:    amqp091.Transient, // 1=non-persistent, 2=persistent
		Priority:        0,                 // 0-9
		// a bunch of application/implementation-specific fields
		// ReplyTo:       replyTo.Get(),
		CorrelationId: event.GetCorrelationId(),
	}

	if replyTo != nil {
		publishing.ReplyTo = replyTo.Get()
	}

	if err = channel.PublishWithContext(ctx,
		event.GetExchange(),   // Exchange
		event.GetRoutingKey(), // RoutingKey or queues
		false,                 // Mandatory
		false,                 // Immediate
		publishing,
	); err != nil {
		lets.LogERL("rabbitmq-publish-error", "RabbitMQ Publisher: %s", err.Error())
		return
	}

	return
}

// Define rabbit service host and port
func RabbitMQ() (disconnectors []func()) {
	if RabbitMQConfig == nil {
		return
	}

	// Start monitoring (every 5 minutes)
	rabbitmq.StartMonitoring(5 * time.Minute)

	// Running RabbitMQ
	if servers := RabbitMQConfig.GetServers(); len(servers) != 0 {
		time.Sleep(100 * time.Millisecond)
		lets.LogI("RabbitMQ Starting ...")

		for _, server := range servers {
			var rs rabbitServer
			rs.init(server)
			rs.connect()
			disconnectors = append(disconnectors, rs.Disconnect)

			// Consuming RabbitMQ.
			if consumers := server.GetConsumers(); len(consumers) != 0 {
				lets.LogI("RabbitMQ Consumer Starting ...")

				wg := &sync.WaitGroup{}
				wg.Add(len(consumers))

				for i, consumer := range consumers {
					var rc = rabbitConsumer{
						engine: &rabbitmq.Engine{
							Debug: consumer.GetDebug(),
						},
					}

					if i == 0 {
						//rc.declare(&rs, consumer)
						// Get channel and setup QoS for first consumer
						if rs.usePool && rs.channelMgr != nil {
							if pc, err := rs.channelMgr.GetChannel(); err == nil {
								rc.qos(pc.GetRawChannel(), consumer)
							}
						} else if rs.channel != nil {
							rc.qos(rs.channel, consumer)
						}
					}

					consumer.GetListener()(rc.engine)

					time.Sleep(100 * time.Millisecond)
					go func(c types.IRabbitMQConsumer) {
						wg.Done()
						rc.declare(&rs, c)

						// Setup QoS for this consumer's channel
						if rs.usePool && rs.channelMgr != nil {
							if pc, err := rs.channelMgr.GetChannel(); err == nil {
								rc.qos(pc.GetRawChannel(), c)
							}
						} else if rs.channel != nil {
							rc.qos(rs.channel, c)
						}

						rc.consume(&rs, c)
					}(consumer)
				}

				go func() {
					wg.Wait()
					lets.LogI("RabbitMQ Consumer Done ...")
				}()
			}

			if publishers := server.GetPublishers(); len(publishers) != 0 {
				lets.LogI("RabbitMQ Publisher Starting ...")
				for _, publisher := range publishers {
					var rp RabbitPublisher
					rp.init(&rs, publisher)

					lets.LogI("RabbitMQ Publisher: %s", publisher.GetName())
					for _, client := range publisher.GetClients() {
						client.SetConnection(&rp)
					}
				}
			}
		}
	}

	// Register graceful shutdown for connection pool
	poolShutdown := func() {
		lets.LogI("RabbitMQ Pool: Initiating graceful shutdown...")
		if err := rabbitmq.GetConnectionPool().CloseAll(); err != nil {
			lets.LogE("RabbitMQ Pool: Error during shutdown: %v", err)
		}
		lets.LogI("RabbitMQ Pool: Shutdown complete")
	}
	disconnectors = append(disconnectors, poolShutdown)

	return
}
