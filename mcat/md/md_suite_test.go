package md_test

import (
	"github.com/cloudfoundry/hm9000/helpers/timeprovider"
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/testhelpers/messagepublisher"
	"github.com/cloudfoundry/hm9000/testhelpers/startstoplistener"
	. "github.com/onsi/ginkgo"
	ginkgoConfig "github.com/onsi/ginkgo/config"
	. "github.com/onsi/gomega"
	"os/exec"
	"strconv"
	"time"

	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/storeadapter"
	"github.com/cloudfoundry/hm9000/testhelpers/desiredstateserver"
	"github.com/cloudfoundry/hm9000/testhelpers/natsrunner"
	"github.com/cloudfoundry/hm9000/testhelpers/storerunner"
	"os"
	"os/signal"
	"testing"
)

var (
	stateServer               *desiredstateserver.DesiredStateServer
	storeRunner               storerunner.StoreRunner
	storeAdapter              storeadapter.StoreAdapter
	natsRunner                *natsrunner.NATSRunner
	conf                      config.Config
	cliRunner                 *CLIRunner
	publisher                 *messagepublisher.MessagePublisher
	startStopListener         *startstoplistener.StartStopListener
	simulator                 *Simulator
	desiredStateServerPort    int
	desiredStateServerBaseUrl string
	natsPort                  int
)

func TestMd(t *testing.T) {
	registerSignalHandler()
	RegisterFailHandler(Fail)

	cmd := exec.Command("go", "install", "github.com/cloudfoundry/hm9000")
	err := cmd.Run()
	Ω(err).ShouldNot(HaveOccured())

	desiredStateServerPort = 6001 + ginkgoConfig.GinkgoConfig.ParallelNode
	desiredStateServerBaseUrl = "http://127.0.0.1:" + strconv.Itoa(desiredStateServerPort)
	natsPort = 4223 + ginkgoConfig.GinkgoConfig.ParallelNode

	natsRunner = natsrunner.NewNATSRunner(natsPort)
	natsRunner.Start()

	stateServer = desiredstateserver.NewDesiredStateServer()
	go stateServer.SpinUp(desiredStateServerPort)

	conf, err = config.DefaultConfig()
	Ω(err).ShouldNot(HaveOccured())

	//for now, run the suite for ETCD...
	startEtcd()
	publisher = messagepublisher.NewMessagePublisher(natsRunner.MessageBus)
	startStopListener = startstoplistener.NewStartStopListener(natsRunner.MessageBus, conf)

	cliRunner = NewCLIRunner(storeRunner.NodeURLS(), desiredStateServerBaseUrl, natsPort, ginkgoConfig.DefaultReporterConfig.Verbose)

	RunSpecs(t, "Md Suite (ETCD)")

	storeAdapter.Disconnect()
	stopAllExternalProcesses()

	//...and then for zookeeper
	// startZookeeper()

	// RunSpecs(t, "Md Suite (Zookeeper)")

	// storeAdapter.Disconnect()
	// storeRunner.Stop()
}

var _ = BeforeEach(func() {
	storeRunner.Reset()
	startStopListener.Reset()
	simulator = NewSimulator(conf, storeRunner, stateServer, cliRunner, publisher)
})

func stopAllExternalProcesses() {
	storeRunner.Stop()
	natsRunner.Stop()
	cliRunner.Cleanup()
}

func startEtcd() {
	etcdPort := 5000 + (ginkgoConfig.GinkgoConfig.ParallelNode-1)*10
	storeRunner = storerunner.NewETCDClusterRunner(etcdPort, 1)
	storeRunner.Start()

	storeAdapter = storeadapter.NewETCDStoreAdapter(storeRunner.NodeURLS(), conf.StoreMaxConcurrentRequests)
	err := storeAdapter.Connect()
	Ω(err).ShouldNot(HaveOccured())
}

func startZookeeper() {
	zookeeperPort := 2181 + (ginkgoConfig.GinkgoConfig.ParallelNode-1)*10
	storeRunner = storerunner.NewZookeeperClusterRunner(zookeeperPort, 1)
	storeRunner.Start()

	storeAdapter = storeadapter.NewZookeeperStoreAdapter(storeRunner.NodeURLS(), conf.StoreMaxConcurrentRequests, &timeprovider.RealTimeProvider{}, time.Second)
	err := storeAdapter.Connect()
	Ω(err).ShouldNot(HaveOccured())
}

func expireHeartbeat(heartbeat models.InstanceHeartbeat) {
	err := storeAdapter.Delete("/actual/" + heartbeat.InstanceGuid)
	Ω(err).ShouldNot(HaveOccured())
}

func sendHeartbeats(timestamp int, heartbeats ...models.Heartbeat) {
	cliRunner.StartListener(timestamp)
	for _, heartbeat := range heartbeats {
		publisher.PublishHeartbeat(heartbeat)
	}
	cliRunner.WaitForHeartbeats(len(heartbeats))
	cliRunner.StopListener()
}

func registerSignalHandler() {
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, os.Kill)

		select {
		case <-c:
			stopAllExternalProcesses()
			os.Exit(0)
		}
	}()
}
