package hm

import (
	"github.com/cloudfoundry/go_cfmessagebus"
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/helpers/logger"
	"github.com/cloudfoundry/hm9000/helpers/timeprovider"
	"github.com/cloudfoundry/hm9000/store"
	"github.com/cloudfoundry/hm9000/storeadapter"
	"github.com/cloudfoundry/hm9000/testhelpers/faketimeprovider"
	"strconv"
	"time"

	"os"
)

func buildTimeProvider(l logger.Logger) timeprovider.TimeProvider {
	if os.Getenv("HM9000_FAKE_TIME") == "" {
		return timeprovider.NewTimeProvider()
	} else {
		timestamp, err := strconv.Atoi(os.Getenv("HM9000_FAKE_TIME"))
		if err != nil {
			l.Error("Failed to load timestamp", err)
			os.Exit(1)
		}
		return &faketimeprovider.FakeTimeProvider{
			TimeToProvide: time.Unix(int64(timestamp), 0),
		}
	}
}

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
	return store.NewStore(conf, etcdStoreAdapter, l)
}
