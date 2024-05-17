/*
NAME
  rtmp.go

DESCRIPTION
  RTMP command functionality.

AUTHORS
  Saxon Nelson-Milton <saxon@ausocean.org>
  Dan Kortschak <dan@ausocean.org>
  Alan Noble <alan@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/


package rtmp

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"strconv"
	"time"

	"github.com/ausocean/av/protocol/rtmp/amf"
)

const (
	pkg            = "rtmp:"
	signatureSize  = 1536
	fullHeaderSize = 12
)

// Link flags.
const (
	linkAuth     = 0x0001 // using auth param
	linkLive     = 0x0002 // stream is live
	linkSWF      = 0x0004 // do SWF verification - not implemented
	linkPlaylist = 0x0008 // send playlist before play - not implemented
	linkBufx     = 0x0010 // toggle stream on BufferEmpty msg - not implemented
)

// Protocol features.
const (
	featureHTTP   = 0x01 // not implemented
	featureEncode = 0x02 // not implemented
	featureSSL    = 0x04 // not implemented
	featureMFP    = 0x08 // not implemented
	featureWrite  = 0x10 // publish, not play
	featureHTTP2  = 0x20 // server-side RTMPT - not implemented
)

// RTMP protocols.
const (
	protoRTMP   = 0
	protoRTMPE  = featureEncode
	protoRTMPT  = featureHTTP
	protoRTMPS  = featureSSL
	protoRTMPTE = (featureHTTP | featureEncode)
	protoRTMPTS = (featureHTTP | featureSSL)
	protoRTMFP  = featureMFP
)

// RTMP tokens (lexemes).
// NB: Underscores are deliberately preserved in const names where they exist in the corresponding tokens.
const (
	av_checkbw                       = "_checkbw"
	av_onbwcheck                     = "_onbwcheck"
	av_onbwdone                      = "_onbwdone"
	av_result                        = "_result"
	avApp                            = "app"
	avAudioCodecs                    = "audioCodecs"
	avCapabilities                   = "capabilities"
	avClose                          = "close"
	avCode                           = "code"
	avConnect                        = "connect"
	avCreatestream                   = "createStream"
	avDeletestream                   = "deleteStream"
	avFCPublish                      = "FCPublish"
	avFCUnpublish                    = "FCUnpublish"
	avFlashver                       = "flashVer"
	avFpad                           = "fpad"
	avLevel                          = "level"
	avLive                           = "live"
	avNetConnectionConnectInvalidApp = "NetConnection.Connect.InvalidApp"
	avNetStreamFailed                = "NetStream.Failed"
	avNetStreamPauseNotify           = "NetStream.Pause.Notify"
	avNetStreamPlayComplete          = "NetStream.Play.Complete"
	avNetStreamPlayFailed            = "NetStream.Play.Failed"
	avNetStreamPlayPublishNotify     = "NetStream.Play.PublishNotify"
	avNetStreamPlayStart             = "NetStream.Play.Start"
	avNetStreamPlayStop              = "NetStream.Play.Stop"
	avNetStreamPlayStreamNotFound    = "NetStream.Play.StreamNotFound"
	avNetStreamPlayUnpublishNotify   = "NetStream.Play.UnpublishNotify"
	avNetStreamPublish_Start         = "NetStream.Publish.Start"
	avNetStreamSeekNotify            = "NetStream.Seek.Notify"
	avNonprivate                     = "nonprivate"
	avObjectEncoding                 = "objectEncoding"
	avOnBWDone                       = "onBWDone"
	avOnFCSubscribe                  = "onFCSubscribe"
	avOnFCUnsubscribe                = "onFCUnsubscribe"
	avOnStatus                       = "onStatus"
	avPageUrl                        = "pageUrl"
	avPing                           = "ping"
	avPlay                           = "play"
	avPlaylist_ready                 = "playlist_ready"
	avPublish                        = "publish"
	avReleasestream                  = "releaseStream"
	avSecureToken                    = "secureToken"
	avSet_playlist                   = "set_playlist"
	avSwfUrl                         = "swfUrl"
	avTcUrl                          = "tcUrl"
	avType                           = "type"
	avVideoCodecs                    = "videoCodecs"
	avVideoFunction                  = "videoFunction"
)

// RTMP protocol strings.
var rtmpProtocolStrings = [...]string{
	"rtmp",
	"rtmpt",
	"rtmpe",
	"rtmpte",
	"rtmps",
	"rtmpts",
	"",
	"",
	"rtmfp",
}

// RTMP errors.
var (
	errUnknownScheme = errors.New("rtmp: unknown scheme")
	errInvalidURL    = errors.New("rtmp: invalid URL")
	errConnected     = errors.New("rtmp: already connected")
	errNotConnected  = errors.New("rtmp: not connected")
	errNotWritable   = errors.New("rtmp: connection not writable")
	errInvalidHeader = errors.New("rtmp: invalid header")
	errInvalidBody   = errors.New("rtmp: invalid body")
	ErrInvalidFlvTag = errors.New("rtmp: invalid FLV tag")
	errUnimplemented = errors.New("rtmp: unimplemented feature")
)

// connect establishes an RTMP connection.
func connect(c *Conn) error {
	addrStr := c.link.host + ":" + strconv.Itoa(int(c.link.port))
	addr, err := net.ResolveTCPAddr("tcp4", addrStr)
	if err != nil {
		return fmt.Errorf("could not resolve tcp address (%s):%w", addrStr, err)
	}
	c.link.conn, err = net.DialTCP("tcp4", nil, addr)
	if err != nil {
		c.log(WarnLevel, pkg+"dial failed", "error", err.Error())
		return fmt.Errorf("could not dial tcp: %w", err)
	}
	c.log(DebugLevel, pkg+"connected")

	defer func() {
		if err != nil {
			c.link.conn.Close()
		}
	}()

	err = handshake(c)
	if err != nil {
		c.log(WarnLevel, pkg+"handshake failed", "error", err.Error())
		return fmt.Errorf("could not handshake: %w", err)
	}
	c.log(DebugLevel, pkg+"handshaked")
	err = sendConnectPacket(c)
	if err != nil {
		c.log(WarnLevel, pkg+"sendConnect failed", "error", err.Error())
		return fmt.Errorf("could not send connect packet: %w", err)
	}

	c.log(DebugLevel, pkg+"negotiating")
	var buf [256]byte
	pkt := packet{buf: buf[:]}
	for !c.isPlaying {
		err = pkt.readFrom(c)
		if err != nil {
			return fmt.Errorf("could not read from packet: %w", err)
		}

		if pkt.isReady() {
			if pkt.bodySize == 0 {
				continue
			}

			switch pkt.packetType {
			case packetTypeAudio, packetTypeVideo, packetTypeInfo:
				c.log(WarnLevel, pkg+"got packet before play; ignoring", "type", pkt.packetType)
			default:
				err = handlePacket(c, &pkt)
				if err != nil {
					return fmt.Errorf("could not handle packet: %w", err)
				}
			}
			pkt = packet{buf: buf[:]}
		}

	}
	return nil
}

// handlePacket handles a packet that the client has received.
// NB: Unsupported packet types are logged fatally.
func handlePacket(c *Conn, pkt *packet) error {
	if pkt.bodySize < 4 {
		return errInvalidBody
	}

	switch pkt.packetType {
	case packetTypeChunkSize:
		c.inChunkSize = amf.DecodeInt32(pkt.body[:4])
		c.log(DebugLevel, pkg+"set inChunkSize", "size", int(c.inChunkSize))

	case packetTypeBytesReadReport:
		c.log(DebugLevel, pkg+"received packetTypeBytesReadReport")

	case packetTypeServerBW:
		c.serverBW = amf.DecodeInt32(pkt.body[:4])
		c.log(DebugLevel, pkg+"set serverBW", "size", int(c.serverBW))

	case packetTypeClientBW:
		c.clientBW = amf.DecodeInt32(pkt.body[:4])
		c.log(DebugLevel, pkg+"set clientBW", "size", int(c.clientBW))
		if pkt.bodySize > 4 {
			c.clientBW2 = pkt.body[4]
			c.log(DebugLevel, pkg+"set clientBW2", "size", int(c.clientBW2))
		} else {
			c.clientBW2 = 0xff
		}

	case packetTypeInvoke:
		err := handleInvoke(c, pkt.body[:pkt.bodySize])
		if err != nil {
			c.log(WarnLevel, pkg+"unexpected error from handleInvoke", "error", err.Error())
			return fmt.Errorf("could not handle invoke: %w", err)
		}

	case packetTypeControl, packetTypeAudio, packetTypeVideo, packetTypeFlashVideo, packetTypeFlexMessage, packetTypeInfo:
		c.log(FatalLevel, pkg+"unsupported packet type "+strconv.Itoa(int(pkt.packetType)))

	default:
		c.log(WarnLevel, pkg+"unknown packet type", "type", pkt.packetType)
	}
	return nil
}

func sendConnectPacket(c *Conn) error {
	var pbuf [4096]byte
	pkt := packet{
		channel:    chanControl,
		headerType: headerSizeLarge,
		packetType: packetTypeInvoke,
		buf:        pbuf[:],
		body:       pbuf[fullHeaderSize:],
	}
	enc := pkt.body

	enc, err := amf.EncodeString(enc, avConnect)
	if err != nil {
		return fmt.Errorf("could not encode string: %w", err)
	}
	c.numInvokes += 1
	enc, err = amf.EncodeNumber(enc, float64(c.numInvokes))
	if err != nil {
		return fmt.Errorf("could not encode number of invokes: %w", err)
	}

	// required link info
	info := amf.Object{Properties: []amf.Property{
		amf.Property{Type: amf.TypeString, Name: avApp, String: c.link.app},
		amf.Property{Type: amf.TypeString, Name: avType, String: avNonprivate},
		amf.Property{Type: amf.TypeString, Name: avTcUrl, String: c.link.url}},
	}
	enc, err = amf.Encode(&info, enc)
	if err != nil {
		return fmt.Errorf("could not encode info: %w", err)
	}

	// optional link auth info
	if c.link.auth != "" {
		enc, err = amf.EncodeBoolean(enc, c.link.flags&linkAuth != 0)
		if err != nil {
			return fmt.Errorf("could not encode link auth bool: %w", err)
		}
		enc, err = amf.EncodeString(enc, c.link.auth)
		if err != nil {
			return fmt.Errorf("could not encode link auth string: %w", err)
		}
	}

	pkt.bodySize = uint32((len(pbuf) - fullHeaderSize) - len(enc))

	err = pkt.writeTo(c, true)
	if err != nil {
		return fmt.Errorf("could not write packet: %w", err)
	}

	return nil
}

func sendCreateStream(c *Conn) error {
	var pbuf [256]byte
	pkt := packet{
		channel:    chanControl,
		headerType: headerSizeMedium,
		packetType: packetTypeInvoke,
		buf:        pbuf[:],
		body:       pbuf[fullHeaderSize:],
	}
	enc := pkt.body

	enc, err := amf.EncodeString(enc, avCreatestream)
	if err != nil {
		return fmt.Errorf("could not encode av create stream token: %w", err)
	}
	c.numInvokes++
	enc, err = amf.EncodeNumber(enc, float64(c.numInvokes))
	if err != nil {
		return fmt.Errorf("could not encode number of invokes: %w", err)
	}
	enc[0] = amf.TypeNull
	enc = enc[1:]

	pkt.bodySize = uint32((len(pbuf) - fullHeaderSize) - len(enc))

	err = pkt.writeTo(c, true)
	if err != nil {
		return fmt.Errorf("could not write packet: %w", err)
	}
	return nil
}

func sendReleaseStream(c *Conn) error {
	var pbuf [1024]byte
	pkt := packet{
		channel:    chanControl,
		headerType: headerSizeMedium,
		packetType: packetTypeInvoke,
		buf:        pbuf[:],
		body:       pbuf[fullHeaderSize:],
	}
	enc := pkt.body

	enc, err := amf.EncodeString(enc, avReleasestream)
	if err != nil {
		return fmt.Errorf("could not encode av release stream token: %w", err)
	}
	c.numInvokes++
	enc, err = amf.EncodeNumber(enc, float64(c.numInvokes))
	if err != nil {
		return fmt.Errorf("could not encode number of invokes: %w", err)
	}
	enc[0] = amf.TypeNull
	enc = enc[1:]
	enc, err = amf.EncodeString(enc, c.link.playpath)
	if err != nil {
		return fmt.Errorf("could not encode playpath: %w", err)
	}
	pkt.bodySize = uint32((len(pbuf) - fullHeaderSize) - len(enc))

	err = pkt.writeTo(c, false)
	if err != nil {
		return fmt.Errorf("could not write packet: %w", err)
	}

	return nil
}

func sendFCPublish(c *Conn) error {
	var pbuf [1024]byte
	pkt := packet{
		channel:    chanControl,
		headerType: headerSizeMedium,
		packetType: packetTypeInvoke,
		buf:        pbuf[:],
		body:       pbuf[fullHeaderSize:],
	}
	enc := pkt.body

	enc, err := amf.EncodeString(enc, avFCPublish)
	if err != nil {
		return fmt.Errorf("could not encode av fc publish token: %w", err)
	}
	c.numInvokes++
	enc, err = amf.EncodeNumber(enc, float64(c.numInvokes))
	if err != nil {
		return fmt.Errorf("could not encode number of invokes: %w", err)
	}
	enc[0] = amf.TypeNull
	enc = enc[1:]
	enc, err = amf.EncodeString(enc, c.link.playpath)
	if err != nil {
		return fmt.Errorf("could not encode playpath: %w", err)
	}

	pkt.bodySize = uint32((len(pbuf) - fullHeaderSize) - len(enc))

	err = pkt.writeTo(c, false)
	if err != nil {
		return fmt.Errorf("could not write packet: %w", err)
	}

	return nil
}

func sendFCUnpublish(c *Conn) error {
	var pbuf [1024]byte
	pkt := packet{
		channel:    chanControl,
		headerType: headerSizeMedium,
		packetType: packetTypeInvoke,
		buf:        pbuf[:],
		body:       pbuf[fullHeaderSize:],
	}
	enc := pkt.body

	enc, err := amf.EncodeString(enc, avFCUnpublish)
	if err != nil {
		return fmt.Errorf("could not encode av fc unpublish token: %w", err)
	}
	c.numInvokes++
	enc, err = amf.EncodeNumber(enc, float64(c.numInvokes))
	if err != nil {
		return fmt.Errorf("could not encode number of invokes: %w", err)
	}
	enc[0] = amf.TypeNull
	enc = enc[1:]
	enc, err = amf.EncodeString(enc, c.link.playpath)
	if err != nil {
		return fmt.Errorf("could not encode link playpath: %w", err)
	}

	pkt.bodySize = uint32((len(pbuf) - fullHeaderSize) - len(enc))

	err = pkt.writeTo(c, false)
	if err != nil {
		return fmt.Errorf("could not write packet: %w", err)
	}

	return nil
}

func sendPublish(c *Conn) error {
	var pbuf [1024]byte
	pkt := packet{
		channel:    chanSource,
		headerType: headerSizeLarge,
		packetType: packetTypeInvoke,
		buf:        pbuf[:],
		body:       pbuf[fullHeaderSize:],
	}
	enc := pkt.body

	enc, err := amf.EncodeString(enc, avPublish)
	if err != nil {
		return fmt.Errorf("could not encode av publish token: %w", err)
	}
	c.numInvokes++
	enc, err = amf.EncodeNumber(enc, float64(c.numInvokes))
	if err != nil {
		return fmt.Errorf("could not encode number of invokes: %w", err)
	}
	enc[0] = amf.TypeNull
	enc = enc[1:]
	enc, err = amf.EncodeString(enc, c.link.playpath)
	if err != nil {
		return fmt.Errorf("could not encode link playpath: %w", err)
	}
	enc, err = amf.EncodeString(enc, avLive)
	if err != nil {
		return fmt.Errorf("could not encode av live token: %w", err)
	}

	pkt.bodySize = uint32((len(pbuf) - fullHeaderSize) - len(enc))

	err = pkt.writeTo(c, true)
	if err != nil {
		return fmt.Errorf("could not write packet: %w", err)
	}

	return nil
}

func sendDeleteStream(c *Conn, streamID float64) error {
	var pbuf [256]byte
	pkt := packet{
		channel:    chanControl,
		headerType: headerSizeMedium,
		packetType: packetTypeInvoke,
		buf:        pbuf[:],
		body:       pbuf[fullHeaderSize:],
	}
	enc := pkt.body

	enc, err := amf.EncodeString(enc, avDeletestream)
	if err != nil {
		return fmt.Errorf("could not encode av delete stream token: %w", err)
	}
	c.numInvokes++
	enc, err = amf.EncodeNumber(enc, float64(c.numInvokes))
	if err != nil {
		return fmt.Errorf("could not encode number of invokes: %w", err)
	}
	enc[0] = amf.TypeNull
	enc = enc[1:]
	enc, err = amf.EncodeNumber(enc, streamID)
	if err != nil {
		return fmt.Errorf("could not encode stream id: %w", err)
	}
	pkt.bodySize = uint32((len(pbuf) - fullHeaderSize) - len(enc))

	err = pkt.writeTo(c, false)
	if err != nil {
		return fmt.Errorf("could not write packet: %w", err)
	}

	return nil
}

// sendBytesReceived tells the server how many bytes the client has received.
func sendBytesReceived(c *Conn) error {
	var pbuf [256]byte
	pkt := packet{
		channel:    chanBytesRead,
		headerType: headerSizeMedium,
		packetType: packetTypeBytesReadReport,
		buf:        pbuf[:],
		body:       pbuf[fullHeaderSize:],
	}
	enc := pkt.body

	c.nBytesInSent = c.nBytesIn

	enc, err := amf.EncodeInt32(enc, c.nBytesIn)
	if err != nil {
		return fmt.Errorf("could not encode number of bytes in: %w", err)
	}
	pkt.bodySize = 4

	err = pkt.writeTo(c, false)
	if err != nil {
		return fmt.Errorf("could not write packet: %w", err)
	}

	return nil
}

func sendCheckBW(c *Conn) error {
	var pbuf [256]byte
	pkt := packet{
		channel:    chanControl,
		headerType: headerSizeLarge,
		packetType: packetTypeInvoke,
		buf:        pbuf[:],
		body:       pbuf[fullHeaderSize:],
	}
	enc := pkt.body

	enc, err := amf.EncodeString(enc, av_checkbw)
	if err != nil {
		return fmt.Errorf("could not encode av check bw token: %w", err)
	}
	c.numInvokes++
	enc, err = amf.EncodeNumber(enc, float64(c.numInvokes))
	if err != nil {
		return fmt.Errorf("could not encode number of invokes: %w", err)
	}
	enc[0] = amf.TypeNull
	enc = enc[1:]

	pkt.bodySize = uint32((len(pbuf) - fullHeaderSize) - len(enc))

	err = pkt.writeTo(c, false)
	if err != nil {
		return fmt.Errorf("could not write packet: %w", err)
	}

	return nil
}

func eraseMethod(m []method, i int) []method {
	copy(m[i:], m[i+1:])
	m[len(m)-1] = method{}
	return m[:len(m)-1]
}

// int handleInvoke handles a packet invoke request
// Side effects: c.isPlaying set to true upon avNetStreamPublish_Start
func handleInvoke(c *Conn, body []byte) error {
	if body[0] != 0x02 {
		return errInvalidBody
	}
	var obj amf.Object
	_, err := amf.Decode(&obj, body, false)
	if err != nil {
		return fmt.Errorf("could not decode: %w", err)
	}

	meth, err := obj.StringProperty("", 0)
	if err != nil {
		return fmt.Errorf("could not get value of string property meth: %w", err)
	}
	txn, err := obj.NumberProperty("", 1)
	if err != nil {
		return fmt.Errorf("could not get value of number property txn: %w", err)
	}

	c.log(DebugLevel, pkg+"invoking method "+meth)
	switch meth {
	case av_result:
		if (c.link.protocol & featureWrite) == 0 {
			return errNotWritable
		}
		var methodInvoked string
		for i, m := range c.methodCalls {
			if float64(m.num) == txn {
				methodInvoked = m.name
				c.methodCalls = eraseMethod(c.methodCalls, i)
				break
			}
		}
		if methodInvoked == "" {
			c.log(WarnLevel, pkg+"received result without matching request", "id", txn)
			return nil
		}
		c.log(DebugLevel, pkg+"received result for "+methodInvoked)

		switch methodInvoked {
		case avConnect:
			err := sendReleaseStream(c)
			if err != nil {
				return fmt.Errorf("could not send release stream: %w", err)
			}
			err = sendFCPublish(c)
			if err != nil {
				return fmt.Errorf("could not send fc publish: %w", err)
			}
			err = sendCreateStream(c)
			if err != nil {
				return fmt.Errorf("could not send create stream: %w", err)
			}

		case avCreatestream:
			n, err := obj.NumberProperty("", 3)
			if err != nil {
				return fmt.Errorf("could not get value for stream id number property: %w", err)
			}
			c.streamID = uint32(n)
			err = sendPublish(c)
			if err != nil {
				return fmt.Errorf("could not send publish: %w", err)
			}

		default:
			c.log(FatalLevel, pkg+"unexpected method invoked"+methodInvoked)
		}

	case avOnBWDone:
		err := sendCheckBW(c)
		if err != nil {
			return fmt.Errorf("could not send check bw: %w", err)
		}

	case avOnStatus:
		obj2, err := obj.ObjectProperty("", 3)
		if err != nil {
			return fmt.Errorf("could not get object property value for obj2: %w", err)
		}
		code, err := obj2.StringProperty(avCode, -1)
		if err != nil {
			return fmt.Errorf("could not get string property value for code: %w", err)
		}
		level, err := obj2.StringProperty(avLevel, -1)
		if err != nil {
			return fmt.Errorf("could not get string property value for level: %w", err)
		}
		c.log(DebugLevel, pkg+"onStatus", "code", code, "level", level)

		if code != avNetStreamPublish_Start {
			c.log(ErrorLevel, pkg+"unexpected response "+code)
			return fmt.Errorf("unimplemented code: %v", code)
		}
		c.log(DebugLevel, pkg+"playing")
		c.isPlaying = true
		for i, m := range c.methodCalls {
			if m.name == avPublish {
				c.methodCalls = eraseMethod(c.methodCalls, i)
			}
		}

	default:
		c.log(FatalLevel, pkg+"unsuppoted method "+meth)
	}
	return nil
}

func handshake(c *Conn) error {
	var clientbuf [signatureSize + 1]byte
	clientsig := clientbuf[1:]

	var serversig [signatureSize]byte
	clientbuf[0] = chanControl
	binary.BigEndian.PutUint32(clientsig, uint32(time.Now().UnixNano()/1000000))
	copy(clientsig[4:8], []byte{0, 0, 0, 0})

	for i := 8; i < signatureSize; i++ {
		clientsig[i] = byte(rand.Intn(256))
	}

	_, err := c.write(clientbuf[:])
	if err != nil {
		return fmt.Errorf("could not write handshake: %w", err)
	}
	c.log(DebugLevel, pkg+"handshake sent")

	var typ [1]byte
	_, err = c.read(typ[:])
	if err != nil {
		return fmt.Errorf("could not read handshake: %w", err)
	}
	c.log(DebugLevel, pkg+"handshake received")

	if typ[0] != clientbuf[0] {
		c.log(WarnLevel, pkg+"handshake type mismatch", "sent", clientbuf[0], "received", typ)
	}
	_, err = c.read(serversig[:])
	if err != nil {
		return fmt.Errorf("could not read server signal: %w", err)
	}

	// decode server response
	suptime := binary.BigEndian.Uint32(serversig[:4])
	c.log(DebugLevel, pkg+"server uptime", "uptime", suptime)

	// 2nd part of handshake
	_, err = c.write(serversig[:])
	if err != nil {
		return fmt.Errorf("could not write part 2 of handshake: %w", err)
	}

	_, err = c.read(serversig[:])
	if err != nil {
		return fmt.Errorf("could not read part 2 of handshake: %w", err)
	}

	if !bytes.Equal(serversig[:signatureSize], clientbuf[1:signatureSize+1]) {
		c.log(WarnLevel, pkg+"signature mismatch", "serversig", serversig[:signatureSize], "clientsig", clientbuf[1:signatureSize+1])
	}
	return nil
}
