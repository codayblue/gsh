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

type options struct {
	group     string
	machines  string
	groupPath string
	workers   int
}

func (flags *options) parseFileOrList() []string {
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
		if node != "" && !strings.HasPrefix(node, "#") {
			nodes = append(nodes, node)
		}
	}

	return nodes
}

type gopher func(string, <-chan string, []string)
type gopherPool struct {
	workerCount int
	nodes       chan string
	worker      gopher
}

func newGopherPool(workCount int, worker gopher) *gopherPool {
	pool := gopherPool{workerCount: workCount, nodes: make(chan string, workCount*2), worker: worker}
	return &pool
}

func (gp *gopherPool) begin(nodes []string, cmd []string) {
	var wg sync.WaitGroup

	for worker := 0; worker < gp.workerCount; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			gp.worker("ssh", gp.nodes, cmd)
		}()
	}

	for _, work := range nodes {
		gp.nodes <- work
	}

	close(gp.nodes)

	wg.Wait()
}

func processor(remoteCmd string, nodes <-chan string, cmd []string) {
	for node := range nodes {
		combineNode := []string{}
		combineNode = append(combineNode, node)
		combineNode = append(combineNode, cmd...)

		remoteExec := exec.Command(remoteCmd, combineNode...)

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

var cliFlags options

func init() {

	homeDir, _ := os.UserHomeDir()

	flag.StringVar(&cliFlags.groupPath, "configpath", path.Join(homeDir, ".gsh/groups"), "Set the path to find groups")
	flag.StringVar(&cliFlags.group, "g", "", "The group of nodes to run commands against")
	flag.StringVar(&cliFlags.machines, "m", "", "Comma delimited list of nodes to run commands against")
	flag.IntVar(&cliFlags.workers, "f", 1, "The amount of nodes to process the commands")
}

func main() {
	flag.Parse()

	nodes := cliFlags.parseFileOrList()
	pool := newGopherPool(cliFlags.workers, processor)

	pool.begin(nodes, flag.Args())

	fmt.Println("All Nodes Have Completed Task")
}
