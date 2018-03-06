package gomob

// Adapted from https://github.com/asim/memberlist

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/memberlist"
	"github.com/pborman/uuid"
)

var (
	mtx             sync.RWMutex
	members         = flag.String("members", "", "comma seperated list of members")
	port            = flag.Int("port", 4001, "http port")
	items           = map[string]string{}
	broadcasts      *memberlist.TransmitLimitedQueue
	isMasterNode    bool
	timeProposeFlag = 'T'
	ackFlag         = 'A'
	ackCount        = 0
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

	fmt.Println("Received message notification:")
	fmt.Printf("\t%s\n", string(b))
	switch b[0] {
	case byte(timeProposeFlag):
		accept, err := validateTimeUpdate(string(b[1:]))
		if err != nil {
			// TODO: send error response? ignore and move on?
		}
		if accept {
			sendTimeAck()
		} else {
			fmt.Println("Time not accepted")
		}
	case byte(ackFlag):
		if isMasterNode {
			mtx.Lock()
			ackCount++
			mtx.Unlock()
			// TODO: if ack count is at right number, start execution sequence
		}
	default:
		fmt.Println("Unknown message type received")
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

func setup() (*memberlist.Memberlist, error) {
	hostname, _ := os.Hostname()
	c := memberlist.DefaultLocalConfig()
	c.Delegate = &delegate{}
	c.BindPort = 7777
	c.Name = hostname + "-" + uuid.NewUUID().String()
	list, err := memberlist.Create(c)
	if err != nil {
		return nil, err
	}
	if len(*members) > 0 {
		parts := strings.Split(*members, ",")
		_, err := list.Join(parts)
		if err != nil {
			return nil, err
		}
	}
	broadcasts = &memberlist.TransmitLimitedQueue{
		NumNodes: func() int {
			return list.NumMembers()
		},
		RetransmitMult: 2,
	}
	node := list.LocalNode()
	fmt.Printf("Local member %s:%d\n", node.Addr, node.Port)
	return list, nil
}

func proposeNewTime() {
	t := time.Now().UTC().Add(time.Second * 30).Unix()
	tStr := fmt.Sprintf("%c%d", timeProposeFlag, t)
	fmt.Printf("Sending message:\n\t%s\n", tStr)
	b := broadcast{
		msg:    []byte(tStr),
		notify: nil,
	}
	broadcasts.QueueBroadcast(&b)
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

func CreateCluster() {
	list, err := setup()
	if err != nil {
		fmt.Println(err)
	}

	hostname, _ := os.Hostname()
	isMasterNode := hostname == "node01"

	for {
		broadcasts.RetransmitMult = list.NumMembers() - 1
		if list.NumMembers() >= 2 {
			if isMasterNode {
				mtx.Lock()
				ackCount = 0
				mtx.Unlock()
				proposeNewTime()
				time.Sleep(time.Second * 30) // wait until expiry time has passed before proposing again
			}
		} else {
			fmt.Println("Num members:", list.NumMembers())
			time.Sleep(time.Second * 5)
		}

		// if hostname, _ := os.Hostname(); hostname != "node01" {
		// 	b := broadcast{msg: []byte("hello world"), notify: nil}
		// 	broadcasts.QueueBroadcast(&b)
		// }

	}
}
