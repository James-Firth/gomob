package gomob

import (
	"fmt"
	"regexp"
	"testing"
	"time"

	"github.com/hashicorp/memberlist"
)

type mockBroadcasts struct{}

func (mockBroadcasts) QueueBroadcast(*interface{}) {
	fmt.Println("Method called")
}

func TestProposeNewTime(t *testing.T) {
	m := &memberlist.TransmitLimitedQueue{}
	broadcasts = m
	pTime := proposeNewTime(false)
	if pTime.Unix() < time.Now().Unix() {
		t.Fail()
	}
}

func TestValidateTimeUpdate(t *testing.T) {
	// Invalid input
	valid, err := validateTimeUpdate("not a number")
	if valid == true || err == nil {
		t.Log(valid)
		t.Log(err)
		t.Fail()
	}

	r, e := regexp.Compile("not a number")
	if e != nil {
		panic(err)
	}

	match := r.Match([]byte(err.Error()))

	if !match {
		t.Error("Error string did not match regex.")
	}

	// Valid case
	future := time.Now().Add(time.Second * 60).Unix()
	fStr := fmt.Sprintf("%d", future)

	valid, err = validateTimeUpdate(fStr)
	if err != nil {
		t.Error("Error on valid time string.")
	}

	if !valid {
		t.Error("False returned on valid time.")
	}

	// Invalid case
	past := time.Now().Add(time.Second * -60).Unix()
	fStr = fmt.Sprintf("%d", past)

	valid, err = validateTimeUpdate(fStr)
	if err != nil {
		t.Error("Error on valid time string.")
	}

	if valid {
		t.Error("True returned on invalid time.")
	}
}

func TestBlockUntilTime(t *testing.T) {
	blockTo := time.Now().Add(time.Second).Unix()
	blockUntilTime(fmt.Sprintf("%d", blockTo))
	if time.Now().Unix() != blockTo {
		t.Error("Blocking exited at incorrect time.")
	}

	blockTo = time.Now().Add(-time.Second).Unix()
	err := blockUntilTime(fmt.Sprintf("%d", blockTo))
	if err == nil {
		t.Error("Time that occurred in past should have thrown error.")
	}

	match, e := regexp.Match("Time string occurred in the past",
		[]byte(err.Error()))
	if e != nil {
		panic(e)
	}
	if !match {
		t.Error("Unexpected error thrown", err)
	}

}
