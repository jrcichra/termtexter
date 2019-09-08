package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"reflect"
	"strconv"

	ttdb "termtexter/db"
	proto "termtexter/proto"

	"github.com/google/uuid"
)

//Server - an instance of a termtexter server
type Server struct {
	db ttdb.DB
}

func (s Server) check(e error) {
	if e != nil {
		panic(e)
	}
}

// Init - Initalizes a termtexter server
func (s Server) Init(port int) {
	service := ":" + strconv.Itoa(port)
	tcpAddr, err := net.ResolveTCPAddr("tcp4", service)
	s.check(err)
	listener, err := net.ListenTCP("tcp", tcpAddr)
	s.check(err)

	// connect to our db package
	s.db.Connect(os.Args[1], os.Args[2])

	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}
		go s.handleClient(conn)
	}
}

func (s Server) handleLogin(l proto.Login, p proto.Proto) {
	log.Println(l.Type)
	log.Println(l.Timestamp)
	log.Println(l.Username)
	log.Println(l.Password)
	if l.Username == "" {
		log.Println("Username cannot be an empty field.")
		return
	}
	if l.Password == "" {
		log.Println("Password cannot be an empty field.")
		return
	}

	// We have a login packet, it has a username and password, let's check it against the database
	id, _ := s.db.GetUserID(l.Username)
	if id != "" {
		res := s.db.IsValidLogin(id, l.Password)
		if res {
			//They are a real user. Give them a unique id for their successful login. This key lets them send messages from their account on the machine they logged in from
			uuid, err := uuid.NewRandom()
			s.check(err)
			// Add this key to the DB, so we can check with this for each message
			err = s.db.AddClient(id, uuid.String())
			s.check(err)
			// Send the packet with the updates
			err = p.SendLoginResponse(uuid.String())
			s.check(err)
		} else {
			// They don't exist, craft a response that doesn't have a good login
			err := p.SendBadLoginResponse()
			s.check(err)
		}
	} else {
		// They don't exist, craft a response that doesn't have a good login
		err := p.SendBadLoginResponse()
		s.check(err)
	}

}

func (s Server) handleMessage(m proto.Message, p proto.Proto) {
	log.Println(m.Type)
	log.Println(m.Timestamp)
	log.Println(m.Message)
	log.Println(m.Key)
}

func (s Server) handleClient(conn net.Conn) {
	//defer conn.Close() // close connection before exit

	//get a proto object which handles the message/protocol for us
	p := proto.Proto{Conn: conn}
	flag := false
	for !flag {
		//based on the message type, take different actions
		switch msg := p.Decode().(type) {
		case proto.Login:
			s.handleLogin(msg, p)
		case proto.Message:
			s.handleMessage(msg, p)
		default:
			if msg == nil {
				log.Println("Somebody left")
				flag = true
				break
			} else {
				r := reflect.TypeOf(msg)
				fmt.Printf("Other:%v\n", r)
			}
		}

	}

}

func main() {
	s := new(Server)
	s.Init(1200)
}
