package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	amqp "github.com/rabbitmq/amqp091-go"
)

const eventQueue = "order.created"
const deadQueue = "order.created.dead"

// 这是跨语言的 JSON 约定。eventId 标识事件，orderId 标识业务订单，Java 用 eventId 去重。
type orderCreated struct {
	EventID int64  `json:"eventId"`
	Type    string `json:"type"`
	OrderID int64  `json:"orderId"`
}

func relay(db *pgxpool.Pool, brokerURL string) error {
	conn, err := amqp.DialConfig(brokerURL, amqp.Config{Dial: amqp.DefaultDial(5 * time.Second)})
	if err != nil {
		return err
	}
	defer conn.Close()
	channel, err := conn.Channel()
	if err != nil {
		return err
	}
	defer channel.Close()

	// 先声明死信队列。被拒绝且不再重试的消息，可以在这里检查。
	if _, err := channel.QueueDeclare(deadQueue, true, false, false, false, nil); err != nil {
		return err
	}
	arguments := amqp.Table{
		"x-queue-type":              "quorum",
		"x-delivery-limit":          int32(5),
		"x-dead-letter-exchange":    "",
		"x-dead-letter-routing-key": deadQueue,
	}
	if _, err := channel.QueueDeclare(eventQueue, true, false, false, false, arguments); err != nil {
		return err
	}
	// 开启发布确认。后面的 WaitContext 等的是 RabbitMQ 对发布的答复。
	if err := channel.Confirm(false); err != nil {
		return err
	}
	returns := channel.NotifyReturn(make(chan amqp.Return, 1))
	closed := channel.NotifyClose(make(chan *amqp.Error, 1))

	for {
		select {
		case err := <-closed:
			return fmt.Errorf("relay channel closed: %v", err)
		default:
		}

		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		sent, err := publishNext(ctx, db, channel, returns)
		cancel()
		if err != nil {
			return err
		}
		if !sent {
			// 当前没有待发事件时稍等，避免一直查询空表。
			time.Sleep(300 * time.Millisecond)
		}
	}
}

func publishNext(ctx context.Context, db *pgxpool.Pool, channel *amqp.Channel, returns <-chan amqp.Return) (bool, error) {
	event := orderCreated{Type: "order.created.v1"}
	err := db.QueryRow(ctx, "SELECT id, order_id FROM outbox WHERE published_at IS NULL ORDER BY id LIMIT 1").
		Scan(&event.EventID, &event.OrderID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	body, err := json.Marshal(event)
	if err != nil {
		return false, err
	}
	// 空字符串选择默认 exchange，routing key 用队列名。mandatory=true 要求退回无法路由的消息。
	confirmation, err := channel.PublishWithDeferredConfirmWithContext(ctx, "", eventQueue, true, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		MessageId:    fmt.Sprint(event.EventID),
		Body:         body,
	})
	if err != nil {
		return false, err
	}
	ok, err := confirmation.WaitContext(ctx)
	if err != nil {
		return false, err
	}
	if !ok {
		return false, fmt.Errorf("broker rejected event %d", event.EventID)
	}
	// 无法路由的消息会先收到退回，再收到发布确认。得检查两者才能标记已发。
	select {
	case returned, open := <-returns:
		if open {
			return false, fmt.Errorf("unroutable event %d: %s", event.EventID, returned.ReplyText)
		}
	default:
	}
	// 如果进程在收到确认后、执行下面的 UPDATE 前退出，同一 eventId 会再次发送。
	// 消费者用数据库唯一键去重，让这两次发布只产生一条通知记录。
	_, err = db.Exec(ctx, "UPDATE outbox SET published_at = now() WHERE id = $1", event.EventID)
	if err == nil {
		log.Printf("published event %d for order %d", event.EventID, event.OrderID)
	}
	return true, err
}
