package models

import (
	"encoding/json"
	"strconv"
	"time"
)

type QueueMessage struct {
	SendOn    int64 `json:"send_on"`
	SentOn    int64 `json:"sent_on"`
	KeepAlive int   `json:"keep_alive"`
}

type QueueStartMessage struct {
	QueueMessage
	AppGuid      string `json:"droplet"`
	AppVersion   string `json:"version"`
	IndexToStart int    `json:"index"`
}

type QueueStopMessage struct {
	QueueMessage
	InstanceGuid string `json:"instance"`
}

func newQueueMessage(now time.Time, delayInSeconds int, keepAliveInSeconds int) QueueMessage {
	return QueueMessage{
		SendOn:    now.Add(time.Duration(delayInSeconds) * time.Second).Unix(),
		SentOn:    0,
		KeepAlive: keepAliveInSeconds,
	}
}

func (message QueueMessage) queueLogDescription() map[string]string {
	return map[string]string{
		"SendOn":    time.Unix(message.SendOn, 0).String(),
		"SentOn":    time.Unix(message.SentOn, 0).String(),
		"KeepAlive": strconv.Itoa(int(message.KeepAlive)),
	}
}

func NewQueueStartMessage(now time.Time, delayInSeconds int, keepAliveInSeconds int, appGuid string, appVersion string, indexToStart int) QueueStartMessage {
	return QueueStartMessage{
		QueueMessage: newQueueMessage(now, delayInSeconds, keepAliveInSeconds),
		AppGuid:      appGuid,
		AppVersion:   appVersion,
		IndexToStart: indexToStart,
	}
}

func NewQueueStartMessageFromJSON(encoded []byte) (QueueStartMessage, error) {
	message := QueueStartMessage{}
	err := json.Unmarshal(encoded, &message)
	if err != nil {
		return QueueStartMessage{}, err
	}
	return message, nil
}

func (message QueueStartMessage) StoreKey() string {
	return message.AppGuid + "-" + message.AppVersion + "-" + strconv.Itoa(message.IndexToStart)
}

func (message QueueStartMessage) ToJSON() []byte {
	encoded, _ := json.Marshal(message)
	return encoded
}

func (message QueueStartMessage) LogDescription() map[string]string {
	base := message.queueLogDescription()
	base["AppGuid"] = message.AppGuid
	base["AppVersion"] = message.AppVersion
	base["IndexToStart"] = strconv.Itoa(message.IndexToStart)
	return base
}

func NewQueueStopMessage(now time.Time, delayInSeconds int, keepAliveInSeconds int, instanceGuid string) QueueStopMessage {
	return QueueStopMessage{
		QueueMessage: newQueueMessage(now, delayInSeconds, keepAliveInSeconds),
		InstanceGuid: instanceGuid,
	}
}

func NewQueueStopMessageFromJSON(encoded []byte) (QueueStopMessage, error) {
	message := QueueStopMessage{}
	err := json.Unmarshal(encoded, &message)
	if err != nil {
		return QueueStopMessage{}, err
	}
	return message, nil
}

func (message QueueStopMessage) ToJSON() []byte {
	encoded, _ := json.Marshal(message)
	return encoded
}

func (message QueueStopMessage) StoreKey() string {
	return message.InstanceGuid
}

func (message QueueStopMessage) LogDescription() map[string]string {
	base := message.queueLogDescription()
	base["InstanceGuid"] = message.InstanceGuid
	return base
}
