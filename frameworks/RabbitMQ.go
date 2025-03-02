package frameworks

import (
 "context"
 "encoding/json"
 "fmt"
 "os"
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
 connection    *amqp091.Connection
 channel       *amqp091.Channel
 retryDuration time.Duration
 autoAck       bool
}

// Initialize RabbitMQ server.
func (r *rabbitServer) init(config types.IRabbitMQServer) {
 r.dsn = fmt.Sprintf("amqp://%s:%s@%s:%s/%s", config.GetUsername(), config.GetPassword(), config.GetHost(), config.GetPort(), config.GetVHost())
 r.config = amqp091.Config{Properties: amqp091.NewConnectionProperties()}
 r.config.Properties.SetClientConnectionName(os.Getenv("SERVICE_ID"))
 r.retryDuration = time.Duration(10) * time.Second
 r.config.Heartbeat = time.Duration(5) * time.Second
 r.autoAck = config.GetAutoAck()
}

// Start consuming.
func (r *rabbitServer) connect() {
 // lets.LogI("RabbitMQ: connecting ...")

 var err error
 for {
  err = nil
  r.connection, err = amqp091.DialConfig(r.dsn, r.config)
  if err != nil {
   lets.LogE("ERR00 RabbitMQ: %s", err.Error())
   lets.LogE("RabbitMQ: wait retry connection ...")
   <-time.After(r.retryDuration)
   continue
  }
  break
 }

 // Listen for error on connection
 go func() {
  lets.LogE("RabbitMQ: %s", <-r.connection.NotifyClose(make(chan *amqp091.Error)))
  go RabbitMQ() // Retry Connection
 }()

 // Create channel connection.
 if r.channel, err = r.connection.Channel(); err != nil {
  lets.LogE("ERR10 RabbitMQ: %s", err.Error())
  return
 }

 // type Queue struct {
 // 	Name  string `json:"name"`
 // 	VHost string `json:"vhost"`
 // }

 // manager := "http://127.0.0.1:15672/api/queues/"
 // client := &http.Client{}
 // req, _ := http.NewRequest("GET", manager, nil)
 // req.SetBasicAuth("guest", "guest")
 // resp, _ := client.Do(req)

 // value := make([]Queue, 0)
 // json.NewDecoder(resp.Body).Decode(&value)

 // for _, queue := range value {
 // 	if strings.Contains(queue.Name, "amq.gen-") {
 // 		lets.LogD("%s deleted", queue.Name)
 // 		count, err := r.channel.QueueDelete(queue.Name, true, true, true)
 // 		lets.LogD("%v %v", count, err)
 // 	}
 // }

 lets.LogI("RabbitMQ: connected")
}

func (r *rabbitServer) Disconnect() {
 lets.LogI("RabbitMQ Client Stopping ...")

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

 // Consume message
 if r.deliveries, err = server.channel.Consume(
  r.queue.Name,       // name
  consumer.GetName(), // consumerTag,
  server.autoAck,     // autoAck
  false,              // exclusive
  false,              // noLocal
  false,              // noWait
  nil,                // arguments
 ); err != nil {
  lets.LogE("ERR24 RabbitMQ: %s", err.Error())

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
  lets.LogE("RabbitMQ Server DC: %s", "Delivery channel is closed.")
  //r.listenMessage(server, consumer)
  time.Sleep(1 * time.Second)
  r.consume(server, consumer)
  lets.LogE("RabbitMQ listenMessage reconsume...")
  //r.done <- nil
 }
 defer cleanup()

 var deliveryCount uint64 = 0
 for delivery := range r.deliveries {
  if consumer.GetDebug() {
   deliveryCount++
   lets.LogD("RabbitMQ Server: %s %d Byte delivery: [%v] \n%q", consumer.GetName(), len(delivery.Body), delivery.DeliveryTag, delivery.Body)
  }

  // Bind body into types.RabbitBody.
  body := consumer.GetBody()
  err := json.Unmarshal(delivery.Body, &body)
  if err != nil {
   lets.LogE("RabbitMQ Server: %s", err.Error())
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
  r.engine.Call(event.Name, &event)

  // If not auto ack, trigger acknowledge for message
  if !server.autoAck {
   delivery.Ack(false)
  }
 }
}

func (r *rabbitConsumer) reconnect(server *rabbitServer, consumer types.IRabbitMQConsumer) {
 var err error
 // Create channel connection.
 if server.channel, err = server.connection.Channel(); err != nil {
  lets.LogE("ERR R00 RabbitMQ: %s", err.Error())
  return
 }

 r.qos(server, consumer)
 r.declare(server, consumer)
}

func (r *rabbitConsumer) qos(server *rabbitServer, consumer types.IRabbitMQConsumer) {
 // Queue channel qos
 if err := server.channel.Qos(
  consumer.GetPrefetchCount(),
  consumer.GetPrefetchSize(),
  false,
 ); err != nil {
  return
 }
}

func (r *rabbitConsumer) declare(server *rabbitServer, consumer types.IRabbitMQConsumer) {
 var err error
 // lets.LogI("RabbitMQ: declare queue ...")
 if server.channel, err = server.connection.Channel(); err != nil {
  lets.LogE("ERR R00 RabbitMQ: %s", err.Error())
  return
 }
 
 // Declare (or using existing) queue.
 if r.queue, err = server.channel.QueueDeclare(
  consumer.GetQueue(), // name of the queue
  true,                // durable
  false,               // delete when unused
  false,               // exclusive
  false,               // noWait
  nil,                 // arguments
 ); err != nil {
  lets.LogE("ERR D01 RabbitMQ: %s", err.Error())
  return
 }

 // Bind queue to exchange.
 if err = server.channel.QueueBind(
  r.queue.Name,             // name of the queue
  consumer.GetRoutingKey(), // routing key
  consumer.GetExchange(),   // sourceExchange
  false,                    // noWait
  nil,                      // arguments
 ); err != nil {
  lets.LogE("ERR D02 RabbitMQ: %s", err.Error())

  // Retry Connection
  time.Sleep(10 * time.Second)
  go r.declare(server, consumer)
  return
 }

 lets.LogD("RabbitMQ: declare queue finish ...")
}

// RabbitMQ publisher definitions.
type RabbitPublisher struct {
 channel *amqp091.Channel
 name    string
}

func (r *RabbitPublisher) init(server *rabbitServer, publisher types.IRabbitMQPublisher) {
 r.channel = server.channel
 r.name = publisher.GetName()
}

func (r *RabbitPublisher) Publish(event types.IEvent) (err error) {
 if r.channel == nil {
  lets.LogE("RabbitMQ Publisher: Channel is nil or publisher not configured.")
  return
 }

 var body = event.GetBody()
 var replyTo = event.GetReplyTo()

 // Encode object to json string
 if event.GetDebug() {
  seqNo := r.channel.GetNextPublishSeqNo()
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

 if err = r.channel.PublishWithContext(ctx,
  event.GetExchange(),   // Exchange
  event.GetRoutingKey(), // RoutingKey or queues
  false,                 // Mandatory
  false,                 // Immediate
  publishing,
 ); err != nil {
  lets.LogE("RabbitMQ Publisher: %s", err.Error())
  return
 }

 return
}

// Define rabbit service host and port
func RabbitMQ() (disconnectors []func()) {
 if RabbitMQConfig == nil {
  return
 }

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
      rc.qos(&rs, consumer)
     }

     consumer.GetListener()(rc.engine)

     time.Sleep(100 * time.Millisecond)
     go func(c types.IRabbitMQConsumer) {
      wg.Done()
      rc.declare(&rs, c)
      rc.qos(&rs, c)
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
 return
}
