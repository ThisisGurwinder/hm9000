package hm

import (
	"fmt"
	"os"

	"github.com/cloudfoundry-incubator/cf_http"
	"github.com/cloudfoundry-incubator/consuladapter"
	"github.com/cloudfoundry-incubator/locket"
	"github.com/cloudfoundry/hm9000/actualstatelisteners"
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/helpers/metricsaccountant"
	"github.com/hashicorp/consul/api"
	"github.com/nu7hatch/gouuid"
	"github.com/pivotal-golang/lager"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/http_server"
	"github.com/tedsuo/ifrit/sigmon"
)

func StartListeningForActual(logger lager.Logger, conf *config.Config) {
	messageBus := connectToMessageBus(logger, conf)
	store, usageTracker := connectToStoreAndTrack(logger, conf)

	consulClient, _ := consuladapter.NewClientFromUrl(conf.ConsulCluster)

	clock := buildClock(logger)

	lockRunner := locket.NewLock(logger, consulClient, "hm9000.listener", make([]byte, 0), clock, locket.RetryInterval, locket.LockTTL)

	syncer := actualstatelisteners.NewSyncer(logger,
		conf,
		store,
		usageTracker,
		metricsaccountant.New(),
		clock,
	)

	natsListener := actualstatelisteners.NewNatsListener(logger,
		conf,
		messageBus,
		syncer,
	)

	httpListener, err := actualstatelisteners.NewHttpListener(logger,
		conf,
		syncer,
	)
	if err != nil {
		logger.Error("exited", err)
		os.Exit(1)
	}

	listenAddr := fmt.Sprintf("%s:%d", conf.HttpHeartbeatServerAddress, conf.HttpHeartbeatPort)

	uuid, err := uuid.NewV4()
	if err != nil {
		logger.Fatal("Couldn't generate uuid", err)
	}

	registration := &api.AgentServiceRegistration{
		Name: "listener-hm9000",
		Port: conf.HttpHeartbeatPort,
		Check: &api.AgentServiceCheck{
			TTL: locket.LockTTL.String(),
		},
		ID: uuid.String(),
	}

	registrationRunner := locket.NewRegistrationRunner(logger, registration, consulClient, locket.RetryInterval, clock)

	tlsConfig, err := cf_http.NewTLSConfig(conf.SSLCerts.CertFile, conf.SSLCerts.KeyFile, conf.SSLCerts.CACertFile)
	if err != nil {
		logger.Error("tls-configuration-failed", err)
		os.Exit(1)
	}
	members := grouper.Members{
		{"lockRunner", lockRunner},
		{"syncer", syncer},
		{"router_registration", registrationRunner},
		{"nats_heartbeat_listener", natsListener},
		{"http_heartbeat_listener", http_server.NewTLSServer(listenAddr, httpListener, tlsConfig)},
	}

	group := grouper.NewOrdered(os.Interrupt, members)

	monitor := ifrit.Invoke(sigmon.New(group))

	logger.Info("Listening for Actual State")

	logger.Info("started")
	logger.Info(listenAddr)

	err = <-monitor.Wait()
	if err != nil {
		logger.Error("exited", err)
		os.Exit(197)
	}

	logger.Info("exited")
	os.Exit(0)
}
