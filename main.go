package main

import (
	"flag"
	"fmt"
	"os"
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

	configPath := path.Join(flags.groupPath, flags.group)
	contents, err := os.ReadFile(configPath)
	if err != nil {
		panic("Config file now found: " + configPath)
	}

	var nodes []string
	rawnodes := strings.Split(string(contents), "\n")

	for _, node := range rawnodes {
		if node != "" {
			nodes = append(nodes, node)
		}
	}

	return nodes
}

type gopherPool struct {
	workerCount int
	jobs        chan string
}

func newGopherPool(workCount int) *gopherPool {
	pool := gopherPool{workerCount: workCount, jobs: make(chan string, workCount*2)}
	return &pool
}

func (gp *gopherPool) begin(nodes []string, cmd []string) {
	var wg sync.WaitGroup
	for worker := 0; worker < gp.workerCount; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			gopher(gp.jobs)
		}()
	}

	for _, work := range nodes {
		gp.jobs <- work
	}

	close(gp.jobs)

	wg.Wait()
}

func gopher(jobs <-chan string) {
	for job := range jobs {
		fmt.Println(job)
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
	pool := newGopherPool(cliFlags.workers)

	pool.begin(nodes, flag.Args())

	fmt.Println("All Nodes Have Completed Task")
}
