package gomob

/**
Adapted from https://github.com/asim/memberlist
Copyright (c) 2017 asim

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
**/

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/hashicorp/logutils"
	"github.com/hashicorp/memberlist"
	"github.com/pborman/uuid"
)

var (
	mtx              sync.RWMutex
	items            = map[string]string{}
	broadcasts       *memberlist.TransmitLimitedQueue
	ackCount         = 0
	done             chan bool
	settings         *ConsensusSettings
	consensusReached bool
	mlist            *memberlist.Memberlist
)

const (
	timeProposeFlag = 'T'
	executeFlag     = 'E'
	ackFlag         = 'A'
	secondsOffset   = 15
)

// ConsensusSettings settings to setup a cluster.
// Members is a comma-seperated list of IP addresses of known nodes (in theory, only one node needs to be known—gossip based discovery will handle the rest).
// Master if true, act as the master node and begin the process of time proposal.
// NumNodes is the expected number of nodes in the cluster. Consensus won't be reached until this number of nodes have ACK'd a proposed start time.
type ConsensusSettings struct {
	Members  []string
	Master   bool
	NumNodes int
}

type broadcast struct {
	msg    []byte
	notify chan<- struct{}
}

type delegate struct{}

type update struct {
	Action string // add, del
	Data   map[string]string
}

func (b *broadcast) Invalidates(other memberlist.Broadcast) bool {
	return false
}

func (b *broadcast) Message() []byte {
	return b.msg
}

func (b *broadcast) Finished() {
	if b.notify != nil {
		close(b.notify)
	}
}

func (d *delegate) NodeMeta(limit int) []byte {
	return []byte{}
}

func (d *delegate) NotifyMsg(b []byte) {
	if len(b) == 0 {
		return
	}

	log.Println("Received message notification:")
	log.Printf("\t%s\n", string(b))
	switch b[0] {
	case byte(timeProposeFlag):
		accept, err := validateTimeUpdate(string(b[1:]))
		if err != nil {
			panic(err)
		}
		if accept {
			sendTimeAck()
		} else {
			log.Println("Time not accepted")
		}
	case byte(ackFlag):
		if settings.Master {
			mtx.Lock()
			ackCount++
			if ackCount == settings.NumNodes-1 {
				t := sendExecutionFlag()
				go func() {
					strTime := fmt.Sprintf("%d", t.Unix())
					err := blockUntilTime(strTime)
					if err != nil {
						panic(err)
					}
					log.Println("Executing now...")
					done <- true
				}()
			}
			mtx.Unlock()
		}
	case byte(executeFlag):
		strTime := string(b[1:])
		log.Printf("Execute message received. Executing at (UNIX) %s\n", strTime)
		consensusReached = true
		err := blockUntilTime(strTime)
		if err != nil {
			panic(err)
		}
		done <- true
	default:
		panic(errors.New("Unkown message type received"))
	}
}

func (d *delegate) GetBroadcasts(overhead, limit int) [][]byte {
	return broadcasts.GetBroadcasts(overhead, limit)
}

func (d *delegate) LocalState(join bool) []byte {
	mtx.RLock()
	m := items
	mtx.RUnlock()
	b, _ := json.Marshal(m)
	return b
}

func (d *delegate) MergeRemoteState(buf []byte, join bool) {
	if len(buf) == 0 {
		return
	}
	if !join {
		return
	}
	var m map[string]string
	if err := json.Unmarshal(buf, &m); err != nil {
		return
	}
	mtx.Lock()
	for k, v := range m {
		items[k] = v
	}
	mtx.Unlock()
}

func setup() error {
	hostname, _ := os.Hostname()
	c := memberlist.DefaultWANConfig()
	c.Delegate = &delegate{}
	c.BindPort = 7777
	c.Name = hostname + "-" + uuid.NewUUID().String()

	filter := &logutils.LevelFilter{
		Levels:   []logutils.LogLevel{"DEBUG", "WARN", "ERROR"},
		MinLevel: logutils.LogLevel("WARN"),
		Writer:   os.Stderr,
	}

	c.LogOutput = filter

	var err error
	mlist, err = memberlist.Create(c)
	if err != nil {
		return err
	}

	if len(settings.Members) > 0 {
		_, err := mlist.Join(settings.Members)
		if err != nil {
			return err
		}
	}

	broadcasts = &memberlist.TransmitLimitedQueue{
		NumNodes: func() int {
			return mlist.NumMembers()
		},
		RetransmitMult: mlist.NumMembers() - 1,
	}

	return nil
}

func getFormattedTimeString(flag rune) (string, time.Time) {
	t := time.Now().UTC().Add(time.Second * secondsOffset).Unix()
	return fmt.Sprintf("%c%d", flag, t), time.Unix(t, 0)
}

func sendExecutionFlag() time.Time {
	tStr, t := getFormattedTimeString(executeFlag)
	// This means the master node will send an execution message to itself, but that's on purpose.
	for _, node := range mlist.Members() {
		mlist.SendReliable(node, []byte(tStr))
	}
	consensusReached = true

	return t
}

func proposeNewTime() time.Time {
	tStr, t := getFormattedTimeString(timeProposeFlag)
	b := broadcast{
		msg:    []byte(tStr),
		notify: nil,
	}
	broadcasts.QueueBroadcast(&b)

	return t
}

func sendTimeAck() {
	b := broadcast{
		msg:    []byte("ACK"),
		notify: nil,
	}
	broadcasts.QueueBroadcast(&b)
}

func validateTimeUpdate(unix string) (bool, error) {
	t, err := strconv.ParseInt(unix, 10, 64)
	if err != nil {
		return false, err
	}

	v := time.Now().UTC().Unix() < t

	return v, nil
}

func blockUntilTime(unix string) error {
	i, err := strconv.ParseInt(unix, 10, 64)
	if err != nil {
		return err
	}
	if i < time.Now().Unix() {
		return errors.New("Time string occurred in the past")
	}

	execTime := time.Unix(i, 0)

	c := time.After(execTime.Sub(time.Now().UTC()))

	<-c // channel blocks until time.After() has executed

	return nil
}

// WaitOnConsensus wait for all nodes to be ready before returning
func WaitOnConsensus(s *ConsensusSettings) error {
	log.SetPrefix(" [gomob] ")
	settings = s
	err := setup()
	if err != nil {
		return err
	}

	if settings.Master {
		log.Println("Starting up as master node.")
	}

	done = make(chan bool)

	go func() {

		// If a single node "cluster" is used, exit immediately. Sync is easy on a single machine...
		if settings.NumNodes == 1 && settings.Master {
			done <- true
		}

		prevMemb := 0
		consensusReached = false
		for {
			currMemb := mlist.NumMembers()
			broadcasts.RetransmitMult = currMemb - 1

			if settings.Master && currMemb >= settings.NumNodes {
				mtx.Lock()
				ackCount = 0
				mtx.Unlock()
				if !consensusReached {
					log.Printf("Attempting to establish consensus with %d other nodes...\n",
						settings.NumNodes-1)
					proposeNewTime()
					consensusReached = false // in case state is somehow wrong; only skip one iteration
				}

				// wait for timeout before proposing again
				time.Sleep(time.Second * secondsOffset)
			} else {
				if settings.Master && prevMemb != currMemb {
					log.Printf("Waiting for cluster to reach size of: %d\n\tCurrent size is: %d\n",
						settings.NumNodes, mlist.NumMembers())
				}
				prevMemb = currMemb
				time.Sleep(time.Second * (secondsOffset / 5))
			}
		}
	}()

	<-done

	return nil
}
