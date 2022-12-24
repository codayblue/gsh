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
	nodes := parseFileOrList("/tmp", true, "node1,node2,pi@node3")

	if len(nodes) < 3 {
		t.Fatalf("All nodes were not found: %v", nodes)
	}

	if nodes[0].address != "node1" {
		t.Fatalf("First entry was not as expected: %s", nodes[0])
	}
}

func TestLocalMachineGroupLoading(t *testing.T) {
	tempDir := t.TempDir()
	testFile := path.Join(tempDir, "sampleGroup")
	err := os.WriteFile(testFile, []byte("   \nnode1\nnode2\npi@node3\n#node4\n"), 0644)

	if err != nil {
		t.Fatal("Failed to create test file")
	}

	nodes := parseFileOrList(tempDir, false, "sampleGroup")

	if len(nodes) < 3 {
		t.Fatalf("All nodes were not found: %v", nodes)
	}

	if nodes[0].address != "node1" {
		t.Fatalf("First entry was not as expected: %s", nodes[0])
	}

	if nodes[2].address != "pi@node3" {
		t.Fatalf("User entry was not as expected: %s", nodes[2])
	}

	for _, node := range nodes {
		if node.address == "node4" {
			t.Fatalf("Found a commented out node: %s", node)
		}

		if node.address == "" {
			t.Fatal("Found blank node")
		}
	}
}

func TestConsulServiceLoading(t *testing.T) {

	apiClient, err := api.NewClient(api.DefaultConfig())

	if err != nil {
		t.Fatal("Could not create consul client")
	}

	consul := &ConsulConnection{client: apiClient}

	for i := 0; i < 3; i++ {
		nodeName := "node" + strconv.Itoa(i+1)
		nodeAddress := "10.0.0." + strconv.Itoa((i+1)*10)
		serviceTags := []string{}

		if i%2 == 0 {
			serviceTags = append(serviceTags, "odd")
		}

		_, err := consul.client.Catalog().Register(
			&api.CatalogRegistration{
				Node:    nodeName,
				Address: nodeAddress,
				Service: &api.AgentService{
					ID:      "redis",
					Service: "redis",
					Tags:    serviceTags,
				},
			},
			&api.WriteOptions{},
		)

		if err != nil {
			t.Fatal("Could not fill consul with test data")
		}
	}

	serviceNodes := consul.getConsulServiceNodes("redis", "odd in ServiceTags")

	if len(serviceNodes) == 0 {
		t.Fatalf("No nodes were found from consul")
	}

	if len(serviceNodes) > 2 {
		t.Fatalf("Consul filter did not limit nodes: found %d expected 2", len(serviceNodes))
	}

	if serviceNodes[0].label != "node1" || serviceNodes[0].address != "10.0.0.10" {
		t.Fatalf("First node was not as expected: %v", serviceNodes[0])
	}

	consulNodes := consul.getConsulNodes("Node contains node")

	if len(consulNodes) == 0 {
		t.Fatalf("No nodes were found from consul")
	}

	if len(serviceNodes) > 3 {
		t.Fatalf("Consul filter did not limit nodes: found %d expected 3", len(serviceNodes))
	}

	if consulNodes[0].label != "node1" || consulNodes[0].address != "10.0.0.10" {
		t.Fatalf("First node was not as expected: %v", consulNodes[0])
	}
}

type TestWorker struct {
	t *testing.T
}

func (testWorker *TestWorker) exec(nodes <-chan *Node, cmd []string) {
	for node := range nodes {
		if !strings.HasPrefix(node.address, "node") {
			testWorker.t.Fatalf("node is missing node in the front: %s", node)
		}

		if cmd[0] != "testCmd" {
			testWorker.t.Fatalf("Command did not come through correctly: %s", cmd[0])
		}
	}
}

func TestGopherPool(t *testing.T) {
	gopherPool := newGopherPool(1, &TestWorker{t: t})

	nodes := []*Node{
		{label: "node1", address: "node1"},
		{label: "node2", address: "node2"},
		{label: "node3", address: "node3"},
	}
	gopherPool.begin(nodes, []string{"testCmd"})
}

func TestGenericWorker(t *testing.T) {
	echoWorker := &GenericGopher{mainCmd: "echo"}
	gopherPool := newGopherPool(1, echoWorker)

	nodes := []*Node{
		{label: "node1", address: "node1"},
		{label: "node2", address: "node2"},
		{label: "node3", address: "node3"},
	}
	gopherPool.begin(nodes, []string{"testCmd"})
}
