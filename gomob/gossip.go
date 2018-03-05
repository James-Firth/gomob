package gomob

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"regexp"
	"sync"
	"time"

	"github.com/hashicorp/memberlist"
)

var (
	mtx        sync.RWMutex
	items      = map[string]string{}
	broadcasts *memberlist.TransmitLimitedQueue
)

type broadcast struct {
	msg    []byte
	notify chan<- struct{}
}

type delegate struct{}

type update struct {
	Action string // add, del
	Data   map[string]string
}

func init() {
	flag.Parse()
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

	switch b[0] {
	case 'd': // data
		var updates []*update
		if err := json.Unmarshal(b[1:], &updates); err != nil {
			return
		}
		mtx.Lock()
		for _, u := range updates {
			for k, v := range u.Data {
				switch u.Action {
				case "add":
					items[k] = v
				case "del":
					delete(items, k)
				}
			}
		}
		mtx.Unlock()
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

func CreateCluster() {
	fmt.Println("Starting up.")
	hostname, _ := os.Hostname()
	master := hostname == "node01"

	// Hard code hostname and IP address
	var hostMap = make(map[string]string)
	hostMap["node01"] = "192.168.50.10"
	hostMap["node02"] = "192.168.50.11"
	hostMap["node03"] = "192.168.50.12"
	hostMap["node04"] = "192.168.50.13"

	config := memberlist.DefaultLocalConfig()
	config.BindAddr = hostMap[hostname]
	list, err := memberlist.Create(config)
	if err != nil {
		fmt.Println("Failed to create memberlist: " + err.Error())
	}

	// Join
	_, err = list.Join([]string{hostMap["node01"]})
	if err != nil {
		fmt.Println("Failed to join cluster: " + err.Error())
	}

	broadcasts = &memberlist.TransmitLimitedQueue{
		NumNodes: func() int {
			return list.NumMembers()
		},
		RetransmitMult: 3,
	}

	c := make(chan bool)

	go func() {
		for {
			time.Sleep(time.Second * 2)
			// for _, member := range list.Members() {
			// 	if list.NumMembers() > 1 {
			// 		b := broadcast{msg: []byte(fmt.Sprintf("hello world — %s", member.Name))}
			// 		broadcasts.QueueBroadcast(&b)
			// 	}
			// }
			// msgs := broadcasts.GetBroadcasts(1, 1024)
			// for _, msg := range msgs {
			// 	fmt.Println(string(msg))
			// }

			if list.NumMembers() > 1 {
				c <- true
			}
		}
	}()

	<-c
	if master {
		proposeTime(broadcasts)
	}

	for {
		time.Sleep(time.Second * 5)
		// numAckd := 0

		if master {
			b := broadcast{msg: []byte("master node")}
			broadcasts.QueueBroadcast(&b)
			// fmt.Printf("Received %d broadcast messages\n", len(msgs))
			// for _, msg := range msgs {
			// 	if string(msg) == "ACK_TIME" { // TODO: include timestamp in ACK message
			// 		numAckd++
			// 	} else {
			// 		proposeTime(broadcasts)
			// 	}
			// 	fmt.Println(string(msg))
			// 	fmt.Println("Num ACKd", numAckd)
			// }

			// if numAckd == list.NumMembers()-1 {
			// 	fmt.Println("All peers ACK'd")
			// }
		} else {
			msgs := broadcasts.GetBroadcasts(5, 1024)
			fmt.Println("Num messages received: ", len(msgs))
			for _, msg := range msgs {
				fmt.Println(string(msg))
				rgx, _ := regexp.Compile("Propose")
				match := rgx.Match(msg)
				fmt.Println(string(msg))
				if match {
					fmt.Println("Acknowledging")
					newMsg := broadcast{msg: []byte("ACK_TIME")}
					broadcasts.QueueBroadcast(&newMsg)
				}
			}
		}
	}
}

func proposeTime(broadcasts *memberlist.TransmitLimitedQueue) {
	t := time.Now().UTC().Add(30 * time.Second)

	fmt.Println("Proposing new time to peers")
	b := broadcast{msg: []byte(fmt.Sprint("Propose: ", t)), notify: nil}
	broadcasts.QueueBroadcast(&b)
}
