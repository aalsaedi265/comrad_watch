package rtmp

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"strings"

	flvtag "github.com/yutopp/go-flv/tag"
	"github.com/yutopp/go-rtmp"
	rtmpmsg "github.com/yutopp/go-rtmp/message"
)

// Compile-time check that connHandler implements rtmp.Handler.
var _ rtmp.Handler = (*connHandler)(nil)

// connHandler handles a single RTMP connection lifecycle.
type connHandler struct {
	rtmp.DefaultHandler
	server    *Server
	streamKey string
}

func (h *connHandler) OnServe(conn *rtmp.Conn) {
	// Nothing to do on serve; connection is already accepted.
}

func (h *connHandler) OnConnect(timestamp uint32, cmd *rtmpmsg.NetConnectionConnect) error {
	log.Println("RTMP client connected")
	return nil
}

func (h *connHandler) OnCreateStream(timestamp uint32, cmd *rtmpmsg.NetConnectionCreateStream) error {
	return nil
}

func (h *connHandler) OnPublish(_ *rtmp.StreamContext, timestamp uint32, cmd *rtmpmsg.NetStreamPublish) error {
	h.streamKey = strings.TrimSpace(cmd.PublishingName)
	if h.streamKey == "" {
		return fmt.Errorf("empty stream key")
	}

	log.Printf("publish request: key=%.8s...", h.streamKey)

	if err := h.server.registerStream(h.streamKey); err != nil {
		return fmt.Errorf("register stream: %w", err)
	}

	return nil
}

// OnSetDataFrame captures stream metadata (codec info, resolution, etc.)
// and writes it to the FLV file.
func (h *connHandler) OnSetDataFrame(timestamp uint32, data *rtmpmsg.NetStreamSetDataFrame) error {
	r := bytes.NewReader(data.Payload)

	var script flvtag.ScriptData
	if err := flvtag.DecodeScriptData(r, &script); err != nil {
		log.Printf("failed to decode script data: %v", err)
		return nil // non-fatal: continue without metadata
	}

	return h.server.writeData(h.streamKey, &flvtag.FlvTag{
		TagType:   flvtag.TagTypeScriptData,
		Timestamp: timestamp,
		Data:      &script,
	})
}

func (h *connHandler) OnAudio(timestamp uint32, payload io.Reader) error {
	var audio flvtag.AudioData
	if err := flvtag.DecodeAudioData(payload, &audio); err != nil {
		return err
	}

	// CRITICAL: Buffer the data reader. The original reader is consumed
	// during decode — we must copy it so the FLV encoder can read it.
	buf := new(bytes.Buffer)
	if _, err := io.Copy(buf, audio.Data); err != nil {
		return err
	}
	audio.Data = buf

	return h.server.writeData(h.streamKey, &flvtag.FlvTag{
		TagType:   flvtag.TagTypeAudio,
		Timestamp: timestamp,
		Data:      &audio,
	})
}

func (h *connHandler) OnVideo(timestamp uint32, payload io.Reader) error {
	var video flvtag.VideoData
	if err := flvtag.DecodeVideoData(payload, &video); err != nil {
		return err
	}

	// CRITICAL: Buffer the data reader (same reason as OnAudio).
	buf := new(bytes.Buffer)
	if _, err := io.Copy(buf, video.Data); err != nil {
		return err
	}
	video.Data = buf

	return h.server.writeData(h.streamKey, &flvtag.FlvTag{
		TagType:   flvtag.TagTypeVideo,
		Timestamp: timestamp,
		Data:      &video,
	})
}

func (h *connHandler) OnClose() {
	log.Printf("RTMP client disconnected: key=%.8s...", h.streamKey)
	if h.streamKey != "" {
		h.server.onDisconnect(h.streamKey, "disconnect")
	}
}
