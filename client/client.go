package main

import (
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	proto "termtexter/proto"

	"github.com/gdamore/tcell"
	"github.com/rivo/tview"
)

const (
	HTTP_OK          = 200
	HTTP_FORBIDDEN   = 403
	HTTP_BADREQUEST  = 400
	HTTP_ERROR       = 500
	HTTP_UNAVAILABLE = 503
)

type channels struct {
	getMessagesResponse chan proto.GetMessagesResponse
	getRoomsResponse    chan proto.GetRoomsResponse
	joinRoomResponse    chan proto.JoinRoomResponse
	registerResponse    chan proto.RegisterResponse
	createRoomResponse  chan proto.CreateRoomResponse
	postMessageResponse chan proto.PostMessageResponse
	loginResponse       chan proto.LoginResponse
	dynamicMessage      chan proto.DynamicMessage
}

//Client - client struct
type Client struct {
	conn     net.Conn
	proto    proto.Proto
	rooms    map[int]*proto.Room
	curRoom  int
	curChan  int
	loggedIn bool
	channels channels
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

	//allocate memory for the channels
	c.channels.createRoomResponse = make(chan proto.CreateRoomResponse)
	c.channels.getMessagesResponse = make(chan proto.GetMessagesResponse)
	c.channels.getRoomsResponse = make(chan proto.GetRoomsResponse)
	c.channels.joinRoomResponse = make(chan proto.JoinRoomResponse)
	c.channels.loginResponse = make(chan proto.LoginResponse)
	c.channels.postMessageResponse = make(chan proto.PostMessageResponse)
	c.channels.registerResponse = make(chan proto.RegisterResponse)
	c.channels.dynamicMessage = make(chan proto.DynamicMessage)
	//listens for incoming packets and sends to the proper channels
	go c.packetListener()
}

func (c *Client) messageHandler(chat *tview.TextView) {
	for {
		//block waiting for new dynamic messages
		msg := <-c.channels.dynamicMessage

		m := proto.Message{}
		m.Created = msg.Created
		m.ID = msg.ID
		m.Message = msg.Message
		m.Received = msg.Received
		m.Timestamp = msg.Timestamp
		m.Type = msg.Type
		m.UserID = msg.UserID

		c.rooms[msg.Room].Channels[msg.Channel].Messages = append(c.rooms[msg.Room].Channels[msg.Channel].Messages, &m)
		//TODO: do this if we're viewing the current chat, otherwise bold the channel this message would be in
		if msg.Room == c.curRoom && msg.Channel == c.curChan {
			chat.SetText(chat.GetText(true) + c.buildMessage(msg.Created.String(), c.rooms[c.curRoom].Users[msg.UserID].DisplayName, msg.Message))
		} else {

		}
	}
}

//GetRegistrationResponse - see what the response is
func (c Client) GetRegistrationResponse() proto.RegisterResponse {
	var ret proto.RegisterResponse
	msg := <-c.channels.registerResponse
	ret = msg
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
	msg := <-c.channels.createRoomResponse

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

	return res.Code
}

//JoinRoom - joins a room. This is a one time per account operation, just to link your account to a room (if you know the name and password). Returns success
func (c *Client) JoinRoom(name string, password string) bool {
	err := c.proto.SendJoinRoom(name, password)
	c.check(err)
	var res proto.JoinRoomResponse
	ret := false
	msg := <-c.channels.joinRoomResponse

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

	return ret
}

//Login - Logs user in and returns http code
func (c *Client) Login(username string, password string) int {
	err := c.proto.SendLogin(username, password)
	c.check(err)
	var ret proto.LoginResponse
	msg := <-c.channels.loginResponse
	ret = msg
	if ret.Code == 200 {
		//Set our proto's session key
		c.proto.SetKey(ret.Key)
		c.loggedIn = true
	} else {
		log.Println("Not setting the session key because we got a bad return code...")
	}
	return ret.Code
}

//UpdateRooms - updates the object's rooms struct value by using the return value of GetRooms
func (c *Client) UpdateRooms() {
	c.rooms = c.GetRooms()
}

//GetRooms - replaces the list of rooms in the object with what the database says
func (c Client) GetRooms() map[int]*proto.Room {
	err := c.proto.SendGetRoomsRequest()
	c.check(err)
	var ret proto.GetRoomsResponse
	msg := <-c.channels.getRoomsResponse
	ret = msg
	if ret.Code == 200 {
		//We got a good response...
		//c.rooms = msg.Rooms
	} else {
		log.Println("Not updating the rooms because we got a bad return code...")
	}

	return ret.Rooms
}

