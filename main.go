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

	"github.com/spf13/cobra"
)

type Gopher interface {
	exec(nodes <-chan *Node, cmd []string)
}

type Options struct {
	confType  string
	machines  bool
	groupPath string
	workers   int

	// Consul options
	consulType    string
	consulFilter  string
	consulService string
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

var config Options
var commit string = ""
var version string = "development"

func printVersion() {
	fmt.Printf("Version: %s\n", version)
	fmt.Printf("Git SHA: %s\n", commit)
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

// func getConsulServiceNodes(consulClient *api.Client, flags Options) []Node {
// 	nodes := []Node{}
// 	catalog := consulClient.Catalog()

// 	catalogService, _, err := catalog.Service(
// 		flags.consulService,
// 		"",
// 		&api.QueryOptions{
// 			Filter: flags.consulFilter,
// 		},
// 	)

// 	if err != nil {
// 		log.Fatal(err)
// 	}

// 	for _, servicenode := range catalogService {
// 		nodes = append(nodes, Node{label: servicenode.Node, address: servicenode.Address})
// 	}

// 	return nodes
// }

// func getConsulNodes(consulClient *api.Client, flags Options) []Node {
// 	nodes := []Node{}
// 	catalog := consulClient.Catalog()

// 	catalogNodes, _, err := catalog.Nodes(
// 		&api.QueryOptions{
// 			Filter: flags.consulFilter,
// 		},
// 	)

// 	if err != nil {
// 		log.Fatal(err)
// 	}

// 	for _, catalognode := range catalogNodes {
// 		nodes = append(nodes, Node{label: catalognode.Node, address: catalognode.Address})
// 	}

// 	return nodes
// }

func executeWorkers(workers int, nodes []*Node, cmd []string) {
	sshGopher := newSSHWorker()
	pool := newGopherPool(workers, sshGopher)

	pool.begin(nodes, cmd)

	fmt.Println("All Nodes Have Completed Task")
}

func main() {

	var rootCmd = &cobra.Command{
		Use:   "gsh",
		Short: "Gsh is a dsh like command with batteries built in",
		Args:  cobra.MinimumNArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			nodes := parseFileOrList(config.groupPath, config.machines, args[0])

			executeWorkers(config.workers, nodes, args[1:])
		},
	}

	var versionCmd = &cobra.Command{
		Use: "version",
		Run: func(cmd *cobra.Command, args []string) {
			printVersion()
		},
	}

	homeDir, _ := os.UserHomeDir()
	configDir := path.Join(homeDir, ".gsh")

	rootCmd.AddCommand(versionCmd)
	rootCmd.Flags().StringVarP(&config.confType, "conftype", "c", "local", "Use local values to find nodes and execute")
	rootCmd.Flags().StringVarP(&config.groupPath, "group-path", "p", path.Join(configDir, "groups"), "Set the path to find groups")
	rootCmd.Flags().StringVar(&config.consulType, "consultype", "service", "Lookup nodes via service or just list nodes")
	rootCmd.Flags().StringVar(&config.consulFilter, "consulfilter", "", "The filters that will be passed to consuls api")
	rootCmd.Flags().StringVar(&config.consulService, "consulservice", "", "The service that will be looked if type set to service")
	rootCmd.Flags().BoolVarP(&config.machines, "machine-list", "m", false, "Use a comma delimited list of nodes to run commands against instead of a group name")
	rootCmd.Flags().IntVarP(&config.workers, "workers", "f", 1, "The amount of workers to process the commands on a number of nodes")

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
