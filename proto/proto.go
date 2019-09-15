package proto

import (
	"bufio"
	"encoding/json"
	"log"
	"net"
	"time"
)

const (
	LOGIN               = "login"
	JOINROOM            = "joinroom"
	REGISTER_RESPONSE   = "register-response"
	REGISTER            = "register"
	LOGIN_RESPONSE      = "login-response"
	MESSAGE             = "message"
	JOINROOMRESPONSE    = "joinroom-response"
	CREATEROOM          = "createroom"
	CREATEROOMRESPONSE  = "createroom-response"
	GETROOMS            = "getrooms"
	GETROOMSRESPONSE    = "getrooms-response"
	GETMESSAGES         = "getmessages"
	GETMESSAGESRESPONSE = "getmessages-response"
	HTTP_OK             = 200
	HTTP_FORBIDDEN      = 403
	HTTP_BADREQUEST     = 400
	HTTP_ERROR          = 500
	HTTP_UNAVAILABLE    = 503
)

//Type - Only gets the type from the decoder
type Type struct {
	Type string `json:"type"`
}

//LoginResponse - tells the client if they were logged in or not, and if so, what their uuid is
type LoginResponse struct {
	Type      string `json:"type"`
	Timestamp int64  `json:"timestamp"`
	Code      int    `json:"code"`
	Key       string `json:"key"`
}

//RegisterResponse - Tells them if their registration was successful. They'll have to login
type RegisterResponse struct {
	Type      string `json:"type"`
	Timestamp int64  `json:"timestamp"`
	Code      int    `json:"code"`
}

//Register - Object for a login
type Register struct {
	Type      string `json:"type"`
	Timestamp int64  `json:"timestamp"`
	Username  string `json:"username"`
	Password  string `json:"password"`
}

//Login - Object for a login
type Login struct {
	Type      string `json:"type"`
	Timestamp int64  `json:"timestamp"`
	Username  string `json:"username"`
	Password  string `json:"password"`
}

//MessageResponse - Object for a message response
type MessageResponse struct {
	Type      string `json:"type"`
	Timestamp int64  `json:"timestamp"`
	Message   string `json:"message"`
	Key       string `json:"key"`
	Code      int    `json:"code"`
}

//Message - Object for a message
type Message struct {
	ID        int       `json:"id"`
	UserID    int       `json:"user_id"`
	Type      string    `json:"type"`
	Timestamp int64     `json:"timestamp"`
	Message   string    `json:"message"`
	Key       string    `json:"key"`
	Created   time.Time `json:"created"`
	Received  time.Time `json:"received"`
}

//MessageOrder - array of message IDs
type MessageOrder struct {
	IDS []int
}

//JoinRoomRequest - Packet representing a room request
type JoinRoomRequest struct {
	Type      string `json:"type"`
	Timestamp int64  `json:"timestamp"`
	Room      string `json:"room"`
	Key       string `json:"key"`
}

//JoinRoomResponse - Packet representing a room request
type JoinRoomResponse struct {
	Type      string `json:"type"`
	Timestamp int64  `json:"timestamp"`
	Room      string `json:"room"`
	Code      int    `json:"code"`
}

//CreateRoomRequest - Packet representing a room create
type CreateRoomRequest struct {
	Type      string `json:"type"`
	Timestamp int64  `json:"timestamp"`
	Room      string `json:"room"`
	Key       string `json:"key"`
	Password  string `json:"password"`
}

//CreateRoomResponse - Packet representing a room create
type CreateRoomResponse struct {
	Type      string `json:"type"`
	Timestamp int64  `json:"timestamp"`
	Room      string `json:"room"`
	Code      int    `json:"code"`
}

//GetRoomsRequest -
type GetRoomsRequest struct {
	Type      string `json:"type"`
	Timestamp int64  `json:"timestamp"`
	Key       string `json:"key"`
}

//Channel - Channel object
type Channel struct {
	ID       int              `json:"id"`
	Name     string           `json:"name"`
	Messages map[int]*Message `json:"messages"`
}

//Room - Room object
type Room struct {
	ID          int              `json:"id"`
	Name        string           `json:"name"`
	DisplayName string           `json:"displayname"`
	Channels    map[int]*Channel `json:"channels"`
	Users       map[int]*User    `json:"users"`
}

//User - represents a user
type User struct {
	ID          int       `json:"id"`
	UserName    string    `json:"username"`
	DisplayName string    `json:"displayname"`
	Created     time.Time `json:"created"`
}

//GetRoomsResponse -
type GetRoomsResponse struct {
	Type      string        `json:"type"`
	Timestamp int64         `json:"timestamp"`
	Rooms     map[int]*Room `json:"rooms"`
	Code      int           `json:"code"`
}

//GetMessagesResponse -
type GetMessagesResponse struct {
	Type      string           `json:"type"`
	Timestamp int64            `json:"timestamp"`
	Messages  map[int]*Message `json:"messages"`
	Code      int              `json:"code"`
}

