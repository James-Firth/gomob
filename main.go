package main

import (
	"fmt"
	"time"

	"github.com/mattmcmurray/gomob/gomob"
)

func main() {
	gomob.WaitOnConsensus()
	fmt.Printf("Consensus at: %d\n", time.Now().Unix())
}
