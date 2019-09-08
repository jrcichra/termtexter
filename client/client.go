package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"
	"reflect"
	"strconv"
	"syscall"
	proto "termtexter/proto"

	"golang.org/x/crypto/ssh/terminal"
)

const (
	HTTP_OK          = 200
	HTTP_FORBIDDEN   = 403
	HTTP_BADREQUEST  = 400
	HTTP_ERROR       = 500
	HTTP_UNAVAILABLE = 503
)

//Client - client struct
type Client struct {
	conn  net.Conn
	proto proto.Proto
	rooms []string
}

func (c Client) check(e error) {
	if e != nil {
		panic(e)
	}
}

//Init - get the client socket ready
func (c *Client) Init(host string, port int) {
	var err error
	a, err := net.Dial("tcp", host+":"+strconv.Itoa(port))
	c.conn = a
	c.check(err)
	c.proto = proto.Proto{Conn: c.conn}

}

//GetCredentials - Gets the login credentials from the user
func (c Client) GetCredentials() (string, string) {
	reader := bufio.NewReader(os.Stdin)
	username := ""
	for username == "" {
		fmt.Println("Enter your username:")
		var err error
		username, err = reader.ReadString('\n')
		c.check(err)
	}
	password := ""
	for password == "" {
		fmt.Println("Enter your password:")
		bytepwd, err := terminal.ReadPassword(int(syscall.Stdin))
		c.check(err)
		password = string(bytepwd)
	}
	return username, password
}

//GetRegistrationResponse - decode the struct so we see its what we expect
func (c Client) GetRegistrationResponse() proto.RegisterResponse {
	var ret proto.RegisterResponse
	switch msg := c.proto.Decode().(type) {
	case proto.RegisterResponse:
		ret = msg
	default:
		r := reflect.TypeOf(msg)
		fmt.Printf("Unexpected type:%v\n", r)
		os.Exit(1)
	}
	return ret
}

//SendRegistration - calls the proto register
func (c Client) SendRegistration(username string, password string) {
	c.proto.SendRegistration(username, password)
}

//CreateRoom - creates a room
func (c *Client) CreateRoom(name string, password string) int {
	err := c.proto.SendCreateRoom(name, password)
	c.check(err)
	var res proto.CreateRoomResponse
	switch msg := c.proto.Decode().(type) {
	case proto.CreateRoomResponse:
		res = msg
		if res.Code == HTTP_OK {
			//We joined the room
			log.Println("Successfully created the", name, "room.")
		} else if res.Code == HTTP_BADREQUEST {
			//The room already exists
			log.Println("This room already exists", name)
		} else {
			log.Println("Unknown return code from the server:", res.Code)
		}
	default:
		r := reflect.TypeOf(msg)
		fmt.Printf("Unexpected type:%v\n", r)
		os.Exit(1)
	}
	return res.Code
}

//JoinRoom - joins a room. This is a one time per account operation, just to link your account to a room (if you know the name and password). Returns success
func (c *Client) JoinRoom(name string, password string) bool {
	err := c.proto.SendJoinRoom(name, password)
	c.check(err)
	var res proto.JoinRoomResponse
	ret := false
	switch msg := c.proto.Decode().(type) {
	case proto.JoinRoomResponse:
		res = msg
		if res.Code == HTTP_OK {
			//We joined the room
			log.Println("Successfully joined the", name, "room.")
			ret = true
		} else if res.Code == HTTP_BADREQUEST {
			//The room doesn't exist
			log.Println("This room doesn't exist (yet):", name)
		} else if res.Code == HTTP_FORBIDDEN {
			//bad
			log.Println("The password you entered for this room is incorrect, or you were banned from this room")
		} else {
			log.Println("Unknown return code from the server:", res.Code)
		}
	default:
		r := reflect.TypeOf(msg)
		fmt.Printf("Unexpected type:%v\n", r)
		os.Exit(1)
	}
	return ret
}

//Login - Logs user in and returns http code
func (c *Client) Login(username string, password string) int {
	err := c.proto.SendLogin(username, password)
	c.check(err)
	var ret proto.LoginResponse
	switch msg := c.proto.Decode().(type) {
	case proto.LoginResponse:
		ret = msg
		if ret.Code == 200 {
			//Set our proto's session key
			c.proto.SetKey(ret.Key)
		} else {
			log.Println("Not setting the session key because we got a bad return code...")
		}
	default:
		r := reflect.TypeOf(msg)
		fmt.Printf("Unexpected type:%v\n", r)
		os.Exit(1)
	}
	return ret.Code
}

//GetRooms - replaces the list of rooms in the object with what the database says
func (c *Client) GetRooms() {
	err := c.proto.SendGetRoomsRequest()
	c.check(err)
	var ret proto.GetRoomsResponse
	switch msg := c.proto.Decode().(type) {
	case proto.GetRoomsResponse:
		ret = msg
		if ret.Code == 200 {
			//We got a good response...
			c.rooms = msg.Rooms
		} else {
			log.Println("Not updating the rooms because we got a bad return code...")
		}
	default:
		r := reflect.TypeOf(msg)
		fmt.Printf("Unexpected type:%v\n", r)
		os.Exit(1)
	}
	return
}

func main() {
	c := new(Client)
	c.Init("localhost", 1200)
	//username, password := c.GetCredentials()

	// c.SendRegistration("justin", "poop")
	// r := c.GetRegistrationResponse()
	// log.Println(r)

	if c.Login("Justin", "password") == 200 {
		//We got a good response, we are logged in and the session key is set in the proto object.
		//We don't have to worry about it as a coder here

		//Create a room
		//if c.CreateRoom("aroom", "test") == 200 {
		//	log.Println("Room was made")
		//}

		c.GetRooms()
		log.Println(c.rooms)
	} else {
		log.Println("Something went wrong logging in. Check your username and password.")
	}
}