//PrintRooms - Nicely prints all the rooms provided
func (c Client) PrintRooms() {
	if !c.loggedIn {
		fmt.Println("You are not logged in")
	}
	for _, v := range c.rooms {
		fmt.Println("\tID:", v.ID) //could also use k
		fmt.Println("\tName:", v.Name)
		fmt.Println("\tDisplay Name:", v.DisplayName)
		fmt.Println("\tChannels:")
		for _, v2 := range v.Channels {
			fmt.Println("\t\tID:", v2.ID)
			fmt.Println("\t\tName:", v2.Name)
		}
	}
}

func (c *Client) sendMessage(msg string, room int, channel int) error {
	if room == -1 || channel == -1 {
		fmt.Println("Please set your channel and room before sending a message.")
		return nil
	}
	err := c.proto.SendPostMessageRequest(msg, room, channel)
	c.check(err)
	var ret proto.PostMessageResponse
	rmsg := <-c.channels.postMessageResponse

	ret = rmsg
	if ret.Code == 200 {
		//We got a good response...
	} else {
		log.Println("We got a bad return code...")
	}

	return err
}

//UpdateMessages - Queries the database and gets the last N messages from the DB for the channel we are currently on
func (c *Client) UpdateMessages() {
	c.rooms[c.curRoom].Channels[c.curChan].Messages = c.GetMessages(c.curRoom, c.curChan)
}

//GetMessages - Queries the database and gets the last N messages from the DB for the channel we are currently on
func (c *Client) GetMessages(room int, channel int) []*proto.Message {
	if room == -1 || channel == -1 {
		fmt.Println("Please set your channel and room before requesting messages.")
		empty := make([]*proto.Message, 0)
		return empty
	}
	err := c.proto.SendGetMessagesRequest(room, channel)
	c.check(err)
	var ret proto.GetMessagesResponse
	msg := <-c.channels.getMessagesResponse

	ret = msg
	if ret.Code == 200 {
		//We got a good response...
		//c.rooms = msg.Rooms
	} else {
		log.Println("Not updating the rooms because we got a bad return code...")
	}

	return ret.Messages
}

func roomTree(c *Client) *tview.TreeView {
	root := tview.NewTreeNode("Rooms").SetColor(tcell.ColorRed)
	tree := tview.NewTreeView().SetRoot(root).SetCurrentNode(root)

	for _, v := range c.rooms {
		node := tview.NewTreeNode(v.DisplayName).SetColor(tcell.ColorGreen)
		for _, v2 := range v.Channels {
			node2 := tview.NewTreeNode(v2.Name).SetColor(tcell.ColorGray)
			node.AddChild(node2)
		}
		root.AddChild(node)
	}

	return tree
}

func loginPage(app *tview.Application, pages *tview.Pages, c *Client) *tview.Grid {
	form := tview.NewGrid().SetColumns(0, 20, 0).SetRows(0, 0, 0).AddItem(tview.NewForm().
		AddInputField("Username", "", 20, nil, nil).
		AddPasswordField("Password", "", 10, '*', nil).
		AddButton("Login", func() {
			pages.SwitchToPage("main")
		}).
		AddButton("Quit", func() {
			app.Stop()
		}).AddCheckbox("Remember", false, nil), 1, 1, 1, 1, 0, 0, true)
	form.SetBorder(true).SetTitle("termtexter").SetTitleAlign(tview.AlignCenter).SetTitleColor(tcell.ColorLimeGreen)
	return form
}

func (c *Client) buildMessage(date string, dispname string, msg string) string {
	return date + " - " + dispname + " <" + msg + ">\n"
}

