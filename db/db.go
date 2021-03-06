package termtexterdb

import (
	"database/sql"
	"fmt"
	"log"

	"golang.org/x/crypto/bcrypt"

	//because I need the driver
	proto "termtexter/proto"

	_ "github.com/go-sql-driver/mysql"
)

//User object
type User struct {
	UserID      int
	Username    string
	Displayname string
	Password    string
}

//DB is an object that will abstract the db stuff into nice methods
type DB struct {
	dbh *sql.DB
}

func check(e error) {
	if e != nil {
		panic(e)
	}
}

//Connect gets the user record from the database
func (d *DB) Connect(hostname string, password string) error {
	// connect to the database
	var err error
	d.dbh, err = sql.Open("mysql", "termtexter:"+password+"@tcp("+hostname+")/termtexter?parseTime=true&loc=America%2FNew_York")
	return err
}

//GetUserIDFromKey - given a login session key, get the userID associated with it
func (d DB) GetUserIDFromKey(key string) (string, error) {
	rows, err := d.dbh.Query("select u.user_id from users u join sessions s on u.user_id = s.user_id where `key` = ?", key)
	check(err)
	defer rows.Close()
	rows.Next()
	var u string
	err = rows.Scan(&u)
	return u, err
}

//DoesRoomExist - returns the room number if it exists
func (d DB) DoesRoomExist(rid string) (int, error) {
	rows, err := d.dbh.Query("select room_id from rooms where name = ?", rid)
	check(err)
	defer rows.Close()
	i := -1
	rows.Next()
	err = rows.Scan(&i)
	return i, err
}

//PostMessage -
func (d *DB) PostMessage(id string, pm proto.PostMessageRequest) (int64, error) {
	v, err := d.dbh.Exec("insert into messages (user_id,channel_id,message) values (?,?,?)", id, pm.Channel, pm.Message)
	check(err)
	i, err := v.LastInsertId()
	return i, err
}

//GetMessages -
func (d DB) GetMessages(room int, channel int) ([]*proto.Message, error) {
	rows, err := d.dbh.Query(`select m.message_id, m.user_id, m.message, m.created, m.received from messages m join channels c
	on m.channel_id = c.channel_id join rooms r on r.room_id = c.room_id where r.room_id = ? and c.channel_id = ? order by m.created`, room, channel)
	check(err)
	defer rows.Close()

	messages := make([]*proto.Message, 0)
	for rows.Next() {
		message := proto.Message{}
		rows.Scan(&message.ID, &message.UserID, &message.Message, &message.Created, &message.Received)
		messages = append(messages, &message)
		log.Println(message)
	}

	return messages, err
}

//GetRooms -
func (d DB) GetRooms(uid string) (map[int]*proto.Room, error) {
	rows, err := d.dbh.Query(`select r.room_id, r.name, r.displayname from rooms r join room_users ru on r.room_id = ru.room_id 
	join users u on u.user_id = ru.user_id where u.user_id = ?`, uid)
	check(err)
	defer rows.Close()
	rooms := make(map[int]*proto.Room)

	for rows.Next() {
		room := proto.Room{}
		rows.Scan(&room.ID, &room.Name, &room.DisplayName)
		rooms[room.ID] = &room
	}

	for k := range rooms {

		//Channels
		rows, err = d.dbh.Query("select c.channel_id, c.name from channels c join rooms r on c.room_id = r.room_id where c.room_id = ?", k)
		check(err)
		for rows.Next() {
			channel := proto.Channel{}
			rows.Scan(&channel.ID, &channel.Name)
			rooms[k].Channels = make(map[int]*proto.Channel)
			rooms[k].Channels[channel.ID] = &channel
		}

		//Users
		rows, err = d.dbh.Query("select u.user_id,u.username,u.created,u.displayname from users u join room_users ru on u.user_id = ru.user_id join rooms r on ru.room_id = r.room_id where r.room_id = ?", k)
		check(err)
		for rows.Next() {
			user := proto.User{}
			rows.Scan(&user.ID, &user.UserName, &user.Created, &user.DisplayName)
			rooms[k].Users = make(map[int]*proto.User)
			rooms[k].Users[user.ID] = &user
		}
	}

	return rooms, err
}

//AddUserToRoom - given an id, add this id into the mapping table
func (d DB) AddUserToRoom(uid string, rid int) error {
	_, err := d.dbh.Exec("insert into room_users (room_id,user_id) values (?,?)", rid, uid)
	return err
}

//CreateRoom - Create a room, set user as admin, and build a default first channel
func (d DB) CreateRoom(rid string, uid string, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	//Handle this as a transaction since we're doing a few changes here
	t, err := d.dbh.Begin()
	defer t.Rollback()
	//Start by creating the room
	res, err := t.Exec("insert into rooms (name,password) values (?,?)", rid, hash)
	check(err)
	dbRoomID, err := res.LastInsertId()
	check(err)
	//Add the requesting user to this room as an admin
	res, err = t.Exec("insert into room_users (room_id,user_id,admin) values (?,?,?)", dbRoomID, uid, 1)
	check(err)
	//Create a channel for this room, with the name of "general"
	res, err = t.Exec("insert into channels (room_id,name) values (?,?)", dbRoomID, "general")
	check(err)
	fmt.Println(res)
	t.Commit()
	return err
}

//GetUserID gets the user id from the database if it exists
func (d DB) GetUserID(username string) (string, error) {
	rows, err := d.dbh.Query("select user_id from users where username = ?", username)
	check(err)
	defer rows.Close()
	rows.Next()
	var u string
	err = rows.Scan(&u)
	return u, err
}

//GetUser gets the user record from the database
func (d DB) GetUser(username string) (User, error) {
	rows, err := d.dbh.Query("select user_id, username, displayname, password from users where username = ?", username)
	check(err)
	defer rows.Close()
	rows.Next()
	var u User
	err = rows.Scan(&u.UserID, &u.Username, &u.Displayname, &u.Password)
	return u, err
}

//Register - Register's a new user
func (d DB) Register(username string, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	_, err = d.dbh.Exec("insert into users (username,password) values (?,?)", username, string(hash))
	return err
}

//UserExists will return a bool if the user is registered
func (d DB) UserExists(username string) (bool, error) {
	rows, err := d.dbh.Query("select 1 from users where username = ?", username)
	check(err)
	defer rows.Close()
	c := 0
	for rows.Next() {
		c++
	}
	var b bool
	if c > 0 {
		b = true
	} else {
		b = false
	}
	return b, err
}

//IsValidLogin will determine if the login was valid. Pass in a plain text password
func (d DB) IsValidLogin(uid string, password string) bool {
	rows, err := d.dbh.Query("select password from users where user_id = ?", uid)
	check(err)
	defer rows.Close()
	rows.Next()
	var epassword string
	err = rows.Scan(&epassword)
	check(err)
	res := bcrypt.CompareHashAndPassword([]byte(epassword), []byte(password))
	if res == nil {
		return true
	} else {
		log.Println(res)
	}
	return false
}

//AddSession inserts the uuid we're handing to this client over to the user
func (d *DB) AddSession(uid, uuid string) error {
	res, err := d.dbh.Exec("insert into sessions (user_id,`key`) values (?,?)", uid, uuid)
	fmt.Println(res)
	return err
}
