package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path"
	"strings"
	"sync"

	"github.com/hashicorp/consul/api"
)

type Options struct {
	confType  string
	group     string
	machines  string
	groupPath string
	workers   int

	// Consul options
	consulType    string
	consulFilter  string
	consulService string
}

type Gopher interface {
	exec(nodes <-chan Node, cmd []string)
}

type GopherPool struct {
	workerCount int
	nodes       chan Node
	worker      Gopher
}

func newGopherPool(workCount int, worker Gopher) *GopherPool {
	pool := GopherPool{workerCount: workCount, nodes: make(chan Node, workCount*2), worker: worker}
	return &pool
}

func (gp *GopherPool) begin(nodes []Node, cmd []string) {
	var wg sync.WaitGroup

	for worker := 0; worker < gp.workerCount; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			gp.worker.exec(gp.nodes, cmd)
		}()
	}

	for _, work := range nodes {
		gp.nodes <- work
	}

	close(gp.nodes)

	wg.Wait()
}

type GenericGopher struct {
	mainCmd string
}

func newSSHWorker() *GenericGopher {
	return &GenericGopher{mainCmd: "ssh"}
}

func (worker *GenericGopher) exec(nodes <-chan Node, cmd []string) {
	for node := range nodes {
		combineNode := []string{}
		combineNode = append(combineNode, node.address)
		combineNode = append(combineNode, cmd...)

		remoteExec := exec.Command(worker.mainCmd, combineNode...)

		remoteStdout, err := remoteExec.StdoutPipe()
		if err != nil {
			log.Fatal("Failed to get stdout reader:", err)
		}

		remoteStderr, err := remoteExec.StderrPipe()
		if err != nil {
			log.Fatal("Failed to get stderr reader:", err)
		}

		outputReader := io.MultiReader(remoteStdout, remoteStderr)
		outputScanner := bufio.NewScanner(outputReader)

		if err := remoteExec.Start(); err != nil {
			log.Fatal("Failed to start remote execution:", err)
		}

		for outputScanner.Scan() {
			fmt.Printf("%s: %s\n", node.label, outputScanner.Text())
		}

		if err := outputScanner.Err(); err != nil {
			log.Fatal("Failed to read output from remote execution:", err)
		}

		remoteExec.Wait()
	}
}

type Node struct {
	label   string
	address string
}

func getNodes(flags Options) []Node {
	var nodes []Node

	switch flags.confType {
	case "local":
		nodes = parseFileOrList(flags)
	case "consul":
		client, err := api.NewClient(api.DefaultConfig())

		if err != nil {
			log.Fatal(err)
		}

		if flags.consulType == "service" {
			nodes = getConsulServiceNodes(client, flags)
		} else {
			nodes = getConsulNodes(client, flags)
		}

	}

	return nodes
}

func parseFileOrList(flags Options) []Node {
	nodes := []Node{}

	if flags.machines != "" {
		for _, foundNode := range strings.Split(flags.machines, ",") {
			nodes = append(nodes, Node{label: foundNode, address: foundNode})
		}

		return nodes
	}

	if flags.group == "" {
		log.Fatal("Group or Machine list is required")
	}

	configPath := path.Join(flags.groupPath, flags.group)
	contents, err := os.ReadFile(configPath)
	if err != nil {
		log.Fatal("Node Group file not found: " + configPath)
	}

	rawnodes := strings.Split(string(contents), "\n")

	for _, node := range rawnodes {
		trimmedNode := strings.TrimSpace(node)

		if strings.TrimSpace(trimmedNode) != "" && !strings.HasPrefix(trimmedNode, "#") {
			nodes = append(nodes, Node{label: trimmedNode, address: trimmedNode})
		}
	}

	return nodes
}

func getConsulServiceNodes(consulClient *api.Client, flags Options) []Node {
	nodes := []Node{}
	catalog := consulClient.Catalog()

	catalogService, _, err := catalog.Service(
		flags.consulService,
		"",
		&api.QueryOptions{
			Filter: flags.consulFilter,
		},
	)

	if err != nil {
		log.Fatal(err)
	}

	for _, servicenode := range catalogService {
		nodes = append(nodes, Node{label: servicenode.Node, address: servicenode.Address})
	}

	return nodes
}

func getConsulNodes(consulClient *api.Client, flags Options) []Node {
	nodes := []Node{}
	catalog := consulClient.Catalog()

	catalogNodes, _, err := catalog.Nodes(
		&api.QueryOptions{
			Filter: flags.consulFilter,
		},
	)

	if err != nil {
		log.Fatal(err)
	}

	for _, catalognode := range catalogNodes {
		nodes = append(nodes, Node{label: catalognode.Node, address: catalognode.Address})
	}

	return nodes
}

var config Options

func init() {

	homeDir, _ := os.UserHomeDir()

	flag.StringVar(&config.confType, "conftype", "local", "Use local values to find nodes and execute")
	flag.StringVar(&config.groupPath, "configpath", path.Join(homeDir, ".gsh/groups"), "Set the path to find groups")
	flag.StringVar(&config.consulType, "consultype", "service", "Lookup nodes via service or just list nodes")
	flag.StringVar(&config.consulFilter, "consulfilter", "", "The filters that will be passed to consuls api")
	flag.StringVar(&config.consulService, "consulservice", "", "The service that will be looked if type set to service")
	flag.StringVar(&config.group, "g", "", "The group of nodes to run commands against")
	flag.StringVar(&config.machines, "m", "", "Comma delimited list of nodes to run commands against")
	flag.IntVar(&config.workers, "f", 1, "The amount of nodes to process the commands")
}

func main() {
	flag.Parse()

	nodes := getNodes(config)
	sshGopher := newSSHWorker()
	pool := newGopherPool(config.workers, sshGopher)

	pool.begin(nodes, flag.Args())

	fmt.Println("All Nodes Have Completed Task")
}
