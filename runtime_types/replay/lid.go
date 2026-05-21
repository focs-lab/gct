package replay

import (
	"sync"
	"fmt"
)

type LogicalId struct {
	Mu           sync.Mutex
	ParentLid    *LogicalId
	Id           uint16
	ChildCounter uint16
	StringName   string
}

func NewLogicalId(parent *LogicalId) *LogicalId {
	var localId uint16
	var stringName string

	if parent == nil {
		localId = 0
		stringName = fmt.Sprintf("%d", localId)
	} else {
		parent.Mu.Lock()
		localId = parent.ChildCounter
		stringName = fmt.Sprintf("%s.%d", parent.StringName, localId)
		parent.Mu.Unlock()
	}
	
	return &LogicalId{
		ParentLid: parent,
		Id:        localId,
		ChildCounter: 0,
		Mu:           sync.Mutex{},
		StringName:   stringName,
	}
}

func (lid *LogicalId) String() string {
	lid.Mu.Lock()
	ret := lid.StringName
	lid.Mu.Unlock()
	return ret
}

func (lid *LogicalId) IncrChildCnt() {
	lid.Mu.Lock()
	lid.ChildCounter++
	lid.Mu.Unlock()
}

func (lid *LogicalId) Equals(other *LogicalId) bool {
	s1 := other.String()
	s2 := lid.String()

	return s1 == s2
}