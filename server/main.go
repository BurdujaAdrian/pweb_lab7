package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
)

func todo(msg string) {
	log.Print("TODO: ", msg)
}

type Player struct {
	Name string
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

	store.Rooms = make(map[int]Room)
	// create a new room with self as the host, returns room id as response on OK
	http.HandleFunc("POST /room/{host}", func(w http.ResponseWriter, r *http.Request) {

		host := r.PathValue("host")

		log.Printf("(/%v %v) room/%v", r.Method, r.Host, host)

		if host == "" {
			w.WriteHeader(http.StatusBadRequest)

			fmt.Fprint(w, "Empty username not allowed when craeting new room")
			return
		}

		new_room := Room{
			Player{host},
			Player{""},
		}
		var new_room_id int

		store.Mutex.Lock()
		{
			store.Rooms[store.Counter] = new_room
			new_room_id = store.Counter
			store.Counter += 1
		}
		store.Mutex.Unlock()

		fmt.Fprintf(w, `{"room_id": %v}`, new_room_id)
	})

	http.HandleFunc("GET /room", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("(/%v %v) /room", r.Method, r.Host)
		store.Mutex.RLock()
		Rooms_json, _ := json.Marshal(store.Rooms)
		store.Mutex.RUnlock()

		w.Write(Rooms_json)
	})

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

		store.Mutex.Lock()
		defer store.Mutex.Unlock()

		target_room, exists := store.Rooms[id]
		if !exists {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		target_room.Guest = Player{guest}
		store.Rooms[id] = target_room

		target_room_json, _ := json.Marshal(target_room)

		w.Write(target_room_json)

		log.Printf("(/%v %v) /room/%v/%v", r.Method, r.Host, guest, id)
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

		store.Mutex.Lock()
		defer store.Mutex.Unlock()

		old_room, exists := store.Rooms[id]
		if !exists {
			w.WriteHeader(http.StatusNoContent)
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
