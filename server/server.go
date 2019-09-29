package main

import (
	"container/list"
	"fmt"
	"log"
	"net"
	"os"
	"reflect"
	"strconv"
	"time"

	ttdb "termtexter/db"
	proto "termtexter/proto"

	"github.com/google/uuid"
)

const (
	HTTP_OK          = 200
	HTTP_FORBIDDEN   = 403
	HTTP_BADREQUEST  = 400
	HTTP_ERROR       = 500
	HTTP_UNAVAILABLE = 503
)

//Server - an instance of a termtexter server
type Server struct {
	db          ttdb.DB
	connections map[int]*list.List  //map of user ids to an array of sockets, because one user can be logged in multiple places at the same time
	Rooms       map[int]*proto.Room //map of rooms to keep track of room information
}

func (s Server) check(e error) {
	if e != nil {
		panic(e)
	}
}

// Init - Initalizes a termtexter server
func (s *Server) Init(port int) {
	service := ":" + strconv.Itoa(port)
	tcpAddr, err := net.ResolveTCPAddr("tcp4", service)
	s.check(err)
	listener, err := net.ListenTCP("tcp", tcpAddr)
	s.check(err)
	//init the maps we have
	s.connections = make(map[int]*list.List)
	s.Rooms = make(map[int]*proto.Room)

	// connect to our db package
	s.db.Connect(os.Args[1], os.Args[2])

	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}
		log.Println("Someone connected.")
		go s.handleClient(conn)
	}
}

//DistributeMessage -
func (s *Server) DistributeMessage(id int, pm proto.PostMessageRequest, rowid int64) {
	dm := proto.DynamicMessage{}
	dm.Channel = pm.Channel
	dm.Timestamp = pm.Timestamp
	dm.Message = pm.Message
	dm.Room = pm.Room
	dm.Channel = pm.Channel
	dm.UserID = id
	dm.ID = int(rowid)
	dm.Type = proto.DYNAMICMESSAGE
	dm.Created = time.Now().Round(time.Second)

	//loop through linked lists for every user in this room
	for _, v := range s.Rooms[dm.Room].Users {
		//go through the linked list, distributing the message to all who care
		if s.connections[v.ID] != nil {
			node := s.connections[v.ID].Front()
			log.Println(s.connections[id].Len())
			for i := 0; i < s.connections[id].Len(); i++ {
				switch p := node.Value.(type) {
				case *proto.Proto:
					p.SendDynamicMessage(&dm)
				default:
					log.Fatalln("Did not get *proto.Proto in the linked list while distributing a message")
				}
				node = node.Next()

			}
		}
	}
}

func (s *Server) updateServerRooms(id string) error {
	res, err := s.db.GetRooms(id)
	s.Rooms = res
	return err
}

