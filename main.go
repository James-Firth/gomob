package main

import (
	"flag"
	"fmt"
	"strings"
	"time"

	"github.com/mattmcmurray/gomob/gomob"
)

var (
	members      = flag.String("peers", "", "Comma seperated list of peers. Only one known peer is required. Gossip will handle the rest.")
	isMasterNode = flag.Bool("master", false, "Define as master node")
	numNodes     = flag.Int("num_nodes", 1, "The number of nodes in the cluster")
)

func init() {
	flag.Parse()
}

func main() {
	m := make([]string, 0)
	if len(*members) > 0 {
		m = strings.Split(*members, ",")
	}

	s := gomob.ConsensusSettings{
		Members:  m,
		Master:   *isMasterNode,
		NumNodes: *numNodes,
	}

	err := gomob.WaitOnConsensus(&s)
	if err != nil {
		panic(err)
	}

	// The code here will execute only when all nodes have reached synchronicity

	fmt.Println("Consensus at:", time.Now())
}
