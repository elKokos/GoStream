package storage

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestLogAppendAndReadSingleRecord(t *testing.T) {
	log := openTestLog(t, t.TempDir(), 1024)
	defer log.Close()

	record, err := log.Append(AppendRecord{
		Key:     []byte("order-1"),
		Value:   []byte("created"),
		Headers: map[string]string{"source": "test"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if record.Offset != 0 {
		t.Fatalf("offset = %d, want 0", record.Offset)
	}
	if record.Checksum == 0 {
		t.Fatal("checksum should be set")
	}

	batch, err := log.Read(0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(batch.Records) != 1 {
		t.Fatalf("records = %d, want 1", len(batch.Records))
	}
	if got := string(batch.Records[0].Value); got != "created" {
		t.Fatalf("value = %q, want created", got)
	}
	if batch.NextOffset != 1 {
		t.Fatalf("next offset = %d, want 1", batch.NextOffset)
	}
}

func TestLogAppendBatch(t *testing.T) {
	log := openTestLog(t, t.TempDir(), 1024)
	defer log.Close()

	batch, err := log.AppendBatch([]AppendRecord{
		{Value: []byte("one")},
		{Value: []byte("two")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if batch.FirstOffset != 0 || batch.LastOffset != 1 || batch.NextOffset != 2 {
		t.Fatalf("unexpected offsets: %+v", batch)
	}

	read, err := log.Read(0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(read.Records) != 2 {
		t.Fatalf("records = %d, want 2", len(read.Records))
	}
}

func TestLogRecoversAfterReopen(t *testing.T) {
	dir := t.TempDir()
	log := openTestLog(t, dir, 1024)
	if _, err := log.Append(AppendRecord{Value: []byte("durable")}); err != nil {
		t.Fatal(err)
	}
	if err := log.Close(); err != nil {
		t.Fatal(err)
	}

	log = openTestLog(t, dir, 1024)
	defer log.Close()

	if log.NextOffset() != 1 {
		t.Fatalf("next offset = %d, want 1", log.NextOffset())
	}
	batch, err := log.Read(0, 1)
	if err != nil {
		t.Fatal(err)
	}
	if string(batch.Records[0].Value) != "durable" {
		t.Fatalf("unexpected value: %q", batch.Records[0].Value)
	}
}

func TestLogTruncatesCorruptedTail(t *testing.T) {
	dir := t.TempDir()
	log := openTestLog(t, dir, 1024)
	if _, err := log.Append(AppendRecord{Value: []byte("valid")}); err != nil {
		t.Fatal(err)
	}
	if err := log.Close(); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(dir, defaultSegmentFileName)
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write([]byte{1, 2, 3, 4}); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	log = openTestLog(t, dir, 1024)
	defer log.Close()

	if log.NextOffset() != 1 {
		t.Fatalf("next offset = %d, want 1", log.NextOffset())
	}
	batch, err := log.Read(0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(batch.Records) != 1 || string(batch.Records[0].Value) != "valid" {
		t.Fatalf("unexpected batch: %+v", batch)
	}
}

func TestLogRejectsOversizedRecord(t *testing.T) {
	log := openTestLog(t, t.TempDir(), 64)
	defer log.Close()

	_, err := log.Append(AppendRecord{Value: bytes.Repeat([]byte("x"), 1024)})
	if !errors.Is(err, ErrRecordTooLarge) {
		t.Fatalf("err = %v, want ErrRecordTooLarge", err)
	}
}

func TestLogReadOffsetOutOfRange(t *testing.T) {
	log := openTestLog(t, t.TempDir(), 1024)
	defer log.Close()

	_, err := log.Read(1, 1)
	if !errors.Is(err, ErrOffsetOutOfRange) {
		t.Fatalf("err = %v, want ErrOffsetOutOfRange", err)
	}
}

func TestLogConcurrentReadsWithSingleWriter(t *testing.T) {
	log := openTestLog(t, t.TempDir(), 1024)
	defer log.Close()

	for i := 0; i < 100; i++ {
		if _, err := log.Append(AppendRecord{Value: []byte("value")}); err != nil {
			t.Fatal(err)
		}
	}

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for offset := Offset(0); offset < 100; offset++ {
				batch, err := log.Read(offset, 1)
				if err != nil {
					t.Errorf("read offset %d: %v", offset, err)
					return
				}
				if len(batch.Records) != 1 || batch.Records[0].Offset != offset {
					t.Errorf("bad batch at offset %d: %+v", offset, batch)
					return
				}
			}
		}()
	}
	wg.Wait()
}

func openTestLog(t *testing.T, dir string, maxRecordBytes int) *Log {
	t.Helper()
	log, err := Open(Options{Dir: dir, MaxRecordBytes: maxRecordBytes})
	if err != nil {
		t.Fatal(err)
	}
	return log
}
