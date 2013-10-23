package md_test

import (
	"github.com/cloudfoundry/hm9000/helpers/timeprovider"
	"github.com/cloudfoundry/hm9000/testhelpers/startstoplistener"
	. "github.com/onsi/ginkgo"
	ginkgoConfig "github.com/onsi/ginkgo/config"
	. "github.com/onsi/gomega"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/storeadapter"
	"github.com/cloudfoundry/hm9000/testhelpers/desiredstateserver"
	"github.com/cloudfoundry/hm9000/testhelpers/natsrunner"
	"github.com/cloudfoundry/hm9000/testhelpers/storerunner"
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
	startStopListener         *startstoplistener.StartStopListener
	simulator                 *Simulator
	desiredStateServerPort    int
	desiredStateServerBaseUrl string
	natsPort                  int
	metricsServerPort         int
	currentStoreType          string
)

func TestMd(t *testing.T) {
	registerSignalHandler()
	RegisterFailHandler(Fail)

	cmd := exec.Command("go", "install", "github.com/cloudfoundry/hm9000")
	output, err := cmd.CombinedOutput()
	if err != nil {
		println("FAILED TO COMPILE HM9000")
		println(string(output))
		os.Exit(1)
	}

	desiredStateServerPort = 6001 + ginkgoConfig.GinkgoConfig.ParallelNode
	desiredStateServerBaseUrl = "http://127.0.0.1:" + strconv.Itoa(desiredStateServerPort)
	natsPort = 4223 + ginkgoConfig.GinkgoConfig.ParallelNode
	metricsServerPort = 7879 + ginkgoConfig.GinkgoConfig.ParallelNode

	natsRunner = natsrunner.NewNATSRunner(natsPort)
	natsRunner.Start()

	stateServer = desiredstateserver.NewDesiredStateServer()
	go stateServer.SpinUp(desiredStateServerPort)

	conf, err = config.DefaultConfig()
	Ω(err).ShouldNot(HaveOccured())

	startStopListener = startstoplistener.NewStartStopListener(natsRunner.MessageBus, conf)

	//run the suite for ETCD...
	currentStoreType = "etcd"
	startStore()

	cliRunner = NewCLIRunner(currentStoreType, storeRunner.NodeURLS(), desiredStateServerBaseUrl, natsPort, metricsServerPort, ginkgoConfig.DefaultReporterConfig.Verbose)

	RunSpecs(t, "MCAT ETCD MD Suite")

	storeAdapter.Disconnect()
	storeRunner.Stop()

	//...and then for zookeeper
	currentStoreType = "ZooKeeper"
	startStore()

	cliRunner = NewCLIRunner(currentStoreType, storeRunner.NodeURLS(), desiredStateServerBaseUrl, natsPort, metricsServerPort, ginkgoConfig.DefaultReporterConfig.Verbose)

	RunSpecs(t, "MCAT ZooKeeper MD Suite")

	storeAdapter.Disconnect()

	stopAllExternalProcesses()
}

var _ = BeforeEach(func() {
	if currentStoreType == "etcd" {
		//etcd's reset sucks.  once we get etcd 0.2 this can go away.
		storeRunner.Stop()
		storeRunner.Start()
	} else if currentStoreType == "ZooKeeper" {
		storeRunner.Reset()
	}
	startStopListener.Reset()
	simulator = NewSimulator(conf, storeRunner, stateServer, cliRunner, natsRunner.MessageBus)
})

func stopAllExternalProcesses() {
	storeRunner.Stop()
	natsRunner.Stop()
	cliRunner.Cleanup()
}

func startStore() {
	if currentStoreType == "etcd" {
		etcdPort := 5000 + (ginkgoConfig.GinkgoConfig.ParallelNode-1)*10
		storeRunner = storerunner.NewETCDClusterRunner(etcdPort, 1)
		storeRunner.Start()

		storeAdapter = storeadapter.NewETCDStoreAdapter(storeRunner.NodeURLS(), conf.StoreMaxConcurrentRequests)
		err := storeAdapter.Connect()
		Ω(err).ShouldNot(HaveOccured())
	} else if currentStoreType == "ZooKeeper" {
		zookeeperPort := 2181 + (ginkgoConfig.GinkgoConfig.ParallelNode-1)*10
		storeRunner = storerunner.NewZookeeperClusterRunner(zookeeperPort, 1)
		storeRunner.Start()

		storeAdapter = storeadapter.NewZookeeperStoreAdapter(storeRunner.NodeURLS(), conf.StoreMaxConcurrentRequests, &timeprovider.RealTimeProvider{}, time.Second)
		err := storeAdapter.Connect()
		Ω(err).ShouldNot(HaveOccured())
	}
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
