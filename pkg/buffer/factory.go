package buffer

import (
	"io"
	"sync"

	"github.com/go-logr/logr"
	"github.com/pion/transport/packetio"
)

type Factory interface {
	GetOrNew(packetType packetio.BufferPacketType, ssrc uint32) io.ReadWriteCloser
	GetBufferPair(ssrc uint32) (Buffer, RTCPReader)
	GetBuffer(ssrc uint32) Buffer
	GetRTCPReader(ssrc uint32) RTCPReader
}

type factory struct {
	sync.RWMutex
	videoPool   *sync.Pool
	audioPool   *sync.Pool
	rtpBuffers  map[uint32]*buffer
	rtcpReaders map[uint32]*reader
	logger      logr.Logger
}

func NewBufferFactory(trackingPackets int, logger logr.Logger) *factory {
	// Enable package wide logging for non-method functions.
	// If logger is empty - use default Logger.
	// Logger is a public variable in buffer package.
	if logger == (logr.Logger{}) {
		logger = Logger
	} else {
		Logger = logger
	}

	return &factory{
		videoPool: &sync.Pool{
			New: func() interface{} {
				b := make([]byte, trackingPackets*maxPktSize)
				return &b
			},
		},
		audioPool: &sync.Pool{
			New: func() interface{} {
				b := make([]byte, maxPktSize*25)
				return &b
			},
		},
		rtpBuffers:  make(map[uint32]*buffer),
		rtcpReaders: make(map[uint32]*reader),
		logger:      logger,
	}
}

func (f *factory) GetOrNew(packetType packetio.BufferPacketType, ssrc uint32) io.ReadWriteCloser {
	f.Lock()
	defer f.Unlock()
	switch packetType {
	case packetio.RTCPBufferPacket:
		if reader, ok := f.rtcpReaders[ssrc]; ok {
			return reader
		}
		reader := NewRTCPReader(ssrc)
		f.rtcpReaders[ssrc] = reader
		reader.OnClose(func() {
			f.Lock()
			delete(f.rtcpReaders, ssrc)
			f.Unlock()
		})
		return reader
	case packetio.RTPBufferPacket:
		if reader, ok := f.rtpBuffers[ssrc]; ok {
			return reader
		}
		buffer := NewBuffer(ssrc, f.videoPool, f.audioPool, f.logger)
		f.rtpBuffers[ssrc] = buffer
		buffer.OnClose(func() {
			f.Lock()
			delete(f.rtpBuffers, ssrc)
			f.Unlock()
		})
		return buffer
	}
	return nil
}

func (f *factory) GetBufferPair(ssrc uint32) (Buffer, RTCPReader) {
	f.RLock()
	defer f.RUnlock()
	return f.rtpBuffers[ssrc], f.rtcpReaders[ssrc]
}

func (f *factory) GetBuffer(ssrc uint32) Buffer {
	f.RLock()
	defer f.RUnlock()
	return f.rtpBuffers[ssrc]
}

func (f *factory) GetRTCPReader(ssrc uint32) RTCPReader {
	f.RLock()
	defer f.RUnlock()
	return f.rtcpReaders[ssrc]
}
