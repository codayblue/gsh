package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path"
	"strings"
	"sync"

	"github.com/hashicorp/consul/api"
	"github.com/spf13/cobra"
)

type Gopher interface {
	exec(nodes <-chan *Node, cmd []string)
}

type GopherPool struct {
	workerCount int
	nodes       chan *Node
	worker      Gopher
}

func newGopherPool(workCount int, worker Gopher) *GopherPool {
	pool := GopherPool{workerCount: workCount, nodes: make(chan *Node, workCount*2), worker: worker}
	return &pool
}

func (gp *GopherPool) begin(nodes []*Node, cmd []string) {
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

func (worker *GenericGopher) exec(nodes <-chan *Node, cmd []string) {
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

type ConsulConnection struct {
	client *api.Client
}

func NewConsulConnection() (*ConsulConnection, error) {

	consulClient, err := api.NewClient(api.DefaultConfig())

	if err != nil {
		return nil, err
	}

	return &ConsulConnection{client: consulClient}, nil

}

func (consul *ConsulConnection) getConsulServiceNodes(service string, filter string) []*Node {
	nodes := []*Node{}
	catalog := consul.client.Catalog()

	catalogService, _, err := catalog.Service(
		service,
		"",
		&api.QueryOptions{
			Filter: filter,
		},
	)

	if err != nil {
		log.Fatal(err)
	}

	for _, servicenode := range catalogService {
		nodes = append(nodes, &Node{label: servicenode.Node, address: servicenode.Address})
	}

	return nodes
}

func (consul *ConsulConnection) getConsulNodes(filter string) []*Node {
	nodes := []*Node{}
	catalog := consul.client.Catalog()

	catalogNodes, _, err := catalog.Nodes(
		&api.QueryOptions{
			Filter: filter,
		},
	)

	if err != nil {
		log.Fatal(err)
	}

	for _, catalognode := range catalogNodes {
		nodes = append(nodes, &Node{label: catalognode.Node, address: catalognode.Address})
	}

	return nodes
}

func parseFileOrList(configDir string, enableMachines bool, group string) []*Node {
	nodes := []*Node{}

	if enableMachines {
		for _, foundNode := range strings.Split(group, ",") {
			nodes = append(nodes, &Node{label: foundNode, address: foundNode})
		}

		return nodes
	}

	configPath := path.Join(configDir, group)
	contents, err := os.ReadFile(configPath)
	if err != nil {
		log.Fatal("Node Group file not found: " + configPath)
	}

	rawnodes := strings.Split(string(contents), "\n")

	for _, node := range rawnodes {
		trimmedNode := strings.TrimSpace(node)

		if strings.TrimSpace(trimmedNode) != "" && !strings.HasPrefix(trimmedNode, "#") {
			nodes = append(nodes, &Node{label: trimmedNode, address: trimmedNode})
		}
	}

	return nodes
}

func executeWorkers(workers int, nodes []*Node, cmd []string) {
	sshGopher := newSSHWorker()
	pool := newGopherPool(workers, sshGopher)

	pool.begin(nodes, cmd)

	fmt.Println("All Nodes Have Completed Task")
}

var groupPath string
var workers int
var consulFilter string
var commit string = ""
var version string = "development"

func main() {

	var rootCmd = &cobra.Command{
		Use:   "gsh",
		Short: "Gsh is a dsh like command with batteries built in",
		Args:  cobra.MinimumNArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			nodes := parseFileOrList(groupPath, false, args[0])

			executeWorkers(workers, nodes, args[1:])
		},
	}

	var machineCmd = &cobra.Command{
		Use:   "machine",
		Short: "Pass comma list of machines",
		Args:  cobra.MinimumNArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			nodes := parseFileOrList(groupPath, true, args[0])

			executeWorkers(workers, nodes, args[1:])
		},
	}

	var versionCmd = &cobra.Command{
		Use:   "version",
		Short: "Shows the version of GSH",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("Version: %s\n", version)
			fmt.Printf("Git SHA: %s\n", commit)
		},
	}

	var consulCmd = &cobra.Command{
		Use:   "consul",
		Short: "Use consul to find nodes",
	}

	var consulServiceCmd = &cobra.Command{
		Use:   "service",
		Short: "Look up nodes based on consul service definitions",
		Run: func(cmd *cobra.Command, args []string) {
			consulConnection, err := NewConsulConnection()

			if err != nil {
				log.Fatalf("Failed to connect to consul: %s", err.Error())
			}

			nodes := consulConnection.getConsulServiceNodes(args[0], consulFilter)

			executeWorkers(workers, nodes, args[1:])
		},
	}

	var consulNodeCmd = &cobra.Command{
		Use:   "node",
		Short: "Look up consul registered nodes",
		Run: func(cmd *cobra.Command, args []string) {
			consulConnection, err := NewConsulConnection()

			if err != nil {
				fmt.Printf("Failed to connect to consul: %s", err.Error())
				os.Exit(1)
			}

			nodes := consulConnection.getConsulNodes(consulFilter)

			executeWorkers(workers, nodes, args)
		},
	}

	consulCmd.AddCommand(consulServiceCmd, consulNodeCmd)
	rootCmd.AddCommand(versionCmd, machineCmd, consulCmd)

	homeDir, _ := os.UserHomeDir()
	configDir := path.Join(homeDir, ".gsh")

	consulCmd.PersistentFlags().StringVarP(&consulFilter, "consul-filter", "f", "", "Pass a filter to consul to limit nodes")
	rootCmd.PersistentFlags().StringVarP(&groupPath, "group-path", "p", path.Join(configDir, "groups"), "Set the path to find groups")
	rootCmd.PersistentFlags().IntVarP(&workers, "workers", "w", 1, "The amount of workers to process the commands on a number of nodes")

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
