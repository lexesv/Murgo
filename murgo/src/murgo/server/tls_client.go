
package server


import (
	"bytes"
	"encoding/binary"
	"errors"
	"github.com/golang/protobuf/proto"
	"net"
	"io"
	"mumble.info/grumble/pkg/mumbleproto"
	"bufio"
	"murgo/config"
	"murgo/data"
)


type TlsClient struct {
	supervisor *Supervisor
	cast chan interface{}



	conn net.Conn
	session uint32
	channel *data.Channel

	username string
	server *TlsServer
	reader  *bufio.Reader

	tcpaddr *net.TCPAddr
	certHash string

	//client auth infomations
	codecs []int32
	tokens []string

	//crypt state
	cryptState *config.CryptState

	//for test
	testCounter int
}

// write 작업과 read 작업 구분 필요


func NewTlsClient(supervisor *Supervisor, conn net.Conn) (*TlsClient){

	//create new object
	tlsClient := new(TlsClient)
	tlsClient.cryptState = new(config.CryptState)

	//set servers
	tlsClient.supervisor = supervisor
	tlsClient.server = supervisor.ts

	//
	tlsClient.conn = conn
	tlsClient.session = tlsClient.server.sessionPool.Get()
	tlsClient.reader = bufio.NewReader(tlsClient.conn)

	tlsClient.testCounter = 0
	return tlsClient
}

func (tlsClient *TlsClient) startTlsClient(){

	go tlsClient.recvLoop()
	for {

		select {
		case castData := <-tlsClient.cast:
			tlsClient.handleCast(castData)
		}
	}
}

func (tlsClient *TlsClient) handleCast (castData interface{}) {

	//tlsClient.readProtoMessage()
}




//send msg to client
func (tlsClient *TlsClient) sendMessage(msg interface{}) error {
	buf := new(bytes.Buffer)
	var (
		kind    uint16
		msgData []byte
		err     error
	)

	kind = mumbleproto.MessageType(msg)
	if kind == mumbleproto.MessageUDPTunnel {
		msgData = msg.([]byte)
	} else {
		protoMsg, ok := (msg).(proto.Message)
		if !ok {
			return errors.New("client: exepcted a proto.Message")
		}
		msgData, err = proto.Marshal(protoMsg)
		if err != nil {
			return err
		}
	}

	err = binary.Write(buf, binary.BigEndian, kind)
	if err != nil {
		return err
	}
	err = binary.Write(buf, binary.BigEndian, uint32(len(msgData)))
	if err != nil {
		return err
	}
	_, err = buf.Write(msgData)
	if err != nil {
		return err
	}

	_, err = tlsClient.conn.Write(buf.Bytes())
	if err != nil {
		return err
	}

	return nil
}

//
func (tlsClient *TlsClient) readProtoMessage() (msg *data.Message, err error) {
	var (
		length uint32
		kind   uint16
	)

	// Read the message type (16-bit big-endian unsigned integer)
	//read data form io.reader
	err = binary.Read(tlsClient.reader, binary.BigEndian, &kind)
	if err != nil {
		return
	}

	// Read the message length (32-bit big-endian unsigned integer)
	err = binary.Read(tlsClient.reader, binary.BigEndian, &length)
	if err != nil {
		return
	}

	buf := make([]byte, length)
	_, err = io.ReadFull(tlsClient.reader, buf)
	if err != nil {
		return
	}
	tlsClient.testCounter++

	//todo : 메세지 상속
	msg = &data.Message{}
	/*{
		buf:    buf,
		kind:   kind,
		client: tlsClient,
		testCounter: tlsClient.testCounter,
	}*/
	msg.SetBuf(buf)
	msg.SetClient(tlsClient)
	msg.SetKind(kind)
	msg.SetTestCounter(tlsClient.testCounter)


	return msg, err
}

func (tlsClient *TlsClient) recvLoop (){
	for {
		msg, err := tlsClient.readProtoMessage()
		if err != nil {
			if err != nil {
				if err == io.EOF {
					tlsClient.Disconnect()
				} else {
					//client.Panicf("%v", err)
				}
				return
			}
		}
		tlsClient.supervisor.mh.cast <- msg
	}
}


func (tlsClient *TlsClient) Disconnect() {

}

func (tlsClient *TlsClient) Session()(uint32) {
	return tlsClient.session
}