func (c *Client) mainPage(app *tview.Application, pages *tview.Pages) (*tview.Flex, *tview.TreeView, *tview.TextView) {

	//data for the rooms
	rooms := roomTree(c)
	rooms.SetBorder(true).SetTitle("Rooms")

	//data for the chat window
	c.UpdateMessages()
	messages := ""
	log.Println(c.rooms[c.curRoom].Channels[c.curChan].Messages)
	for _, v := range c.rooms[c.curRoom].Channels[c.curChan].Messages {
		messages += c.buildMessage(v.Created.String(), c.rooms[c.curRoom].Users[v.UserID].DisplayName, v.Message)
	}

	chat := tview.NewTextView().SetScrollable(true).ScrollToEnd().SetText(strings.Repeat("\n", 1000) + messages)
	chat.SetBorder(true).SetTitle("Chat")
	//handles new messages that are dynamically sent in
	go c.messageHandler(chat)

	//chatbox
	chatbox := tview.NewInputField()
	chatbox.SetBorder(true).SetTitle("Chatbox")

	//users
	users := tview.NewList()
	for _, v := range c.rooms[c.curRoom].Users {
		users.AddItem(v.DisplayName, "", '+', nil)
	}
	users.SetBorder(true).SetTitle("Users")

	//flex for the page
	flex := tview.NewFlex().
		AddItem(rooms, 20, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			//AddItem(tview.NewBox().SetBorder(true).SetTitle("Top"), 0, 1, false).
			AddItem(chat, 0, 3, false).
			AddItem(chatbox, 3, 1, false), 0, 2, false).
		AddItem(users, 20, 1, false)

	chat.SetTitleColor(tcell.ColorRed)

	rooms.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		var ret *tcell.EventKey
		ret = nil
		if event.Key() == tcell.KeyRight {
			rooms.SetTitleColor(tcell.ColorWhite)
			chat.SetTitleColor(tcell.ColorRed)
			app.SetFocus(chat)
		} else {
			ret = event
		}
		return ret
	})

	chat.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		var ret *tcell.EventKey
		ret = nil
		if event.Key() == tcell.KeyLeft {
			chat.SetTitleColor(tcell.ColorWhite)
			rooms.SetTitleColor(tcell.ColorRed)
			app.SetFocus(rooms)
			ret = event
		} else if event.Key() == tcell.KeyDown {
			chat.SetTitleColor(tcell.ColorWhite)
			chatbox.SetTitleColor(tcell.ColorRed)
			app.SetFocus(chatbox)
		} else if event.Key() == tcell.KeyRight {
			chat.SetTitleColor(tcell.ColorWhite)
			users.SetTitleColor(tcell.ColorRed)
			app.SetFocus(users)
			ret = event
		} else if event.Key() == tcell.KeyPgUp {
			ret = tcell.NewEventKey(tcell.KeyUp, event.Rune(), event.Modifiers())
		} else if event.Key() == tcell.KeyPgDn {
			ret = tcell.NewEventKey(tcell.KeyDown, event.Rune(), event.Modifiers())
		}
		return ret
	})

	users.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		var ret *tcell.EventKey
		ret = nil
		if event.Key() == tcell.KeyLeft {
			users.SetTitleColor(tcell.ColorWhite)
			chat.SetTitleColor(tcell.ColorRed)
			app.SetFocus(chat)
		} else {
			ret = event
		}
		return ret
	})

	chatbox.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		var ret *tcell.EventKey
		ret = nil
		if event.Key() == tcell.KeyUp {
			chatbox.SetTitleColor(tcell.ColorWhite)
			chat.SetTitleColor(tcell.ColorRed)
			app.SetFocus(chat)
		} else if event.Key() == tcell.KeyEnter {
			//We want to send a message to the server on an enter
			err := c.sendMessage(chatbox.GetText(), c.curRoom, c.curChan)
			c.check(err)
			//chat.SetText(chat.GetText(true) + chatbox.GetText() + "\n")
			chatbox.SetText("")
		} else {
			ret = event
		}
		return ret
	})

	return flex, rooms, chat
}

//packetListener - one go routine that listens on the socket and sends the appropriate object to the appropriate channel
func (c *Client) packetListener() {
	for {
		switch msg := c.proto.Decode().(type) {
		case proto.GetMessagesResponse:
			c.channels.getMessagesResponse <- msg
		case proto.GetRoomsResponse:
			c.channels.getRoomsResponse <- msg
		case proto.JoinRoomResponse:
			c.channels.joinRoomResponse <- msg
		case proto.RegisterResponse:
			c.channels.registerResponse <- msg
		case proto.CreateRoomResponse:
			c.channels.createRoomResponse <- msg
		case proto.PostMessageResponse:
			c.channels.postMessageResponse <- msg
		case proto.LoginResponse:
			c.channels.loginResponse <- msg
		case proto.DynamicMessage:
			c.channels.dynamicMessage <- msg
		default:
			log.Println("I don't know what I just got")
			log.Println(msg)
		}
	}
}

func main() {

	c := new(Client)
	c.curRoom = 3
	c.curChan = 1
	c.Init("localhost", 1200)

	c.Login("justin", "poop")
	c.UpdateRooms()

	app := tview.NewApplication()
	pages := tview.NewPages()

	flex, _, chat := c.mainPage(app, pages)

	pages.AddPage("main", flex, true, true)
	pages.AddPage("login", loginPage(app, pages, c), true, false)

	if err := app.SetRoot(pages, true).SetFocus(chat).Run(); err != nil {
		panic(err)
	}
}
