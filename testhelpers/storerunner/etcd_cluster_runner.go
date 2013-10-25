package storerunner

import (
	"fmt"
	. "github.com/onsi/gomega"

	etcdclient "github.com/coreos/go-etcd/etcd"

	"os"
	"os/exec"
	"syscall"
)

type ETCDClusterRunner struct {
	startingPort int
	numNodes     int
	etcdCommands []*exec.Cmd
	running      bool
}

func NewETCDClusterRunner(startingPort int, numNodes int) *ETCDClusterRunner {
	return &ETCDClusterRunner{
		startingPort: startingPort,
		numNodes:     numNodes,
	}
}

func (etcd *ETCDClusterRunner) Start() {
	etcd.etcdCommands = make([]*exec.Cmd, etcd.numNodes)

	for i := 0; i < etcd.numNodes; i++ {
		etcd.nukeArtifacts(i)
		os.MkdirAll(etcd.tmpPath(i), 0700)
		args := []string{"-d", etcd.tmpPath(i), "-c", etcd.clientUrl(i), "-s", etcd.serverUrl(i), "-n", etcd.nodeName(i)}
		if i != 0 {
			args = append(args, "-C", etcd.serverUrl(0))
		}

		etcd.etcdCommands[i] = exec.Command("etcd", args...)

		err := etcd.etcdCommands[i].Start()
		Ω(err).ShouldNot(HaveOccured(), "Make sure etcd is compiled and on your $PATH.")

		Eventually(func() bool {
			client := etcdclient.NewClient([]string{})
			return client.SetCluster([]string{"http://" + etcd.clientUrl(i)})
		}, 3, 0.05).Should(BeTrue(), "Expected ETCD to be up and running")
	}

	etcd.running = true
}

func (etcd *ETCDClusterRunner) Stop() {
	if etcd.running {
		for i := 0; i < etcd.numNodes; i++ {
			etcd.etcdCommands[i].Process.Signal(syscall.SIGINT)
			etcd.etcdCommands[i].Process.Wait()
			etcd.nukeArtifacts(i)
		}
		etcd.etcdCommands = nil
		etcd.running = false
	}
}

func (etcd *ETCDClusterRunner) NodeURLS() []string {
	urls := make([]string, etcd.numNodes)
	for i := 0; i < etcd.numNodes; i++ {
		urls[i] = "http://" + etcd.clientUrl(i)
	}
	return urls
}

func (etcd *ETCDClusterRunner) DiskUsage() (bytes int64, err error) {
	fi, err := os.Stat(etcd.tmpPathTo("log", 0))
	if err != nil {
		return 0, err
	}
	return fi.Size(), nil
}

func (etcd *ETCDClusterRunner) Reset() {
	if etcd.running {
		client := etcdclient.NewClient(etcd.NodeURLS())
		response, err := client.Get("/?please=true", false) //TODO: Ugly hack, fix when goetcd merges our pull request.
		Ω(err).ShouldNot(HaveOccured())
		for _, doomed := range response.Kvs {
			client.DeleteAll(doomed.Key)
		}
	}
}

func (etcd *ETCDClusterRunner) FastForwardTime(seconds int) {
	if etcd.running {
		client := etcdclient.NewClient(etcd.NodeURLS())

		//TODO: can't do a recursive get (YET!) because etcd does not return TTLs when getting recursively.  This should be fixed in 0.2 soon.  For now we do a (slower) manual fetch.

		etcd.fastForwardTime(client, "/?garbage=true", seconds) //TODO: Ugly hack, fix when goetcd merges our pull request.
	}
}

func (etcd *ETCDClusterRunner) fastForwardTime(client *etcdclient.Client, key string, seconds int) {
	response, err := client.Get(key, false)
	Ω(err).ShouldNot(HaveOccured())
	if response.Dir == true {
		for _, child := range response.Kvs {
			etcd.fastForwardTime(client, child.Key, seconds)
		}
	} else {
		if response.TTL == 0 {
			return
		}
		if response.TTL <= int64(seconds) {
			_, err := client.Delete(response.Key)
			Ω(err).ShouldNot(HaveOccured())
		} else {
			_, err := client.Set(response.Key, response.Value, uint64(response.TTL-int64(seconds)))
			Ω(err).ShouldNot(HaveOccured())
		}
	}
}

func (etcd *ETCDClusterRunner) clientUrl(index int) string {
	return fmt.Sprintf("127.0.0.1:%d", etcd.port(index))
}

func (etcd *ETCDClusterRunner) serverUrl(index int) string {
	return fmt.Sprintf("127.0.0.1:%d", etcd.port(index)+3000)
}

func (etcd *ETCDClusterRunner) nodeName(index int) string {
	return fmt.Sprintf("node%d", index)
}

func (etcd *ETCDClusterRunner) port(index int) int {
	return etcd.startingPort + index
}

func (etcd *ETCDClusterRunner) tmpPath(index int) string {
	return fmt.Sprintf("/tmp/ETCD_%d", etcd.port(index))
}

func (etcd *ETCDClusterRunner) tmpPathTo(subdir string, index int) string {
	return fmt.Sprintf("/%s/%s", etcd.tmpPath(index), subdir)
}

func (etcd *ETCDClusterRunner) nukeArtifacts(index int) {
	os.RemoveAll(etcd.tmpPath(index))
}
