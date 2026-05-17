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

const AUTH = false

func todo(msg string) {
	log.Print("TODO: ", msg)
	panic("")
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
	Host      Player
	Guest     Player
	quit      chan struct{}
	quit_once sync.Once
	host_ch   chan Card
	guest_ch  chan Card
}

var store struct {
	Room_mutex   *sync.RWMutex
	Rooms        map[int]*Room
	SRoom_mutex  *sync.RWMutex
	Started_room map[int]*Room
	Counter      int
}

func main() {
	todo("Check game wincon")
	Populate_cards_index()
	Populate_default_deck()

	store.Rooms = make(map[int]*Room)
	store.Started_room = make(map[int]*Room)
	store.Room_mutex = new(sync.RWMutex)
	store.SRoom_mutex = new(sync.RWMutex)

	// serve the frontend
	http.Handle("/", http.FileServer(http.Dir("../docs/")))

	http.HandleFunc("POST /token", Token)

	/*
		Game related endpoints
	*/
	{
		type Result struct {
			New_gamestate GameRepr     `json:"new_gamestate"`
			Op_gamestate  OpGameRepr   `json:"op_gamestate"`
			Action_result ActionResult `json:"action_result"`
		}
		// end/leave a game
		http.HandleFunc("DELETE /game/{role}/{id}", func(w http.ResponseWriter, r *http.Request) {

			role := r.PathValue("role")

			id_string := r.PathValue("id")
			id, err := strconv.Atoi(id_string)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprint(w, "Error parsing room id:", err)
				log.Print("Error parsing room id:", err)
				return
			}

			defer log.Printf("(/%v %v) game/%v/%v", r.Method, r.Host, role, id)

			if AUTH {
				var room *Room
				var exists bool
				store.SRoom_mutex.RLock()
				{
					room, exists = store.Rooms[id]
				}
				store.SRoom_mutex.RUnlock()
				if !exists {
					w.WriteHeader(http.StatusNotFound)
					return
				}
				is_host := role == "HOST"

				jwt_claims := ExtractClaims(r)
				if jwt_claims == nil {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}

				if VerifyAdmin(jwt_claims) {
					goto auth_end
				}

				if is_host {
					if !Verify(jwt_claims, "HOST", room.Host.Name, id_string) {
						w.WriteHeader(http.StatusForbidden)
						return
					}
				} else {
					if !Verify(jwt_claims, "GUEST", room.Guest.Name, id_string) {
						w.WriteHeader(http.StatusForbidden)
						return
					}
				}
			}

		auth_end:

			store.SRoom_mutex.Lock()
			defer store.SRoom_mutex.Unlock()

			room, ok := store.Started_room[id]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}

			room.quit_once.Do(func() { close(room.quit) })

			delete(store.Started_room, id)

			w.WriteHeader(http.StatusOK)

		})
		// start a game
		http.HandleFunc("GET /game/start/{role}/{id}", func(w http.ResponseWriter, r *http.Request) {
			// step 0: get credentials
			role := r.PathValue("role")

			id_string := r.PathValue("id")
			id, err := strconv.Atoi(id_string)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprint(w, "Error parsing room id:", err)
				log.Print("Error parsing room id:", err)
				return
			}
			// endof step 0

			defer log.Printf("(/%v %v) game/start/%v/%v", r.Method, r.Host, role, id)

			// step 1: get information about the room
			var room *Room
			var exists bool
			store.SRoom_mutex.RLock()
			{
				room, exists = store.Started_room[id]
			}
			store.SRoom_mutex.RUnlock()

			// check if the room was actually there
			if !exists {
				w.WriteHeader(http.StatusNotFound)
				return
			}

			var player Player
			var is_host bool = false
			switch role {
			case "HOST":
				is_host, player = true, room.Host
			case "GUEST":
				player = room.Guest
			default:
				w.WriteHeader(http.StatusForbidden)
				return
			}
			// endof step 1

			// step 2: authorise
			if AUTH { // verify jwt token
				jwt_claims := ExtractClaims(r)
				if jwt_claims == nil {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}

				if is_host {
					if !Verify(jwt_claims, "HOST", room.Host.Name, id_string) {
						w.WriteHeader(http.StatusForbidden)
						return
					}
				} else {
					if !Verify(jwt_claims, "GUEST", room.Guest.Name, id_string) {
						w.WriteHeader(http.StatusForbidden)
						return
					}
				}

				// else, free to go
			}
			// endof step 2

			// step 3: wait for player randevouz, if timeout, abort operation
			timeout := time.After(30 * time.Second)

			if is_host {
				// step 3a1: Host resolved their attack, blocks till guest responds
				select {
				case <-room.host_ch:
				case <-room.quit:
					{
						w.WriteHeader(http.StatusGone)
						return
					}
				case <-timeout:
					w.WriteHeader(http.StatusGatewayTimeout)
					return
				}
				// endof step 3a1

				// step 3a2: Host notifies guest that it recieved messege
				select {
				case room.guest_ch <- Card{}:
				case <-room.quit:
					{
						w.WriteHeader(http.StatusGone)
						return
					}
				case <-timeout:
					w.WriteHeader(http.StatusGatewayTimeout)
					return
				}
				// endof step 3a2

			} else {
				// step 3b1: Guest resolved their attack, notify host of that
				select {
				case room.host_ch <- Card{}:
				case <-room.quit:
					{
						w.WriteHeader(http.StatusGone)
						return
					}
				case <-timeout:
					w.WriteHeader(http.StatusGatewayTimeout)
					return
				}
				// endof step 3b1

				// step 3b2: Recieve the acknowledgement from host
				select {
				case <-room.guest_ch:
				case <-room.quit:
					{
						w.WriteHeader(http.StatusGone)
						return
					}
				case <-timeout:
					w.WriteHeader(http.StatusGatewayTimeout)
					return
				}
				// endof step 3b2
			}
			// endof step 3

			// step 4: move the room in the started rooms section if the host sent the request
			if is_host {
				store.Room_mutex.Lock()
				{
					delete(store.Rooms, id)
				}
				store.Room_mutex.Unlock()

				store.SRoom_mutex.Lock()
				{
					store.Started_room[id] = room
				}
				store.SRoom_mutex.Unlock()
			}
			// endof step 4

			// step 5: initialise game state with default values
			reset(player.gamestate)
			result := Result{}
			result.Action_result = ActionResult{false, false, false}
			result.New_gamestate = format_gamestate(player.gamestate)
			result.Op_gamestate = OpGameRepr{
				Deck_remaining: len(player.gamestate.deck),
				Hp:             player.gamestate.hp,
				Weapon:         "",
				Played_card:    "",
			}
			// endof step 5

			// step 6: send both player current gamestate
			json.NewEncoder(w).Encode(result)
			// endof step 6
		})

		// register actions/commands from players and returns state of the game after action
		http.HandleFunc("POST /game/{role}/{room_id}", func(w http.ResponseWriter, r *http.Request) {
			// step 0: retrieve room information
			room_id_string := r.PathValue("room_id")
			room_id, err := strconv.Atoi(room_id_string)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprint(w, "Room_id must be a valid integer")
				return
			}
			role := r.PathValue("role")

			defer log.Printf("(/%v %v) game/%v/%v", r.Method, r.Host, role, room_id)

			var action Action
			err = json.NewDecoder(r.Body).Decode(&action)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprintf(w, "Bad json body, parsing error: %v", err)
				return
			}

			var room *Room
			var exists bool
			store.SRoom_mutex.RLock()
			{
				room, exists = store.Started_room[room_id]
			}
			store.SRoom_mutex.RUnlock()

			// theres a small chance the guest has sent an action before the room was moved to started rooms
			if !exists {
				w.WriteHeader(http.StatusNotFound)
				return
			}
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

			if AUTH { // verify player id via jwt
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

			// step 1a: verify the legality of player's action
			todo("verify that the action the player wants to do is legal before starting the double randezvous sequence")
			player.Mutex.Lock()
			{
				legal := is_move_legal(player.gamestate, action)
				if !legal {
					player.Mutex.Unlock()
					w.WriteHeader(http.StatusForbidden)
					return
				}
			}
			player.Mutex.Unlock()

			// step 2 : synchronise players actions; if timeout abort action
			timeout := time.After(30 * time.Second)

			if is_host {
				// step 2a1: Host waits for guest's messege
				select {
				case <-room.host_ch:
				case <-room.quit:
					{
						w.WriteHeader(http.StatusGone)
						return
					}
				case <-timeout:
					w.WriteHeader(http.StatusGatewayTimeout)
					return
				}
				// endof step 2a1

				// step 2a2: Host notifies guest that it recieved messege
				select {
				case room.guest_ch <- Card{}:
				case <-room.quit:
					{
						w.WriteHeader(http.StatusGone)
						return
					}
				case <-timeout:
					w.WriteHeader(http.StatusGatewayTimeout)
					return
				}
				// endof step 2a2

			} else {
				// step 2b1: Guest sends messege to host
				select {
				case room.host_ch <- Card{}:
				case <-room.quit:
					{
						w.WriteHeader(http.StatusGone)
						return
					}
				case <-timeout:
					w.WriteHeader(http.StatusGatewayTimeout)
					return
				}
				// endof step 2b1

				// step 2b2: Recieve the acknowledgement from host
				select {
				case <-room.guest_ch:
				case <-room.quit:
					{
						w.WriteHeader(http.StatusGone)
						return
					}
				case <-timeout:
					w.WriteHeader(http.StatusGatewayTimeout)
					return
				}
				// endof step 2b2
			}
			// endof step 2

			// step 3: execute active player action
			var selected_card Card
			var is_attack, failed bool

			player.Mutex.Lock()
			{
				selected_card, is_attack, _ = Execute_action(player.gamestate, action)
			}
			player.Mutex.Unlock()
			// endof step 3

			// step 4: attack defending player
			var blocked, defended bool

			if is_attack {
				opponent.Mutex.Lock()
				{
					blocked, defended = attack(opponent.gamestate, selected_card)
				}
				opponent.Mutex.Unlock()
			}
			// endof step 4

			// step 5 : synchronise turn end
			timeout = time.After(30 * time.Second)

			var op_card Card

			if is_host {
				// step 5a1: Host resolved their attack, blocks till guest responds
				select {
				case op_card = <-room.host_ch:
				case <-room.quit:
					{
						w.WriteHeader(http.StatusGone)
						return
					}
				case <-timeout:
					w.WriteHeader(http.StatusGatewayTimeout)
					return
				}
				// endof step 5a1

				// step 5a2: Host notifies guest that it recieved messege
				select {
				case room.guest_ch <- selected_card:
				case <-room.quit:
					{
						w.WriteHeader(http.StatusGone)
						return
					}
				case <-timeout:
					w.WriteHeader(http.StatusGatewayTimeout)
					return
				}
				// endof step 5a2

			} else {
				// step 5b1: Guest resolved their attack, notify host of that
				select {
				case room.host_ch <- selected_card:
				case <-room.quit:
					{
						w.WriteHeader(http.StatusGone)
						return
					}
				case <-timeout:
					w.WriteHeader(http.StatusGatewayTimeout)
					return
				}
				// endof step 5b1

				// step 5b2: Recieve the acknowledgement from host
				select {
				case op_card = <-room.guest_ch:
				case <-room.quit:
					{
						w.WriteHeader(http.StatusGone)
						return
					}
				case <-timeout:
					w.WriteHeader(http.StatusGatewayTimeout)
					return
				}
				// endof step 5b2
			}
			// endof step 5

			empty_card := Card{}
			if op_card == empty_card {
				panic("somehow opponent sent an empty card")
			}

			// step 6: package the new gamestate and send it

			result := Result{}
			result.Action_result = ActionResult{failed, blocked, defended}
			result.New_gamestate = format_gamestate(player.gamestate)
			result.Op_gamestate = OpGameRepr{
				Deck_remaining: len(opponent.gamestate.deck),
				Hp:             opponent.gamestate.hp,
				Weapon:         card_to_repr(opponent.gamestate.weapon),
				Played_card:    card_to_repr(op_card),
			}
			// endof step 6

			// step 7: send the result
			json.NewEncoder(w).Encode(result)
			// endof step 7
		})
	}

	/*
		Room related endpoints
	*/
	{
		// create a new room with self as the host, returns room id as response on OK
		http.HandleFunc("POST /room/{host}", func(w http.ResponseWriter, r *http.Request) {
			host := r.PathValue("host")

			defer log.Printf("(/%v %v) room/%v", r.Method, r.Host, host)

			if AUTH { // verify jwt token
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

			new_room := new(Room{
				Player{},
				Player{},
				make(chan struct{}),
				sync.Once{},
				make(chan Card),
				make(chan Card),
			})
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

			var room *Room
			var exists bool
			store.Room_mutex.RLock()
			{
				room, exists = store.Rooms[id]
			}
			store.Room_mutex.RUnlock()

			// check if the room was actually there
			if !exists {
				w.WriteHeader(http.StatusNotFound)
				return
			}

			if AUTH { // verify jwt token
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
			Room_json, _ := json.Marshal(store.Rooms[id])
			store.Room_mutex.RUnlock()

			w.Write(Room_json)
		})
		// get list of all rooms as a list [{host: <s>, guest:<s>}, ...] as json
		http.HandleFunc("GET /room", func(w http.ResponseWriter, r *http.Request) {
			defer log.Printf("(/%v %v) /room", r.Method, r.Host)

			if AUTH { // verify jwt token
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

			if AUTH { // verify jwt token
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

		http.HandleFunc("DELETE /room/{guest}/{id}", func(w http.ResponseWriter, r *http.Request) {
			id_string := r.PathValue("id")
			id, err := strconv.Atoi(id_string)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprint(w, "Error parsing room id:", err)
				log.Print("Error parsing room id:", err)
				return
			}
			guest := r.PathValue("guest")
			defer log.Printf("(/%v %v) /room/%v/%v", r.Method, r.Host, guest, id)

			var old_room *Room
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

			if old_room.Guest.Name != guest {
				w.WriteHeader(http.StatusForbidden)
				return
			}

			if AUTH { // verify jwt token and extract info

				jwt_claims := ExtractClaims(r)
				if jwt_claims == nil {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}

				if VerifyAdmin(jwt_claims) {
					goto delete_guest
				}

				// only the host and the guest themselves has the authority to remove the guest from the room
				if !Verify(jwt_claims, "GUEST", old_room.Guest.Name, id_string) && !Verify(jwt_claims, "HOST", old_room.Host.Name, id_string) {
					w.WriteHeader(http.StatusForbidden)
					return
				}

			}

		delete_guest:
			old_room.Guest.Name = ""

			store.Room_mutex.Lock()
			store.Rooms[id] = old_room
			store.Room_mutex.Unlock()

			w.WriteHeader(http.StatusOK)

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

			var old_room *Room
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
			if AUTH { // verify jwt token and extract info

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
	}

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("(/%v %v) Health", r.Method, r.Host)
		w.WriteHeader(http.StatusOK)
	})

	http.ListenAndServe(":8080", nil)
}
