package storage

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const defaultSegmentFileName = "00000000000000000000.log"

type Log struct {
	mu             sync.RWMutex
	file           *os.File
	path           string
	maxRecordBytes int
	fsyncOnAppend  bool
	nextOffset     Offset
	index          []indexEntry
}

type indexEntry struct {
	offset Offset
	pos    int64
}

func Open(opts Options) (*Log, error) {
	if opts.Dir == "" {
		return nil, fmt.Errorf("storage dir is required")
	}
	if opts.MaxRecordBytes <= 0 {
		opts.MaxRecordBytes = 1 << 20
	}
	if opts.SegmentFileName == "" {
		opts.SegmentFileName = defaultSegmentFileName
	}
	if err := os.MkdirAll(opts.Dir, 0o755); err != nil {
		return nil, err
	}

	path := filepath.Join(opts.Dir, opts.SegmentFileName)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}

	log := &Log{
		file:           file,
		path:           path,
		maxRecordBytes: opts.MaxRecordBytes,
		fsyncOnAppend:  opts.FsyncOnAppend,
	}
	if err := log.recover(); err != nil {
		_ = file.Close()
		return nil, err
	}
	if _, err := file.Seek(0, io.SeekEnd); err != nil {
		_ = file.Close()
		return nil, err
	}
	return log, nil
}

func (l *Log) Append(input AppendRecord) (Record, error) {
	record := Record{
		Offset:    0,
		Timestamp: time.Now().UTC(),
		Key:       append([]byte(nil), input.Key...),
		Value:     append([]byte(nil), input.Value...),
		Headers:   cloneHeaders(input.Headers),
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file == nil {
		return Record{}, ErrClosed
	}
	record.Offset = l.nextOffset
	headersBytes, err := marshalHeaders(record.Headers)
	if err != nil {
		return Record{}, fmt.Errorf("%w: headers: %v", ErrInvalidRecord, err)
	}
	record.Checksum = checksum(record.Key, record.Value, headersBytes)
	encoded, err := encodeRecord(record)
	if err != nil {
		return Record{}, err
	}
	if len(encoded) > l.maxRecordBytes {
		return Record{}, ErrRecordTooLarge
	}

	pos, err := l.file.Seek(0, io.SeekCurrent)
	if err != nil {
		return Record{}, err
	}
	if _, err := l.file.Write(encoded); err != nil {
		return Record{}, err
	}
	if l.fsyncOnAppend {
		if err := l.file.Sync(); err != nil {
			return Record{}, err
		}
	}

	l.index = append(l.index, indexEntry{offset: record.Offset, pos: pos})
	l.nextOffset++
	return record, nil
}

func (l *Log) AppendBatch(inputs []AppendRecord) (Batch, error) {
	batch := Batch{Records: make([]Record, 0, len(inputs))}
	for _, input := range inputs {
		record, err := l.Append(input)
		if err != nil {
			return Batch{}, err
		}
		if len(batch.Records) == 0 {
			batch.FirstOffset = record.Offset
		}
		batch.LastOffset = record.Offset
		batch.NextOffset = record.Offset + 1
		batch.Records = append(batch.Records, record)
	}
	if len(batch.Records) == 0 {
		batch.FirstOffset = l.NextOffset()
		batch.LastOffset = batch.FirstOffset - 1
		batch.NextOffset = batch.FirstOffset
	}
	return batch, nil
}

func (l *Log) Read(offset Offset, limit int) (Batch, error) {
	if limit <= 0 {
		limit = 1
	}

	l.mu.RLock()
	defer l.mu.RUnlock()

	if l.file == nil {
		return Batch{}, ErrClosed
	}
	if offset < 0 || offset > l.nextOffset {
		return Batch{}, ErrOffsetOutOfRange
	}
	if offset == l.nextOffset {
		return Batch{
			Records:     []Record{},
			FirstOffset: offset,
			LastOffset:  offset - 1,
			NextOffset:  offset,
		}, nil
	}

	idx := int(offset)
	if idx >= len(l.index) || l.index[idx].offset != offset {
		return Batch{}, ErrOffsetOutOfRange
	}

	file, err := os.Open(l.path)
	if err != nil {
		return Batch{}, err
	}
	defer file.Close()

	if _, err := file.Seek(l.index[idx].pos, io.SeekStart); err != nil {
		return Batch{}, err
	}

	batch := Batch{
		Records:     make([]Record, 0, limit),
		FirstOffset: offset,
		LastOffset:  offset - 1,
		NextOffset:  offset,
	}
	for len(batch.Records) < limit {
		record, _, err := decodeRecord(file, l.maxRecordBytes)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return Batch{}, err
		}
		batch.Records = append(batch.Records, record)
		batch.LastOffset = record.Offset
		batch.NextOffset = record.Offset + 1
	}
	return batch, nil
}

func (l *Log) NextOffset() Offset {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.nextOffset
}

func (l *Log) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file == nil {
		return nil
	}
	err := l.file.Close()
	l.file = nil
	return err
}

func (l *Log) recover() error {
	if _, err := l.file.Seek(0, io.SeekStart); err != nil {
		return err
	}

	var pos int64
	var lastGood int64
	var expected Offset
	for {
		record, bytesRead, err := decodeRecord(l.file, l.maxRecordBytes)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			if truncateErr := l.file.Truncate(lastGood); truncateErr != nil {
				return fmt.Errorf("truncate corrupted tail after %v: %w", err, truncateErr)
			}
			break
		}
		if record.Offset != expected {
			return fmt.Errorf("%w: unexpected offset %d, expected %d", ErrInvalidRecord, record.Offset, expected)
		}
		l.index = append(l.index, indexEntry{offset: record.Offset, pos: pos})
		expected++
		pos += bytesRead
		lastGood = pos
	}
	l.nextOffset = expected
	return nil
}

func cloneHeaders(headers map[string]string) map[string]string {
	if len(headers) == 0 {
		return nil
	}
	out := make(map[string]string, len(headers))
	for key, value := range headers {
		out[key] = value
	}
	return out
}
