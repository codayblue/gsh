# GSH aka Gopher Shell

This project barrows inspiration from DSH aka Dancer Shell. This project aims to allow folks to find nodes through multiple methods and execute SSH commands on those nodes. There is also plans to support kubectl and to allow commands to be run across multiple pods.

## How to Use

```bash
gsh <options> -- <command to run against nodes>
```

Flags for all implementations:
`-f 1` will set the number of workers that will be created and will begin working on the set of nodes. Currently 1 worker is the default.
`--` will cut off the flags and pass the rest to the workers. This is optional but might be needed when passing certain commands that contains flags of its own.
`-h` will print to the screen all the options that are available.

### Local Discovery (Currently default)

This method of finding nodes operates by either passing a comma delimited string of node adresses and users or creating a file like the example below at `~/.gsh/groups/<name of group>`. You pick the group with the `-g` flag. You can change the config path using `-configpath`. 

```text
node1
node2
node3
#node4
```

For that group it will use the default logged in user for nodes 1-2 and then switch to pi user for node 3. Node 4 is commented out and wont be found. Any blank space will be ignored. 

### Consul Discovery

This method will use consul to find nodes for a given service or just listing all the nodes. Filters can be passed through to the api that will limit the nodes pulled. By default it will connect to `http://127.0.0.1:8500` but that can be changed to a new connection point using consul environment variables or any other way the consul cli client can be configured (except through cli params, those are unsupported). Links below for relevant documentation.

To use consul make sure to have GSH v0.2.x installed. Binaries can be downloaded from github releases page.

Set `-conftype` to consul then the consul flags will be used to find nodes. By default it will look for a service and you must pass `-consulservice` via the cli to set the service you are interested in. To pass filters to consul use `-consulfilter` and filter will be passed directly to consul.

If you want to just grab all nodes and filter thoughs the filter flag is the same but set `-consulservice` to `nodes`. That will pull a list of nodes and apply the filters. Leave filter blank if you want to pull all nodes from the api.

Relevant documentation:
(Consul ENV)[https://www.consul.io/commands#environment-variables]
(Consul Filtering)[https://www.consul.io/api-docs/features/filtering]

## Plans of what features come next

Right now local discovery and consul are the current supported implementations but there is plans to support Kubectl as well to find pods and to execute commands within the pod context. Also eventually I would like to find a way to allow people to compile the binary with their own workers and ways of discovering node types to be able to run commands against them. This might end up using the golang plugin system and be dynamically found during execution.

## Contributing

PRs are welcome and so are issues. I will get to them as much as I can. Though there are a couple of things that you should please add with your PRs.

1. Add Tests or modify to verify functionality
2. Documentation is added to the readme
3. Be kind to others

If I start to see issues I will make sure to put in a Code of Conduct but I will only do that if I start to see conversations or hear from people about misdoings happening behind the scenes and in conversations.
