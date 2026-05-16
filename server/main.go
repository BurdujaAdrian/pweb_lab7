package main

import (
	"encoding/json"
	"fmt"
	"log"
	"maps"
	"net/http"
	"slices"
	"strconv"
	"sync"
	"time"
)

const auth = false

func todo(msg string) {
	log.Print("TODO: ", msg)
}

type ActionResult struct {
	Failed   bool `json:"failed"`
	Blocked  bool `json:"blocked"`
	Defended bool `json:"defended"`
}
type GameAction struct {
	selected  Card
	is_attack bool
	failed    bool
}
type Player struct {
	Mutex     *sync.Mutex `json:"-"`
	Name      string
	gamestate *Gamestate
}

type Room struct {
	Host     Player
	Guest    Player
	host_ch  chan bool
	guest_ch chan bool
}

var store struct {
	Room_mutex   *sync.RWMutex
	Rooms        map[int]Room
	SRoom_mutex  *sync.RWMutex
	Started_room map[int]Room
	Counter      int
}

func main() {
	Populate_cards_index()
	Populate_default_deck()

	store.Rooms = make(map[int]Room)
	store.Started_room = make(map[int]Room)
	store.Room_mutex = new(sync.RWMutex)
	store.SRoom_mutex = new(sync.RWMutex)

	// serve the frontend
	http.Handle("/", http.FileServer(http.Dir("../docs/")))

	http.HandleFunc("POST /token", Token)

	// TODO: add endpoint to fetch gamestate maybe

	// register actions/commands from players
	http.HandleFunc("POST /action/{room_id}/{role}", func(w http.ResponseWriter, r *http.Request) {
		// step 0: retrieve room information
		room_id_string := r.PathValue("room_id")
		room_id, err := strconv.Atoi(room_id_string)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, "Room_id must be a valid integer")
			return
		}

		role := r.PathValue("role")

		var action Action
		err = json.NewDecoder(r.Body).Decode(&action)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "Bad json body, parsing error: %v", err)
			return
		}

		var room Room
		store.Room_mutex.RLock()
		{
			room = store.Rooms[room_id]
		}
		store.Room_mutex.RUnlock()
		// endof step 0

		// step 1: retrive player information
		var is_host bool = false
		var player Player
		var opponent Player
		switch role {
		case "HOST":
			is_host = true
			player = room.Host
			opponent = room.Guest
		case "GUEST":
			player = room.Guest
			opponent = room.Host
		default:
			{
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprintf(w, "Role [%v] doesn't exist, use [host] or [guest]", role)
				return
			}
		}

		if auth { // verify player id via jwt
			jwt_claims := ExtractClaims(r)
			if jwt_claims == nil {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			if !Verify(jwt_claims, role, player.Name, room_id_string) {
				w.WriteHeader(http.StatusForbidden)
				return
			}

			// else, free to go
		}
		// endof step 1

		// step 2: execute active player action
		var selected_card Card
		var is_attack, failed bool

		player.Mutex.Lock()
		{
			selected_card, is_attack, failed = Execute_action(player.gamestate, action)
		}
		player.Mutex.Unlock()
		// endof step 2

		// step 2a: if failed(because tried to run away) ff to step 5 and let player try again
		if failed {

			// step 5: package the new gamestate and send it
			type Result struct {
				New_gamestate GameRepr     `json:"new_gamestate"`
				Op_gamestate  OpGameRepr   `json:"opponent_gamestate"`
				Action_result ActionResult `json:"action_result"`
			}

			result := Result{}
			result.Action_result = ActionResult{failed, false, false}
			result.New_gamestate = format_gamestate(player.gamestate)
			// endof step 5

			// step 6: send the result
			json.NewEncoder(w).Encode(result)
			// endof step 6

			return
		}
		// endof step 2a

		// step 3: attack defending player
		var blocked, defended bool

		if is_attack {
			opponent.Mutex.Lock()
			{
				blocked, defended = attack(opponent.gamestate, selected_card)
			}
			opponent.Mutex.Unlock()
		}
		// endof step 3

		// step 4 : synchronise turn
		timeout := time.After(30 * time.Second)

		if is_host {
			// step 4a1: Host resolved their attack, blocks till guest responds
			select {
			case <-room.host_ch:
			case <-timeout:
				w.WriteHeader(http.StatusGatewayTimeout)
				return
			}
			// endof step 4a1

			// step 4a2: Host notifies guest that it recieved messege
			select {
			case room.guest_ch <- true:
			case <-timeout:
				w.WriteHeader(http.StatusGatewayTimeout)
				return
			}
			// endof step 4a2

		} else {
			// step 4b1: Guest resolved their attack, notify host of that
			select {
			case room.host_ch <- true:
			case <-timeout:
				w.WriteHeader(http.StatusGatewayTimeout)
				return
			}
			// endof step 4b1

			// step 4b2: Recieve the acknowledgement from host
			select {
			case <-room.guest_ch:
			case <-timeout:
				w.WriteHeader(http.StatusGatewayTimeout)
				return
			}
			// endof step 4b2
		}
		// endof step 4

		// step 5: package the new gamestate and send it
		type Result struct {
			New_gamestate GameRepr     `json:"new_gamestate"`
			Action_result ActionResult `json:"action_result"`
		}

		result := Result{}
		result.Action_result = ActionResult{failed, blocked, defended}
		result.New_gamestate = format_gamestate(player.gamestate)
		// endof step 5

		// step 6: send the result
		json.NewEncoder(w).Encode(result)
		// endof step 6
	})

	// TODO: Create an endpoint to start a game in a certain room

	// create a new room with self as the host, returns room id as response on OK
	http.HandleFunc("POST /room/{host}", func(w http.ResponseWriter, r *http.Request) {
		// TODO: guard against double post
		host := r.PathValue("host")

		defer log.Printf("(/%v %v) room/%v", r.Method, r.Host, host)

		if auth { // verify jwt token
			jwt_claims := ExtractClaims(r)
			if jwt_claims == nil {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			if VerifyAdmin(jwt_claims) {
				goto auth_end
			}
			if !Verify(jwt_claims, "PLAYER", host, "0") {
				w.WriteHeader(http.StatusForbidden)
				return
			}

			// else, free to go
		}
	auth_end:

		if host == "" {
			w.WriteHeader(http.StatusBadRequest)

			fmt.Fprint(w, "Empty username not allowed when craeting new room")
			return
		}

		new_room := Room{
			Player{},
			Player{},
			make(chan bool),
			make(chan bool),
		}
		new_room.Host.Name = host
		new_room.Host.gamestate = new(Gamestate)
		new_room.Host.Mutex = new(sync.Mutex)

		var new_room_id int

		store.Room_mutex.Lock()
		{
			store.Rooms[store.Counter] = new_room
			new_room_id = store.Counter
			store.Counter += 1
		}
		store.Room_mutex.Unlock()

		fmt.Fprintf(w, `{"room_id": %v}`, new_room_id)
	})

	// Get an update on the room
	http.HandleFunc("GET /room/{id}", func(w http.ResponseWriter, r *http.Request) {
		id_string := r.PathValue("id")
		id, err := strconv.Atoi(id_string)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, "Error parsing room id:", err)
			log.Print("Error parsing room id:", err)
			return
		}

		defer log.Printf("(/%v %v) /room/%v", r.Method, r.Host, id)

		var room Room
		store.Room_mutex.RLock()
		{
			room = store.Rooms[id]
		}
		store.Room_mutex.RUnlock()

		if auth { // verify jwt token
			jwt_claims := ExtractClaims(r)
			if jwt_claims == nil {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			if VerifyAdmin(jwt_claims) {
				goto auth_end
			}

			if !Verify(jwt_claims, "HOST", room.Host.Name, id_string) &&
				!Verify(jwt_claims, "GUEST", room.Guest.Name, id_string) {
				w.WriteHeader(http.StatusForbidden)
				return
			}

		}

	auth_end:
		store.Room_mutex.RLock()
		Rooms_json, _ := json.Marshal(store.Rooms[id])
		store.Room_mutex.RUnlock()

		w.Write(Rooms_json)
	})
	// get list of all rooms as a list [{host: <s>, guest:<s>}, ...] as json
	http.HandleFunc("GET /room", func(w http.ResponseWriter, r *http.Request) {
		defer log.Printf("(/%v %v) /room", r.Method, r.Host)

		if auth { // verify jwt token
			jwt_claims := ExtractClaims(r)
			if jwt_claims == nil {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			role, exists := jwt_claims["role"]
			if !exists {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			if role != "VIEWER" && role != "ADMIN" {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			// else, free to go
		}
		store.Room_mutex.RLock()
		Rooms_json, _ := json.Marshal(slices.Collect(maps.Values(store.Rooms)))
		store.Room_mutex.RUnlock()

		w.Write(Rooms_json)
	})

	// join a room, returns { room_id : <int> } json as response on OK
	http.HandleFunc("PATCH /room/{guest}/{id}", func(w http.ResponseWriter, r *http.Request) {
		guest := r.PathValue("guest")
		id_string := r.PathValue("id")
		id, err := strconv.Atoi(id_string)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, "Error parsing room id:", err)
			log.Print("Error parsing room id:", err)
			return
		}

		defer log.Printf("(/%v %v) /room/%v/%v", r.Method, r.Host, guest, id)

		if auth { // verify jwt token
			jwt_claims := ExtractClaims(r)
			if jwt_claims == nil {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			if VerifyAdmin(jwt_claims) {
				goto end_auth
			}

			if !Verify(jwt_claims, "PLAYER", guest, id_string) {
				w.WriteHeader(http.StatusForbidden)
				return
			}
		}

	end_auth:
		store.Room_mutex.Lock()
		defer store.Room_mutex.Unlock()

		target_room, exists := store.Rooms[id]
		if !exists {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		reset(target_room.Guest.gamestate)
		reset(target_room.Host.gamestate)

		target_room.Guest = Player{new(sync.Mutex), guest, new(Gamestate)}
		store.Rooms[id] = target_room

		target_room_json, _ := json.Marshal(target_room)

		w.Write(target_room_json)

	})

	http.HandleFunc("DELETE /room/{id}", func(w http.ResponseWriter, r *http.Request) {
		id_string := r.PathValue("id")
		id, err := strconv.Atoi(id_string)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, "Error parsing room id:", err)
			log.Print("Error parsing room id:", err)
			return
		}
		defer log.Printf("(/%v %v) /room/%v", r.Method, r.Host, id)

		var old_room Room
		var exists bool
		store.Room_mutex.RLock()
		{
			old_room, exists = store.Rooms[id]
		}
		store.Room_mutex.RUnlock()
		if !exists {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		var is_admin bool
		if auth { // verify jwt token and extract info

			jwt_claims := ExtractClaims(r)
			if jwt_claims == nil {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			is_admin = VerifyAdmin(jwt_claims)
			if is_admin {
				goto delete_room
			}

			if !Verify(jwt_claims, "HOST", old_room.Host.Name, id_string) {
				w.WriteHeader(http.StatusForbidden)
				return
			}

		}

	delete_room:
		store.Room_mutex.Lock()
		delete(store.Rooms, id)
		store.Room_mutex.Unlock()

		old_room_json, _ := json.Marshal(old_room)
		w.Write(old_room_json)

	})

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("(/%v %v) Health", r.Method, r.Host)
		w.WriteHeader(http.StatusOK)
	})

	http.ListenAndServe(":8080", nil)
}
