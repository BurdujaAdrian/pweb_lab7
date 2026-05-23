const API = 'https://pweb-lab7.onrender.com';

document.addEventListener('alpine:init', () => {
Alpine.store('lobby', {
rooms: {},
username: '',
token: '',
room_id: null,

async init() {
    const res = await fetch(`${API}/token`, {
	method: 'POST',
	headers: { 'Content-Type': 'application/json' },
	body: JSON.stringify({ role: 'VISITOR', name: '' })
    });
    const data = await res.json();

    this.token = data.token;
    this.fetchRooms();
    setInterval(() => this.fetchRooms(), 3000);
},

async fetchRooms() {
    try {
        const res = await fetch(`${API}/room`, {
            headers: { Authorization: this.token }
        });
        this.rooms = await res.json();
    } catch {
        this.rooms = {};
    }
},

async createRoom() {
    if (!this.username) return;

    const tokenRes = await fetch(`${API}/token`, {
	method: 'POST',
	headers: { 'Content-Type': 'application/json' },
	body: JSON.stringify({ role: 'PLAYER', name: this.username })
    });
    const tokenData = await tokenRes.json();
    this.token = tokenData.token;

    const res = await fetch(`${API}/room/${this.username}`, { 
        method: 'POST', 
        headers: { Authorization: this.token }
    });
    const data = await res.json();
    this.room_id = data.room_id;

    window.location.href = `game.html`;
},

async joinRoom(id, hostName) {
    if (!this.username) return;
    if (this.username === hostName) return;

    const tokenRes = await fetch(`${API}/token`, {
	method: 'POST',
	headers: { 'Content-Type': 'application/json' },
	body: JSON.stringify({ role: 'PLAYER', name: this.username })
    });
    const tokenData = await tokenRes.json();
    this.token = tokenData.token;

    const response = await fetch(`${API}/room/${this.username}/${id}`, { 
	method: 'PATCH' ,
	body: JSON.stringify({ role: 'PLAYER', name: this.username })
    });
    console.log(response)
    window.location.href = `game.html`;
},

async leaveRoom() {
    try {
        const res = await fetch(`${API}/room/${this.room_id}`, {
            method: 'DELETE',
            headers: { Authorization: this.token }
        });
        console.log('response', res.status);
    } catch(e) {
        console.log('fetch error', e);
    }
    window.location.href = '../index.html';
},

isFull(room) {
    return room.Guest && room.Guest.Name !== '';
},

entries() {
    return Object.entries(this.rooms).map(([id, room]) => ({ id, room }));
}
});

Alpine.store('lobby').init();
});
