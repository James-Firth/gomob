package main

import (
	"flag"
	"fmt"
	"strings"
	"time"

	"github.com/mattmcmurray/gomob/gomob"
)

var (
	members      = flag.String("members", "", "comma seperated list of members")
	isMasterNode = flag.Bool("master", false, "define as master node")
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
		NumNodes: 3,
	}

	err := gomob.WaitOnConsensus(&s)
	if err != nil {
		panic(err)
	}

	// The code here will execute only when all nodes have reached synchronicity

	fmt.Println("Consensus at:", time.Now())
}
