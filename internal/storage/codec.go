package storage

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"io"
	"time"
)

const (
	recordMagic   uint32 = 0x47535431 // GST1
	recordVersion uint16 = 1
	recordHeader         = 40
)

func encodeRecord(record Record) ([]byte, error) {
	headers, err := marshalHeaders(record.Headers)
	if err != nil {
		return nil, fmt.Errorf("%w: headers: %v", ErrInvalidRecord, err)
	}

	record.Checksum = checksum(record.Key, record.Value, headers)
	size := recordHeader + len(record.Key) + len(record.Value) + len(headers)
	buf := make([]byte, size)

	binary.BigEndian.PutUint32(buf[0:4], recordMagic)
	binary.BigEndian.PutUint16(buf[4:6], recordVersion)
	binary.BigEndian.PutUint16(buf[6:8], 0)
	binary.BigEndian.PutUint64(buf[8:16], uint64(record.Offset))
	binary.BigEndian.PutUint64(buf[16:24], uint64(record.Timestamp.UnixNano()))
	binary.BigEndian.PutUint32(buf[24:28], uint32(len(record.Key)))
	binary.BigEndian.PutUint32(buf[28:32], uint32(len(record.Value)))
	binary.BigEndian.PutUint32(buf[32:36], uint32(len(headers)))
	binary.BigEndian.PutUint32(buf[36:40], record.Checksum)

	pos := recordHeader
	copy(buf[pos:], record.Key)
	pos += len(record.Key)
	copy(buf[pos:], record.Value)
	pos += len(record.Value)
	copy(buf[pos:], headers)

	return buf, nil
}

func decodeRecord(r io.Reader, maxRecordBytes int) (Record, int64, error) {
	header := make([]byte, recordHeader)
	if _, err := io.ReadFull(r, header); err != nil {
		if err == io.EOF {
			return Record{}, 0, io.EOF
		}
		if err == io.ErrUnexpectedEOF {
			return Record{}, 0, fmt.Errorf("%w: partial header", ErrInvalidRecord)
		}
		return Record{}, 0, err
	}

	if binary.BigEndian.Uint32(header[0:4]) != recordMagic {
		return Record{}, 0, fmt.Errorf("%w: bad magic", ErrInvalidRecord)
	}
	if binary.BigEndian.Uint16(header[4:6]) != recordVersion {
		return Record{}, 0, fmt.Errorf("%w: unsupported version", ErrInvalidRecord)
	}

	offset := Offset(binary.BigEndian.Uint64(header[8:16]))
	timestamp := int64(binary.BigEndian.Uint64(header[16:24]))
	keyLen := int(binary.BigEndian.Uint32(header[24:28]))
	valueLen := int(binary.BigEndian.Uint32(header[28:32]))
	headersLen := int(binary.BigEndian.Uint32(header[32:36]))
	wantChecksum := binary.BigEndian.Uint32(header[36:40])

	payloadLen := keyLen + valueLen + headersLen
	if payloadLen < 0 || recordHeader+payloadLen > maxRecordBytes {
		return Record{}, 0, fmt.Errorf("%w: payload size", ErrInvalidRecord)
	}

	payload := make([]byte, payloadLen)
	if _, err := io.ReadFull(r, payload); err != nil {
		if err == io.ErrUnexpectedEOF {
			return Record{}, 0, fmt.Errorf("%w: partial payload", ErrInvalidRecord)
		}
		return Record{}, 0, err
	}

	key := payload[:keyLen]
	value := payload[keyLen : keyLen+valueLen]
	headersBytes := payload[keyLen+valueLen:]
	gotChecksum := checksum(key, value, headersBytes)
	if gotChecksum != wantChecksum {
		return Record{}, 0, fmt.Errorf("%w: checksum mismatch", ErrInvalidRecord)
	}

	headers := map[string]string{}
	if len(headersBytes) > 0 && string(headersBytes) != "null" {
		if err := json.Unmarshal(headersBytes, &headers); err != nil {
			return Record{}, 0, fmt.Errorf("%w: headers: %v", ErrInvalidRecord, err)
		}
	}

	record := Record{
		Offset:    offset,
		Timestamp: time.Unix(0, timestamp).UTC(),
		Key:       append([]byte(nil), key...),
		Value:     append([]byte(nil), value...),
		Headers:   headers,
		Checksum:  wantChecksum,
	}
	return record, int64(recordHeader + payloadLen), nil
}

func checksum(parts ...[]byte) uint32 {
	h := crc32.NewIEEE()
	for _, part := range parts {
		_, _ = h.Write(part)
	}
	return h.Sum32()
}

func marshalHeaders(headers map[string]string) ([]byte, error) {
	if headers == nil {
		headers = map[string]string{}
	}
	return json.Marshal(headers)
}
