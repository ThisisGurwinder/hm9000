package hm

import (
	"github.com/cloudfoundry/hm9000/actualstatelistener"
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/helpers/logger"
	"github.com/cloudfoundry/hm9000/helpers/metricsaccountant"
)

func StartListeningForActual(l logger.Logger, conf *config.Config) {
	messageBus := connectToMessageBus(l, conf)
	store, usageTracker := connectToStoreAndTrack(l, conf)

	acquireLock(l, conf, "listener")

	listener := actualstatelistener.New(conf,
		messageBus,
		store,
		usageTracker,
		metricsaccountant.New(),
		buildClock(l),
		l,
	)

	listener.Start()
	l.Info("Listening for Actual State")
	select {}
}