//GetMessagesRequest -
type GetMessagesRequest struct {
	Type      string `json:"type"`
	Timestamp int64  `json:"timestamp"`
	Key       string `json:"key"`
	Room      int    `json:"room"`
	Channel   int    `json:"channel"`
}

// Proto - Main object to use. Has functions to interact with stuff
type Proto struct {
	Conn net.Conn
	key  string
}

//SendGetMessagesRequest -sends a request to get messages for a specific channel
func (p *Proto) SendGetMessagesRequest(room int, channel int) error {
	mr := GetMessagesRequest{}
	mr.Timestamp = time.Now().Unix()
	mr.Room = room
	mr.Channel = channel
	mr.Type = GETMESSAGES
	mr.Key = p.key
	j, err := json.Marshal(mr)
	if err != nil {
		return err
	}
	//TODO compress into one call
	p.Conn.Write([]byte(j))
	p.Conn.Write([]byte("\n"))
	return nil
}

//SendGetMessagesResponse - sends a response to a getmessages request
func (p *Proto) SendGetMessagesResponse(c int, m map[int]*Message) error {
	gmr := GetMessagesResponse{}
	gmr.Timestamp = time.Now().Unix()
	gmr.Type = GETMESSAGESRESPONSE
	gmr.Messages = m
	gmr.Code = c
	j, err := json.Marshal(gmr)
	if err != nil {
		return err
	}
	//TODO compress into one call
	p.Conn.Write([]byte(j))
	p.Conn.Write([]byte("\n"))
	return nil
}

// SendJoinRoom -  sends a request to join a room
func (p *Proto) SendJoinRoom(name string, password string) error {
	jr := JoinRoomRequest{}
	jr.Room = name
	jr.Timestamp = time.Now().Unix()
	jr.Type = JOINROOM
	jr.Key = p.key
	j, err := json.Marshal(jr)
	if err != nil {
		return err
	}
	//TODO compress into one call
	p.Conn.Write([]byte(j))
	p.Conn.Write([]byte("\n"))
	return nil
}

//SetKey - set the session key for the protocol to use
func (p *Proto) SetKey(key string) {
	p.key = key
}

//SendLogin - For clients to send their credentials to the server
func (p Proto) SendLogin(username string, password string) error {
	l := Login{}
	l.Username = username
	l.Password = password
	l.Type = LOGIN
	l.Timestamp = time.Now().Unix()
	j, err := json.Marshal(l)
	if err != nil {
		return err
	}
	//TODO compress into one call
	p.Conn.Write([]byte(j))
	p.Conn.Write([]byte("\n"))
	return nil
}

//SendCreateRoom - tells the server to create a room
func (p Proto) SendCreateRoom(name string, password string) error {
	cr := CreateRoomRequest{}
	cr.Room = name
	cr.Timestamp = time.Now().Unix()
	cr.Type = CREATEROOM
	cr.Key = p.key
	cr.Password = password
	j, err := json.Marshal(cr)
	if err != nil {
		return err
	}
	//TODO compress into one call
	p.Conn.Write([]byte(j))
	p.Conn.Write([]byte("\n"))
	return nil
}

//SendCreateRoomResponse - sends a create room response to the client
func (p Proto) SendCreateRoomResponse(r string, code int) error {
	jrr := CreateRoomResponse{}
	jrr.Timestamp = time.Now().Unix()
	jrr.Type = CREATEROOMRESPONSE
	jrr.Code = code
	jrr.Room = r
	j, err := json.Marshal(jrr)
	if err != nil {
		return err
	}
	//TODO compress into one call
	p.Conn.Write([]byte(j))
	p.Conn.Write([]byte("\n"))
	return nil
}

//SendJoinRoomResponse - sends a join room response to the client
func (p Proto) SendJoinRoomResponse(r string, code int) error {
	jrr := JoinRoomResponse{}
	jrr.Timestamp = time.Now().Unix()
	jrr.Type = JOINROOMRESPONSE
	jrr.Code = code
	jrr.Room = r
	j, err := json.Marshal(jrr)
	if err != nil {
		return err
	}
	//TODO compress into one call
	p.Conn.Write([]byte(j))
	p.Conn.Write([]byte("\n"))
	return nil
}

//SendRegistrationResponse - sends a registration packet to the client
func (p Proto) SendRegistrationResponse(code int) error {
	rr := RegisterResponse{}
	rr.Timestamp = time.Now().Unix()
	rr.Type = REGISTER_RESPONSE
	rr.Code = code
	j, err := json.Marshal(rr)
	if err != nil {
		return err
	}
	//TODO compress into one call
	p.Conn.Write([]byte(j))
	p.Conn.Write([]byte("\n"))
	return nil
}

