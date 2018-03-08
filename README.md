# goMob
[![Build Status](https://travis-ci.org/MattMcMurray/gomob.svg?branch=master)](https://travis-ci.org/MattMcMurray/gomob)

goMob is an easy-to-use library and commandline tool to synchronize code execution across a small number of nodes.


## How to Use

### From the Terminal
goMob needs to be run on all nodes that are being synchronized.

#### Node 1
**IP = `192.168.50.10`**
```bash
# The node defined as master sets up the synchronicity.
gomob -master -num_nodes 3 && \
    ./my-code-to-run.sh
```

#### Node 2
**IP = `192.168.50.20`**
```bash
# peers points to master node.
gomob -peers 192.168.50.10 && \
    ./my-code-to-run.sh
```

#### Node 3
**IP = `192.168.50.30`**

`-peers` need not point directly to master. In this case, `-peers` points to the 2nd node. Gossip based discovery will connect all nodes together eventually.

`-peers` can also point to multiple other nodes by providing a comma-separated list of addresses.
```bash
gomob -peers 192.168.50.20 && \
    ./my-code-to-run.sh
```

### As a Library
`main.go` provides a good example. The main idea is to setup the cluster, then call `gomob.WaitOnConsensus()` which will block until all nodes have synchronized.

e.g.,

```go
// Master Node
func main() {
    s := gomob.ConsensusSettings{
		Members:  "",
		Master:   true,
		NumNodes: 2,

    gomob.WaitOnConsensus(&s) // this will block until ready

    foo() // this should execute at the same time as bar() on the 2nd node

    return
}

// Worker node
func main() {
    s := gomob.ConsensusSettings{
		Members:  "192.168.50.10",
		Master:   false,
		NumNodes: 2,

    gomob.WaitOnConsensus(&s) // this will block until ready

    bar() // this should execute at the same time as foo() on the master node

    return
}
```

