package hm

import (
	"github.com/cloudfoundry/gosteno"
	collectorregistrar "github.com/cloudfoundry/hm9000/cfcomponent/registrars/legacycollectorregistrar"
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/helpers/logger"
	"github.com/cloudfoundry/hm9000/helpers/metricsaccountant"
	"github.com/cloudfoundry/hm9000/metricsserver"
)

func ServeMetrics(steno *gosteno.Logger, l logger.Logger, conf *config.Config) {
	store := connectToStore(l, conf)
	messageBus := connectToMessageBus(l, conf)

	acquireLock(l, conf, "metrics-server")

	collectorRegistrar := collectorregistrar.NewCollectorRegistrar(messageBus, steno)

	metricsServer := metricsserver.New(
		collectorRegistrar,
		steno,
		metricsaccountant.New(store),
		l,
		store,
		buildClock(l),
		conf,
	)

	err := metricsServer.Start()
	if err != nil {
		l.Error("Failed to serve metrics", err)
	}
	l.Info("Serving Metrics")
	select {}
}
