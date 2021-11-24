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
)

type Options struct {
	confType  string
	group     string
	machines  string
	groupPath string
	workers   int
}

func (flags *Options) getNodes() []string {
	var nodes []string

	switch flags.confType {
	case "local":
		nodes = flags.parseFileOrList()
	}

	return nodes
}

func (flags *Options) parseFileOrList() []string {
	if flags.machines != "" {
		return strings.Split(flags.machines, ",")
	}

	if flags.group == "" {
		log.Fatal("Group or Machine list is required")
	}

	configPath := path.Join(flags.groupPath, flags.group)
	contents, err := os.ReadFile(configPath)
	if err != nil {
		log.Fatal("Node Group file not found: " + configPath)
	}

	var nodes []string
	rawnodes := strings.Split(string(contents), "\n")

	for _, node := range rawnodes {
		if strings.TrimSpace(node) != "" && !strings.HasPrefix(node, "#") {
			nodes = append(nodes, node)
		}
	}

	return nodes
}

type Gopher interface {
	exec(nodes <-chan string, cmd []string)
}

type GopherPool struct {
	workerCount int
	nodes       chan string
	worker      Gopher
}

func newGopherPool(workCount int, worker Gopher) *GopherPool {
	pool := GopherPool{workerCount: workCount, nodes: make(chan string, workCount*2), worker: worker}
	return &pool
}

func (gp *GopherPool) begin(nodes []string, cmd []string) {
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

func (worker *GenericGopher) exec(nodes <-chan string, cmd []string) {
	for node := range nodes {
		combineNode := []string{}
		combineNode = append(combineNode, node)
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
			fmt.Printf("%s: %s\n", node, outputScanner.Text())
		}

		if err := outputScanner.Err(); err != nil {
			log.Fatal("Failed to read output from remote execution:", err)
		}

		remoteExec.Wait()
	}
}

var gsh Options

func init() {

	homeDir, _ := os.UserHomeDir()

	flag.StringVar(&gsh.confType, "conftype", "local", "Use local values to find nodes and execute")
	flag.StringVar(&gsh.groupPath, "configpath", path.Join(homeDir, ".gsh/groups"), "Set the path to find groups")
	flag.StringVar(&gsh.group, "g", "", "The group of nodes to run commands against")
	flag.StringVar(&gsh.machines, "m", "", "Comma delimited list of nodes to run commands against")
	flag.IntVar(&gsh.workers, "f", 1, "The amount of nodes to process the commands")
}

func main() {
	flag.Parse()

	nodes := gsh.getNodes()
	sshGopher := newSSHWorker()
	pool := newGopherPool(gsh.workers, sshGopher)

	pool.begin(nodes, flag.Args())

	fmt.Println("All Nodes Have Completed Task")
}
