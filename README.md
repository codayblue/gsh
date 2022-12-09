# GSH aka Gopher Shell

This project barrows inspiration from DSH aka Dancer Shell. This project aims to allow folks to find nodes through multiple methods and execute SSH commands on those nodes. There is also plans to support kubectl and to allow commands to be run across multiple pods.

## How to Use

Flags for all implementations:
`-w 1` will set the number of workers that will be created and will begin working on the set of nodes. Currently 1 worker is the default.
`--` will cut off the flags and pass the rest to the workers. This is optional but might be needed when passing certain commands that contains flags of its own and quotes are not used.
`-h` will print to the screen all the options that are available.

If you wanna check the version you can do so by running

```bash
gsh --version
```

### Local Discovery (Default)

```bash
gsh <group name> [--] <command>
```

This method of finding nodes operates by creating a file like the example below at `~/.gsh/groups/<name of group>`. You can change the config path to search using `--group-path` or `-p`.

```text
node1
node2
pi@node3
#node4
```

For that group it will use the default logged in user for nodes 1-2 and then switch to pi user for node 3. Node 4 is commented out and wont be found. Any blank space will be ignored.

```bash
gsh machine <comma list of nodes> -- <command>
```

If you dont want a inventory file of nodes or just have a dynamic list being built you can pass a comma delimited list of node configurations as the api above.

### Consul Discovery

This method will use consul to find nodes for a given service or just listing all the nodes. Filters can be passed through to the api that will limit the nodes pulled. By default it will connect to `http://127.0.0.1:8500` but that can be changed to a new connection point using consul environment variables or any other way the consul cli client can be configured (except through cli flags, those are not supported at this time). Links below for relevant documentation.

```bash
CONSUL_HTTP_ADDR=10.10.10.10:8500 gsh consul node [--consul-filter="some filter"] [--] <commands to send to nodes>
```

`gsh consul node` command will pull all the nodes registered to the consul cluster. To limit the nodes or to find specific nodes use the consul filter flag.

```bash
CONSUL_HTTP_ADDR=10.10.10.10:8500 gsh consul service <service to lookup> [--consul-filter="some filter"] [--] <commands to send to nodes>
```

`gsh consul service` will lookup all the nodes in the group passed to gsh. The consul filter flag will limit the nodes based on the filter passed.

Relevant documentation:
[Consul ENV](https://www.consul.io/commands#environment-variables)
[Consul Filtering](https://www.consul.io/api-docs/features/filtering)

## Plans of what features come next

Right now local discovery and consul are the current supported implementations but there is plans to support Kubectl as well to find pods and to execute commands within the pod context. Also eventually I would like to find a way to allow people to compile the binary with their own workers and ways of discovering node types to be able to run commands against them. This might end up using the golang plugin system and be dynamically found during execution.

## Contributing

PRs are welcome and so are issues. I will get to them as much as I can. Though there are a couple of things that you should please add with your PRs.

1. Add Tests or modify to verify functionality
2. Documentation is added to the readme
3. Be kind to others

If I start to see issues I will make sure to put in a Code of Conduct but I will only do that if I start to see conversations or hear from people about misdoings happening behind the scenes and in conversations.
