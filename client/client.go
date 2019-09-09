package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"
	"reflect"
	"strconv"
	"strings"
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
	conn     net.Conn
	proto    proto.Proto
	rooms    []proto.Room
	curRoom  string
	curChan  string
	loggedIn bool
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
			c.loggedIn = true
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

//UpdateRooms - updates the object's rooms struct value by using the return value of GetRooms
func (c *Client) UpdateRooms() {
	c.rooms = c.GetRooms()
}

//GetRooms - replaces the list of rooms in the object with what the database says
func (c Client) GetRooms() []proto.Room {
	err := c.proto.SendGetRoomsRequest()
	c.check(err)
	var ret proto.GetRoomsResponse
	switch msg := c.proto.Decode().(type) {
	case proto.GetRoomsResponse:
		ret = msg
		if ret.Code == 200 {
			//We got a good response...
			//c.rooms = msg.Rooms
		} else {
			log.Println("Not updating the rooms because we got a bad return code...")
		}
	default:
		r := reflect.TypeOf(msg)
		fmt.Printf("Unexpected type:%v\n", r)
		os.Exit(1)
	}
	return ret.Rooms
}

//PrintRooms - Nicely prints all the rooms provided
func (c Client) PrintRooms(rooms []proto.Room) {
	if !c.loggedIn {
		fmt.Println("You are not logged in")
	}
	for i := 0; i < len(rooms); i++ {
		fmt.Println("Room index:", i)
		fmt.Println("\tID:", rooms[i].ID)
		fmt.Println("\tName:", rooms[i].Name)
		fmt.Println("\tDisplay Name:", rooms[i].DisplayName)
		fmt.Println("\tChannels:")
		for j := 0; j < len(rooms[i].Channels); j++ {
			fmt.Println("\t\tID:", rooms[i].Channels[j].ID)
			fmt.Println("\t\tName:", rooms[i].Channels[j].Name)
		}
	}
}

//HandleUserInput - A dumb CLI to interface with the program
func (c *Client) HandleUserInput() {
	reader := bufio.NewReader(os.Stdin)
	for {
		prompt := "$"
		if c.curRoom != "" {
			prompt += "(" + c.curRoom
		}
		if c.curChan != "" {
			prompt += ":" + c.curChan
		}
		if c.curRoom != "" || c.curChan != "" {
			prompt += ")"
		}

		fmt.Print(prompt)
		action, err := reader.ReadString('\n')
		action = strings.TrimRight(action, "\r\n")
		c.check(err)
		if action == "show rooms" {
			c.UpdateRooms()
			c.PrintRooms(c.rooms)
		} else if action == "help" {
			fmt.Println("help:\tthis screen")
			fmt.Println("show rooms:\tprint rooms")
			fmt.Println("use room <ID>:\tswitches focus to a specific room")
			fmt.Println("use channel <ID>:\tswitches to a specific channel")
			fmt.Println("join room <name,passwd>:\tlinks your account with a room")
			fmt.Println("create room <name,passwd>:\tcreate a new room. automatically adds you as an admin and creates a default channel")
		} else if action == "login" {
			if c.Login("Justin", "poop") == 200 {
				c.UpdateRooms()
			} else {
				fmt.Println("Something went wrong when logging in")
			}
		} else if len(strings.Fields(action)) > 2 && strings.Fields(action)[0] == "use" && strings.Fields(action)[1] == "room" {
			c.curRoom = strings.Fields(action)[2]
			c.check(err)
		} else if len(strings.Fields(action)) > 2 && strings.Fields(action)[0] == "use" && strings.Fields(action)[1] == "channel" {
			if c.curRoom == "" {
				fmt.Println("You must set a room before you can set a channel")
			} else {
				c.curChan = strings.Fields(action)[2]
				c.check(err)
			}
		} else if len(strings.Fields(action)) > 3 && strings.Fields(action)[0] == "create" && strings.Fields(action)[1] == "room" {
			rname := strings.Fields(action)[2]
			rpass := strings.Fields(action)[3]
			c.CreateRoom(rname, rpass)
		} else if len(strings.Fields(action)) > 3 && strings.Fields(action)[0] == "join" && strings.Fields(action)[1] == "room" {
			jname := strings.Fields(action)[2]
			jpass := strings.Fields(action)[3]
			c.JoinRoom(jname, jpass)
		}
	}

}

func main() {
	c := new(Client)
	c.Init("localhost", 1200)
	//username, password := c.GetCredentials()
	c.HandleUserInput()
}
