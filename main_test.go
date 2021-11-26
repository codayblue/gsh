package main

import (
	"os"
	"path"
	"strconv"
	"strings"
	"testing"

	"github.com/hashicorp/consul/api"
)

func TestLocalMachineListLoading(t *testing.T) {
	gsh := &Options{confType: "local", machines: "node1,node2,node3"}

	nodes := gsh.getNodes()

	if len(nodes) < 3 {
		t.Fatalf("All nodes were not found: %v", nodes)
	}

	if nodes[0] != "node1" {
		t.Fatalf("First entry was not as expected: %s", nodes[0])
	}
}

func TestLocalMachineGroupLoading(t *testing.T) {
	tempDir := t.TempDir()
	testFile := path.Join(tempDir, "sampleGroup")
	err := os.WriteFile(testFile, []byte("   \nnode1\nnode2\nnode3\n#node4\n"), 0644)

	if err != nil {
		t.Fatal("Failed to create test file")
	}

	gsh := &Options{confType: "local", group: "sampleGroup", groupPath: tempDir}
	nodes := gsh.getNodes()

	if len(nodes) < 3 {
		t.Fatalf("All nodes were not found: %v", nodes)
	}

	if nodes[0] != "node1" {
		t.Fatalf("First entry was not as expected: %s", nodes[0])
	}

	for _, node := range nodes {
		if node == "node4" {
			t.Fatalf("Found a commented out node: %s", node)
		}

		if node == "" {
			t.Fatal("Found blank node")
		}
	}
}

func TestConsulLoading(t *testing.T) {

	consulClient, err := api.NewClient(api.DefaultConfig())

	if err != nil {
		t.Fatal("Could not create consul client")
	}

	for i := 0; i < 3; i++ {
		_, err := consulClient.Catalog().Register(
			&api.CatalogRegistration{
				Node:    "node" + strconv.Itoa(i+1),
				Address: "10.0.0." + strconv.Itoa((i+1)*10),
				Service: &api.AgentService{
					ID:      "redis",
					Service: "redis",
				},
			},
			&api.WriteOptions{},
		)

		if err != nil {
			t.Fatal("Could not fill consul with test data")
		}
	}

	gsh := &Options{confType: "consul"}
	nodes := gsh.getNodes()

	t.Fatal(nodes)
}

type TestWorker struct {
	t *testing.T
}

func (testWorker *TestWorker) exec(nodes <-chan string, cmd []string) {
	for node := range nodes {
		if !strings.HasPrefix(node, "node") {
			testWorker.t.Fatalf("node is missing node in the front: %s", node)
		}

		if cmd[0] != "testCmd" {
			testWorker.t.Fatalf("Command did not come through correctly: %s", cmd[0])
		}
	}
}

func TestGopherPool(t *testing.T) {
	gopherPool := newGopherPool(1, &TestWorker{t: t})

	nodes := []string{
		"node1",
		"node2",
		"node3",
	}
	gopherPool.begin(nodes, []string{"testCmd"})
}

func TestGenericWorker(t *testing.T) {
	echoWorker := &GenericGopher{mainCmd: "echo"}
	gopherPool := newGopherPool(1, echoWorker)

	nodes := []string{
		"node1",
		"node2",
		"node3",
	}
	gopherPool.begin(nodes, []string{"testCmd"})
}
