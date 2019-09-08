package proto

import (
	"encoding/json"
	"log"
	"net"
	"time"
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
	Type      string `json:"type"`
	Timestamp int64  `json:"timestamp"`
	Message   string `json:"message"`
	Key       string `json:"key"`
}

// Proto - Main object to use. Has functions to interact with stuff
type Proto struct {
	conn net.Conn
}

//Init - Sets up the proto object
func (p Proto) Init(c net.Conn) {
	p.conn = c
}

//SendLogin - For clients to send their credentials to the server
func (p Proto) SendLogin(username string, password string) error {
	l := Login{}
	l.Username = username
	l.Password = password
	l.Type = "login"
	l.Timestamp = time.Now().Unix()
	j, err := json.Marshal(l)
	if err != nil {
		return err
	}
	//we might need a newline but if the other end is go we should be fine?
	p.conn.Write([]byte(j))
	return nil
}

//SendBadLoginResponse - send a bad login response back ot the client
func (p Proto) SendBadLoginResponse() error {
	lr := LoginResponse{}
	lr.Timestamp = time.Now().Unix()
	lr.Code = 403
	lr.Key = ""
	lr.Type = "login-response"
	j, err := json.Marshal(lr)
	if err != nil {
		return err
	}
	//we might need a newline but if the other end is go we should be fine?
	p.conn.Write([]byte(j))
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
	lr.Code = 200
	lr.Key = key
	lr.Type = "login-response"
	j, err := json.Marshal(lr)
	if err != nil {
		return err
	}
	//we might need a newline but if the other end is go we should be fine?
	p.conn.Write([]byte(j))
	return nil
}

//Decode - returns a type defined in this package
func (p Proto) Decode() interface{} {

	d := json.NewDecoder(p.conn)
	var a Type
	err := d.Decode(&a)
	check(err)

	var i interface{}

	if a.Type == "login" {
		err := d.Decode(&i)
		check(err)
	} else if a.Type == "message" {
		err := d.Decode(&i)
		check(err)
	}
	return i
}

func check(e error) {
	if e != nil {
		panic(e)
	}
}