func (s *Server) handleLogin(l proto.Login, p proto.Proto) int {
	if l.Username == "" {
		log.Println("Username cannot be an empty field.")
		p.SendBadLoginResponse()
		return -1
	}
	if l.Password == "" {
		log.Println("Password cannot be an empty field.")
		p.SendBadLoginResponse()
		return -1
	}

	// We have a login packet, it has a username and password, let's check it against the database
	id, err := s.db.GetUserID(l.Username)
	var intid int
	if err != nil {
		intid = -1
		log.Println("Bad login")
		// They don't exist, craft a response that doesn't have a good login
		err := p.SendBadLoginResponse()
		s.check(err)
	} else {
		intid, err = strconv.Atoi(id)
		s.check(err)
		if id != "" {
			res := s.db.IsValidLogin(id, l.Password)
			if res {
				//They are a real user. Give them a unique id for their successful login. This key lets them send messages from their account on the machine they logged in from
				uuid, err := uuid.NewRandom()
				s.check(err)
				// Add this key to the DB, so we can check with this for each message
				err = s.db.AddSession(id, uuid.String())
				s.check(err)
				// Send the packet with the updates
				err = p.SendLoginResponse(uuid.String())
				//add this proto object to our linked list of sockets for this user
				//see if it has been initalized yet
				if s.connections[intid] == nil {
					s.connections[intid] = list.New()
				}
				s.connections[intid].PushBack(&p)
				log.Println("Added the user to the linked list")
				//See what rooms this user is in (for the server's records)
				s.updateServerRooms(id)
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
	return intid
}

func (s *Server) handleMessage(m proto.Message, p proto.Proto) {
	log.Println(m.Type)
	log.Println(m.Timestamp)
	log.Println(m.Message)
	log.Println(m.Key)
}

func (s *Server) handleRegistration(r proto.Register, p proto.Proto) {
	//Make sure this username doesn't already exist
	exists, err := s.db.UserExists(r.Username)
	s.check(err)
	if exists {
		log.Println("Sorry, someone with this username already exists")
		p.SendRegistrationResponse(HTTP_BADREQUEST)
	} else {
		//This username is not used, continue with the registration
		err := s.db.Register(r.Username, r.Password)
		s.check(err)
		//Let them know how the registeration went
		p.SendRegistrationResponse(HTTP_OK)
	}
}
func (s *Server) handleCreateRoom(cr proto.CreateRoomRequest, p proto.Proto) {
	if cr.Room == "" {
		log.Println("Room name cannot be empty")
		p.SendCreateRoomResponse(cr.Room, HTTP_ERROR)
		return
	}
	if cr.Key == "" {
		log.Println("Key cannot be empty")
		p.SendCreateRoomResponse(cr.Room, HTTP_ERROR)
		return
	}

	//cr.Password can be left empty, if they don't want a password on their server

	// Figure out what user is behind this key:
	id, err := s.db.GetUserIDFromKey(cr.Key)
	s.check(err)
	if id == "" {
		//They're not a person in the database
		p.SendCreateRoomResponse(cr.Room, HTTP_FORBIDDEN)
		return
	}

	//See if the room exists
	res, err := s.db.DoesRoomExist(cr.Room)
	s.check(err)

	if res {
		//The room exists...give them an error
		p.SendCreateRoomResponse(cr.Room, HTTP_BADREQUEST)
	} else {
		//We can make the room, put the requester as an admin, and create a default channel
		err := s.db.CreateRoom(cr.Room, id, cr.Password)
		s.check(err)
		//update the server cache
		err = s.updateServerRooms(id)
		s.check(err)
		//We did it all, tell them how it went
		p.SendCreateRoomResponse(cr.Room, HTTP_OK)
	}
}

func (s *Server) handleJoinRoom(jr proto.JoinRoomRequest, p proto.Proto) {
	if jr.Room == "" {
		log.Println("Room name cannot be empty")
		return
	}
	if jr.Key == "" {
		log.Println("Key cannot be empty")
		return
	}

	// Figure out what user is behind this key:
	id, err := s.db.GetUserIDFromKey(jr.Key)
	s.check(err)
	if id == "" {
		//They're not a person in the database
		p.SendJoinRoomResponse(jr.Room, HTTP_FORBIDDEN)
		return
	}

	//See if the room exists
	res, err := s.db.DoesRoomExist(jr.Room)
	s.check(err)
	if res {
		//This room does exist...
		err = s.db.AddUserToRoom(id, jr.Room)
		s.check(err)
		if err == nil {
			err = s.updateServerRooms(id)
			if err == nil {
				p.SendJoinRoomResponse(jr.Room, HTTP_OK)
			} else {
				//Something went wrong updating the server cache
				p.SendJoinRoomResponse(jr.Room, HTTP_ERROR)
			}
		}
	} else {
		//The room does not exist...send them a sad response
		p.SendJoinRoomResponse(jr.Room, HTTP_BADREQUEST)
	}

}

func (s *Server) handlePostMessage(pm proto.PostMessageRequest, p proto.Proto) {
	if pm.Key == "" {
		log.Println("Key cannot be empty")
		p.SendPostMessageResponse(HTTP_FORBIDDEN)
		return
	}

	// Figure out what user is behind this key:
	id, err := s.db.GetUserIDFromKey(pm.Key)
	s.check(err)
	if id == "" {
		//They're not a person in the database
		p.SendPostMessageResponse(HTTP_FORBIDDEN)
		return
	}

	//try to insert the message in the proper place
	rowID, err := s.db.PostMessage(id, pm)
	s.check(err)
	//distribute the message to all the proper connections
	intid, err := strconv.Atoi(id)
	s.check(err)
	s.DistributeMessage(intid, pm, rowID)
	//send a good response to the sender
	p.SendPostMessageResponse(HTTP_OK)

}

func (s *Server) handleGetMessages(gm proto.GetMessagesRequest, p proto.Proto) {
	if gm.Key == "" {
		log.Println("Key cannot be empty")
		p.SendGetMessagesResponse(HTTP_FORBIDDEN, nil)
		return
	}

	// Figure out what user is behind this key:
	id, err := s.db.GetUserIDFromKey(gm.Key)
	s.check(err)
	if id == "" {
		//They're not a person in the database
		p.SendGetMessagesResponse(HTTP_FORBIDDEN, nil)
		return
	}

	//See what messages this room has
	res, err := s.db.GetMessages(gm.Room, gm.Channel)
	s.check(err)

	//Send them the list back
	p.SendGetMessagesResponse(HTTP_OK, res)

}

func (s *Server) handleGetRooms(gr proto.GetRoomsRequest, p proto.Proto) {
	if gr.Key == "" {
		log.Println("Key cannot be empty")
		p.SendGetRoomsResponse(HTTP_FORBIDDEN, nil)
		return
	}

	// Figure out what user is behind this key:
	id, err := s.db.GetUserIDFromKey(gr.Key)
	s.check(err)
	if id == "" {
		//They're not a person in the database
		p.SendGetRoomsResponse(HTTP_FORBIDDEN, nil)
		return
	}

	//See what rooms this user is in
	res, err := s.db.GetRooms(id)
	s.check(err)

	//Send them the list back
	p.SendGetRoomsResponse(HTTP_OK, res)

}

func (s *Server) handleClient(conn net.Conn) {
	//defer conn.Close() // close connection before exit

	//get a proto object which handles the message/protocol for us
	p := proto.Proto{Conn: conn}
	id := -1 //the id of the client, if we get that far
	flag := false
	for !flag {
		//based on the message type, take different actions
		switch msg := p.Decode().(type) {
		case proto.Login:
			id = s.handleLogin(msg, p)
		case proto.Message:
			s.handleMessage(msg, p)
		case proto.Register:
			s.handleRegistration(msg, p)
		case proto.JoinRoomRequest:
			s.handleJoinRoom(msg, p)
		case proto.CreateRoomRequest:
			s.handleCreateRoom(msg, p)
		case proto.GetRoomsRequest:
			s.handleGetRooms(msg, p)
		case proto.GetMessagesRequest:
			s.handleGetMessages(msg, p)
		case proto.PostMessageRequest:
			s.handlePostMessage(msg, p)
		default:
			if msg == nil {
				log.Println("Somebody left")
				//drop this connection from our records, if it's not empty
				if id != -1 {
					found := false
					node := s.connections[id].Front()
					for node != nil && !found {
						//for this id, see if one of these connection memory addresses match
						switch p := node.Value.(type) {
						case *proto.Proto:
							if conn == p.Conn {
								//if so, drop the node we are on from the linked list
								s.connections[id].Remove(node)
								//stop the loop, we found and removed the connection
								found = true
								log.Println("We dropped the connection from the linked list for the user who just left")
							}
						default:
							log.Fatalln("Did not get *proto.Proto in the linked list")
						}
						node = node.Next()

					}
					if !found {
						log.Println("Weird...we didn't find that connection in the linked list...")
					}
				}
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
