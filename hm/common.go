package hm

import (
	"github.com/cloudfoundry/go_cfmessagebus"
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/helpers/logger"
	"github.com/cloudfoundry/hm9000/store"
	"github.com/cloudfoundry/hm9000/storeadapter"

	"os"
)

func connectToMessageBus(l logger.Logger, conf config.Config) cfmessagebus.MessageBus {
	messageBus, err := cfmessagebus.NewMessageBus("NATS")
	if err != nil {
		l.Info("Failed to initialize the message bus", map[string]string{"Error": err.Error()})
		os.Exit(1)
	}

	messageBus.Configure(conf.NATS.Host, conf.NATS.Port, conf.NATS.User, conf.NATS.Password)
	err = messageBus.Connect()
	if err != nil {
		l.Info("Failed to connect to the message bus", map[string]string{"Error": err.Error()})
		os.Exit(1)
	}

	return messageBus
}

func connectToETCDStoreAdapter(l logger.Logger, conf config.Config) storeadapter.StoreAdapter {
	etcdStoreAdapter := storeadapter.NewETCDStoreAdapter(conf.StoreURLs, conf.StoreMaxConcurrentRequests)
	err := etcdStoreAdapter.Connect()
	if err != nil {
		l.Info("Failed to connect to the store", map[string]string{"Error": err.Error()})
		os.Exit(1)
	}

	return etcdStoreAdapter
}

func connectToStore(l logger.Logger, conf config.Config) store.Store {
	etcdStoreAdapter := connectToETCDStoreAdapter(l, conf)
	return store.NewStore(conf, etcdStoreAdapter)
}
