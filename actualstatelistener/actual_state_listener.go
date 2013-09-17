package actualstatelistener

import (
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/helpers/bel_air"
	"github.com/cloudfoundry/hm9000/helpers/logger"
	"github.com/cloudfoundry/hm9000/helpers/time_provider"
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/store"

	"github.com/cloudfoundry/go_cfmessagebus"
)

type ActualStateListener struct {
	logger.Logger
	config         config.Config
	messageBus     cfmessagebus.MessageBus
	heartbeatStore store.Store
	freshPrince    bel_air.FreshPrince
	timeProvider   time_provider.TimeProvider
}

func New(config config.Config,
	messageBus cfmessagebus.MessageBus,
	heartbeatStore store.Store,
	freshPrince bel_air.FreshPrince,
	timeProvider time_provider.TimeProvider,
	logger logger.Logger) *ActualStateListener {

	return &ActualStateListener{
		Logger:         logger,
		config:         config,
		messageBus:     messageBus,
		heartbeatStore: heartbeatStore,
		freshPrince:    freshPrince,
		timeProvider:   timeProvider,
	}
}

func (listener *ActualStateListener) Start() {
	listener.messageBus.Subscribe("dea.heartbeat", func(messageBody []byte) {
		listener.bumpFreshness()

		heartbeat, err := models.NewHeartbeatFromJSON(messageBody)

		if err != nil {
			listener.Info("Could not unmarshal heartbeat",
				map[string]string{
					"Error":       err.Error(),
					"MessageBody": string(messageBody),
				})
			return
		}

		nodes := make([]store.StoreNode, len(heartbeat.InstanceHeartbeats))
		for i, instance := range heartbeat.InstanceHeartbeats {
			nodes[i] = store.StoreNode{
				Key:   "/actual/" + instance.InstanceGuid,
				Value: instance.ToJson(),
				TTL:   listener.config.HeartbeatTTL,
			}
		}

		err = listener.heartbeatStore.Set(nodes)

		if err != nil {
			listener.Info("Could not put instance heartbeats in store:",
				map[string]string{
					"Error": err.Error(),
				})
		}
	})
}

func (listener *ActualStateListener) bumpFreshness() {
	err := listener.freshPrince.Bump(listener.config.ActualFreshnessKey, listener.config.ActualFreshnessTTL, listener.timeProvider.Time())

	if err != nil {
		listener.Info("Could not update actual freshness",
			map[string]string{
				"Error": err.Error(),
			})
	}
}
