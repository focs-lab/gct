package replay

type Recorder interface {
	Record(lid *LogicalId, caseIdx int, hasCase bool)
	RecordPair(sendLid, recvLid *LogicalId, sendCaseIdx int, sendHasCase bool, recvCaseIdx int, recvHasCase bool)
	RecordSeed(seed int64)
	ToFile() error
	BuildFromTrace(traceLoc string)
	GetTotalNumEvents() int
	GetEvent(idx int) string
}