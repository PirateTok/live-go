package connection

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"strconv"

	pb "github.com/PirateTok/live-go/proto"
	"google.golang.org/protobuf/proto"
)

func buildHeartbeat(roomID string) ([]byte, error) {
	rid, err := strconv.ParseUint(roomID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("heartbeat: invalid room_id %q: %w", roomID, err)
	}
	hb := &pb.HeartbeatMessage{RoomId: rid}
	hbBytes, err := proto.Marshal(hb)
	if err != nil {
		return nil, fmt.Errorf("heartbeat: marshal inner: %w", err)
	}
	frame := &pb.WebcastPushFrame{
		PayloadEncoding: "pb",
		PayloadType:     "hb",
		Payload:         hbBytes,
	}
	return proto.Marshal(frame)
}

func buildEnterRoom(roomID string) ([]byte, error) {
	rid, err := strconv.ParseInt(roomID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("enter room: invalid room_id %q: %w", roomID, err)
	}
	msg := &pb.WebcastImEnterRoomMessage{
		RoomId:           rid,
		LiveId:           12,
		Identity:         "audience",
		FilterWelcomeMsg: "0",
	}
	msgBytes, err := proto.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("enter room: marshal inner: %w", err)
	}
	frame := &pb.WebcastPushFrame{
		PayloadEncoding: "pb",
		PayloadType:     "im_enter_room",
		Payload:         msgBytes,
	}
	return proto.Marshal(frame)
}

func buildAck(logID uint64, internalExt []byte) ([]byte, error) {
	frame := &pb.WebcastPushFrame{
		LogId:           logID,
		PayloadEncoding: "pb",
		PayloadType:     "ack",
		Payload:         internalExt,
	}
	return proto.Marshal(frame)
}

// DecompressIfGzipped checks for gzip magic bytes and decompresses if present.
func DecompressIfGzipped(data []byte) ([]byte, error) {
	if len(data) < 2 || data[0] != 0x1f || data[1] != 0x8b {
		return data, nil
	}
	r, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("gzip open: %w", err)
	}
	defer r.Close()
	out, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("gzip read: %w", err)
	}
	return out, nil
}
