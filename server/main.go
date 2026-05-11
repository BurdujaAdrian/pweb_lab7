package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"

	"github.com/golang-jwt/jwt/v5"
)

func todo(msg string) {
	log.Print("TODO: ", msg)
}

type Player struct {
	Mutex     *sync.Mutex `json:"-"`
	Name      string
	gamestate *Gamestate
}

type Room struct {
	Host  Player
	Guest Player
}

var store struct {
	Mutex   sync.RWMutex
	Rooms   map[int]Room
	Counter int
}

func main() {
	// serve the frontend
	http.Handle("/", http.FileServer(http.Dir("../client")))

	// handle actions/commands from players
	http.HandleFunc("POST /action/{room_id}/{role}", func(w http.ResponseWriter, r *http.Request) {
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
		store.Mutex.Lock()
		{
			room = store.Rooms[room_id]
		}
		store.Mutex.Unlock()

		var player Player
		var opponent Player
		switch role {
		case "host":
			player = room.Host
			opponent = room.Guest
		case "guest":
			player = room.Guest
			opponent = room.Host
		default:
			{
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprintf(w, "Role [%v] doesn't exist, use [host] or [guest]", role)
				return
			}
		}

		var selected_card Card
		var is_attack bool
		var failed bool

		player.Mutex.Lock()
		{
			selected_card, is_attack, failed = Execute_action(player.gamestate, action)
		}
		player.Mutex.Unlock()

		var blocked bool
		var defended bool
		if is_attack {
			opponent.Mutex.Lock()
			{
				blocked, defended = attack(opponent.gamestate, selected_card)
			}
			opponent.Mutex.Unlock()
		}

		type Action_result struct {
			Failed   bool `json:"failed"`
			Blocked  bool `json:"blocked"`
			Defended bool `json:"defended"`
		}

		action_result := Action_result{failed, blocked, defended}

		_ = json.NewEncoder(w).Encode(action_result)

	})

	store.Rooms = make(map[int]Room)

	http.HandleFunc("GET /token", Token)

	// create a new room with self as the host, returns room id as response on OK
	http.HandleFunc("POST /room/{host}", requireAuth("PLAYER", func(w http.ResponseWriter, r *http.Request) {

		host := r.PathValue("host")

		log.Printf("(/%v %v) room/%v", r.Method, r.Host, host)

		if host == "" {
			w.WriteHeader(http.StatusBadRequest)

			fmt.Fprint(w, "Empty username not allowed when craeting new room")
			return
		}

		new_room := Room{
			Player{},
			Player{},
		}
		new_room.Host.Name = host
		new_room.Host.gamestate = new(Gamestate)
		new_room.Host.Mutex = new(sync.Mutex)

		var new_room_id int

		store.Mutex.Lock()
		{
			store.Rooms[store.Counter] = new_room
			new_room_id = store.Counter
			store.Counter += 1
		}
		store.Mutex.Unlock()

		fmt.Fprintf(w, `{"room_id": %v}`, new_room_id)
	}))

	http.HandleFunc("GET /room", requireAuth("VISITOR", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("(/%v %v) /room", r.Method, r.Host)
		store.Mutex.RLock()
		Rooms_json, _ := json.Marshal(store.Rooms)
		store.Mutex.RUnlock()

		w.Write(Rooms_json)
	}))

	http.HandleFunc("PATCH /room/{guest}/{id}", requireAuth("PLAYER", func(w http.ResponseWriter, r *http.Request) {
		guest := r.PathValue("guest")
		id_string := r.PathValue("id")
		id, err := strconv.Atoi(id_string)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, "Error parsing room id:", err)
			log.Print("Error parsing room id:", err)
			return
		}

		store.Mutex.Lock()
		defer store.Mutex.Unlock()

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

		log.Printf("(/%v %v) /room/%v/%v", r.Method, r.Host, guest, id)
	}))

	http.HandleFunc("DELETE /room/{id}", func(w http.ResponseWriter, r *http.Request) {
		id_string := r.PathValue("id")
		id, err := strconv.Atoi(id_string)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, "Error parsing room id:", err)
			log.Print("Error parsing room id:", err)
			return
		}

		token, err := jwt.Parse(r.Header.Get("Authorization"), func(t *jwt.Token) (interface{}, error) {
			return secret, nil
		})
		if err != nil || !token.Valid {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		claims := token.Claims.(jwt.MapClaims)
		role := claims["role"].(string)
		name := claims["name"].(string)

		store.Mutex.Lock()
		defer store.Mutex.Unlock()

		old_room, exists := store.Rooms[id]
		if !exists {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		if role == "PLAYER" && old_room.Host.Name != name {
			w.WriteHeader(http.StatusForbidden)
			return
		}

		delete(store.Rooms, id)

		old_room_json, _ := json.Marshal(old_room)
		w.Write(old_room_json)

		log.Printf("(/%v %v) /room/%v", r.Method, r.Host, id)
	})

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("(/%v %v) Health", r.Method, r.Host)
		w.WriteHeader(http.StatusOK)
	})

	http.ListenAndServe(":8080", nil)
}
