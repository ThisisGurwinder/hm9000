package mcat_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"time"

	"github.com/onsi/ginkgo"

	"github.com/cloudfoundry/hm9000/config"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

type CLIRunner struct {
	configPath       string
	listenerSession  *gexec.Session
	metricsSession   *gexec.Session
	apiServerSession *gexec.Session
	evacuatorSession *gexec.Session
	hm9000Binary     string
	config           *config.Config

	verbose bool
}

func NewCLIRunner(hm9000Binary string, storeURLs []string, ccBaseURL string, natsPort int, dropsondePort int, verbose bool) *CLIRunner {
	runner := &CLIRunner{
		hm9000Binary: hm9000Binary,
		verbose:      verbose,
	}
	runner.config = runner.generateConfig(storeURLs, ccBaseURL, natsPort, dropsondePort)
	return runner
}

func (runner *CLIRunner) generateConfig(storeURLs []string, ccBaseURL string, natsPort int, dropsondePort int) *config.Config {
	tmpFile, err := ioutil.TempFile("/tmp", "hm9000_clirunner")
	defer tmpFile.Close()
	Ω(err).ShouldNot(HaveOccurred())

	runner.configPath = tmpFile.Name()

	conf, err := config.DefaultConfig()
	Ω(err).ShouldNot(HaveOccurred())
	conf.StoreURLs = storeURLs
	conf.CCBaseURL = ccBaseURL
	conf.NATS[0].Port = natsPort
	conf.SenderMessageLimit = 8
	conf.MaximumBackoffDelayInHeartbeats = 6
	conf.DropsondePort = dropsondePort
	conf.StoreMaxConcurrentRequests = 10
	conf.ListenerHeartbeatSyncIntervalInMilliseconds = 100
	conf.APIServerPort = int(5155 + ginkgo.GinkgoParallelNode())
	conf.LogLevelString = "DEBUG"

	err = json.NewEncoder(tmpFile).Encode(conf)
	Ω(err).ShouldNot(HaveOccurred())

	return conf
}

func (runner *CLIRunner) StartListener(timestamp int) {
	if runner.listenerSession != nil {
		runner.StopListener()
	}
	runner.listenerSession = runner.start("listen", timestamp, "Listening for Actual State")
}

func (runner *CLIRunner) StopListener() {
	runner.listenerSession.Interrupt().Wait(time.Second)
}

func (runner *CLIRunner) StartAPIServer(timestamp int) {
	runner.apiServerSession = runner.start("serve_api", timestamp, "started")
}

func (runner *CLIRunner) StopAPIServer() {
	runner.apiServerSession.Interrupt().Wait(time.Second)
}

func (runner *CLIRunner) StartEvacuator(timestamp int) {
	runner.evacuatorSession = runner.start("evacuator", timestamp, "Listening for DEA Evacuations")
}

func (runner *CLIRunner) StopEvacuator() {
	runner.evacuatorSession.Interrupt().Wait(time.Second)
}

func (runner *CLIRunner) Cleanup() {
	os.Remove(runner.configPath)
}

func (runner *CLIRunner) start(command string, timestamp int, message string) *gexec.Session {
	cmd := exec.Command(runner.hm9000Binary, command, fmt.Sprintf("--config=%s", runner.configPath))
	cmd.Env = append(os.Environ(), fmt.Sprintf("HM9000_FAKE_TIME=%d", timestamp))

	session, err := gexec.Start(cmd, ginkgo.GinkgoWriter, ginkgo.GinkgoWriter)
	Ω(err).ShouldNot(HaveOccurred())
	Eventually(session, 10*time.Second).Should(gbytes.Say(message))

	return session
}

func (runner *CLIRunner) Run(command string, timestamp int) {
	cmd := exec.Command(runner.hm9000Binary, command, fmt.Sprintf("--config=%s", runner.configPath))
	cmd.Env = append(os.Environ(), fmt.Sprintf("HM9000_FAKE_TIME=%d", timestamp))

	session, err := gexec.Start(cmd, ginkgo.GinkgoWriter, ginkgo.GinkgoWriter)
	Ω(err).ShouldNot(HaveOccurred())

	session.Wait(10 * time.Second)
	time.Sleep(50 * time.Millisecond)
}

func (runner *CLIRunner) StartSession(command string, timestamp int, extraArgs ...string) *gexec.Session {
	args := []string{command, fmt.Sprintf("--config=%s", runner.configPath)}
	args = append(args, extraArgs...)

	cmd := exec.Command(runner.hm9000Binary, args...)
	cmd.Env = append(os.Environ(), fmt.Sprintf("HM9000_FAKE_TIME=%d", timestamp))

	session, err := gexec.Start(cmd, ginkgo.GinkgoWriter, ginkgo.GinkgoWriter)
	Ω(err).ShouldNot(HaveOccurred())

	return session
}