//SendRegistration - sends a registration packet to the server
func (p Proto) SendRegistration(username string, password string) error {
	r := Register{}
	r.Timestamp = time.Now().Unix()
	r.Type = REGISTER
	r.Username = username
	r.Password = password
	j, err := json.Marshal(r)
	if err != nil {
		return err
	}
	//TODO compress into one call
	p.Conn.Write([]byte(j))
	p.Conn.Write([]byte("\n"))
	return nil
}

//SendBadLoginResponse - send a bad login response back ot the client
func (p Proto) SendBadLoginResponse() error {
	lr := LoginResponse{}
	lr.Timestamp = time.Now().Unix()
	lr.Code = HTTP_FORBIDDEN
	lr.Key = ""
	lr.Type = LOGIN_RESPONSE
	j, err := json.Marshal(lr)
	if err != nil {
		return err
	}
	//we might need a newline but if the other end is go we should be fine?
	p.Conn.Write([]byte(j))
	p.Conn.Write([]byte("\n"))
	return nil
}

//SendLoginResponse - send a login response back ot the client
func (p Proto) SendLoginResponse(key string) error {
	if key == "" {
		log.Println("UUID must be invalid")
		return nil
	}
	lr := LoginResponse{}
	lr.Timestamp = time.Now().Unix()
	lr.Code = HTTP_OK
	lr.Key = key
	lr.Type = LOGIN_RESPONSE
	j, err := json.Marshal(lr)
	if err != nil {
		return err
	}
	//we might need a newline but if the other end is go we should be fine?
	p.Conn.Write([]byte(j))
	p.Conn.Write([]byte("\n"))
	return nil
}

//SendGetRoomsRequest - tell the server you want some room data
func (p Proto) SendGetRoomsRequest() error {
	gr := GetRoomsRequest{}
	gr.Timestamp = time.Now().Unix()
	gr.Type = GETROOMS
	gr.Key = p.key
	j, err := json.Marshal(gr)
	if err != nil {
		return err
	}
	//we might need a newline but if the other end is go we should be fine?
	p.Conn.Write([]byte(j))
	p.Conn.Write([]byte("\n"))
	return nil
}

//SendGetRoomsResponse -
func (p Proto) SendGetRoomsResponse(code int, rooms map[int]*Room) error {
	grr := GetRoomsResponse{}
	grr.Timestamp = time.Now().Unix()
	grr.Rooms = rooms
	grr.Type = GETROOMSRESPONSE
	grr.Code = code
	j, err := json.Marshal(grr)
	if err != nil {
		return err
	}
	//we might need a newline but if the other end is go we should be fine?
	p.Conn.Write([]byte(j))
	p.Conn.Write([]byte("\n"))
	return nil
}

//Decode - returns a type defined in this package
func (p Proto) Decode() interface{} {

	text, err := bufio.NewReader(p.Conn).ReadBytes('\n')
	if err != nil {
		log.Println(err)
		return nil
	}

	var a Type
	err = json.Unmarshal(text, &a)
	check(err)

	if a.Type == LOGIN {
		var l Login
		err := json.Unmarshal(text, &l)
		check(err)
		return l
	} else if a.Type == MESSAGE {
		var m Message
		err := json.Unmarshal(text, &m)
		check(err)
		return m
	} else if a.Type == LOGIN_RESPONSE {
		var lr LoginResponse
		err := json.Unmarshal(text, &lr)
		check(err)
		return lr
	} else if a.Type == REGISTER {
		var r Register
		err := json.Unmarshal(text, &r)
		check(err)
		return r
	} else if a.Type == REGISTER_RESPONSE {
		var rr RegisterResponse
		err := json.Unmarshal(text, &rr)
		check(err)
		return rr
	} else if a.Type == JOINROOM {
		var jr JoinRoomRequest
		err := json.Unmarshal(text, &jr)
		check(err)
		return jr
	} else if a.Type == JOINROOMRESPONSE {
		var jrr JoinRoomResponse
		err := json.Unmarshal(text, &jrr)
		check(err)
		return jrr
	} else if a.Type == CREATEROOMRESPONSE {
		var crr CreateRoomResponse
		err := json.Unmarshal(text, &crr)
		check(err)
		return crr
	} else if a.Type == CREATEROOM {
		var cr CreateRoomRequest
		err := json.Unmarshal(text, &cr)
		check(err)
		return cr
	} else if a.Type == GETROOMS {
		var gr GetRoomsRequest
		err := json.Unmarshal(text, &gr)
		check(err)
		return gr
	} else if a.Type == GETROOMSRESPONSE {
		var grr GetRoomsResponse
		err := json.Unmarshal(text, &grr)
		check(err)
		return grr
	} else if a.Type == GETMESSAGES {
		var gmr GetMessagesRequest
		err := json.Unmarshal(text, &gmr)
		check(err)
		return gmr
	} else if a.Type == GETMESSAGESRESPONSE {
		var gmr GetMessagesResponse
		err := json.Unmarshal(text, &gmr)
		check(err)
		return gmr
	}
	return nil
}

func check(e error) {
	if e != nil {
		panic(e)
	}
}
