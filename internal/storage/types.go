package storage

import "time"

type Offset int64

const (
	OffsetBeginning Offset = 0
)

type Record struct {
	Offset    Offset
	Timestamp time.Time
	Key       []byte
	Value     []byte
	Headers   map[string]string
	Checksum  uint32
}

type AppendRecord struct {
	Key     []byte
	Value   []byte
	Headers map[string]string
}

type Batch struct {
	Records     []Record
	FirstOffset Offset
	LastOffset  Offset
	NextOffset  Offset
}

type Options struct {
	Dir             string
	MaxRecordBytes  int
	FsyncOnAppend   bool
	SegmentFileName string
}
