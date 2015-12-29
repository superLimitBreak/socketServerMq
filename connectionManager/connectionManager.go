package connectionManager

import (
	"encoding/json"
	log "github.com/Sirupsen/logrus"
	"github.com/SuperLimitBreak/channelMq"
	"github.com/SuperLimitBreak/socketServerMq/connectionTypes"
	"github.com/SuperLimitBreak/socketServerMq/schema"
)

func NewConnectionManager(mq *channelMq.MQ) *ConnectionManager {
	return &ConnectionManager{
		mq,
	}
}

type ConnectionManager struct {
	mq *channelMq.MQ
}

func (c *ConnectionManager) AddConnection(conn connectionTypes.Connection) {
	go c.manage(conn)
}

func (c *ConnectionManager) manage(conn connectionTypes.Connection) {
	log.Info("New Connection in manager")

	in := conn.GetIngressChan()
	eg := c.mq.Subscribe([]string{})
	conn.SetEgressChan(eg)

	for {
		msg, ok := <-in
		if !ok {
			log.Warn("ingressChan closed")

			c.mq.Unsubscribe(eg)
			return
		}

		var generic schema.GenericMessage
		err := json.Unmarshal(msg, &generic)
		if err != nil {
			log.WithError(err).WithFields(log.Fields{
				"json": string(msg),
			}).Error("malformed Json in ingress queue")
			continue
		}

		switch {
		case generic.IsAction(schema.MESSAGE_ACTION):
			messages, err := generic.Messages()
			if err != nil {
				log.WithError(err).Error("Failed to get messages")
			}

			for _, msg := range messages {
				key, err := msg.Key()
				if err != nil {
					log.WithError(err).Error("Failed to get key")
					continue
				}

				if key == nil {
					c.mq.Send(*msg.Data, []string{})
				} else {
					c.mq.Send(*msg.Data, []string{*key})
				}
			}

		case generic.IsAction(schema.SUBSCRIBE_ACTION):
			log.Info("Change Subscription")

			keys, err := generic.Keys()
			if err != nil {
				log.WithError(err).WithFields(log.Fields{
					"json": string(msg),
				}).Error("malformed Json subscribe in ingress queue")
				continue
			}

			c.mq.Unsubscribe(eg)
			eg = c.mq.Subscribe(keys)
			conn.SetEgressChan(eg)

		default:
			log.WithFields(log.Fields{
				"json":   string(msg),
				"action": generic.Action,
			}).Error("Unknown action in json")
			continue
		}

	}
}