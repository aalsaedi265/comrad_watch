package rtmp

import (
	"fmt"
	"io"
	"log"
	"strings"

	flvtag "github.com/yutopp/go-flv/tag"
	"github.com/yutopp/go-rtmp"
	rtmpmsg "github.com/yutopp/go-rtmp/message"
)

// connHandler handles a single RTMP connection lifecycle.
type connHandler struct {
	rtmp.DefaultHandler
	server    *Server
	streamKey string
}

func (h *connHandler) OnConnect(timestamp uint32, cmd *rtmpmsg.NetConnectionConnect) error {
	log.Println("RTMP client connected")
	return nil
}

func (h *connHandler) OnCreateStream(timestamp uint32, cmd *rtmpmsg.NetConnectionCreateStream) error {
	return nil
}

func (h *connHandler) OnPublish(_ *rtmp.StreamContext, timestamp uint32, cmd *rtmpmsg.NetStreamPublish) error {
	// The stream key is passed as the publishing name.
	// RTMP URL format: rtmp://server:1935/live/STREAM_KEY
	// cmd.PublishingName will be the stream key.
	h.streamKey = strings.TrimSpace(cmd.PublishingName)

	if h.streamKey == "" {
		return fmt.Errorf("empty stream key")
	}

	log.Printf("publish request: key=%s", h.streamKey)

	if err := h.server.registerStream(h.streamKey); err != nil {
		return fmt.Errorf("register stream: %w", err)
	}

	return nil
}

func (h *connHandler) OnAudio(timestamp uint32, payload io.Reader) error {
	tag := &flvtag.FlvTag{
		TagType:   flvtag.TagTypeAudio,
		Timestamp: timestamp,
	}

	// Read audio data
	var audio flvtag.AudioData
	if err := flvtag.DecodeAudioData(payload, &audio); err != nil {
		return err
	}
	tag.Data = &audio

	return h.server.writeData(h.streamKey, tag)
}

func (h *connHandler) OnVideo(timestamp uint32, payload io.Reader) error {
	tag := &flvtag.FlvTag{
		TagType:   flvtag.TagTypeVideo,
		Timestamp: timestamp,
	}

	// Read video data
	var video flvtag.VideoData
	if err := flvtag.DecodeVideoData(payload, &video); err != nil {
		return err
	}
	tag.Data = &video

	return h.server.writeData(h.streamKey, tag)
}

func (h *connHandler) OnClose() {
	log.Printf("RTMP client disconnected: key=%s", h.streamKey)

	if h.streamKey != "" {
		h.server.onDisconnect(h.streamKey, "disconnect")
	}
}
