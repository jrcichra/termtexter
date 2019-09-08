package proto

import (
	"bufio"
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
	Conn net.Conn
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
	//TODO compress into one call
	p.Conn.Write([]byte(j))
	p.Conn.Write([]byte("\n"))
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
	lr.Code = 200
	lr.Key = key
	lr.Type = "login-response"
	j, err := json.Marshal(lr)
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

	if a.Type == "login" {
		var l Login
		err := json.Unmarshal(text, &l)
		check(err)
		return l
	} else if a.Type == "message" {
		var m Message
		err := json.Unmarshal(text, &m)
		check(err)
		return m
	} else if a.Type == "login-response" {
		var lr LoginResponse
		err := json.Unmarshal(text, &lr)
		check(err)
		return lr
	}
	return nil
}

func check(e error) {
	if e != nil {
		panic(e)
	}
}
